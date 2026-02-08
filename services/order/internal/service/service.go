package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

// OrderService содержит бизнес-логику работы с заказами
type OrderService struct {
	logger                *zap.Logger
	inventoryClient       InventoryClient
	paymentClient         PaymentClient
	orderRepo             repository.OrderRepository
	paymentCompletedTopic string
	metrics               OrderMetricsRecorder // опционально, может быть nil
}

// NewOrderService создаёт новый экземпляр OrderService.
// metrics может быть nil — тогда метрики не записываются.
func NewOrderService(
	logger *zap.Logger,
	inventoryClient InventoryClient,
	paymentClient PaymentClient,
	orderRepo repository.OrderRepository,
	topic string,
	metrics OrderMetricsRecorder,
) *OrderService {
	return &OrderService{
		logger:                logger,
		inventoryClient:       inventoryClient,
		paymentClient:         paymentClient,
		orderRepo:             orderRepo,
		paymentCompletedTopic: topic,
		metrics:               metrics,
	}
}

// CreateOrderInput содержит входные данные для создания заказа
// Использует доменную модель repository.OrderItem для работы с несколькими товарами
type CreateOrderInput struct {
	UserID string
	Items  []repository.OrderItem
}

// CreateOrderOutput содержит результат создания заказа
// Использует доменную модель repository.OrderItem
type CreateOrderOutput struct {
	OrderID string
	UserID  string
	Status  string
	Items   []repository.OrderItem
}

