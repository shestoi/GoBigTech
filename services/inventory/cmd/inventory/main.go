package main

import (
	"context"
	"log"
	"net"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	"google.golang.org/grpc"
)

type server struct {
	inventorypb.UnimplementedInventoryServiceServer
}

func (s *server) GetStock(ctx context.Context, req *inventorypb.GetStockRequest) (*inventorypb.GetStockResponse, error) {
	log.Printf("GetStock called for product: %s", req.GetProductId())
	return &inventorypb.GetStockResponse{
		ProductId: req.GetProductId(),
		Available: 42, // Всегда в наличии 42 штуки
	}, nil
}

func (s *server) ReserveStock(ctx context.Context, req *inventorypb.ReserveStockRequest) (*inventorypb.ReserveStockResponse, error) {
	log.Printf("ReserveStock called: product=%s, quantity=%d", req.GetProductId(), req.GetQuantity())
	// Простая логика: резервируем если количество ≤ 42
	return &inventorypb.ReserveStockResponse{
		Success: req.GetQuantity() <= 42,
	}, nil
}

func main() {
	// Слушаем на localhost (IPv4)
	l, err := net.Listen("tcp4", "127.0.0.1:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Создаем gRPC сервер
	grpcSrv := grpc.NewServer()

	// Регистрируем наш сервер
	inventorypb.RegisterInventoryServiceServer(grpcSrv, &server{})

	log.Println("Inventory gRPC server listening on 127.0.0.1:50051")

	// Запускаем сервер
	if err := grpcSrv.Serve(l); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
