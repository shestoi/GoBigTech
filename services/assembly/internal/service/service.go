package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

// ErrEventIDRequired возвращается когда event_id отсутствует в событии
var ErrEventIDRequired = errors.New("event_id is required")

// Service содержит бизнес-логику обработки событий оплаты заказа
type Service struct {
	logger         *zap.Logger
	publisher      AssemblyEventPublisher
	store          ProcessedEventsStore
	sleeper        Sleeper
	idempotencyTTL time.Duration // TTL для idempotency store
}

// NewService создаёт новый экземпляр Service
func NewService(logger *zap.Logger, publisher AssemblyEventPublisher, store ProcessedEventsStore, idempotencyTTL time.Duration) *Service {
	return &Service{
		logger:         logger,
		publisher:      publisher,
		store:          store,
		sleeper:        &DefaultSleeper{},
		idempotencyTTL: idempotencyTTL,
	}
}

// NewServiceWithSleeper создаёт новый экземпляр Service с кастомным sleeper (для тестов)
func NewServiceWithSleeper(logger *zap.Logger, publisher AssemblyEventPublisher, store ProcessedEventsStore, sleeper Sleeper, idempotencyTTL time.Duration) *Service {
	return &Service{
		logger:         logger,
		publisher:      publisher,
		store:          store,
		sleeper:        sleeper,
		idempotencyTTL: idempotencyTTL,
	}
}

// HandleOrderPaid обрабатывает событие успешной оплаты заказа
// Имитирует сборку заказа (ждёт 10 секунд) и публикует событие завершения сборки
// Обеспечивает idempotency: если событие с тем же event_id уже обработано, не выполняет side-effect повторно
func (s *Service) HandleOrderPaid(ctx context.Context, event OrderPaidEvent) error {
	// Проверяем, что event_id присутствует (обязательное поле для idempotency)
	if event.EventID == "" {
		s.logger.Error("event_id is required for idempotency",
			zap.String("order_id", event.OrderID),
		)
		return ErrEventIDRequired
	}

	s.logger.Info("handling order paid event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int64("amount", event.Amount),
	)

	// Проверяем, не было ли это событие уже обработано
	processed, err := s.store.IsProcessed(ctx, event.EventID)
	if err != nil {
		s.logger.Error("failed to check if event is processed",
			zap.Error(err),
			zap.String("event_id", event.EventID),
		)
		return err
	}

	if processed {
		s.logger.Info("event already processed, skipping",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		// Возвращаем nil - событие уже обработано, side-effect не выполняем
		return nil
	}

	// Имитация сборки заказа - ждём 10 секунд
	s.logger.Info("assembling order", zap.String("order_id", event.OrderID))
	if err := s.sleeper.Sleep(ctx, 10*time.Second); err != nil {
		return err
	}

	s.logger.Info("order assembly completed", zap.String("order_id", event.OrderID))

	// Формируем событие завершения сборки
	assemblyEvent := OrderAssemblyCompletedEvent{
		EventID:      "", // будет сгенерирован в publisher
		EventType:    "order.assembly.completed",
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		OrderID:      event.OrderID,
		UserID:       event.UserID,
	}

	// Публикуем событие (side-effect)
	if err := s.publisher.PublishOrderAssemblyCompleted(ctx, assemblyEvent); err != nil {
		s.logger.Error("failed to publish assembly completed event",
			zap.Error(err),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// После успешной публикации помечаем событие как обработанное
	// Если MarkProcessed вернёт ошибку, это вызовет retry, но событие уже могло быть отправлено
	// В production с durable store это не проблема, но сейчас это допустимо
	if err := s.store.MarkProcessed(ctx, event.EventID, s.idempotencyTTL); err != nil {
		s.logger.Error("failed to mark event as processed",
			zap.Error(err),
			zap.String("event_id", event.EventID),
		)
		return err
	}

	s.logger.Info("order assembly event published successfully",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
	)

	return nil
}
