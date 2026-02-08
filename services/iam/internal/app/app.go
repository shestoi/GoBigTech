package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	platformhealth "github.com/shestoi/GoBigTech/platform/health/grpc"
	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformobservability "github.com/shestoi/GoBigTech/platform/observability"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	grpcapi "github.com/shestoi/GoBigTech/services/iam/internal/api/grpc"
	httpapi "github.com/shestoi/GoBigTech/services/iam/internal/api/http"
	"github.com/shestoi/GoBigTech/services/iam/internal/config"
	"github.com/shestoi/GoBigTech/services/iam/internal/repository/postgres"
	redisrepo "github.com/shestoi/GoBigTech/services/iam/internal/repository/redis"
	"github.com/shestoi/GoBigTech/services/iam/internal/service"
	iampb "github.com/shestoi/GoBigTech/services/iam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// App содержит все зависимости для запуска и корректного shutdown IAM Service
type App struct {
	logger      *zap.Logger
	grpcServer  *grpc.Server
	httpServer  *http.Server
	listener    net.Listener
	health      *platformhealth.Health
	shutdownMgr *platformshutdown.Manager
	wg          sync.WaitGroup
}

// Build создаёт и настраивает все зависимости IAM Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "iam",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("op", op))
	logger.Info("Building IAM service", zap.String("grpc_addr", cfg.GRPCAddr))

	// OpenTelemetry
	otelCfg := platformobservability.Config{
		Enabled:               cfg.OTelEnabled,
		OTLPEndpoint:          cfg.OTelEndpoint,
		SamplingRatio:         cfg.OTelSamplingRatio,
		ServiceName:           "iam",
		DeploymentEnvironment: string(cfg.AppEnv),
	}
	otelShutdown, err := platformobservability.Init(context.Background(), otelCfg)
	if err != nil {
		return nil, err
	}

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

	// Применяем миграции
	logger.Info("Applying database migrations")
	db, err := goose.OpenDBWithDriver("pgx", cfg.PostgresDSN)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer db.Close()

	// Путь к миграциям: получаем абсолютный путь относительно текущего файла
	// app.go находится в services/iam/internal/app/, миграции в services/iam/migrations/
	wd, err := os.Getwd()
	if err != nil {
		pool.Close()
		return nil, err
	}

	// internal/app -> internal -> iam
	//iamDir := filepath.Dir(filepath.Dir(wd))

	migrationsDir := filepath.Join(wd, "migrations")

	if err := goose.Up(db, migrationsDir); err != nil {
		pool.Close()
		return nil, err
	}
	logger.Info("Database migrations applied successfully")

	// Подключаемся к Redis
	logger.Info("Connecting to Redis", zap.String("addr", cfg.RedisAddr))
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	// Проверяем подключение к Redis
	ctxRedis, cancelRedis := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRedis()
	if err := redisClient.Ping(ctxRedis).Err(); err != nil {
		pool.Close()
		return nil, err
	}
	logger.Info("Redis connection established")

	// Создаём PostgreSQL репозиторий
	userRepo := postgres.NewRepository(pool)

	// Создаём Redis session repository
	sessionRepo := redisrepo.NewSessionRepository(redisClient, logger)

	// Создаём service слой
	iamService := service.NewService(logger, userRepo, sessionRepo, cfg.SessionTTL)

	// Создаём gRPC handler
	grpcHandler := grpcapi.NewHandler(iamService, logger)

	// Слушаем на указанном адресе
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		pool.Close()
		redisClient.Close()
		return nil, err
	}

	// gRPC сервер с tracing interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(platformobservability.GRPCUnaryServerInterceptor("iam")),
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
	iampb.RegisterIAMServiceServer(grpcServer, grpcHandler)

	logger.Info("IAM gRPC server configured", zap.String("addr", cfg.GRPCAddr))

	// Внутренний HTTP-сервер для Envoy: POST /internal/validate (проверка сессии по x-session-id)
	validateHandler := httpapi.NewValidateHandler(iamService, logger)
	httpMux := http.NewServeMux()
	httpMux.Handle("POST /internal/validate", validateHandler)
	httpServer := &http.Server{
		Addr:              cfg.HTTPInternalAddr,
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("IAM HTTP internal server configured", zap.String("addr", cfg.HTTPInternalAddr))

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	shutdownMgr.Add("otel", otelShutdown)
	shutdownMgr.Add("http_server", platformshutdown.ShutdownHTTPServer(httpServer))
	shutdownMgr.Add("grpc_server", platformshutdown.ShutdownGRPCServer(grpcServer))
	shutdownMgr.Add("health_readiness", platformshutdown.SetHealthNotServing(health))
	shutdownMgr.Add("redis_client", func(ctx context.Context) error {
		return redisClient.Close()
	})
	shutdownMgr.Add("postgres_pool", platformshutdown.ClosePool(pool))

	return &App{
		logger:      logger,
		grpcServer:  grpcServer,
		httpServer:  httpServer,
		listener:    listener,
		health:      health,
		shutdownMgr: shutdownMgr,
	}, nil
}

// Run запускает сервис и блокируется до получения сигнала shutdown
func (a *App) Run() error {
	defer platformlogging.Sync(a.logger)

	a.logger.Info("Starting IAM service", zap.String("addr", a.listener.Addr().String()))

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.grpcServer.Serve(a.listener); err != nil && err != grpc.ErrServerStopped {
			a.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

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
	a.logger.Info("IAM service stopped")
	return nil
}
