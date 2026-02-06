package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	httpapi "github.com/shestoi/GoBigTech/services/notification/internal/api/http"
	grpcclient "github.com/shestoi/GoBigTech/services/notification/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/notification/internal/config"
	eventkafka "github.com/shestoi/GoBigTech/services/notification/internal/event/kafka"
	"github.com/shestoi/GoBigTech/services/notification/internal/repository/postgres"
	"github.com/shestoi/GoBigTech/services/notification/internal/service"
	"github.com/shestoi/GoBigTech/services/notification/internal/telegram"
	"github.com/shestoi/GoBigTech/services/notification/internal/templates"
)

// App содержит все зависимости для запуска и корректного shutdown Notification Service
type App struct {
	logger           *zap.Logger
	alertServer      *http.Server
	paymentConsumer  *eventkafka.OrderPaidConsumer
	assemblyConsumer *eventkafka.OrderAssemblyCompletedConsumer
	shutdownMgr      *platformshutdown.Manager
	wg               sync.WaitGroup
}

// Build создаёт и настраивает все зависимости Notification Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "notification",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("op", op))
	logger.Info("Building Notification service",
		zap.Strings("kafka_brokers", cfg.KafkaBrokers),
		zap.String("payment_topic", cfg.PaymentCompletedTopic),
		zap.String("assembly_topic", cfg.AssemblyCompletedTopic),
		zap.Int("retry_max_attempts", cfg.NotificationKafkaRetryMaxAttempts),
		zap.Duration("retry_backoff_base", cfg.NotificationKafkaRetryBackoffBase),
	)

	// Подключаемся к PostgreSQL
	logger.Info("Connecting to PostgreSQL")
	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}

	// Проверяем подключение к PostgreSQL
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
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
	readiness()
	logger.Info("Readiness check enabled")

	// Создаём PostgreSQL репозиторий
	notificationRepo := postgres.NewRepository(pool)

	// Создаём Telegram sender
	var telegramSender telegram.Sender
	if cfg.TelegramEnabled {
		telegramSender = telegram.NewTelegramSender(logger, cfg.TelegramBotToken)
		logger.Info("Telegram sender enabled",
			zap.String("chat_id", cfg.TelegramChatID),
		)
	} else {
		telegramSender = telegram.NewNoOpSender(logger)
		logger.Warn("Telegram disabled, using no-op sender")
	}

	// Создаём template renderer
	renderer, err := templates.NewRenderer(logger, cfg.TemplatesDir)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create template renderer: %w", err)
	}

	// Подключаемся к IAM Service для получения контактной информации пользователей
	logger.Info("Connecting to IAM service", zap.String("addr", cfg.IAMGRPCAddr))
	iamClient, iamConn, err := grpcclient.NewIAMGRPCClient(cfg.IAMGRPCAddr, logger)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to IAM service: %w", err)
	}

	// Создаём адаптер для IAM клиента
	iamClientAdapter := grpcclient.NewIAMClientAdapter(iamClient, logger)

	// Создаём service слой
	notificationService := service.NewNotificationService(
		logger,
		notificationRepo,
		telegramSender,
		renderer,
		iamClientAdapter,
	)

	// Создаём DLQ publisher
	dlqPublisher := eventkafka.NewDLQPublisher(
		logger,
		cfg.KafkaBrokers,
		cfg.DLQTopic,
	)

	// Создаём Kafka consumers
	paymentConsumer := eventkafka.NewOrderPaidConsumer(
		logger,
		cfg.KafkaBrokers,
		cfg.NotificationPaymentGroupID,
		cfg.PaymentCompletedTopic,
		notificationService,
		dlqPublisher,
		cfg.NotificationKafkaRetryMaxAttempts,
		cfg.NotificationKafkaRetryBackoffBase,
	)

	assemblyConsumer := eventkafka.NewOrderAssemblyCompletedConsumer(
		logger,
		cfg.KafkaBrokers,
		cfg.NotificationAssemblyGroupID,
		cfg.AssemblyCompletedTopic,
		notificationService,
		dlqPublisher,
		cfg.NotificationKafkaRetryMaxAttempts,
		cfg.NotificationKafkaRetryBackoffBase,
	)

	// HTTP сервер для приёма webhook от Alertmanager (алерты в Telegram)
	var alertServer *http.Server
	alertListenAddr := cfg.AlertsHTTPAddr
	if alertListenAddr == "" && cfg.HTTPAlertPort != "" {
		alertListenAddr = ":" + cfg.HTTPAlertPort
	}
	if alertListenAddr != "" {
		alertChatID := cfg.AlertTelegramChatID
		if cfg.TelegramDisable {
			alertChatID = ""
		}
		alertHandler := httpapi.NewAlertmanagerHandler(logger, telegramSender, alertChatID)
		alertRouter := httpapi.NewAlertRouter(alertHandler)
		alertServer = &http.Server{
			Addr:         alertListenAddr,
			Handler:      alertRouter,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 15 * time.Second,
		}
		logger.Info("Alertmanager webhook server configured", zap.String("addr", alertServer.Addr), zap.String("path", "/alerts"))
	}

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	if alertServer != nil {
		shutdownMgr.Add("alert_http_server", platformshutdown.ShutdownHTTPServer(alertServer))
	}
	shutdownMgr.Add("kafka_assembly_consumer", func(ctx context.Context) error {
		return assemblyConsumer.Close()
	})
	shutdownMgr.Add("kafka_payment_consumer", func(ctx context.Context) error {
		return paymentConsumer.Close()
	})
	shutdownMgr.Add("dlq_publisher", func(ctx context.Context) error {
		return dlqPublisher.Close()
	})
	shutdownMgr.Add("iam_conn", func(ctx context.Context) error {
		iamConn.Close()
		return nil
	})
	shutdownMgr.Add("postgres_pool", platformshutdown.ClosePool(pool))

	return &App{
		logger:           logger,
		alertServer:      alertServer,
		paymentConsumer:  paymentConsumer,
		assemblyConsumer: assemblyConsumer,
		shutdownMgr:      shutdownMgr,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting Notification service")

	// Создаём контексты для consumers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем HTTP сервер для алертов (webhook)
	if a.alertServer != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			if err := a.alertServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Error("alert HTTP server error", zap.Error(err))
			}
		}()
		a.logger.Info("Alert webhook server listening", zap.String("addr", a.alertServer.Addr))
	}

	// Запускаем payment consumer в отдельной горутине
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.paymentConsumer.Start(ctx); err != nil {
			a.logger.Error("kafka payment consumer error", zap.Error(err))
		}
	}()

	// Запускаем assembly consumer в отдельной горутине
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.assemblyConsumer.Start(ctx); err != nil {
			a.logger.Error("kafka assembly consumer error", zap.Error(err))
		}
	}()

	a.logger.Info("Kafka consumers started")

	// Ожидаем сигнал и выполняем shutdown
	a.shutdownMgr.Wait()

	// Отменяем контекст consumers
	cancel()

	// Ждём завершения всех горутин
	a.wg.Wait()

	a.logger.Info("Kafka consumers stopped")
	a.logger.Info("Notification service stopped")
	return nil
}
