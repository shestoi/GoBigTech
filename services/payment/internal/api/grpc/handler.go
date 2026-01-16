package grpcapi

import (
	"context"

	"github.com/shestoi/GoBigTech/services/payment/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
)

// Handler содержит gRPC-обработчики для Payment Service
// Зависит от service слоя, но не знает о деталях реализации (repository, БД и т.д.)
type Handler struct {
	paymentpb.UnimplementedPaymentServiceServer
	paymentService *service.PaymentService
}

// NewHandler создаёт новый gRPC handler
func NewHandler(paymentService *service.PaymentService) *Handler {
	return &Handler{
		paymentService: paymentService,
	}
}

// ProcessPayment обрабатывает gRPC запрос ProcessPayment
// Тонкий слой: преобразует protobuf типы в простые типы и вызывает service
func (h *Handler) ProcessPayment(ctx context.Context, req *paymentpb.ProcessPaymentRequest) (*paymentpb.ProcessPaymentResponse, error) {
	// Вызываем service слой для обработки платежа
	// gRPC handler только преобразует типы protobuf <-> простые типы
	transactionID, success, err := h.paymentService.ProcessPayment(
		ctx,
		req.GetOrderId(),
		req.GetUserId(),
		req.GetAmount(),
		req.GetMethod(),
	)

	if err != nil {
		return nil, err
	}

	return &paymentpb.ProcessPaymentResponse{
		Success:       success,
		TransactionId: transactionID,
	}, nil
}


