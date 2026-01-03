package memory

import (
	"context"
	"sync"
)

const (
	// DefaultStock - значение по умолчанию для товаров, которых нет в хранилище
	// Используется для обратной совместимости с текущим поведением (всегда 42)
	DefaultStock int32 = 42
)

// MemoryRepository реализует InventoryRepository используя in-memory хранилище
// Используется для разработки и тестирования
// В production будет заменён на реализацию с БД (MongoDB)
type MemoryRepository struct {
	mu           sync.RWMutex
	stock        map[string]int32
	defaultStock int32
}

// NewMemoryRepository создаёт новый in-memory репозиторий
// Если initialStock == nil, создаётся пустое хранилище с default=42
// Если initialStock != nil, используется переданная карта
func NewMemoryRepository(initialStock map[string]int32) *MemoryRepository {
	stock := make(map[string]int32)
	for k, v := range initialStock {
		stock[k] = v
	}

	return &MemoryRepository{
		stock:        stock,
		defaultStock: DefaultStock,
	}
}

// GetStock получает количество товара из памяти
// Если товар отсутствует, возвращает default=42 для обратной совместимости
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) GetStock(ctx context.Context, productID string) (int32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	available, exists := r.stock[productID]
	if !exists {
		// Возвращаем default для обратной совместимости
		// В production можно вернуть repository.ErrNotFound
		return r.defaultStock, nil
	}

	return available, nil
}

// ReserveStock резервирует товар на складе
// Проверяет доступность, уменьшает остаток при успешном резервировании
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) ReserveStock(ctx context.Context, productID string, quantity int32) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Получаем текущий остаток (или default, если товара нет)
	currentStock := r.defaultStock
	if available, exists := r.stock[productID]; exists {
		currentStock = available
	}

	// Проверяем, хватает ли товара
	if currentStock < quantity {
		// Недостаточно товара - возвращаем false без изменения остатка
		return false, nil
	}

	// Достаточно товара - резервируем (уменьшаем остаток)
	newStock := currentStock - quantity

	// Если новый остаток равен default, можно удалить запись
	// Но для простоты оставляем запись с нулём
	if !r.stockExists(productID) {
		// Если товара не было, создаём запись
		r.stock[productID] = newStock
	} else {
		// Обновляем существующую запись
		r.stock[productID] = newStock
	}

	return true, nil
}

// stockExists проверяет, существует ли товар в хранилище
// Вызывается только внутри заблокированного мьютекса
func (r *MemoryRepository) stockExists(productID string) bool {
	_, exists := r.stock[productID]
	return exists
}
