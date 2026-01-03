package repository

import (
	"context"
	"errors"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=InventoryRepository --dir=. --output=./mocks --outpkg=mocks

// InventoryRepository определяет интерфейс для работы с хранилищем инвентаря
// Service слой зависит от этого интерфейса, а не от конкретной реализации
type InventoryRepository interface {
	// GetStock получает количество товара на складе
	// Возвращает ErrNotFound, если товар не найден
	GetStock(ctx context.Context, productID string) (int32, error)

	// ReserveStock резервирует товар на складе
	// Проверяет доступность и уменьшает остаток при успешном резервировании
	// Возвращает true, если резервирование успешно, false если недостаточно товара
	ReserveStock(ctx context.Context, productID string, quantity int32) (bool, error)
}

// ErrNotFound возвращается, когда товар не найден в хранилище
var ErrNotFound = errors.New("product not found")
