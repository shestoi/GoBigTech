package app

import (
	"context"
	"net"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	platformhealth "github.com/shestoi/GoBigTech/platform/health/grpc"
	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
	platformshutdown "github.com/shestoi/GoBigTech/platform/shutdown"
	grpcapi "github.com/shestoi/GoBigTech/services/inventory/internal/api/grpc"
	iamclient "github.com/shestoi/GoBigTech/services/inventory/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/inventory/internal/config"
	"github.com/shestoi/GoBigTech/services/inventory/internal/interceptor"
	mongorepo "github.com/shestoi/GoBigTech/services/inventory/internal/repository/mongo"
	"github.com/shestoi/GoBigTech/services/inventory/internal/service"
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// App содержит все зависимости для запуска и корректного shutdown Inventory Service
type App struct {
	logger      *zap.Logger
	grpcServer  *grpc.Server
	listener    net.Listener
	health      *platformhealth.Health
	shutdownMgr *platformshutdown.Manager
	wg          sync.WaitGroup
}

// Build создаёт и настраивает все зависимости Inventory Service
func Build(cfg config.Config) (*App, error) {
	const op = "app.Build"

	// Создаём logger
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "inventory",
		Env:         string(cfg.AppEnv),
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
	})
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("op", op))
	logger.Info("Building Inventory service", zap.String("grpc_addr", cfg.GRPCAddr))

	// Создаём health check с начальным статусом NOT_SERVING
	health := platformhealth.New(grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Подключаемся к MongoDB
	logger.Info("Connecting to MongoDB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, err
	}

	// Проверяем подключение к MongoDB
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return nil, err
	}
	logger.Info("MongoDB connection established")

	// После успешного ping устанавливаем readiness в SERVING
	health.SetServing("")
	logger.Info("Readiness status set to SERVING")

	// Создаём MongoDB репозиторий
	inventoryRepo := mongorepo.NewRepository(client, cfg.MongoDBName)

	// Создаём service слой
	inventoryService := service.NewInventoryService(inventoryRepo)

	// Подключаемся к IAM Service для проверки сессий
	logger.Info("Connecting to IAM service", zap.String("addr", cfg.IAMGRPCAddr))
	iamClient, iamConn, err := iamclient.NewIAMGRPCClient(cfg.IAMGRPCAddr, logger)
	if err != nil {
		client.Disconnect(ctx)
		return nil, err
	}

	// Создаём адаптер для IAM клиента
	iamClientAdapter := iamclient.NewIAMClientAdapter(iamClient, logger)

	// Создаём auth interceptor
	authInterceptor := interceptor.NewAuthInterceptor(iamClientAdapter, logger)

	// Создаём gRPC handler
	grpcHandler := grpcapi.NewHandler(inventoryService)

	// Слушаем на указанном адресе
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		iamConn.Close()
		client.Disconnect(ctx)
		return nil, err
	}

	// Создаем gRPC сервер с auth interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.Unary()),
	)

	// Включаем reflection, если указано в конфиге
	if cfg.EnableGRPCReflection {
		reflection.Register(grpcServer)
		logger.Info("gRPC reflection enabled")
	}

	// Регистрируем gRPC health service
	health.Register(grpcServer)

	// Регистрируем gRPC handler
	inventorypb.RegisterInventoryServiceServer(grpcServer, grpcHandler) //без него “сервер есть, а методов нет”
	//“Эй gRPC сервер, вот сервис inventory.v1.InventoryService, и вот объект, на котором надо вызывать методы.”

	logger.Info("Inventory gRPC server configured", zap.String("addr", cfg.GRPCAddr))

	// Создаём shutdown manager
	shutdownMgr := platformshutdown.New(cfg.ShutdownTimeout, logger)

	// Регистрируем shutdown функции в обратном порядке выполнения
	shutdownMgr.Add("mongodb", platformshutdown.DisconnectMongo(client))
	shutdownMgr.Add("iam_conn", func(ctx context.Context) error {
		iamConn.Close()
		return nil
	})
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

	a.logger.Info("Starting Inventory service", zap.String("addr", a.listener.Addr().String()))

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
	a.logger.Info("Inventory service stopped")
	return nil
}
