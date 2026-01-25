package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/assembly/internal/service"
)

// KafkaAssemblyEventPublisher реализует AssemblyEventPublisher используя Kafka
type KafkaAssemblyEventPublisher struct {
	logger *zap.Logger
	writer *kafka.Writer //writer для отправки сообщений в Kafka
	topic  string
}

// NewKafkaAssemblyEventPublisher создаёт новый Kafka publisher для событий сборки заказа
func NewKafkaAssemblyEventPublisher(logger *zap.Logger, brokers []string, topic string) *KafkaAssemblyEventPublisher {
	writer := &kafka.Writer{ //создаём writer для отправки сообщений в Kafka
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{}, //алгоритм балансировки нагрузки
	}

	return &KafkaAssemblyEventPublisher{
		logger: logger,
		writer: writer,
		topic:  topic,
	}
}

// Close закрывает Kafka writer
func (p *KafkaAssemblyEventPublisher) Close() error {
	return p.writer.Close()
}

// PublishOrderAssemblyCompleted публикует событие успешной сборки заказа в Kafka
func (p *KafkaAssemblyEventPublisher) PublishOrderAssemblyCompleted(ctx context.Context, event service.OrderAssemblyCompletedEvent) error {
	// Генерируем event_id, если он не задан
	eventID := event.EventID
	if eventID == "" {
		eventID = uuid.New().String() //генерируем уникальный ID для события
	}

	// Формируем JSON payload события
	payload := map[string]interface{}{
		"event_id":      eventID,
		"event_type":    event.EventType,
		"event_version": event.EventVersion,
		"occurred_at":   event.OccurredAt.Format(time.RFC3339),
		"order_id":      event.OrderID,
		"user_id":       event.UserID,
	}

	valueBytes, err := json.Marshal(payload) //преобразуем данные события в JSON
	if err != nil {
		p.logger.Error("failed to marshal assembly completed event",
			zap.Error(err),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// Отправляем сообщение в Kafka
	message := kafka.Message{
		Key:   []byte(event.OrderID),
		Value: valueBytes,
	}

	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		p.logger.Error("failed to publish assembly completed event",
			zap.Error(err),
			zap.String("topic", p.topic),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
		)
		return err
	}

	p.logger.Info("assembly completed event published",
		zap.String("topic", p.topic),
		zap.String("event_id", eventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	return nil
}
