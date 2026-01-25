package service

import (
	"context"
	"time"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=InventoryClient --dir=. --output=./mocks --outpkg=mocks

// InventoryClient определяет интерфейс для работы с Inventory сервисом
// Использует доменные типы вместо protobuf - это делает service независимым от gRPC
type InventoryClient interface {
	// ReserveStock резервирует товар на складе
	// Возвращает ошибку, если резервирование не удалось
	ReserveStock(ctx context.Context, productID string, quantity int32) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=PaymentClient --dir=. --output=./mocks --outpkg=mocks

// PaymentClient определяет интерфейс для работы с Payment сервисом
// Использует доменные типы вместо protobuf - это делает service независимым от gRPC
type PaymentClient interface {
	// ProcessPayment обрабатывает оплату заказа
	// Возвращает transaction ID и ошибку
	ProcessPayment(ctx context.Context, orderID, userID string, amount float64, method string) (string, error)
}

// OrderPaidEvent представляет событие успешной оплаты заказа
type OrderPaidEvent struct {
	OrderID       string
	UserID        string
	Amount        int64 // сумма в минимальных единицах (копейки, центы)
	PaymentMethod string
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=PaymentEventPublisher --dir=. --output=./mocks --outpkg=mocks

// PaymentEventPublisher определяет интерфейс для публикации событий оплаты заказа
// Используется для отправки событий в Kafka или другие системы событий
type PaymentEventPublisher interface {
	// PublishOrderPaid публикует событие успешной оплаты заказа
	PublishOrderPaid(ctx context.Context, event OrderPaidEvent) error
}

// OrderAssemblyCompletedEvent представляет событие завершения сборки заказа (входящее из Kafka)
type OrderAssemblyCompletedEvent struct {
	EventID      string
	EventType    string
	EventVersion int
	OccurredAt   time.Time
	OrderID      string
	UserID       string
}
