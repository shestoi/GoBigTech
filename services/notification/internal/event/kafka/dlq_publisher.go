package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// DLQPublisher публикует сообщения в Dead Letter Queue
type DLQPublisher struct {
	logger *zap.Logger
	writer *kafka.Writer
}

// NewDLQPublisher создаёт новый DLQ publisher
func NewDLQPublisher(logger *zap.Logger, brokers []string, topic string) *DLQPublisher {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &DLQPublisher{
		logger: logger,
		writer: writer,
	}
}

// DLQMessage представляет сообщение для DLQ
type DLQMessage struct {
	OriginalTopic     string    `json:"original_topic"`
	OriginalPartition int       `json:"original_partition"`
	OriginalOffset    int64     `json:"original_offset"`
	OriginalKey       string    `json:"original_key"`
	OriginalValue     string    `json:"original_value"`
	ErrorMessage      string    `json:"error_message"`
	FailedAt          time.Time `json:"failed_at"`
	EventType         string    `json:"event_type,omitempty"`
	EventID           string    `json:"event_id,omitempty"`
	OrderID           string    `json:"order_id,omitempty"`
}

// Publish публикует сообщение в DLQ
func (p *DLQPublisher) Publish(ctx context.Context, originalMessage kafka.Message, originalErr error, eventType, eventID, orderID string) error {
	errorMsg := ""
	if originalErr != nil {
		errorMsg = originalErr.Error()
	}
	//dlqMsg - сообщение для DLQ
	dlqMsg := DLQMessage{
		OriginalTopic:     originalMessage.Topic,
		OriginalPartition: originalMessage.Partition,
		OriginalOffset:    originalMessage.Offset,
		OriginalKey:       string(originalMessage.Key),
		OriginalValue:     string(originalMessage.Value),
		ErrorMessage:      errorMsg,
		FailedAt:          time.Now().UTC(),
		EventType:         eventType,
		EventID:           eventID,
		OrderID:           orderID,
	}

	//payload - сообщение для DLQ в формате JSON
	payload, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Используем orderID как key, если доступен, иначе original_key
	key := originalMessage.Key
	if orderID != "" {
		key = []byte(orderID)
	}

	//msg - сообщение для DLQ в формате Kafka
	msg := kafka.Message{
		Key:   key,
		Value: payload,
	}

	//writeErr - ошибка при записи сообщения в DLQ
	if writeErr := p.writer.WriteMessages(ctx, msg); writeErr != nil {
		p.logger.Error("failed to publish message to DLQ",
			zap.Error(writeErr),
			zap.String("original_topic", originalMessage.Topic),
			zap.Int("original_partition", originalMessage.Partition),
			zap.Int64("original_offset", originalMessage.Offset),
		)
		return writeErr
	}

	//logger - логгер для записи сообщения в DLQ
	p.logger.Info("message published to DLQ",
		zap.String("original_topic", originalMessage.Topic),
		zap.Int("original_partition", originalMessage.Partition),
		zap.Int64("original_offset", originalMessage.Offset),
		zap.String("error_message", errorMsg),
	)

	return nil
}

// Close закрывает writer
func (p *DLQPublisher) Close() error {
	p.logger.Info("closing DLQ publisher")
	return p.writer.Close()
}