// CreateOrder создаёт новый заказ
// Вся бизнес-логика здесь: резервирование товара, оплата, формирование заказа
func (s *OrderService) CreateOrder(ctx context.Context, input CreateOrderInput) (*CreateOrderOutput, error) {
	tracer := otel.Tracer("order")
	ctx, span := tracer.Start(ctx, "CreateOrder", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	log.Printf("Creating order for user %s with %d items", input.UserID, len(input.Items))

	// Валидация: должен быть хотя бы один товар
	if len(input.Items) == 0 {
		err := fmt.Errorf("order must contain at least one item")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// 1. Резервируем товары через Inventory сервис
	ctx, reserveSpan := tracer.Start(ctx, "Inventory.ReserveStock", trace.WithSpanKind(trace.SpanKindClient))
	for _, item := range input.Items {
		err := s.inventoryClient.ReserveStock(ctx, item.ProductID, item.Quantity)
		if err != nil {
			log.Printf("Inventory ReserveStock error for product %s: %v", item.ProductID, err)
			reserveSpan.RecordError(err)
			reserveSpan.SetStatus(codes.Error, err.Error())
			reserveSpan.End()
			return nil, fmt.Errorf("inventory service error for product %s: %w", item.ProductID, err)
		}
	}
	reserveSpan.End()

	log.Printf("All inventory items reserved successfully")

	// 2. Генерируем ID заказа (в будущем можно использовать UUID или другой генератор)
	orderID := fmt.Sprintf("order-%d", time.Now().UnixNano()) //генерируем уникальный ID для заказа

	// 3. Вычисляем сумму заказа (упрощённо: каждый товар стоит 100 единиц)
	// В реальном приложении нужно получать цены из каталога товаров

	const pricePerItemCents = 100 * 100 // 100 условных единиц, каждая = 100 копеек

	totalAmount := int64(0)
	for _, item := range input.Items {
		totalAmount += int64(item.Quantity) * pricePerItemCents
	}

	// 4. Обрабатываем оплату через Payment сервис
	ctx, paymentSpan := tracer.Start(ctx, "Payment.Charge", trace.WithSpanKind(trace.SpanKindClient))
	paymentMethod := "card" // можно передавать из input в будущем
	amountFloat := float64(totalAmount) / 100.0
	transactionID, err := s.paymentClient.ProcessPayment(ctx, orderID, input.UserID, amountFloat, paymentMethod)
	if err != nil {
		paymentSpan.RecordError(err)
		paymentSpan.SetStatus(codes.Error, err.Error())
		paymentSpan.End()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Printf("Payment ProcessPayment error: %v", err)
		return nil, fmt.Errorf("payment service error: %w", err)
	}
	paymentSpan.End()

	log.Printf("Payment processed successfully, transaction ID: %s", transactionID)

	// 5. Создаём доменную модель заказа
	order := repository.Order{
		ID:     orderID,
		UserID: input.UserID,
		Status: "paid",
		Items:  input.Items, // Используем Items из input напрямую
	}

	// 6. Формируем событие успешной оплаты заказа
	eventID := fmt.Sprintf("payment-%s-%d", orderID, time.Now().UnixNano())
	eventType := "order.payment.completed"
	occurredAt := time.Now().UTC()

	eventPayload := map[string]interface{}{
		"event_id":       eventID,
		"event_type":     eventType,
		"event_version":  1,
		"occurred_at":    occurredAt.Format(time.RFC3339),
		"order_id":       orderID,
		"user_id":        input.UserID,
		"amount":         totalAmount,
		"payment_method": paymentMethod,
	}

	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// 7. Сохраняем заказ и событие в outbox в одной транзакции
	topic := s.paymentCompletedTopic
	if err := s.orderRepo.SaveWithOutbox(ctx, order, eventID, eventType, occurredAt, payloadBytes, topic); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Printf("Failed to save order with outbox: %v", err)
		return nil, fmt.Errorf("failed to save order with outbox: %w", err)
	}

	if s.metrics != nil {
		s.metrics.RecordOrderCreated(totalAmount)
	}

	log.Printf("Order saved successfully with outbox event: %s", orderID)

	return &CreateOrderOutput{
		OrderID: orderID,
		UserID:  input.UserID,
		Status:  "paid",
		Items:   input.Items, // Возвращаем Items из input
	}, nil
}

// GetOrderInput содержит входные данные для получения заказа
type GetOrderInput struct {
	OrderID string
}

// GetOrderOutput содержит результат получения заказа
// Использует доменную модель repository.OrderItem
type GetOrderOutput struct {
	OrderID string
	UserID  string
	Status  string
	Items   []repository.OrderItem
}

// GetOrder получает заказ по ID
// Бизнес-логика здесь, а не в HTTP-обработчике
func (s *OrderService) GetOrder(ctx context.Context, input GetOrderInput) (*GetOrderOutput, error) {
	log.Printf("Getting order: %s", input.OrderID)

	// Получаем заказ из репозитория
	order, err := s.orderRepo.GetByID(ctx, input.OrderID)
	if err != nil {
		log.Printf("Failed to get order: %v", err)
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Преобразуем доменную модель в DTO
	// Возвращаем Items целиком, без извлечения первого элемента
	return &GetOrderOutput{
		OrderID: order.ID,
		UserID:  order.UserID,
		Status:  order.Status,
		Items:   order.Items, // Возвращаем все Items
	}, nil
}

// HandleOrderAssemblyCompleted обрабатывает событие завершения сборки заказа
// Обеспечивает idempotency через inbox таблицу: если событие уже обработано, просто возвращает nil
func (s *OrderService) HandleOrderAssemblyCompleted(ctx context.Context, event OrderAssemblyCompletedEvent) error {
	s.logger.Info("handling order assembly completed event",
		zap.String("event_id", event.EventID),
		zap.String("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
	)

	// Вызываем repository метод, который делает insert в inbox + update status в одной транзакции
	inserted, rowsAffected, err := s.orderRepo.HandleAssemblyCompletedTx(
		ctx,
		event.EventID,
		event.EventType,
		event.OccurredAt,
		event.OrderID,
	)
	if err != nil {
		s.logger.Error("failed to handle assembly completed event",
			zap.Error(err),
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return err
	}

	// Если событие уже было обработано (duplicate), просто возвращаем nil
	if !inserted {
		s.logger.Info("event already processed (duplicate)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
		return nil
	}

	// Событие впервые обработано
	if rowsAffected == 0 { //если количество обновлённых строк равно 0, то заказ уже assembled или не найден
		// Заказ уже assembled или не найден - это не ошибка, но логируем warning
		s.logger.Warn("order status not updated (already assembled or not found)",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
	} else {
		s.logger.Info("order status updated to assembled",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
	}

	return nil
}
