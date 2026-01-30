package grpcclient

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	iampb "github.com/shestoi/GoBigTech/services/iam/v1"
)

// IAMClient определяет интерфейс для работы с IAM Service
type IAMClient interface {
	// GetUserContact получает контактную информацию пользователя
	GetUserContact(ctx context.Context, userID string) (telegramID *string, preferredChannel string, err error)
}

// IAMClientAdapter адаптирует gRPC клиент к интерфейсу IAMClient
type IAMClientAdapter struct {
	client iampb.IAMServiceClient
	logger *zap.Logger
}

// NewIAMClientAdapter создаёт новый адаптер для IAM клиента
func NewIAMClientAdapter(client iampb.IAMServiceClient, logger *zap.Logger) IAMClient {
	return &IAMClientAdapter{
		client: client,
		logger: logger,
	}
}

// GetUserContact реализует IAMClient интерфейс
func (a *IAMClientAdapter) GetUserContact(ctx context.Context, userID string) (*string, string, error) {
	req := &iampb.GetUserContactRequest{
		UserId: userID,
	}

	resp, err := a.client.GetUserContact(ctx, req)
	if err != nil {
		return nil, "", err
	}

	var telegramID *string
	if resp.TelegramId != nil && *resp.TelegramId != "" {
		telegramID = resp.TelegramId
	}

	return telegramID, resp.GetPreferredChannel(), nil
}

// NewIAMGRPCClient создаёт новый gRPC клиент для IAM Service
func NewIAMGRPCClient(addr string, logger *zap.Logger) (iampb.IAMServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	client := iampb.NewIAMServiceClient(conn)
	return client, conn, nil
}
