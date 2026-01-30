package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/service"
)

// OrderAssemblyCompletedConsumer обрабатывает события завершения сборки заказа из Kafka
type OrderAssemblyCompletedConsumer struct {
	logger      *zap.Logger
	reader      *kafka.Reader
	service     *service.OrderService
	maxAttempts int
	backoffBase time.Duration
}

// NewOrderAssemblyCompletedConsumer создаёт новый consumer для событий завершения сборки заказа
func NewOrderAssemblyCompletedConsumer(
	logger *zap.Logger,
	brokers []string,
	groupID, topic string,
	svc *service.OrderService,
	maxAttempts int,
	backoffBase time.Duration,
) *OrderAssemblyCompletedConsumer {

	// ✅ Safety defaults (на случай кривого env/config)
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if backoffBase <= 0 {
		backoffBase = 1 * time.Second
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
	})

	return &OrderAssemblyCompletedConsumer{
		logger:      logger,
		reader:      reader,
		service:     svc,
		maxAttempts: maxAttempts,
		backoffBase: backoffBase,
	}
}

// Start запускает consumer и начинает обработку сообщений
// Использует at-least-once семантику: FetchMessage + CommitMessages после успешной обработки
func (c *OrderAssemblyCompletedConsumer) Start(ctx context.Context) error {

	if c.maxAttempts <= 0 {
		c.maxAttempts = 3
	}
	if c.backoffBase <= 0 {
		c.backoffBase = 1 * time.Second
	}

	c.logger.Info("starting kafka consumer",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
		zap.Int("max_retry_attempts", c.maxAttempts),
		zap.Duration("retry_backoff_base", c.backoffBase),
	)

	for {
		// FetchMessage вместо ReadMessage для ручного контроля commit
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			// Если контекст отменён, выходим
			if ctx.Err() != nil {
				c.logger.Info("consumer context cancelled, stopping")
				return nil
			}
			c.logger.Error("failed to fetch message from kafka",
				zap.Error(err),
			)
			// Продолжаем обработку, не паникуем
			continue
		}

		// Обрабатываем сообщение
		shouldCommit := c.processMessage(ctx, m)

		// Коммитим offset только после успешной обработки
		if shouldCommit {
			if err := c.reader.CommitMessages(ctx, m); err != nil {
				c.logger.Error("failed to commit message offset",
					zap.Error(err),
					zap.String("topic", m.Topic),
					zap.Int("partition", m.Partition),
					zap.Int64("offset", m.Offset),
				)
				// Продолжаем обработку следующего сообщения
				continue
			}

			c.logger.Debug("message offset committed",
				zap.String("topic", m.Topic),
				zap.Int("partition", m.Partition),
				zap.Int64("offset", m.Offset),
			)
		}
	}
}

// processMessage обрабатывает одно сообщение из Kafka
// Возвращает true, если нужно закоммитить offset (успешная обработка)
func (c *OrderAssemblyCompletedConsumer) processMessage(ctx context.Context, m kafka.Message) bool {
	// Парсим JSON сообщение
	var payload map[string]interface{}
	if err := json.Unmarshal(m.Value, &payload); err != nil {
		c.logger.Error("failed to unmarshal kafka message",
			zap.Error(err),
			zap.String("topic", m.Topic),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)
		// Коммитим poison pill, чтобы не зациклиться
		return true
	}

	// Преобразуем payload в OrderAssemblyCompletedEvent
	event, err := c.parseOrderAssemblyCompletedEvent(payload)
	if err != nil {
		c.logger.Error("failed to parse order assembly completed event",
			zap.Error(err),
			zap.String("topic", m.Topic),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)
		// Коммитим poison pill
		return true
	}

	c.logger.Info("received order assembly completed event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int("partition", m.Partition),
		zap.Int64("offset", m.Offset),
	)

	// Пытаемся обработать событие с retry
	success := c.handleWithRetry(ctx, m, event)

	if !success {
		// После исчерпания retry не коммитим (Kafka повторит)
		c.logger.Error("failed to handle order assembly completed event after all retries",
			zap.String("order_id", event.OrderID),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)
		return false
	}

	c.logger.Info("order assembly completed event processed successfully",
		zap.String("order_id", event.OrderID),
		zap.Int("partition", m.Partition),
		zap.Int64("offset", m.Offset),
	)

	return true // Коммитим после успешной обработки
}

// handleWithRetry обрабатывает событие с retry логикой
// Возвращает true при успешной обработке, false при исчерпании попыток
func (c *OrderAssemblyCompletedConsumer) handleWithRetry(ctx context.Context, m kafka.Message, event service.OrderAssemblyCompletedEvent) bool {
	var lastErr error

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		// Вычисляем backoff: 1s, 2s, 4s (экспоненциально)
		if attempt > 1 {
			backoff := c.backoffBase * time.Duration(1<<uint(attempt-2))
			c.logger.Info("retrying order assembly completed event",
				zap.String("order_id", event.OrderID),
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", c.maxAttempts),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return false
			case <-time.After(backoff):
				// Продолжаем retry
			}
		}

		// Пытаемся обработать событие
		err := c.service.HandleOrderAssemblyCompleted(ctx, event)
		if err == nil {
			if attempt > 1 {
				c.logger.Info("order assembly completed event processed successfully after retry",
					zap.String("order_id", event.OrderID),
					zap.Int("attempt", attempt),
				)
			}
			return true
		}

		lastErr = err
		c.logger.Warn("failed to handle order assembly completed event",
			zap.Error(err),
			zap.String("order_id", event.OrderID),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", c.maxAttempts),
		)
	}

	c.logger.Error("exhausted all retry attempts",
		zap.Error(lastErr),
		zap.String("order_id", event.OrderID),
		zap.Int("max_attempts", c.maxAttempts),
	)

	return false
}

// parseOrderAssemblyCompletedEvent преобразует payload в OrderAssemblyCompletedEvent
func (c *OrderAssemblyCompletedConsumer) parseOrderAssemblyCompletedEvent(payload map[string]interface{}) (service.OrderAssemblyCompletedEvent, error) {
	event := service.OrderAssemblyCompletedEvent{}

	// Извлекаем поля из payload
	if v, ok := payload["event_id"].(string); ok {
		event.EventID = v
	}
	if v, ok := payload["event_type"].(string); ok {
		event.EventType = v
	}
	if v, ok := payload["event_version"].(float64); ok {
		event.EventVersion = int(v)
	}
	if v, ok := payload["occurred_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			event.OccurredAt = t
		}
	}
	if v, ok := payload["order_id"].(string); ok {
		event.OrderID = v
	} else {
		return event, &ParseError{Field: "order_id", Message: "order_id is required"}
	}
	if v, ok := payload["user_id"].(string); ok {
		event.UserID = v
	}

	return event, nil
}

// ParseError представляет ошибку парсинга события
type ParseError struct {
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

// Close закрывает Kafka reader
func (c *OrderAssemblyCompletedConsumer) Close() error {
	c.logger.Info("closing kafka consumer")
	return c.reader.Close()
}
