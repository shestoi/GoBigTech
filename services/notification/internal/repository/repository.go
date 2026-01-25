package repository

import (
	"context"
	"time"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=NotificationRepository --dir=. --output=./mocks --outpkg=mocks

// NotificationRepository определяет интерфейс для работы с хранилищем уведомлений
type NotificationRepository interface {
	// InsertInboxEventTx вставляет событие в inbox таблицу в транзакции
	// Возвращает inserted=true если событие впервые обработано, inserted=false если duplicate
	InsertInboxEventTx(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID, topic string, partition int, message_offset int64) (inserted bool, err error)
}
