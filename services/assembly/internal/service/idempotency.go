package service

import (
	"context"
	"time"
)

// ProcessedEventsStore хранит информацию об обработанных событиях для обеспечения idempotency
//
//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=ProcessedEventsStore --dir=. --output=./mocks --outpkg=mocks
type ProcessedEventsStore interface {
	// MarkProcessed сохраняет eventID как обработанный. Должен быть idempotent сам по себе.
	// ttl определяет время жизни записи (после истечения ttl событие может быть обработано повторно).
	MarkProcessed(ctx context.Context, eventID string, ttl time.Duration) error

	// IsProcessed возвращает true если eventID уже был обработан и ещё не истёк ttl.
	IsProcessed(ctx context.Context, eventID string) (bool, error)
}

// Sleeper определяет интерфейс для задержки (используется для тестирования)
type Sleeper interface {
	// Sleep выполняет задержку на указанное время или до отмены контекста
	Sleep(ctx context.Context, d time.Duration) error
}

// DefaultSleeper реализует Sleeper используя time.After
type DefaultSleeper struct{}

// Sleep выполняет задержку используя time.After
func (s *DefaultSleeper) Sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
