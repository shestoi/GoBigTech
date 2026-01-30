package grpcapi

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/shestoi/GoBigTech/services/iam/internal/service"
	iampb "github.com/shestoi/GoBigTech/services/iam/v1"
)

// Handler содержит gRPC-обработчики для IAM Service
// Зависит от service слоя, но не знает о деталях реализации (repository, БД и т.д.)
type Handler struct {
	iampb.UnimplementedIAMServiceServer
	iamService *service.Service
	logger     *zap.Logger
}

// NewHandler создаёт новый gRPC handler
func NewHandler(iamService *service.Service, logger *zap.Logger) *Handler {
	return &Handler{
		iamService: iamService,
		logger:     logger,
	}
}

// Register обрабатывает gRPC запрос Register
func (h *Handler) Register(ctx context.Context, req *iampb.RegisterRequest) (*iampb.RegisterResponse, error) {
	// Валидация входных данных
	if req.GetLogin() == "" {
		return nil, status.Error(codes.InvalidArgument, "login is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	// Вызываем service слой
	var telegramID *string
	if req.TelegramId != nil {
		tgID := req.GetTelegramId()
		telegramID = &tgID
	}

	result, err := h.iamService.Register(ctx, service.RegisterInput{
		Login:      req.GetLogin(),
		Password:   req.GetPassword(),
		TelegramID: telegramID,
	})

	if err != nil {
		// Маппим ошибки в gRPC status
		if err.Error() == "user with login "+req.GetLogin()+" already exists" {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		if err.Error() == "login is required" || err.Error() == "password is required" || err.Error() == "password must be at least 6 characters" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.logger.Error("failed to register user", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &iampb.RegisterResponse{
		UserId: result.UserID,
	}, nil
}

// Login обрабатывает gRPC запрос Login
func (h *Handler) Login(ctx context.Context, req *iampb.LoginRequest) (*iampb.LoginResponse, error) {
	// Валидация входных данных
	if req.GetLogin() == "" {
		return nil, status.Error(codes.InvalidArgument, "login is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	// Вызываем service слой
	result, err := h.iamService.Login(ctx, service.LoginInput{
		Login:    req.GetLogin(),
		Password: req.GetPassword(),
	})

	if err != nil {
		// Маппим ошибки в gRPC status
		if err.Error() == "invalid login or password" {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if err.Error() == "login is required" || err.Error() == "password is required" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.logger.Error("failed to login user", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &iampb.LoginResponse{
		UserId:    result.UserID,
		SessionId: result.SessionID,
	}, nil
}

// GetUser обрабатывает gRPC запрос GetUser
func (h *Handler) GetUser(ctx context.Context, req *iampb.GetUserRequest) (*iampb.GetUserResponse, error) {
	// Валидация входных данных
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Вызываем service слой
	result, err := h.iamService.GetUser(ctx, service.GetUserInput{
		UserID: req.GetUserId(),
	})

	if err != nil {
		// Маппим ошибки в gRPC status
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err.Error() == "user_id is required" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.logger.Error("failed to get user", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	response := &iampb.GetUserResponse{
		UserId: result.UserID,
		Login:  result.Login,
	}
	if result.TelegramID != nil {
		response.TelegramId = result.TelegramID
	}

	return response, nil
}

// GetUserContact обрабатывает gRPC запрос GetUserContact
func (h *Handler) GetUserContact(ctx context.Context, req *iampb.GetUserContactRequest) (*iampb.GetUserContactResponse, error) {
	// Валидация входных данных
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Вызываем service слой
	result, err := h.iamService.GetUserContact(ctx, service.GetUserContactInput{
		UserID: req.GetUserId(),
	})

	if err != nil {
		// Маппим ошибки в gRPC status
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err.Error() == "user_id is required" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.logger.Error("failed to get user contact", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	response := &iampb.GetUserContactResponse{
		PreferredChannel: result.PreferredChannel,
	}
	if result.TelegramID != nil {
		response.TelegramId = result.TelegramID
	}

	return response, nil
}

// ValidateSession обрабатывает gRPC запрос ValidateSession
func (h *Handler) ValidateSession(ctx context.Context, req *iampb.ValidateSessionRequest) (*iampb.ValidateSessionResponse, error) {
	// Валидация входных данных
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Вызываем service слой
	result, err := h.iamService.ValidateSession(ctx, service.ValidateSessionInput{
		SessionID: req.GetSessionId(),
	})

	if err != nil {
		if errors.Is(err, service.ErrSessionNotFoundOrExpired) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if err.Error() == "session_id is required" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.logger.Error("failed to validate session", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &iampb.ValidateSessionResponse{
		UserId: result.UserID,
	}, nil
}
