package app

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	httpapi "github.com/shestoi/GoBigTech/services/order/internal/api/http"
	grpcclient "github.com/shestoi/GoBigTech/services/order/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/order/internal/config"
	eventkafka "github.com/shestoi/GoBigTech/services/order/internal/event/kafka"
	"github.com/shestoi/GoBigTech/services/order/internal/repository/postgres"
	"github.com/shestoi/GoBigTech/services/order/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
)

// App содержит все зависимости для запуска и корректного shutdown Order Service
type App struct {
	logger           *zap.Logger
	httpServer       *http.Server
	assemblyConsumer *eventkafka.OrderAssemblyCompletedConsumer
	outboxDispatcher *eventkafka.OutboxDispatcher
	shutdownMgr      *platformshutdown.Manager
	readiness        func() bool
	wg               sync.WaitGroup
}

// Build создаёт и настраивает все зависимости Order Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "order",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("op", op))
	logger.Info("Building Order service", zap.String("http_addr", cfg.HTTPAddr))

	// Подключаемся к Inventory сервису
	logger.Info("Connecting to Inventory service", zap.String("addr", cfg.InventoryGRPCAddr))
	inventoryConn, err := grpc.NewClient(cfg.InventoryGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	inventoryClient := inventorypb.NewInventoryServiceClient(inventoryConn)

	// Подключаемся к Payment сервису
	logger.Info("Connecting to Payment service", zap.String("addr", cfg.PaymentGRPCAddr))
	paymentConn, err := grpc.NewClient(cfg.PaymentGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		inventoryConn.Close()
		return nil, err
	}

	paymentClient := paymentpb.NewPaymentServiceClient(paymentConn)

	// Обёртываем gRPC клиенты в адаптеры
	inventoryClientAdapter := grpcclient.NewInventoryClientAdapter(inventoryClient)
	paymentClientAdapter := grpcclient.NewPaymentClientAdapter(paymentClient)

	// Подключаемся к PostgreSQL
	logger.Info("Connecting to PostgreSQL")
	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		inventoryConn.Close()
		paymentConn.Close()
		return nil, err
	}

	// Проверяем подключение к PostgreSQL
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		inventoryConn.Close()
		paymentConn.Close()
		return nil, err
	}
	logger.Info("PostgreSQL connection established")

	// Функция readiness для health check
	readiness := func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return false
		}
		return true
	}

	// Устанавливаем readiness после успешного ping
	readiness() // Первая проверка
	logger.Info("Readiness check enabled")

	// Создаём PostgreSQL репозиторий
	orderRepo := postgres.NewRepository(pool)

	// Создаем service слой с зависимостями (без publisher, используем outbox)
	orderService := service.NewOrderService(logger, inventoryClientAdapter, paymentClientAdapter, orderRepo, cfg.PaymentCompletedTopic)

	// Создаём outbox dispatcher для публикации событий из outbox таблицы
	var outboxDispatcher *eventkafka.OutboxDispatcher
	if len(cfg.Brokers) > 0 && cfg.PaymentCompletedTopic != "" {
		logger.Info("Initializing outbox dispatcher",
			zap.Strings("brokers", cfg.Brokers),
			zap.String("topic", cfg.PaymentCompletedTopic),
		)
		outboxDispatcher = eventkafka.NewOutboxDispatcher(
			logger,
			orderRepo,
			cfg.Brokers,
			10,            // batch size
			2*time.Second, // interval
			3,             // max retries
			1*time.Second, // backoff
		)
	} else {
		logger.Warn("Kafka brokers or topic not configured, outbox dispatcher will not be started")
	}

	// Создаём Kafka consumer для событий завершения сборки заказа
	var assemblyConsumer *eventkafka.OrderAssemblyCompletedConsumer
	if len(cfg.Brokers) > 0 && cfg.AssemblyCompletedTopic != "" {
		logger.Info("Initializing Kafka assembly completed consumer",
			zap.Strings("brokers", cfg.Brokers),
			zap.String("topic", cfg.AssemblyCompletedTopic),
			zap.String("group_id", cfg.OrderConsumerGroupID),
		)
		assemblyConsumer = eventkafka.NewOrderAssemblyCompletedConsumer(
			logger,
			cfg.Brokers,
			cfg.OrderConsumerGroupID,
			cfg.AssemblyCompletedTopic,
			orderService,
			cfg.AssemblyConsumerRetryMaxAttempts,
			cfg.AssemblyConsumerRetryBackoffBase,
		)
	} else {
		logger.Warn("Kafka brokers or assembly topic not configured, assembly events will not be consumed")
	}

	// Создаем HTTP handler
	handler := httpapi.NewHandler(orderService, logger)

	// Настраиваем роутер
	router := httpapi.NewRouter(handler, readiness)

	// Создаём HTTP сервер
	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	if assemblyConsumer != nil {
		shutdownMgr.Add("kafka_assembly_consumer", func(ctx context.Context) error {
			return assemblyConsumer.Close()
		})
	}
	if outboxDispatcher != nil {
		shutdownMgr.Add("outbox_dispatcher", func(ctx context.Context) error {
			return outboxDispatcher.Close()
		})
	}
	shutdownMgr.Add("postgres_pool", platformshutdown.ClosePool(pool))
	shutdownMgr.Add("http_server", platformshutdown.ShutdownHTTPServer(httpServer))

	// Закрываем gRPC соединения при shutdown
	shutdownMgr.Add("inventory_conn", func(ctx context.Context) error {
		inventoryConn.Close()
		return nil
	})
	shutdownMgr.Add("payment_conn", func(ctx context.Context) error {
		paymentConn.Close()
		return nil
	})

	return &App{
		logger:           logger,
		httpServer:       httpServer,
		assemblyConsumer: assemblyConsumer,
		outboxDispatcher: outboxDispatcher,
		shutdownMgr:      shutdownMgr,
		readiness:        readiness,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting Order service", zap.String("addr", a.httpServer.Addr))
	a.logger.Info("Health check available", zap.String("url", "http://"+a.httpServer.Addr+"/health"))

	// Создаём контекст для consumer (если настроен)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Запускаем Kafka consumer в отдельной горутине (если настроен)
	if a.assemblyConsumer != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			if err := a.assemblyConsumer.Start(consumerCtx); err != nil {
				a.logger.Error("kafka consumer error", zap.Error(err))
			}
		}()

		a.logger.Info("Kafka assembly consumer started")
	}

	// Запускаем outbox dispatcher в отдельной горутине (если настроен)
	if a.outboxDispatcher != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			if err := a.outboxDispatcher.Start(consumerCtx); err != nil {
				a.logger.Error("outbox dispatcher error", zap.Error(err))
			}
		}()

		a.logger.Info("Outbox dispatcher started")
	}

	// Ожидаем сигнал и выполняем shutdown
	a.shutdownMgr.Wait()

	// Отменяем контекст для остановки consumers/dispatcher
	consumerCancel()

	// Ждём завершения всех горутин (consumers/dispatcher должны завершиться по ctx.Done())
	a.wg.Wait()

	a.logger.Info("Order service stopped")
	return nil
}
