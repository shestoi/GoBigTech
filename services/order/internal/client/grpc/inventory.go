package grpcclient

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	"github.com/shestoi/GoBigTech/services/order/internal/authctx"
	"github.com/shestoi/GoBigTech/services/order/internal/service"
)

// InventoryClientAdapter адаптирует gRPC клиент к интерфейсу service.InventoryClient
// Это позволяет service слою не зависеть от protobuf типов
type InventoryClientAdapter struct {
	client inventorypb.InventoryServiceClient
}

// NewInventoryClientAdapter создаёт новый адаптер для Inventory клиента
func NewInventoryClientAdapter(client inventorypb.InventoryServiceClient) service.InventoryClient {
	return &InventoryClientAdapter{
		client: client,
	}
}

// ReserveStock реализует service.InventoryClient интерфейс
// Прокидывает x-session-id из context в gRPC metadata для Inventory interceptor
func (a *InventoryClientAdapter) ReserveStock(ctx context.Context, productID string, quantity int32) error {
	sid, ok := authctx.SessionIDFromContext(ctx) // извлекаем session_id из контекста
	if !ok || sid == "" {
		return status.Error(codes.Unauthenticated, "session_id is required")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-session-id", sid) // добавляем session_id в metadata

	req := &inventorypb.ReserveStockRequest{ // создаём запрос на резервирование товара
		ProductId: productID, // id товара
		Quantity:  quantity, // количество товара
	}

	resp, err := a.client.ReserveStock(ctx, req) // вызываем gRPC метод на резервирование товара
	if err != nil {
		return err
	}

	// Проверяем успешность резервирования
	if !resp.Success {
		return &ReservationError{Message: "failed to reserve stock"}
	}

	return nil
}

// ReservationError представляет ошибку резервирования товара
type ReservationError struct {
	Message string
}

func (e *ReservationError) Error() string {
	return e.Message
}
