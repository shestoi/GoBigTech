package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcclient "github.com/shestoi/GoBigTech/services/notification/internal/client/grpc"
	"github.com/shestoi/GoBigTech/services/notification/internal/repository"
	"github.com/shestoi/GoBigTech/services/notification/internal/telegram"
	"github.com/shestoi/GoBigTech/services/notification/internal/templates"
)

// NotificationService содержит бизнес-логику обработки уведомлений
type NotificationService struct {
	logger    *zap.Logger
	repo      repository.NotificationRepository
	sender    telegram.Sender
	renderer  *templates.Renderer
	iamClient grpcclient.IAMClient
}

// NewNotificationService создаёт новый экземпляр NotificationService
func NewNotificationService(
	logger *zap.Logger,
	repo repository.NotificationRepository,
	sender telegram.Sender,
	renderer *templates.Renderer,
	iamClient grpcclient.IAMClient,
) *NotificationService {
	return &NotificationService{
		logger:    logger,
		repo:      repo,
		sender:    sender,
		renderer:  renderer,
		iamClient: iamClient,
	}
}

// HandleOrderPaid обрабатывает событие успешной оплаты заказа.
// Идемпотентность через inbox со статусом pending/sent: retry не считает событие duplicate пока не sent.
func (s *NotificationService) HandleOrderPaid(ctx context.Context, event OrderPaidEvent, topic string, partition int, offset int64) error {
	s.logger.Info("handling order paid event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int64("amount", event.Amount),
	)

	res, err := s.repo.UpsertInboxPending(ctx, event.EventID, event.EventType, event.OccurredAt, event.OrderID, topic, partition, offset)
	if err != nil {
		s.logger.Error("failed to upsert inbox event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}
	if res.AlreadyProcessed {
		s.logger.Info("event already processed (sent)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return nil
	}
	if !res.CanProcess {
		return nil
	}

	telegramID, preferredChannel, err := s.iamClient.GetUserContact(ctx, event.UserID)
	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if ok && grpcStatus.Code() == codes.NotFound {
			s.logger.Warn("user not found in IAM, marking as sent (no notification)",
				zap.String("event_id", event.EventID),
				zap.String("order_id", event.OrderID),
				zap.String("user_id", event.UserID),
			)
			_ = s.repo.MarkInboxSent(ctx, event.EventID)
			return nil
		}
		s.logger.Error("failed to get user contact from IAM (transient), will retry",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return fmt.Errorf("failed to get user contact: %w", err)
	}

	if telegramID == nil || *telegramID == "" {
		s.logger.Info("user has no telegram_id, marking as sent (no notification)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("preferred_channel", preferredChannel),
		)
		_ = s.repo.MarkInboxSent(ctx, event.EventID)
		return nil
	}

	if preferredChannel != "telegram" {
		s.logger.Info("user preferred_channel is not telegram, using telegram fallback",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("preferred_channel", preferredChannel),
		)
	}

	text, err := s.renderer.RenderPaymentCompleted(event)
	if err != nil {
		s.logger.Error("failed to render payment template",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return err
	}

	if err := s.sender.Send(ctx, *telegramID, text); err != nil {
		s.logger.Error("failed to send telegram notification, will retry",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("telegram_id", *telegramID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return err
	}

	_ = s.repo.MarkInboxSent(ctx, event.EventID)
	s.logger.Info("notification sent for order paid",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.String("telegram_id", *telegramID),
	)
	return nil
}

// HandleOrderAssemblyCompleted обрабатывает событие завершения сборки заказа.
// Идемпотентность через inbox со статусом pending/sent: retry не считает событие duplicate пока не sent.
func (s *NotificationService) HandleOrderAssemblyCompleted(ctx context.Context, event OrderAssemblyCompletedEvent, topic string, partition int, offset int64) error {
	s.logger.Info("handling order assembly completed event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	res, err := s.repo.UpsertInboxPending(ctx, event.EventID, event.EventType, event.OccurredAt, event.OrderID, topic, partition, offset)
	if err != nil {
		s.logger.Error("failed to upsert inbox event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}
	if res.AlreadyProcessed {
		s.logger.Info("event already processed (sent)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return nil
	}
	if !res.CanProcess {
		return nil
	}

	telegramID, preferredChannel, err := s.iamClient.GetUserContact(ctx, event.UserID)
	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if ok && grpcStatus.Code() == codes.NotFound {
			s.logger.Warn("user not found in IAM, marking as sent (no notification)",
				zap.String("event_id", event.EventID),
				zap.String("order_id", event.OrderID),
				zap.String("user_id", event.UserID),
			)
			_ = s.repo.MarkInboxSent(ctx, event.EventID)
			return nil
		}
		s.logger.Error("failed to get user contact from IAM (transient), will retry",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return fmt.Errorf("failed to get user contact: %w", err)
	}

	if telegramID == nil || *telegramID == "" {
		s.logger.Info("user has no telegram_id, marking as sent (no notification)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("preferred_channel", preferredChannel),
		)
		_ = s.repo.MarkInboxSent(ctx, event.EventID)
		return nil
	}

	if preferredChannel != "telegram" {
		s.logger.Info("user preferred_channel is not telegram, using telegram fallback",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("preferred_channel", preferredChannel),
		)
	}

	text, err := s.renderer.RenderAssemblyCompleted(event)
	if err != nil {
		s.logger.Error("failed to render assembly template",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return err
	}

	if err := s.sender.Send(ctx, *telegramID, text); err != nil {
		s.logger.Error("failed to send telegram notification, will retry",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
			zap.String("telegram_id", *telegramID),
		)
		_ = s.repo.MarkInboxFailed(ctx, event.EventID, err.Error())
		return err
	}

	_ = s.repo.MarkInboxSent(ctx, event.EventID)
	s.logger.Info("notification sent for order assembly completed",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.String("telegram_id", *telegramID),
	)
	return nil
}
