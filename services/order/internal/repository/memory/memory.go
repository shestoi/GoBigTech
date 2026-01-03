package memory

import (
	"context"
	"sync"
	"time"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

// MemoryRepository реализует OrderRepository используя in-memory хранилище
// Используется для разработки и тестирования
// В production будет заменён на реализацию с БД
type MemoryRepository struct {
	mu     sync.RWMutex
	orders map[string]repository.Order
}

// NewMemoryRepository создаёт новый in-memory репозиторий
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		orders: make(map[string]repository.Order),
	}
}

// Save сохраняет заказ в памяти
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) Save(ctx context.Context, order repository.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Если у заказа нет CreatedAt, устанавливаем текущее время
	if order.CreatedAt == 0 {
		order.CreatedAt = time.Now().Unix()
	}

	r.orders[order.ID] = order
	return nil
}

// GetByID получает заказ по ID из памяти
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) GetByID(ctx context.Context, id string) (repository.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, exists := r.orders[id]
	if !exists {
		return repository.Order{}, repository.ErrNotFound
	}

	return order, nil
}
