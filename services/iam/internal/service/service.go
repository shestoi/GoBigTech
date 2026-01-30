package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/shestoi/GoBigTech/services/iam/internal/repository"
)

// ErrSessionNotFoundOrExpired возвращается при невалидной/истёкшей сессии (handler маппит в codes.Unauthenticated)
var ErrSessionNotFoundOrExpired = errors.New("session not found or expired")

// Service содержит бизнес-логику работы с пользователями
type Service struct {
	logger      *zap.Logger
	repo        repository.UserRepository
	sessionRepo repository.SessionRepository
	sessionTTL  time.Duration
}

// NewService создаёт новый экземпляр Service
func NewService(logger *zap.Logger, repo repository.UserRepository, sessionRepo repository.SessionRepository, sessionTTL time.Duration) *Service {
	return &Service{
		logger:      logger,
		repo:        repo,
		sessionRepo: sessionRepo,
		sessionTTL:  sessionTTL,
	}
}

// RegisterInput содержит входные данные для регистрации пользователя
type RegisterInput struct {
	Login      string
	Password   string
	TelegramID *string
}

// RegisterOutput содержит результат регистрации пользователя
type RegisterOutput struct {
	UserID string
}

// Register регистрирует нового пользователя
func (s *Service) Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	// Валидация входных данных
	if input.Login == "" {
		return nil, fmt.Errorf("login is required")
	}
	if input.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(input.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	// Хэшируем пароль через bcrypt
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Создаём доменную модель пользователя
	user := repository.User{
		ID:           "", // будет сгенерирован в БД
		Login:        input.Login,
		PasswordHash: string(passwordHash),
		TelegramID:   input.TelegramID,
		CreatedAt:    time.Now(),
	}

	// Сохраняем пользователя в репозитории
	if err := s.repo.CreateUser(ctx, user); err != nil {
		if err == repository.ErrAlreadyExists {
			return nil, fmt.Errorf("user with login %s already exists", input.Login)
		}
		s.logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Получаем созданного пользователя для получения ID
	createdUser, err := s.repo.GetByLogin(ctx, input.Login)
	if err != nil {
		s.logger.Error("failed to get created user", zap.Error(err))
		return nil, fmt.Errorf("failed to get created user: %w", err)
	}

	s.logger.Info("user registered successfully",
		zap.String("user_id", createdUser.ID),
		zap.String("login", input.Login),
	)

	return &RegisterOutput{
		UserID: createdUser.ID,
	}, nil
}

// LoginInput содержит входные данные для входа пользователя
type LoginInput struct {
	Login    string
	Password string
}

// LoginOutput содержит результат входа пользователя
type LoginOutput struct {
	UserID    string
	SessionID string
}

// Login аутентифицирует пользователя
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	// Валидация входных данных
	if input.Login == "" {
		return nil, fmt.Errorf("login is required")
	}
	if input.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Получаем пользователя по login
	user, err := s.repo.GetByLogin(ctx, input.Login)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, fmt.Errorf("invalid login or password")
		}
		s.logger.Error("failed to get user by login", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Сравниваем пароль с хэшем
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		s.logger.Warn("invalid password attempt",
			zap.String("login", input.Login),
		)
		return nil, fmt.Errorf("invalid login or password")
	}

	// Создаём сессию в Redis
	sessionID, err := s.sessionRepo.CreateSession(ctx, user.ID, s.sessionTTL)
	if err != nil {
		s.logger.Error("failed to create session",
			zap.Error(err),
			zap.String("user_id", user.ID),
		)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Info("user logged in successfully",
		zap.String("user_id", user.ID),
		zap.String("login", input.Login),
		zap.String("session_id", sessionID),
	)

	return &LoginOutput{
		UserID:    user.ID,
		SessionID: sessionID,
	}, nil
}

// GetUserInput содержит входные данные для получения пользователя
type GetUserInput struct {
	UserID string
}

// GetUserOutput содержит результат получения пользователя
type GetUserOutput struct {
	UserID     string
	Login      string
	TelegramID *string
}

// GetUser получает информацию о пользователе по ID
func (s *Service) GetUser(ctx context.Context, input GetUserInput) (*GetUserOutput, error) {
	// Валидация входных данных
	if input.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Получаем пользователя по ID
	user, err := s.repo.GetByID(ctx, input.UserID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, fmt.Errorf("user not found")
		}
		s.logger.Error("failed to get user by id", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &GetUserOutput{
		UserID:     user.ID,
		Login:      user.Login,
		TelegramID: user.TelegramID,
	}, nil
}

// GetUserContactInput содержит входные данные для получения контакта пользователя
type GetUserContactInput struct {
	UserID string
}

// GetUserContactOutput содержит результат получения контакта пользователя
type GetUserContactOutput struct {
	TelegramID       *string
	PreferredChannel string // на будущее
}

// GetUserContact получает контактную информацию пользователя
func (s *Service) GetUserContact(ctx context.Context, input GetUserContactInput) (*GetUserContactOutput, error) {
	// Валидация входных данных
	if input.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Получаем пользователя по ID
	user, err := s.repo.GetByID(ctx, input.UserID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, fmt.Errorf("user not found")
		}
		s.logger.Error("failed to get user by id", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &GetUserContactOutput{
		TelegramID:       user.TelegramID,
		PreferredChannel: "telegram", // на будущее
	}, nil
}

// ValidateSessionInput содержит входные данные для валидации сессии
type ValidateSessionInput struct {
	SessionID string
}

// ValidateSessionOutput содержит результат валидации сессии
type ValidateSessionOutput struct {
	UserID string
}

// ValidateSession проверяет валидность сессии и возвращает user_id; при успехе продлевает TTL (sliding window)
func (s *Service) ValidateSession(ctx context.Context, input ValidateSessionInput) (*ValidateSessionOutput, error) {
	if input.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	userID, err := s.sessionRepo.GetUserIDBySession(ctx, input.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionNotFoundOrExpired
		}
		s.logger.Error("failed to validate session",
			zap.Error(err),
			zap.String("session_id", input.SessionID),
		)
		return nil, fmt.Errorf("failed to validate session: %w", err)
	}

	// Sliding TTL: продлеваем сессию на cfg.SessionTTL при каждом успешном ValidateSession
	if err := s.sessionRepo.RefreshSession(ctx, input.SessionID, s.sessionTTL); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionNotFoundOrExpired
		}
		s.logger.Error("failed to refresh session TTL",
			zap.Error(err),
			zap.String("session_id", input.SessionID),
		)
		return nil, fmt.Errorf("failed to refresh session: %w", err)
	}

	return &ValidateSessionOutput{
		UserID: userID,
	}, nil
}
