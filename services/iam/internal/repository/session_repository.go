package repository

import (
	"context"
	"errors"
	"time"
)

// SessionRepository определяет интерфейс для работы с сессиями
// Service слой зависит от этого интерфейса, а не от конкретной реализации
type SessionRepository interface {
	// CreateSession создаёт новую сессию для пользователя
	// Возвращает sessionID и ошибку
	CreateSession(ctx context.Context, userID string, ttl time.Duration) (sessionID string, err error)

	// GetUserIDBySession получает user_id по session_id
	// Возвращает ErrSessionNotFound, если сессия не найдена или истекла
	GetUserIDBySession(ctx context.Context, sessionID string) (userID string, err error)

	// DeleteSession удаляет сессию
	DeleteSession(ctx context.Context, sessionID string) error

	// RefreshSession обновляет TTL сессии
	RefreshSession(ctx context.Context, sessionID string, ttl time.Duration) error
}

// ErrSessionNotFound возвращается, когда сессия не найдена или истекла
var ErrSessionNotFound = errors.New("session not found")
