package grpcapi

import (
	"context"

	"github.com/shestoi/GoBigTech/services/inventory/internal/service"
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
)

// Handler содержит gRPC-обработчики для Inventory Service
// Зависит от service слоя, но не знает о деталях реализации (repository, БД и т.д.)
type Handler struct {
	inventorypb.UnimplementedInventoryServiceServer
	inventoryService *service.InventoryService
}

// NewHandler создаёт новый gRPC handler
func NewHandler(inventoryService *service.InventoryService) *Handler {
	return &Handler{
		inventoryService: inventoryService,
	}
}

// GetStock обрабатывает gRPC запрос GetStock
// Тонкий слой: преобразует protobuf типы в простые типы и вызывает service
func (h *Handler) GetStock(ctx context.Context, req *inventorypb.GetStockRequest) (*inventorypb.GetStockResponse, error) {
	// Вызываем service слой для получения количества товара
	// gRPC handler только преобразует типы protobuf <-> простые типы
	available, err := h.inventoryService.GetStock(ctx, req.GetProductId())
	if err != nil {
		return nil, err
	}

	return &inventorypb.GetStockResponse{
		ProductId: req.GetProductId(),
		Available: available,
	}, nil
}

// ReserveStock обрабатывает gRPC запрос ReserveStock
// Тонкий слой: преобразует protobuf типы в простые типы и вызывает service
func (h *Handler) ReserveStock(ctx context.Context, req *inventorypb.ReserveStockRequest) (*inventorypb.ReserveStockResponse, error) {
	// Вызываем service слой для резервирования товара
	// gRPC handler только преобразует типы protobuf <-> простые типы
	success, err := h.inventoryService.ReserveStock(ctx, req.GetProductId(), req.GetQuantity())
	if err != nil {
		return nil, err
	}

	return &inventorypb.ReserveStockResponse{
		Success: success,
	}, nil
}

