package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	httpapi "github.com/shestoi/GoBigTech/services/order/internal/api/http"
	grpcclient "github.com/shestoi/GoBigTech/services/order/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/order/internal/repository/postgres"
	"github.com/shestoi/GoBigTech/services/order/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
)

func main() {
	log.Println("Starting Order service...")

	// Подключаемся к Inventory сервису
	inventoryAddr := "127.0.0.1:50051"
	log.Printf("Connecting to Inventory service at %s", inventoryAddr)

	inventoryConn, err := grpc.NewClient(inventoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Inventory gRPC client: %v", err)
	}
	defer inventoryConn.Close()

	inventoryClient := inventorypb.NewInventoryServiceClient(inventoryConn)

	// Подключаемся к Payment сервису
	paymentAddr := "127.0.0.1:50052"
	log.Printf("Connecting to Payment service at %s", paymentAddr)

	paymentConn, err := grpc.NewClient(paymentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Payment gRPC client: %v", err)
	}
	defer paymentConn.Close()

	paymentClient := paymentpb.NewPaymentServiceClient(paymentConn)

	// Обёртываем gRPC клиенты в адаптеры
	// Адаптеры преобразуют protobuf типы в доменные типы service слоя
	inventoryClientAdapter := grpcclient.NewInventoryClientAdapter(inventoryClient)
	paymentClientAdapter := grpcclient.NewPaymentClientAdapter(paymentClient)

	// Подключаемся к PostgreSQL
	dsn := getPostgresDSN()
	log.Printf("Connecting to PostgreSQL at %s", maskDSN(dsn))

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL connection pool: %v", err)
	}
	defer pool.Close()

	// Проверяем подключение
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("PostgreSQL connection established")

	// Создаём PostgreSQL репозиторий
	orderRepo := postgres.NewRepository(pool)

	// Создаем service слой с зависимостями
	// Service содержит всю бизнес-логику, отделённую от HTTP, gRPC и БД
	orderService := service.NewOrderService(inventoryClientAdapter, paymentClientAdapter, orderRepo)

	// Создаем HTTP handler, который использует service
	handler := httpapi.NewHandler(orderService)

	// Настраиваем роутер и регистрируем все маршруты
	router := httpapi.NewRouter(handler)

	// Запускаем HTTP сервер
	port := ":8080"
	log.Printf("Order HTTP server starting on %s", port)
	log.Printf("Health check available at http://localhost%s/health", port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// getPostgresDSN возвращает DSN для подключения к PostgreSQL
// Читает из переменной окружения ORDER_POSTGRES_DSN или использует дефолт
func getPostgresDSN() string {
	dsn := os.Getenv("ORDER_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable"
	}
	return dsn
}

// maskDSN маскирует пароль в DSN для безопасного логирования
func maskDSN(dsn string) string {
	// Простая маскировка: заменяем пароль на ***
	// Формат: postgres://user:password@host:port/db
	// Ищем :password@ и заменяем на :***@
	masked := dsn
	for i := 0; i < len(dsn)-1; i++ {
		if dsn[i] == ':' && dsn[i+1] != '/' {
			// Нашли начало пароля, ищем @
			for j := i + 1; j < len(dsn); j++ {
				if dsn[j] == '@' {
					masked = dsn[:i+1] + "***" + dsn[j:]
					break
				}
			}
			break
		}
	}
	return masked
}
