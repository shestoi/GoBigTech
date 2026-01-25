package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository реализует NotificationRepository используя PostgreSQL
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository создаёт новый PostgreSQL репозиторий
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// InsertInboxEventTx вставляет событие в inbox таблицу в транзакции
// Возвращает inserted=true если событие впервые обработано, inserted=false если duplicate
func (r *Repository) InsertInboxEventTx(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID, topic string, partition int, message_offset int64) (inserted bool, err error) {
	// Начинаем транзакцию
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	// Пытаемся вставить событие в inbox
	_, err = tx.Exec(ctx,
		`INSERT INTO notification_inbox_events (event_id, event_type, occurred_at, order_id, topic, partition, message_offset)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		eventID, eventType, occurredAt, orderID, topic, partition, message_offset)

	if err != nil {
		// Проверяем, это duplicate key error?
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			// Событие уже обработано
			return false, nil
		}
		// Другая ошибка
		return false, err
	}

	// Событие впервые обработано
	inserted = true

	// Коммитим транзакцию
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}

	return inserted, nil
}
