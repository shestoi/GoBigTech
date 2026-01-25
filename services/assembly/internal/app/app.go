package app

import (
	"context"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	"github.com/shestoi/GoBigTech/services/assembly/internal/config"
	eventkafka "github.com/shestoi/GoBigTech/services/assembly/internal/event/kafka"
	"github.com/shestoi/GoBigTech/services/assembly/internal/service"
)

// App содержит все зависимости для запуска и корректного shutdown Assembly Service
type App struct {
	logger      *zap.Logger
	consumer    *eventkafka.OrderPaidConsumer
	shutdownMgr *platformshutdown.Manager
	wg          sync.WaitGroup
}

// Build создаёт и настраивает все зависимости Assembly Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "assembly",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	// Создаём store для idempotency (in-memory для dev/test, в production будет Postgres/Redis)
	idempotencyStore := service.NewMemoryProcessedEventsStore()
	const idempotencyTTL = 24 * time.Hour

	logger = logger.With(zap.String("op", op))
	logger.Info("Building Assembly service",
		zap.Strings("kafka_brokers", cfg.KafkaBrokers),
		zap.String("payment_topic", cfg.PaymentCompletedTopic),
		zap.String("assembly_topic", cfg.AssemblyCompletedTopic),
		zap.String("dlq_topic", cfg.DLQTopic),
		zap.Int("retry_max_attempts", cfg.RetryMaxAttempts),
		zap.Duration("retry_backoff_base", cfg.RetryBackoffBase),
		zap.Duration("idempotency_ttl", idempotencyTTL),
	)

	// Создаём Kafka publisher для событий сборки
	publisher := eventkafka.NewKafkaAssemblyEventPublisher(
		logger,
		cfg.KafkaBrokers,
		cfg.AssemblyCompletedTopic,
	)

	// Создаём DLQ publisher
	dlqPublisher := eventkafka.NewDLQPublisher(
		logger,
		cfg.KafkaBrokers,
		cfg.DLQTopic,
	)

	// Создаём service слой
	assemblyService := service.NewService(logger, publisher, idempotencyStore, idempotencyTTL)

	// Создаём Kafka consumer для событий оплаты
	consumer := eventkafka.NewOrderPaidConsumer(
		logger,
		cfg.KafkaBrokers,
		cfg.ConsumerGroupID,
		cfg.PaymentCompletedTopic,
		assemblyService,
		dlqPublisher,
		cfg.RetryMaxAttempts,
		cfg.RetryBackoffBase,
	)

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	shutdownMgr.Add("kafka_consumer", func(ctx context.Context) error {
		return consumer.Close()
	})
	shutdownMgr.Add("kafka_publisher", func(ctx context.Context) error {
		return publisher.Close()
	})
	shutdownMgr.Add("kafka_dlq_publisher", func(ctx context.Context) error {
		return dlqPublisher.Close()
	})

	return &App{
		logger:      logger,
		consumer:    consumer,
		shutdownMgr: shutdownMgr,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting Assembly service")

	// Создаём контекст для consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем consumer в отдельной горутине
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.consumer.Start(ctx); err != nil {
			a.logger.Error("kafka consumer error", zap.Error(err))
		}
	}()

	// Ожидаем сигнал и выполняем shutdown
	a.shutdownMgr.Wait()

	// Отменяем контекст consumer
	cancel()

	// Ждём завершения consumer
	a.wg.Wait()

	a.logger.Info("Assembly service stopped")
	return nil
}
