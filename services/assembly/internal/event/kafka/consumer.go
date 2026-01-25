package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/assembly/internal/service"
)

// OrderPaidConsumer обрабатывает события оплаты заказа из Kafka
type OrderPaidConsumer struct {
	logger       *zap.Logger
	reader       *kafka.Reader
	service      *service.Service
	dlqPublisher *DLQPublisher
	maxAttempts  int
	backoffBase  time.Duration
}

// NewOrderPaidConsumer создаёт новый consumer для событий оплаты заказа
func NewOrderPaidConsumer(
	logger *zap.Logger,
	brokers []string,
	groupID, topic string,
	svc *service.Service,
	dlqPublisher *DLQPublisher,
	maxAttempts int,
	backoffBase time.Duration,
) *OrderPaidConsumer {
	// Safety defaults (на случай кривого env/config)
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

	return &OrderPaidConsumer{
		logger:       logger,
		reader:       reader,
		service:      svc,
		dlqPublisher: dlqPublisher,
		maxAttempts:  maxAttempts,
		backoffBase:  backoffBase,
	}
}

// Start запускает consumer и начинает обработку сообщений
// Использует at-least-once семантику: FetchMessage + CommitMessages после успешной обработки
func (c *OrderPaidConsumer) Start(ctx context.Context) error {
	c.logger.Info("starting kafka consumer",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
		zap.Int("max_retry_attempts", c.maxAttempts),
	)

	for { //бесконечный цикл для чтения сообщений из Kafka
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
		shouldCommit := c.processMessage(ctx, m) //true, если нужно закоммитить offset (успешная обработка или отправка в DLQ)

		// Коммитим offset только после успешной обработки или отправки в DLQ
		if shouldCommit {
			if err := c.reader.CommitMessages(ctx, m); err != nil {
				c.logger.Error("failed to commit message offset",
					zap.Error(err),
					zap.String("topic", m.Topic),
					zap.Int("partition", m.Partition),
					zap.Int64("offset", m.Offset),
				)
				// Продолжаем обработку следующего сообщения
				// В production можно добавить retry для commit
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
// Возвращает true, если нужно закоммитить offset (успешная обработка или отправка в DLQ)
func (c *OrderPaidConsumer) processMessage(ctx context.Context, m kafka.Message) bool {
	// Парсим JSON сообщение
	var payload map[string]interface{}
	if err := json.Unmarshal(m.Value, &payload); err != nil {
		c.logger.Error("failed to unmarshal kafka message - sending to DLQ",
			zap.Error(err),
			zap.String("topic", m.Topic),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)

		// Отправляем в DLQ и коммитим (poison pill)
		if err := c.dlqPublisher.Publish(ctx, m, err, "", "", ""); err != nil {
			c.logger.Error("failed to send message to DLQ",
				zap.Error(err),
				zap.String("topic", m.Topic),
				zap.Int("partition", m.Partition),
				zap.Int64("offset", m.Offset),
			)
			// Не коммитим, если не удалось отправить в DLQ
			return false
		}

		return true // Коммитим после отправки в DLQ
	}

	// Преобразуем payload в OrderPaidEvent
	event, err := c.parseOrderPaidEvent(payload)
	if err != nil {
		c.logger.Error("failed to parse order paid event - sending to DLQ",
			zap.Error(err),
			zap.String("topic", m.Topic),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)

		// Извлекаем event_type и event_id для DLQ
		eventType, _ := payload["event_type"].(string)
		eventID, _ := payload["event_id"].(string)
		orderID, _ := payload["order_id"].(string)

		// Отправляем в DLQ и коммитим (poison pill)
		if err := c.dlqPublisher.Publish(ctx, m, err, eventType, eventID, orderID); err != nil {
			c.logger.Error("failed to send message to DLQ",
				zap.Error(err),
				zap.String("topic", m.Topic),
				zap.Int("partition", m.Partition),
				zap.Int64("offset", m.Offset),
			)
			return false
		}

		return true // Коммитим после отправки в DLQ
	}

	c.logger.Info("received order paid event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int("partition", m.Partition),
		zap.Int64("offset", m.Offset),
	)

	// Пытаемся обработать событие с retry
	success := c.handleWithRetry(ctx, m, event)

	if !success {
		// После исчерпания retry отправляем в DLQ
		c.logger.Error("failed to handle order paid event after all retries - sending to DLQ",
			zap.String("order_id", event.OrderID),
			zap.Int("partition", m.Partition),
			zap.Int64("offset", m.Offset),
		)

		// Создаём ошибку для DLQ
		dlqErr := &ProcessingError{
			Message: "failed after all retry attempts",
			OrderID: event.OrderID,
		}

		if err := c.dlqPublisher.Publish(ctx, m, dlqErr, event.EventType, event.EventID, event.OrderID); err != nil {
			c.logger.Error("failed to send message to DLQ",
				zap.Error(err),
				zap.String("topic", m.Topic),
				zap.Int("partition", m.Partition),
				zap.Int64("offset", m.Offset),
			)
			return false
		}

		return true // Коммитим после отправки в DLQ
	}

	c.logger.Info("order paid event processed successfully",
		zap.String("order_id", event.OrderID),
		zap.Int("partition", m.Partition),
		zap.Int64("offset", m.Offset),
	)

	return true // Коммитим после успешной обработки
}

// handleWithRetry обрабатывает событие с retry логикой
// Возвращает true при успешной обработке, false при исчерпании попыток
func (c *OrderPaidConsumer) handleWithRetry(ctx context.Context, m kafka.Message, event service.OrderPaidEvent) bool {
	var lastErr error

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		// Вычисляем backoff: 1s, 2s, 4s (экспоненциально)
		if attempt > 1 {
			backoff := c.backoffBase * time.Duration(1<<uint(attempt-2))
			c.logger.Info("retrying order paid event",
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
		err := c.service.HandleOrderPaid(ctx, event)
		if err == nil {
			if attempt > 1 {
				c.logger.Info("order paid event processed successfully after retry",
					zap.String("order_id", event.OrderID),
					zap.Int("attempt", attempt),
				)
			}
			return true
		}

		lastErr = err
		c.logger.Warn("failed to handle order paid event",
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

// ProcessingError представляет ошибку обработки для DLQ
type ProcessingError struct {
	Message string
	OrderID string
}

func (e *ProcessingError) Error() string {
	return e.Message
}

// parseOrderPaidEvent преобразует payload в OrderPaidEvent
func (c *OrderPaidConsumer) parseOrderPaidEvent(payload map[string]interface{}) (service.OrderPaidEvent, error) {
	event := service.OrderPaidEvent{}

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
	} else {
		return event, &ParseError{Field: "user_id", Message: "user_id is required"}
	}
	if v, ok := payload["amount"].(float64); ok {
		event.Amount = int64(v)
	}
	if v, ok := payload["payment_method"].(string); ok {
		event.PaymentMethod = v
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
func (c *OrderPaidConsumer) Close() error {
	c.logger.Info("closing kafka consumer")
	return c.reader.Close()
}
