package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shestoi/GoBigTech/services/notification/internal/repository"
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

// UpsertInboxPending создаёт запись со статусом pending если её нет; если есть sent — AlreadyProcessed; если pending — CanProcess (retry)
func (r *Repository) UpsertInboxPending(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID, topic string, partition int, messageOffset int64) (*repository.InboxUpsertResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	_, err = tx.Exec(ctx,
		`INSERT INTO notification_inbox_events (event_id, event_type, occurred_at, order_id, topic, partition, message_offset, status, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
		 ON CONFLICT (event_id) DO NOTHING`,
		eventID, eventType, occurredAt, orderID, topic, partition, messageOffset, now)
	if err != nil {
		return nil, err
	}

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM notification_inbox_events WHERE event_id = $1`, eventID).Scan(&status)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	res := &repository.InboxUpsertResult{} //res - результат UpsertInboxPending
	switch status { //status - статус события
	case "sent":
		res.AlreadyProcessed = true //если событие уже обработано, устанавливаем флаг AlreadyProcessed в true
	case "pending":
		res.CanProcess = true //если событие ещё не обработано, устанавливаем флаг CanProcess в true
	}
	return res, nil
}

// MarkInboxSent переводит запись в статус sent
func (r *Repository) MarkInboxSent(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notification_inbox_events SET status = 'sent', updated_at = now(), last_error = NULL WHERE event_id = $1`,
		eventID)
	return err
}

// MarkInboxFailed сохраняет last_error для записи (остаётся pending для retry)
func (r *Repository) MarkInboxFailed(ctx context.Context, eventID string, errString string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notification_inbox_events SET last_error = $2, updated_at = now() WHERE event_id = $1 AND status = 'pending'`,
		eventID, errString)
	return err
}
