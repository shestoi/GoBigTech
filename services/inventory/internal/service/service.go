package service

import (
	"context"
	"log"

	"github.com/shestoi/GoBigTech/services/inventory/internal/repository"
)

// InventoryService содержит бизнес-логику работы с инвентарём
// Использует только простые типы Go, не зависит от protobuf
// Зависит от интерфейса InventoryRepository, а не от конкретной реализации
type InventoryService struct {
	repo repository.InventoryRepository
}

// NewInventoryService создаёт новый экземпляр InventoryService
// Принимает repository как зависимость - это позволяет легко подменять его в тестах
func NewInventoryService(repo repository.InventoryRepository) *InventoryService {
	return &InventoryService{
		repo: repo,
	}
}

// GetStock возвращает количество товара на складе
// Делегирует запрос в repository и обрабатывает бизнес-логику
func (s *InventoryService) GetStock(ctx context.Context, productID string) (int32, error) {
	log.Printf("GetStock called for product: %s", productID)

	// Получаем остаток из repository
	available, err := s.repo.GetStock(ctx, productID)
	if err != nil {
		// Если товар не найден, repository вернёт ErrNotFound
		// Возвращаем ошибку, а не дефолтное значение
		if err == repository.ErrNotFound {
			log.Printf("Product %s not found", productID)
		}
		return 0, err
	}

	return available, nil
}

// ReserveStock резервирует товар на складе
// Делегирует запрос в repository, который проверяет доступность и уменьшает остаток
// Возвращает true, если резервирование успешно
func (s *InventoryService) ReserveStock(ctx context.Context, productID string, quantity int32) (bool, error) {
	log.Printf("ReserveStock called: product=%s, quantity=%d", productID, quantity)

	// Делегируем резервирование в repository
	// Repository проверит доступность и уменьшит остаток при успехе
	success, err := s.repo.ReserveStock(ctx, productID, quantity)
	if err != nil {
		log.Printf("ReserveStock error: %v", err)
		return false, err
	}

	if success {
		log.Printf("ReserveStock successful: product=%s, quantity=%d", productID, quantity)
	} else {
		log.Printf("ReserveStock failed: insufficient stock for product=%s, quantity=%d", productID, quantity)
	}

	return success, nil
}
