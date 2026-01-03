package main

import (
	"log"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	httpapi "github.com/shestoi/GoBigTech/services/order/internal/api/http"
	grpcclient "github.com/shestoi/GoBigTech/services/order/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/order/internal/repository/memory"
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

	// Создаём in-memory репозиторий для хранения заказов
	// В production будет заменён на реализацию с БД (PostgreSQL)
	orderRepo := memory.NewMemoryRepository()

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
