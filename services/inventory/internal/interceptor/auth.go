package interceptor

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	iamclient "github.com/shestoi/GoBigTech/services/inventory/internal/client/grpc"
)

const (
	// SessionIDHeader ключ для передачи session_id в gRPC metadata
	SessionIDHeader = "x-session-id"
)

// ctxKeyUserID типизированный ключ для хранения user_id в context
type ctxKeyUserID struct{}

var userIDKey = ctxKeyUserID{}

// UserIDFromContext извлекает user_id из context
// Возвращает user_id и true, если значение найдено, иначе пустую строку и false
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

// AuthInterceptor проверяет сессию через IAM Service
type AuthInterceptor struct {
	iamClient iamclient.IAMClient
	logger    *zap.Logger
}

// NewAuthInterceptor создаёт новый auth interceptor
func NewAuthInterceptor(iamClient iamclient.IAMClient, logger *zap.Logger) *AuthInterceptor {
	return &AuthInterceptor{
		iamClient: iamClient,
		logger:    logger,
	}
}

// Unary возвращает unary interceptor для проверки аутентификации
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Пропускаем health check и reflection без проверки сессии
		if a.isPublicMethod(info.FullMethod) { // если метод публичный, пропускаем проверку сессии
			return handler(ctx, req) // вызываем следующий handler
		}

		// Извлекаем metadata из контекста
		md, ok := metadata.FromIncomingContext(ctx) // md - metadata, ok - флаг, который показывает, есть ли metadata в контексте
		if !ok {                                    // если metadata нет в контексте, возвращаем ошибку
			a.logger.Warn("no metadata in context",
				zap.String("method", info.FullMethod),
			)
			return nil, status.Error(codes.Unauthenticated, "session_id is required")
		}

		// Получаем session_id из metadata
		sessionIDs := md.Get(SessionIDHeader)
		if len(sessionIDs) == 0 || sessionIDs[0] == "" {
			a.logger.Warn("session_id not found in metadata",
				zap.String("method", info.FullMethod),
			)
			return nil, status.Error(codes.Unauthenticated, "session_id is required")
		}

		sessionID := sessionIDs[0]

		// Валидируем сессию через IAM Service
		userID, err := a.iamClient.ValidateSession(ctx, sessionID)
		if err != nil {
			a.logger.Warn("session validation failed",
				zap.Error(err),
				zap.String("session_id", sessionID),
				zap.String("method", info.FullMethod),
			)
			return nil, status.Error(codes.Unauthenticated, "invalid or expired session")
		}

		// Добавляем user_id в контекст для использования в handlers
		ctx = context.WithValue(ctx, userIDKey, userID)

		a.logger.Debug("session validated",
			zap.String("user_id", userID),
			zap.String("method", info.FullMethod),
		)

		// Вызываем следующий handler
		return handler(ctx, req)
	}
}

// isPublicMethod проверяет, является ли метод публичным (не требует аутентификации)
func (a *AuthInterceptor) isPublicMethod(fullMethod string) bool {
	// Health check методы
	if fullMethod == "/grpc.health.v1.Health/Check" ||
		fullMethod == "/grpc.health.v1.Health/Watch" {
		return true
	}

	// Reflection методы (начинаются с /grpc.reflection)
	if len(fullMethod) >= 18 && fullMethod[:18] == "/grpc.reflection" {
		return true
	}

	return false
}
