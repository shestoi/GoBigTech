package app

import (
	"context"
	"net"
	"os"
	"sync"

	"go.uber.org/zap"

	platformhealth "github.com/shestoi/GoBigTech/platform/health/grpc"
	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformobservability "github.com/shestoi/GoBigTech/platform/observability"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	grpcapi "github.com/shestoi/GoBigTech/services/payment/internal/api/grpc"
	"github.com/shestoi/GoBigTech/services/payment/internal/config"
	"github.com/shestoi/GoBigTech/services/payment/internal/repository/memory"
	"github.com/shestoi/GoBigTech/services/payment/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// App содержит все зависимости для запуска и корректного shutdown Payment Service
type App struct {
	logger      *zap.Logger
	grpcServer  *grpc.Server
	listener    net.Listener
	health      *platformhealth.Health
	shutdownMgr *platformshutdown.Manager
	wg          sync.WaitGroup
}

// Build создаёт и настраивает все зависимости Payment Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "payment",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("op", op))
	logger.Info("Building Payment service", zap.String("grpc_addr", cfg.GRPCAddr))

	// OpenTelemetry
	otelCfg := platformobservability.Config{
		Enabled:               cfg.OTelEnabled,
		OTLPEndpoint:          cfg.OTelEndpoint,
		SamplingRatio:         cfg.OTelSamplingRatio,
		ServiceName:           "payment",
		DeploymentEnvironment: string(cfg.AppEnv),
	}
	otelShutdown, err := platformobservability.Init(context.Background(), otelCfg)
	if err != nil {
		return nil, err
	}

	// Создаём in-memory репозиторий
	paymentRepo := memory.NewMemoryRepository()

	// Создаём service слой
	paymentService := service.NewPaymentService(paymentRepo)

	// Создаём gRPC handler
	grpcHandler := grpcapi.NewHandler(paymentService)

	// Слушаем на указанном адресе
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return nil, err
	}

	// gRPC сервер с tracing interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(platformobservability.GRPCUnaryServerInterceptor("payment")),
	)

	// Включаем reflection, если указано в конфиге
	if cfg.EnableGRPCReflection {
		reflection.Register(grpcServer)
		logger.Info("gRPC reflection enabled")
	}

	// Создаём health check с начальным статусом SERVING
	health := platformhealth.New(grpc_health_v1.HealthCheckResponse_SERVING)
	health.Register(grpcServer)
	logger.Info("Health check initialized with SERVING status")

	// Регистрируем gRPC handler
	paymentpb.RegisterPaymentServiceServer(grpcServer, grpcHandler)

	logger.Info("Payment gRPC server configured", zap.String("addr", cfg.GRPCAddr))

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	shutdownMgr.Add("otel", otelShutdown)
	shutdownMgr.Add("grpc_server", platformshutdown.ShutdownGRPCServer(grpcServer))
	shutdownMgr.Add("health_readiness", platformshutdown.SetHealthNotServing(health))

	return &App{
		logger:      logger,
		grpcServer:  grpcServer,
		listener:    listener,
		health:      health,
		shutdownMgr: shutdownMgr,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting Payment service", zap.String("addr", a.listener.Addr().String()))

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.grpcServer.Serve(a.listener); err != nil {
			a.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// Ожидаем сигнал и выполняем shutdown
	a.shutdownMgr.Wait()

	a.wg.Wait()
	a.logger.Info("Payment service stopped")
	return nil
}
