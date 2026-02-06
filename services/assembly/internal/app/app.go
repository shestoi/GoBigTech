package app

import (
	"context"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformobservability "github.com/shestoi/GoBigTech/platform/observability"
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

	// OpenTelemetry: traces + metrics (noop если OTEL_ENABLED=false)
	otelCfg := platformobservability.Config{
		Enabled:               cfg.OTelEnabled,
		OTLPEndpoint:          cfg.OTelEndpoint,
		SamplingRatio:         cfg.OTelSamplingRatio,
		ServiceName:           "assembly",
		DeploymentEnvironment: string(cfg.AppEnv),
	}
	otelShutdown, err := platformobservability.Init(context.Background(), otelCfg)
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

	// Метрики сборки (assembly_duration_ms); при отключённом OTEL — noop
	var assemblyMetrics service.AssemblyMetricsRecorder
	if cfg.OTelEnabled {
		assemblyMetrics = newAssemblyMetricsRecorder()
	}

	// Создаём service слой
	assemblyService := service.NewService(logger, publisher, idempotencyStore, idempotencyTTL, assemblyMetrics)

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

	// Регистрируем shutdown: otel последним, чтобы успели записаться spans/metrics
	shutdownMgr.Add("otel", otelShutdown)
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

// assemblyMetricsRecorder записывает assembly_duration_ms в OTLP histogram.
type assemblyMetricsRecorder struct {
	histogram metric.Float64Histogram
}

func newAssemblyMetricsRecorder() *assemblyMetricsRecorder {
	meter := otel.Meter("assembly")
	hist, _ := meter.Float64Histogram("assembly_duration_ms", metric.WithDescription("Assembly duration in milliseconds"))
	return &assemblyMetricsRecorder{histogram: hist}
}

func (r *assemblyMetricsRecorder) RecordAssemblyDuration(d time.Duration, result string) {
	if r.histogram == nil {
		return
	}
	r.histogram.Record(context.Background(), float64(d.Milliseconds()), metric.WithAttributes(attribute.String("result", result)))
}
