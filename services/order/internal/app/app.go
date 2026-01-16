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
	"github.com/shestoi/GoBigTech/services/order/internal/repository/postgres"
	"github.com/shestoi/GoBigTech/services/order/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
)

// App содержит все зависимости для запуска и корректного shutdown Order Service
type App struct {
	logger      *zap.Logger
	httpServer  *http.Server
	shutdownMgr *platformshutdown.Manager
	readiness   func() bool
	wg          sync.WaitGroup
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

	// Создаем service слой с зависимостями
	orderService := service.NewOrderService(inventoryClientAdapter, paymentClientAdapter, orderRepo)

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
		logger:      logger,
		httpServer:  httpServer,
		shutdownMgr: shutdownMgr,
		readiness:   readiness,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting Order service", zap.String("addr", a.httpServer.Addr))
	a.logger.Info("Health check available", zap.String("url", "http://"+a.httpServer.Addr+"/health"))

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Ожидаем сигнал и выполняем shutdown
	a.shutdownMgr.Wait()

	a.wg.Wait()
	a.logger.Info("Order service stopped")
	return nil
}
