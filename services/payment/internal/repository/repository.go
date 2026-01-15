package repository

import (
	"context"
	"errors"
)

// Transaction представляет доменную модель транзакции платежа
// Это бизнес-сущность, не привязанная к HTTP, gRPC или БД
type Transaction struct {
	OrderID       string
	UserID        string
	Amount        float64
	Method        string
	TransactionID string
	Status        string
	CreatedAt     int64 // Unix timestamp
}

// PaymentRepository определяет интерфейс для работы с хранилищем транзакций
// Service слой зависит от этого интерфейса, а не от конкретной реализации
type PaymentRepository interface {
	// GetByOrderID получает транзакцию по orderID
	// Возвращает ErrNotFound, если транзакция не найдена
	GetByOrderID(ctx context.Context, orderID string) (Transaction, error)
	
	// Save сохраняет транзакцию в хранилище
	Save(ctx context.Context, tx Transaction) error
}

// ErrNotFound возвращается, когда транзакция не найдена в хранилище
var ErrNotFound = errors.New("transaction not found")


