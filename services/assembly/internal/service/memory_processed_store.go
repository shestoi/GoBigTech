package service

import (
	"context"
	"sync"
	"time"
)

// MemoryProcessedEventsStore реализует ProcessedEventsStore используя in-memory map
// Используется для dev/test окружений. В production должен быть заменён на Postgres/Redis.
type MemoryProcessedEventsStore struct {
	mu      sync.RWMutex
	events  map[string]time.Time // eventID -> expiresAt
	cleanup bool                 // флаг для ленивой очистки
}

// NewMemoryProcessedEventsStore создаёт новый in-memory store
func NewMemoryProcessedEventsStore() *MemoryProcessedEventsStore {
	return &MemoryProcessedEventsStore{
		events:  make(map[string]time.Time),
		cleanup: false,
	}
}

// MarkProcessed сохраняет eventID как обработанный с указанным ttl
func (s *MemoryProcessedEventsStore) MarkProcessed(ctx context.Context, eventID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ленивая очистка протухших записей
	s.cleanupExpiredLocked()

	// Сохраняем время истечения
	expiresAt := time.Now().Add(ttl)
	s.events[eventID] = expiresAt //сохраняем время истечения

	return nil
}

// IsProcessed проверяет, был ли eventID уже обработан
func (s *MemoryProcessedEventsStore) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	s.mu.RLock()         //читаем данные
	defer s.mu.RUnlock() //освобождаем lock

	// Ленивая очистка протухших записей (нужно переключиться на write lock)
	s.mu.RUnlock()           //освобождаем read lock
	s.mu.Lock()              //захватываем write lock
	s.cleanupExpiredLocked() //очищаем протухшие записи
	s.mu.Unlock()            //освобождаем write lock
	s.mu.RLock()             //захватываем read lock

	expiresAt, exists := s.events[eventID] //получаем время истечения
	if !exists {
		return false, nil //если запись не найдена, возвращаем false
	}

	// Проверяем, не истёк ли ttl
	if time.Now().After(expiresAt) {
		// Запись протухла, удаляем её
		delete(s.events, eventID)
		return false, nil
	}

	return true, nil
}

// cleanupExpiredLocked удаляет протухшие записи (вызывается с уже захваченным lock)
func (s *MemoryProcessedEventsStore) cleanupExpiredLocked() {
	now := time.Now()
	for eventID, expiresAt := range s.events {
		if now.After(expiresAt) {
			delete(s.events, eventID)
		}
	}
}
