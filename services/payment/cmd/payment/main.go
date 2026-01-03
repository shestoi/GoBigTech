package main

import (
	"log"
	"net"

	grpcapi "github.com/shestoi/GoBigTech/services/payment/internal/api/grpc"
	"github.com/shestoi/GoBigTech/services/payment/internal/repository/memory"
	"github.com/shestoi/GoBigTech/services/payment/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting Payment service...")

	// Создаём in-memory репозиторий для хранения транзакций
	// В production будет заменён на реализацию с БД
	paymentRepo := memory.NewMemoryRepository()

	// Создаём service слой с зависимостью от repository
	paymentService := service.NewPaymentService(paymentRepo)

	// Создаём gRPC handler, который использует service
	grpcHandler := grpcapi.NewHandler(paymentService)

	// Слушаем на localhost (IPv4)
	l, err := net.Listen("tcp4", "127.0.0.1:50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Создаем gRPC сервер
	grpcSrv := grpc.NewServer()

	// Регистрируем gRPC handler
	paymentpb.RegisterPaymentServiceServer(grpcSrv, grpcHandler)

	log.Println("Payment gRPC server listening on 127.0.0.1:50052")

	// Запускаем сервер
	if err := grpcSrv.Serve(l); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
