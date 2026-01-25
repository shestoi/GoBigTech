package service

import (
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

// OrderAssemblyCompletedEvent представляет событие завершения сборки заказа (входящее из Kafka)
type OrderAssemblyCompletedEvent struct {
	EventID      string
	EventType    string
	EventVersion int
	OccurredAt   time.Time
	OrderID      string
	UserID       string
}

