package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// DLQMessage представляет сообщение для Dead Letter Queue
type DLQMessage struct {
	OriginalTopic     string `json:"original_topic"`     //топик, из которого пришло сообщение
	OriginalPartition int    `json:"original_partition"` //раздел, из которого пришло сообщение
	OriginalOffset    int64  `json:"original_offset"`    //смещение, из которого пришло сообщение
	OriginalKey       string `json:"original_key"`       // base64 encoded сообщение
	OriginalValue     string `json:"original_value"`     // значение, из которого пришло сообщение
	ErrorMessage      string `json:"error_message"`      //сообщение об ошибке
	FailedAt          string `json:"failed_at"`          // время, когда произошла ошибка, в формате RFC3339
	EventType         string `json:"event_type"`         // если удалось извлечь тип события
	EventID           string `json:"event_id"`           // если удалось извлечь ID события
	OrderID           string `json:"order_id"`           // если удалось извлечь ID заказа
}

// DLQPublisher публикует сообщения в Dead Letter Queue
type DLQPublisher struct {
	logger *zap.Logger
	writer *kafka.Writer
	topic  string
}

// NewDLQPublisher создаёт новый publisher для DLQ
func NewDLQPublisher(logger *zap.Logger, brokers []string, topic string) *DLQPublisher {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &DLQPublisher{
		logger: logger,
		writer: writer,
		topic:  topic,
	}
}

// Publish отправляет сообщение в DLQ
func (p *DLQPublisher) Publish(ctx context.Context, msg kafka.Message, err error, eventType, eventID, orderID string) error {
	// Формируем сообщение об ошибке
	errorMsg := "unknown error"
	if err != nil {
		errorMsg = err.Error()
	}

	// Формируем DLQ сообщение
	dlqMsg := DLQMessage{
		OriginalTopic:     msg.Topic,
		OriginalPartition: msg.Partition,
		OriginalOffset:    msg.Offset,
		OriginalKey:       base64.StdEncoding.EncodeToString(msg.Key),
		OriginalValue:     base64.StdEncoding.EncodeToString(msg.Value),
		ErrorMessage:      errorMsg,
		FailedAt:          time.Now().UTC().Format(time.RFC3339),
		EventType:         eventType,
		EventID:           eventID,
		OrderID:           orderID,
	}

	// Сериализуем в JSON
	valueBytes, err := json.Marshal(dlqMsg)
	if err != nil {
		p.logger.Error("failed to marshal DLQ message",
			zap.Error(err),
			zap.String("original_topic", msg.Topic),
			zap.Int("original_partition", msg.Partition),
			zap.Int64("original_offset", msg.Offset),
		)
		return err
	}

	// Определяем ключ для DLQ: order_id если есть, иначе original_key
	key := msg.Key
	if orderID != "" {
		key = []byte(orderID)
	}

	// Отправляем в DLQ
	kafkaMsg := kafka.Message{
		Key:   key,
		Value: valueBytes,
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		p.logger.Error("failed to publish message to DLQ",
			zap.Error(err),
			zap.String("dlq_topic", p.topic),
			zap.String("original_topic", msg.Topic),
			zap.Int("original_partition", msg.Partition),
			zap.Int64("original_offset", msg.Offset),
		)
		return err
	}

	p.logger.Info("message sent to DLQ",
		zap.String("dlq_topic", p.topic),
		zap.String("original_topic", msg.Topic),
		zap.Int("original_partition", msg.Partition),
		zap.Int64("original_offset", msg.Offset),
		zap.String("error", errorMsg),
	)

	return nil
}

// Close закрывает Kafka writer
func (p *DLQPublisher) Close() error {
	return p.writer.Close()
}
