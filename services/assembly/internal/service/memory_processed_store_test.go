package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryProcessedEventsStore_MarkProcessed_IsProcessed(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryProcessedEventsStore()

	eventID := "evt-1"
	ttl := 100 * time.Millisecond

	// Сначала событие не обработано
	processed, err := store.IsProcessed(ctx, eventID)
	assert.NoError(t, err)
	assert.False(t, processed)

	// Помечаем как обработанное
	err = store.MarkProcessed(ctx, eventID, ttl)
	assert.NoError(t, err)

	// Теперь должно быть обработано
	processed, err = store.IsProcessed(ctx, eventID)
	assert.NoError(t, err)
	assert.True(t, processed)
}

func TestMemoryProcessedEventsStore_TTLExpiration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryProcessedEventsStore()

	eventID := "evt-1"
	ttl := 10 * time.Millisecond // очень короткий ttl для теста

	// Помечаем как обработанное
	err := store.MarkProcessed(ctx, eventID, ttl)
	assert.NoError(t, err)

	// Сразу проверяем - должно быть обработано
	processed, err := store.IsProcessed(ctx, eventID)
	assert.NoError(t, err)
	assert.True(t, processed)

	// Ждём истечения ttl
	time.Sleep(20 * time.Millisecond)

	// Теперь должно быть не обработано (ttl истёк)
	processed, err = store.IsProcessed(ctx, eventID)
	assert.NoError(t, err)
	assert.False(t, processed)
}

func TestMemoryProcessedEventsStore_MultipleEvents(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryProcessedEventsStore()

	ttl := 100 * time.Millisecond

	// Помечаем несколько событий
	err := store.MarkProcessed(ctx, "evt-1", ttl)
	assert.NoError(t, err)

	err = store.MarkProcessed(ctx, "evt-2", ttl)
	assert.NoError(t, err)

	// Проверяем оба
	processed1, err := store.IsProcessed(ctx, "evt-1")
	assert.NoError(t, err)
	assert.True(t, processed1)

	processed2, err := store.IsProcessed(ctx, "evt-2")
	assert.NoError(t, err)
	assert.True(t, processed2)

	// Необработанное событие
	processed3, err := store.IsProcessed(ctx, "evt-3")
	assert.NoError(t, err)
	assert.False(t, processed3)
}

func TestMemoryProcessedEventsStore_IdempotentMarkProcessed(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryProcessedEventsStore()

	eventID := "evt-1"
	ttl := 100 * time.Millisecond

	// Помечаем несколько раз
	err := store.MarkProcessed(ctx, eventID, ttl)
	assert.NoError(t, err)

	err = store.MarkProcessed(ctx, eventID, ttl)
	assert.NoError(t, err)

	err = store.MarkProcessed(ctx, eventID, ttl)
	assert.NoError(t, err)

	// Должно быть обработано
	processed, err := store.IsProcessed(ctx, eventID)
	assert.NoError(t, err)
	assert.True(t, processed)
}
