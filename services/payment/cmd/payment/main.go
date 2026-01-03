package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
	"google.golang.org/grpc"
)

type server struct {
	paymentpb.UnimplementedPaymentServiceServer
}

func (s *server) ProcessPayment(ctx context.Context, req *paymentpb.ProcessPaymentRequest) (*paymentpb.ProcessPaymentResponse, error) {
	log.Printf("ProcessPayment called: order=%s, user=%s, amount=%f",
		req.GetOrderId(), req.GetUserId(), req.GetAmount())

	// Всегда успешная оплата
	return &paymentpb.ProcessPaymentResponse{
		Success:       true,
		TransactionId: fmt.Sprintf("tx_%s_%d", req.GetOrderId(), time.Now().Unix()),
	}, nil
}

func main() {
	// Слушаем на localhost (IPv4)
	l, err := net.Listen("tcp4", "127.0.0.1:50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Создаем gRPC сервер
	grpcSrv := grpc.NewServer()

	// Регистрируем наш сервер
	paymentpb.RegisterPaymentServiceServer(grpcSrv, &server{})

	log.Println("Payment gRPC server listening on 127.0.0.1:50052")

	// Запускаем сервер
	if err := grpcSrv.Serve(l); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
