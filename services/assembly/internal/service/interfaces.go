package service

import (
	"context"
	"time"
)

// OrderPaidEvent представляет событие успешной оплаты заказа (входящее из Kafka)
type OrderPaidEvent struct {
	EventID       string
	EventType     string
	EventVersion  int
	OccurredAt    time.Time
	OrderID       string
	UserID        string
	Amount        int64
	PaymentMethod string
}

// OrderAssemblyCompletedEvent представляет событие завершения сборки заказа (исходящее в Kafka)
type OrderAssemblyCompletedEvent struct {
	EventID      string
	EventType    string // "order.assembly.completed"
	EventVersion int
	OccurredAt   time.Time
	OrderID      string
	UserID       string
}

// AssemblyEventPublisher определяет интерфейс для публикации событий завершения сборки заказа
type AssemblyEventPublisher interface {
	// PublishOrderAssemblyCompleted публикует событие успешной сборки заказа
	PublishOrderAssemblyCompleted(ctx context.Context, event OrderAssemblyCompletedEvent) error
}
