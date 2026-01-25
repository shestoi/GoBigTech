package repository

import (
	"context"
	"errors"
	"time"
)

// Order представляет доменную модель заказа
// Это бизнес-сущность, не привязанная к HTTP или БД
type Order struct {
	ID        string
	UserID    string
	Status    string
	Items     []OrderItem
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

	// HandleAssemblyCompletedTx обрабатывает событие завершения сборки заказа в транзакции
	// Возвращает (inserted, rowsAffected, error):
	//   - inserted=true если событие впервые обработано
	//   - inserted=false если событие уже было обработано (duplicate)
	//   - rowsAffected - количество обновлённых строк (0 или 1)
	HandleAssemblyCompletedTx(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID string) (inserted bool, rowsAffected int64, err error)

	// SaveWithOutbox сохраняет заказ и добавляет событие в outbox в одной транзакции
	SaveWithOutbox(ctx context.Context, order Order, eventID, eventType string, occurredAt time.Time, payload []byte, topic string) error

	// GetPendingOutboxEvents получает pending события из outbox для отправки
	GetPendingOutboxEvents(ctx context.Context, limit int) ([]OutboxEvent, error)

	// MarkOutboxEventSent отмечает событие как отправленное
	MarkOutboxEventSent(ctx context.Context, eventID string) error

	// MarkOutboxEventFailed отмечает событие как failed и увеличивает attempts
	MarkOutboxEventFailed(ctx context.Context, eventID string, errMsg string) error

	// ResetOutboxEventPending сбрасывает статус события на pending для retry
	ResetOutboxEventPending(ctx context.Context, eventID string) error
}

// OutboxEvent представляет событие в outbox таблице
type OutboxEvent struct {
	EventID     string
	EventType   string
	OccurredAt  time.Time
	AggregateID string // order_id
	Payload     []byte // JSON payload
	Topic       string
	Status      string // pending, sent, failed
	Attempts    int
	LastError   *string
	CreatedAt   time.Time
	SentAt      time.Time
}

// ErrNotFound возвращается, когда заказ не найден в хранилище
var ErrNotFound = errors.New("order not found")
