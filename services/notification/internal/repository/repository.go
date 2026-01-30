package repository

import (
	"context"
	"time"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=NotificationRepository --dir=. --output=./mocks --outpkg=mocks

// InboxUpsertResult результат UpsertInboxPending: уже обработано (sent) или можно продолжать (pending)
type InboxUpsertResult struct {
	AlreadyProcessed bool // true — запись есть со статусом sent, не обрабатывать
	CanProcess       bool // true — запись pending (новая или retry), продолжать обработку
}

// NotificationRepository определяет интерфейс для работы с хранилищем уведомлений
type NotificationRepository interface {
	// UpsertInboxPending создаёт запись со статусом pending если её нет; если есть sent — AlreadyProcessed; если pending — CanProcess (retry)
	UpsertInboxPending(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID, topic string, partition int, messageOffset int64) (*InboxUpsertResult, error)
	// MarkInboxSent переводит запись в статус sent
	MarkInboxSent(ctx context.Context, eventID string) error
	// MarkInboxFailed сохраняет last_error для записи (остаётся pending для retry)
	MarkInboxFailed(ctx context.Context, eventID string, errString string) error
}
