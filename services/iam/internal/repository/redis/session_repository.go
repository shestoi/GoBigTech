package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/iam/internal/repository"
)

const (
	hashFieldUserID     = "user_id" // hash user_id - id пользователя
	hashFieldCreatedAt  = "created_at" // hashFieldCreatedAt - поле created_at в hash
	hashFieldLastSeenAt = "last_seen_at" // hashFieldLastSeenAt - поле last_seen_at в hash
)

// SessionRepository реализует SessionRepository используя Redis hash
type SessionRepository struct {
	client *redis.Client
	logger *zap.Logger
}

// NewSessionRepository создаёт новый Redis session repository
func NewSessionRepository(client *redis.Client, logger *zap.Logger) *SessionRepository {
	return &SessionRepository{
		client: client,
		logger: logger,
	}
}

func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// CreateSession создаёт новую сессию для пользователя в Redis (hash)
func (r *SessionRepository) CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	sessionID := uuid.NewString()
	key := sessionKey(sessionID)
	now := time.Now().UTC().Format(time.RFC3339)

	pipe := r.client.Pipeline() //pipe для выполнения команд в Redis
	pipe.HSet(ctx, key, hashFieldUserID, userID, hashFieldCreatedAt, now, hashFieldLastSeenAt, now) //HSet для установки значений в hash
	pipe.Expire(ctx, key, ttl) //Expire для установки TTL для hash
	_, err := pipe.Exec(ctx) //Exec для выполнения команд в Redis
	if err != nil {
		r.logger.Error("failed to create session hash in redis",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	r.logger.Info("session hash created",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID),
		zap.Duration("ttl", ttl),
	)

	return sessionID, nil
}

// GetUserIDBySession получает user_id по session_id из Redis hash
func (r *SessionRepository) GetUserIDBySession(ctx context.Context, sessionID string) (string, error) {
	key := sessionKey(sessionID)

	userID, err := r.client.HGet(ctx, key, hashFieldUserID).Result()
	if err != nil {
		if err == redis.Nil {
			r.logger.Debug("session hash not found",
				zap.String("session_id", sessionID),
			)
			return "", repository.ErrSessionNotFound
		}
		r.logger.Error("failed to get session hash from redis",
			zap.Error(err),
			zap.String("session_id", sessionID),
		)
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	if userID == "" {
		r.logger.Debug("session hash has empty user_id",
			zap.String("session_id", sessionID),
		)
		return "", repository.ErrSessionNotFound
	}

	return userID, nil
}

// DeleteSession удаляет сессию (hash) из Redis
func (r *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	key := sessionKey(sessionID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		r.logger.Error("failed to delete session hash from redis",
			zap.Error(err),
			zap.String("session_id", sessionID),
		)
		return fmt.Errorf("failed to delete session: %w", err)
	}

	r.logger.Info("session hash deleted",
		zap.String("session_id", sessionID),
	)

	return nil
}

// RefreshSession обновляет last_seen_at и TTL сессии в Redis hash; если ключ отсутствует — ErrSessionNotFound
func (r *SessionRepository) RefreshSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	key := sessionKey(sessionID)

	// Проверяем существование ключа через HGET (HSET на несуществующем ключе создаст его — не делаем так)
	_, err := r.client.HGet(ctx, key, hashFieldUserID).Result()
	if err != nil {
		if err == redis.Nil {
			return repository.ErrSessionNotFound
		}
		r.logger.Error("failed to check session hash in redis",
			zap.Error(err),
			zap.String("session_id", sessionID),
		)
		return fmt.Errorf("failed to check session: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, hashFieldLastSeenAt, now)
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("failed to refresh session hash TTL in redis",
			zap.Error(err),
			zap.String("session_id", sessionID),
		)
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	r.logger.Debug("session hash refreshed",
		zap.String("session_id", sessionID),
		zap.Duration("ttl", ttl),
	)

	return nil
}
