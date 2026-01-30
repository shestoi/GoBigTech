package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

// OutboxDispatcher обрабатывает события из outbox таблицы и публикует их в Kafka
type OutboxDispatcher struct {
	logger     *zap.Logger
	repo       repository.OrderRepository
	writer     *kafka.Writer
	batchSize  int
	interval   time.Duration
	maxRetries int
	backoff    time.Duration
}

// NewOutboxDispatcher создаёт новый outbox dispatcher
func NewOutboxDispatcher(
	logger *zap.Logger,
	repo repository.OrderRepository,
	brokers []string,
	batchSize int, //batchSize - количество событий, которые будут обработаны за один раз
	interval time.Duration, //interval - интервал между обработками
	maxRetries int, //maxRetries - максимальное количество попыток обработки события
	backoff time.Duration, //backoff - интервал между попытками обработки события
) *OutboxDispatcher {
	writer := &kafka.Writer{
		//writer - writer для записи событий в Kafka
		Addr:     kafka.TCP(brokers...),
		Balancer: &kafka.LeastBytes{},
	}

	return &OutboxDispatcher{
		logger:     logger,
		repo:       repo,
		writer:     writer,
		batchSize:  batchSize,
		interval:   interval,
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

// Start запускает dispatcher в фоновом режиме
func (d *OutboxDispatcher) Start(ctx context.Context) error {
	d.logger.Info("starting outbox dispatcher",
		zap.Int("batch_size", d.batchSize),
		zap.Duration("interval", d.interval),
		zap.Int("max_retries", d.maxRetries),
	)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	// Обрабатываем сразу при старте dispatcher
	if err := d.processBatch(ctx); err != nil {
		d.logger.Error("failed to process initial batch", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("outbox dispatcher context cancelled, stopping")
			return nil
		case <-ticker.C: //ticker.C - канал, который отправляет сигнал через интервал
			if err := d.processBatch(ctx); err != nil {
				d.logger.Error("failed to process batch", zap.Error(err))
			}
		}
	}
}

// processBatch обрабатывает батч pending событий
func (d *OutboxDispatcher) processBatch(ctx context.Context) error {
	// Проверяем контекст перед запросом к БД, если контекст отменён, возвращаем ошибку
	if ctx.Err() != nil {
		return ctx.Err()
	}

	events, err := d.repo.GetPendingOutboxEvents(ctx, d.batchSize) //d.batchSize - количество событий, которые будут обработаны за один раз
	if err != nil {
		// Если контекст отменён, не логируем как ошибку
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to get pending events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	d.logger.Debug("processing outbox batch",
		zap.Int("count", len(events)),
	)

	for _, event := range events {
		// Проверяем контекст перед обработкой каждого события
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := d.processEvent(ctx, event); err != nil { //processEvent - функция для обработки события
			// Если контекст отменён, прекращаем обработку
			if ctx.Err() != nil {
				return ctx.Err()
			}
			d.logger.Error("failed to process event",
				zap.Error(err),
				zap.String("event_id", event.EventID),
				zap.String("topic", event.Topic),
			)
			// Продолжаем обработку следующих событий
		}
	}

	return nil
}

// processEvent обрабатывает одно событие с retry
func (d *OutboxDispatcher) processEvent(ctx context.Context, event repository.OutboxEvent) error {
	var lastErr error

	for attempt := 1; attempt <= d.maxRetries; attempt++ {
		// Публикуем в Kafka
		msg := kafka.Message{
			Topic: event.Topic,               // topic из outbox таблицы
			Key:   []byte(event.AggregateID), // order_id как key
			Value: event.Payload,
		}

		err := d.writer.WriteMessages(ctx, msg)
		if err == nil {
			// Проверяем контекст перед записью в БД
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Успешно опубликовано - отмечаем как sent
			if markErr := d.repo.MarkOutboxEventSent(ctx, event.EventID); markErr != nil {
				// Если контекст отменён, не логируем как ошибку
				if ctx.Err() != nil {
					return ctx.Err()
				}
				d.logger.Error("failed to mark event as sent",
					zap.Error(markErr),
					zap.String("event_id", event.EventID),
				)
				return markErr
			}

			d.logger.Info("outbox event published successfully",
				zap.String("event_id", event.EventID),
				zap.String("topic", event.Topic),
				zap.String("aggregate_id", event.AggregateID),
				zap.Int("attempt", attempt),
			)
			return nil
		}

		lastErr = err
		d.logger.Warn("failed to publish outbox event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("topic", event.Topic),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", d.maxRetries),
		)

		// Backoff перед следующей попыткой
		if attempt < d.maxRetries {
			backoff := d.backoff * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Продолжаем retry
			}
		}
	}

	// Все попытки исчерпаны - отмечаем как failed
	// Проверяем контекст перед записью в БД
	if ctx.Err() != nil {
		return ctx.Err()
	}

	errMsg := fmt.Sprintf("failed after %d attempts: %v", d.maxRetries, lastErr)
	if markErr := d.repo.MarkOutboxEventFailed(ctx, event.EventID, errMsg); markErr != nil {
		// Если контекст отменён, не логируем как ошибку
		if ctx.Err() != nil {
			return ctx.Err()
		}
		d.logger.Error("failed to mark event as failed",
			zap.Error(markErr),
			zap.String("event_id", event.EventID),
		)
		return markErr
	}

	// Сбрасываем на pending для следующего цикла (retry на уровне dispatcher)
	if resetErr := d.repo.ResetOutboxEventPending(ctx, event.EventID); resetErr != nil {
		// Если контекст отменён, не логируем как ошибку
		if ctx.Err() != nil {
			return ctx.Err()
		}
		d.logger.Error("failed to reset event to pending",
			zap.Error(resetErr),
			zap.String("event_id", event.EventID),
		)
	}

	return fmt.Errorf("failed to publish event after %d attempts: %w", d.maxRetries, lastErr)
}

// Close закрывает Kafka writer
func (d *OutboxDispatcher) Close() error {
	d.logger.Info("closing outbox dispatcher")
	return d.writer.Close()
}
