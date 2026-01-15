package grpcclient

import (
	"context"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
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
// Преобразует простые типы в protobuf структуры и обратно
func (a *InventoryClientAdapter) ReserveStock(ctx context.Context, productID string, quantity int32) error {
	// Преобразуем простые типы в protobuf запрос
	req := &inventorypb.ReserveStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	}

	// Вызываем gRPC клиент
	resp, err := a.client.ReserveStock(ctx, req)
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
