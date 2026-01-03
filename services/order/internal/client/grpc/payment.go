package grpcclient

import (
	"context"

	"github.com/shestoi/GoBigTech/services/order/internal/service"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
)

// PaymentClientAdapter адаптирует gRPC клиент к интерфейсу service.PaymentClient
// Это позволяет service слою не зависеть от protobuf типов
type PaymentClientAdapter struct {
	client paymentpb.PaymentServiceClient
}

// NewPaymentClientAdapter создаёт новый адаптер для Payment клиента
func NewPaymentClientAdapter(client paymentpb.PaymentServiceClient) service.PaymentClient {
	return &PaymentClientAdapter{
		client: client,
	}
}

// ProcessPayment реализует service.PaymentClient интерфейс
// Преобразует простые типы в protobuf структуры и обратно
func (a *PaymentClientAdapter) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, method string) (string, error) {
	// Преобразуем простые типы в protobuf запрос
	req := &paymentpb.ProcessPaymentRequest{
		OrderId: orderID,
		UserId:  userID,
		Amount:  amount,
		Method:  method,
	}

	// Вызываем gRPC клиент
	resp, err := a.client.ProcessPayment(ctx, req)
	if err != nil {
		return "", err
	}

	// Проверяем успешность оплаты
	if !resp.Success {
		return "", &PaymentError{Message: "payment processing failed"}
	}

	// Возвращаем transaction ID как простую строку
	return resp.TransactionId, nil
}

// PaymentError представляет ошибку обработки оплаты
type PaymentError struct {
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}

