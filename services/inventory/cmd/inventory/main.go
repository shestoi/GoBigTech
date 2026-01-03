package main

import (
	"log"
	"net"

	grpcapi "github.com/shestoi/GoBigTech/services/inventory/internal/api/grpc"
	"github.com/shestoi/GoBigTech/services/inventory/internal/repository/memory"
	"github.com/shestoi/GoBigTech/services/inventory/internal/service"
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting Inventory service...")

	// Создаём in-memory репозиторий для хранения инвентаря
	// В production будет заменён на реализацию с БД (MongoDB)
	inventoryRepo := memory.NewMemoryRepository(nil)

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
