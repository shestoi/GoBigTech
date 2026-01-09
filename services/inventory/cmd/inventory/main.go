package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	grpcapi "github.com/shestoi/GoBigTech/services/inventory/internal/api/grpc"
	mongorepo "github.com/shestoi/GoBigTech/services/inventory/internal/repository/mongo"
	"github.com/shestoi/GoBigTech/services/inventory/internal/service"
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting Inventory service...")

	// Подключаемся к MongoDB
	mongoURI := getMongoURI()
	dbName := getMongoDBName()
	log.Printf("Connecting to MongoDB at %s (database: %s)", maskMongoURI(mongoURI), dbName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting MongoDB: %v", err)
		}
	}()

	// Проверяем подключение
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Println("MongoDB connection established")

	// Создаём MongoDB репозиторий
	inventoryRepo := mongorepo.NewRepository(client, dbName)

	// Создаём service слой с зависимостью от repository
	inventoryService := service.NewInventoryService(inventoryRepo)

	// Создаём gRPC handler, который использует service
	grpcHandler := grpcapi.NewHandler(inventoryService)

	// Слушаем на localhost (IPv4)
	l, err := net.Listen("tcp4", "127.0.0.1:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Создаем gRPC сервер
	grpcSrv := grpc.NewServer()

	// Регистрируем gRPC handler
	inventorypb.RegisterInventoryServiceServer(grpcSrv, grpcHandler)

	log.Println("Inventory gRPC server listening on 127.0.0.1:50051")

	// Запускаем сервер
	if err := grpcSrv.Serve(l); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// getMongoURI возвращает URI для подключения к MongoDB
// Читает из переменной окружения INVENTORY_MONGO_URI или использует дефолт
func getMongoURI() string {
	uri := os.Getenv("INVENTORY_MONGO_URI")
	if uri == "" {
		// Дефолт для docker-compose: порт 15417 на хосте
		uri = "mongodb://inventory_user:inventory_password@127.0.0.1:15417/?authSource=admin"
	}
	return uri
}

// getMongoDBName возвращает имя базы данных MongoDB
// Читает из переменной окружения INVENTORY_MONGO_DB или использует дефолт
func getMongoDBName() string {
	dbName := os.Getenv("INVENTORY_MONGO_DB")
	if dbName == "" {
		dbName = "inventory"
	}
	return dbName
}

// maskMongoURI маскирует пароль в MongoDB URI для безопасного логирования
func maskMongoURI(uri string) string {
	// Простая маскировка: заменяем пароль на ***
	// Формат: mongodb://user:password@host:port/...
	masked := uri
	for i := 0; i < len(uri)-1; i++ {
		if uri[i] == ':' && i+1 < len(uri) && uri[i+1] != '/' {
			// Нашли начало пароля, ищем @
			for j := i + 1; j < len(uri); j++ {
				if uri[j] == '@' {
					masked = uri[:i+1] + "***" + uri[j:]
					break
				}
			}
			break
		}
	}
	return masked
}
