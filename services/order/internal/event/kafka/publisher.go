package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/service"
)

// KafkaPaymentEventPublisher реализует PaymentEventPublisher используя Kafka
type KafkaPaymentEventPublisher struct {
	logger *zap.Logger
	writer *kafka.Writer
	topic  string
}

// NewKafkaPaymentEventPublisher создаёт новый Kafka publisher для событий оплаты
func NewKafkaPaymentEventPublisher(logger *zap.Logger, brokers []string, topic string) *KafkaPaymentEventPublisher {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &KafkaPaymentEventPublisher{
		logger: logger,
		writer: writer,
		topic:  topic,
	}
}

// Close закрывает Kafka writer
func (p *KafkaPaymentEventPublisher) Close() error {
	return p.writer.Close()
}

// PublishOrderPaid публикует событие успешной оплаты заказа в Kafka
func (p *KafkaPaymentEventPublisher) PublishOrderPaid(ctx context.Context, event service.OrderPaidEvent) error {
	// Формируем JSON payload события - это данные, которые будут отправлены в Kafka
	payload := map[string]interface{}{
		"event_id":       uuid.New().String(), //генерируем уникальный ID для события
		"event_type":     "order.payment.completed",
		"event_version":  1,                                     //версия события
		"occurred_at":    time.Now().UTC().Format(time.RFC3339), //время события
		"order_id":       event.OrderID,                         //ID заказа
		"user_id":        event.UserID,                          //ID пользователя
		"amount":         event.Amount,                          //сумма оплаты
		"payment_method": event.PaymentMethod,                   //метод оплаты
	}

	valueBytes, err := json.Marshal(payload) //преобразуем данные события в JSON
	if err != nil {
		p.logger.Error("failed to marshal order paid event",
			zap.Error(err),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// Отправляем сообщение в Kafka
	message := kafka.Message{
		Key:   []byte(event.OrderID), //ключ для сообщения - ID заказа
		Value: valueBytes,            //значение для сообщения - данные события
	}

	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		p.logger.Error("failed to publish order paid event",
			zap.Error(err),
			zap.String("topic", p.topic),
			zap.String("order_id", event.OrderID),
			zap.String("user_id", event.UserID),
		)
		return err
	}

	p.logger.Info("order paid event published",
		zap.String("topic", p.topic),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int64("amount", event.Amount),
	)

	return nil
}
