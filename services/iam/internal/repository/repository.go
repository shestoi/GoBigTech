package repository

import (
	"context"
	"errors"
	"time"
)

// User представляет доменную модель пользователя
// Это бизнес-сущность, не привязанная к gRPC или БД
type User struct {
	ID           string
	Login        string
	PasswordHash string
	TelegramID   *string // nullable
	CreatedAt    time.Time
}

//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name=UserRepository --dir=. --output=./mocks --outpkg=mocks

// UserRepository определяет интерфейс для работы с хранилищем пользователей
// Service слой зависит от этого интерфейса, а не от конкретной реализации
type UserRepository interface {
	// CreateUser создаёт нового пользователя
	// Возвращает ErrAlreadyExists, если пользователь с таким login уже существует
	CreateUser(ctx context.Context, user User) error

	// GetByLogin получает пользователя по login
	// Возвращает ErrNotFound, если пользователь не найден
	GetByLogin(ctx context.Context, login string) (User, error)

	// GetByID получает пользователя по ID
	// Возвращает ErrNotFound, если пользователь не найден
	GetByID(ctx context.Context, userID string) (User, error)
}

// ErrNotFound возвращается, когда пользователь не найден в хранилище
var ErrNotFound = errors.New("user not found")

// ErrAlreadyExists возвращается, когда пользователь с таким login уже существует
var ErrAlreadyExists = errors.New("user already exists")
