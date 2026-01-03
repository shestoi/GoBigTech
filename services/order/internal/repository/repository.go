package repository

import (
	"context"
	"errors"
)

// Order представляет доменную модель заказа
// Это бизнес-сущность, не привязанная к HTTP или БД
type Order struct {
	ID       string
	UserID   string
	Status   string
	Items    []OrderItem
	CreatedAt int64 // Unix timestamp для простоты
}

// OrderItem представляет товар в заказе
type OrderItem struct {
	ProductID string
	Quantity  int32
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=OrderRepository --dir=. --output=./mocks --outpkg=mocks

// OrderRepository определяет интерфейс для работы с хранилищем заказов
// Service слой зависит от этого интерфейса, а не от конкретной реализации
type OrderRepository interface {
	// Save сохраняет заказ в хранилище
	Save(ctx context.Context, order Order) error
	
	// GetByID получает заказ по ID
	// Возвращает ErrNotFound, если заказ не найден
	GetByID(ctx context.Context, id string) (Order, error)
}

// ErrNotFound возвращается, когда заказ не найден в хранилище
var ErrNotFound = errors.New("order not found")

