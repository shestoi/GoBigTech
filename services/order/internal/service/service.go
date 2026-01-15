package service

import (
	"context"
	"fmt"
	"log"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

// OrderService содержит бизнес-логику работы с заказами
// Теперь зависит от интерфейсов, а не от конкретных gRPC клиентов и репозиториев
type OrderService struct {
	inventoryClient InventoryClient
	paymentClient   PaymentClient
	orderRepo      repository.OrderRepository
}

// NewOrderService создаёт новый экземпляр OrderService
// Принимает интерфейсы как зависимости - это позволяет легко подменять их в тестах
// и делает service независимым от конкретной реализации (gRPC, HTTP, моки, БД)
func NewOrderService(
	inventoryClient InventoryClient,
	paymentClient PaymentClient,
	orderRepo repository.OrderRepository,
) *OrderService {
	return &OrderService{
		inventoryClient: inventoryClient,
		paymentClient:   paymentClient,
		orderRepo:      orderRepo,
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
	log.Printf("Creating order for user %s with %d items", input.UserID, len(input.Items))

	// Валидация: должен быть хотя бы один товар
	if len(input.Items) == 0 {
		return nil, fmt.Errorf("order must contain at least one item")
	}

	// 1. Резервируем товары через Inventory сервис
	// Для каждого товара вызываем ReserveStock
	for _, item := range input.Items {
		err := s.inventoryClient.ReserveStock(ctx, item.ProductID, item.Quantity)
		if err != nil {
			log.Printf("Inventory ReserveStock error for product %s: %v", item.ProductID, err)
			return nil, fmt.Errorf("inventory service error for product %s: %w", item.ProductID, err)
		}
	}

	log.Printf("All inventory items reserved successfully")

	// 2. Обрабатываем оплату через Payment сервис
	// TODO: генерировать реальный ID заказа
	// TODO: вычислять реальную сумму из товаров
	transactionID, err := s.paymentClient.ProcessPayment(ctx, "order-123", input.UserID, 100.0, "card")
	if err != nil {
		log.Printf("Payment ProcessPayment error: %v", err)
		return nil, fmt.Errorf("payment service error: %w", err)
	}

	log.Printf("Payment processed successfully, transaction ID: %s", transactionID)

	// 3. Создаём доменную модель заказа
	orderID := fmt.Sprintf("order-%s", transactionID)
	order := repository.Order{
		ID:     orderID,
		UserID: input.UserID,
		Status: "paid",
		Items:  input.Items, // Используем Items из input напрямую
	}

	// 4. Сохраняем заказ в репозитории
	if err := s.orderRepo.Save(ctx, order); err != nil {
		log.Printf("Failed to save order: %v", err)
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	log.Printf("Order saved successfully: %s", orderID)

	// 5. Формируем результат
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


