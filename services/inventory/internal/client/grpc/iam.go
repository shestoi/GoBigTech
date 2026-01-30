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
	// ValidateSession проверяет валидность сессии и возвращает user_id
	ValidateSession(ctx context.Context, sessionID string) (userID string, err error)
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

// ValidateSession реализует IAMClient интерфейс
func (a *IAMClientAdapter) ValidateSession(ctx context.Context, sessionID string) (string, error) {
	req := &iampb.ValidateSessionRequest{
		SessionId: sessionID,
	}

	resp, err := a.client.ValidateSession(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetUserId(), nil
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
