package memory

import (
	"context"
	"sync"

	"github.com/shestoi/GoBigTech/services/payment/internal/repository"
)

// MemoryRepository реализует PaymentRepository используя in-memory хранилище
// Используется для разработки и тестирования
// В production будет заменён на реализацию с БД
type MemoryRepository struct {
	mu          sync.RWMutex
	transactions map[string]repository.Transaction // ключ = orderID
}

// NewMemoryRepository создаёт новый in-memory репозиторий
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		transactions: make(map[string]repository.Transaction),
	}
}

// GetByOrderID получает транзакцию по orderID из памяти
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) GetByOrderID(ctx context.Context, orderID string) (repository.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tx, exists := r.transactions[orderID]
	if !exists {
		return repository.Transaction{}, repository.ErrNotFound
	}

	return tx, nil
}

// Save сохраняет транзакцию в памяти
// Защищён мьютексом для безопасного доступа из разных горутин
func (r *MemoryRepository) Save(ctx context.Context, tx repository.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.transactions[tx.OrderID] = tx
	return nil
}

