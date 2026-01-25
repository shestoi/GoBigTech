package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/notification/internal/repository"
	"github.com/shestoi/GoBigTech/services/notification/internal/telegram"
	"github.com/shestoi/GoBigTech/services/notification/internal/templates"
)

// NotificationService содержит бизнес-логику обработки уведомлений
type NotificationService struct {
	logger   *zap.Logger
	repo     repository.NotificationRepository
	sender   telegram.Sender
	renderer *templates.Renderer
	chatID   string
}

// NewNotificationService создаёт новый экземпляр NotificationService
func NewNotificationService(
	logger *zap.Logger,
	repo repository.NotificationRepository,
	sender telegram.Sender,
	renderer *templates.Renderer,
	chatID string,
) *NotificationService {
	return &NotificationService{
		logger:   logger,
		repo:     repo,
		sender:   sender,
		renderer: renderer,
		chatID:   chatID,
	}
}

// HandleOrderPaid обрабатывает событие успешной оплаты заказа
// Обеспечивает idempotency через inbox таблицу
func (s *NotificationService) HandleOrderPaid(ctx context.Context, event OrderPaidEvent, topic string, partition int, offset int64) error {
	s.logger.Info("handling order paid event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int64("amount", event.Amount),
	)

	// Вставляем событие в inbox (idempotency)
	inserted, err := s.repo.InsertInboxEventTx(ctx, event.EventID, event.EventType, event.OccurredAt, event.OrderID, topic, partition, offset)
	if err != nil {
		s.logger.Error("failed to insert inbox event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// Если событие уже обработано (duplicate), просто возвращаем nil
	if !inserted {
		s.logger.Info("event already processed (duplicate)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return nil
	}

	// Событие впервые обработано - рендерим шаблон и отправляем уведомление
	text, err := s.renderer.RenderPaymentCompleted(event)
	if err != nil {
		s.logger.Error("failed to render payment template",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	if err := s.sender.Send(ctx, s.chatID, text); err != nil {
		s.logger.Error("failed to send telegram notification",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	s.logger.Info("notification sent for order paid",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	return nil
}

// HandleOrderAssemblyCompleted обрабатывает событие завершения сборки заказа
// Обеспечивает idempotency через inbox таблицу
func (s *NotificationService) HandleOrderAssemblyCompleted(ctx context.Context, event OrderAssemblyCompletedEvent, topic string, partition int, offset int64) error {
	s.logger.Info("handling order assembly completed event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	// Вставляем событие в inbox (idempotency)
	inserted, err := s.repo.InsertInboxEventTx(ctx, event.EventID, event.EventType, event.OccurredAt, event.OrderID, topic, partition, offset)
	if err != nil {
		s.logger.Error("failed to insert inbox event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// Если событие уже обработано (duplicate), просто возвращаем nil
	if !inserted {
		s.logger.Info("event already processed (duplicate)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return nil
	}

	// Событие впервые обработано - рендерим шаблон и отправляем уведомление
	text, err := s.renderer.RenderAssemblyCompleted(event)
	if err != nil {
		s.logger.Error("failed to render assembly template",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	if err := s.sender.Send(ctx, s.chatID, text); err != nil {
		s.logger.Error("failed to send telegram notification",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	s.logger.Info("notification sent for order assembly completed",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	return nil
}
