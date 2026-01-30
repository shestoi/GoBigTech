package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shestoi/GoBigTech/services/iam/internal/repository"
)

// Repository реализует UserRepository используя PostgreSQL
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository создаёт новый PostgreSQL репозиторий
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// CreateUser создаёт нового пользователя в PostgreSQL
func (r *Repository) CreateUser(ctx context.Context, user repository.User) error {
	var userID uuid.UUID
	var err error

	// Если ID не задан, генерируем новый UUID
	if user.ID == "" {
		userID = uuid.New()
	} else {
		userID, err = uuid.Parse(user.ID)
		if err != nil {
			return err
		}
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO users (id, login, password_hash, telegram_id, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, user.Login, user.PasswordHash, user.TelegramID, user.CreatedAt)

	if err != nil {
		// Проверяем, это duplicate key error?
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return repository.ErrAlreadyExists
		}
		return err
	}

	return nil
}

// GetByLogin получает пользователя по login из PostgreSQL
func (r *Repository) GetByLogin(ctx context.Context, login string) (repository.User, error) {
	var user repository.User
	var createdAt time.Time
	var telegramID *string

	err := r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, telegram_id, created_at
		 FROM users
		 WHERE login = $1`,
		login).Scan(&user.ID, &user.Login, &user.PasswordHash, &telegramID, &createdAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.User{}, repository.ErrNotFound
		}
		return repository.User{}, err
	}

	user.TelegramID = telegramID
	user.CreatedAt = createdAt

	return user, nil
}

// GetByID получает пользователя по ID из PostgreSQL
func (r *Repository) GetByID(ctx context.Context, userID string) (repository.User, error) {
	var user repository.User
	var createdAt time.Time
	var telegramID *string

	parsedUUID, err := uuid.Parse(userID)
	if err != nil {
		return repository.User{}, err
	}

	err = r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, telegram_id, created_at
		 FROM users
		 WHERE id = $1`,
		parsedUUID).Scan(&user.ID, &user.Login, &user.PasswordHash, &telegramID, &createdAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.User{}, repository.ErrNotFound
		}
		return repository.User{}, err
	}

	user.TelegramID = telegramID
	user.CreatedAt = createdAt

	return user, nil
}
