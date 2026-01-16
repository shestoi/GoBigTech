package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
	"github.com/shestoi/GoBigTech/services/order/internal/service"
)

// Handler содержит HTTP-обработчики для Order Service
// Зависит от service слоя, но не знает о деталях реализации (gRPC, БД и т.д.)
type Handler struct {
	orderService *service.OrderService
	logger       *zap.Logger
}

// NewHandler создаёт новый HTTP handler
func NewHandler(orderService *service.OrderService, logger *zap.Logger) *Handler {
	return &Handler{
		orderService: orderService,
		logger:       logger,
	}
}

// OrderItem представляет товар в HTTP запросе/ответе
type OrderItem struct {
	ProductID *string `json:"product_id"`
	Quantity  *int    `json:"quantity"`
}

// OrderRequest представляет HTTP запрос на создание заказа
type OrderRequest struct {
	UserID *string      `json:"user_id"`
	Items  *[]OrderItem `json:"items"`
}

// OrderResponse представляет HTTP ответ с информацией о заказе
type OrderResponse struct {
	ID     *string      `json:"id"`
	UserID *string      `json:"user_id"`
	Status *string      `json:"status"`
	Items  *[]OrderItem `json:"items"`
}

// PostOrders обрабатывает POST /orders - создание нового заказа
func (h *Handler) PostOrders(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.PostOrders"
	ctx := r.Context()

	logger := h.logger.With(zap.String("op", op))
	logger.Info("Received request", zap.String("method", r.Method), zap.String("path", r.URL.Path))

	// Декодируем JSON тело запроса
	var reqBody OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Warn("JSON decode error", zap.Error(err))
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Валидация входных данных
	if reqBody.UserID == nil || reqBody.Items == nil || len(*reqBody.Items) == 0 {
		logger.Warn("Validation failed: missing required fields")
		http.Error(w, "Invalid payload: user_id and items are required", http.StatusBadRequest)
		return
	}

	// Валидация всех items: product_id не пустой, quantity > 0
	for i, item := range *reqBody.Items {
		if item.ProductID == nil || *item.ProductID == "" {
			logger.Warn("Validation failed: product_id is required", zap.Int("item_index", i))
			http.Error(w, fmt.Sprintf("Invalid payload: product_id is required in items[%d]", i), http.StatusBadRequest)
			return
		}
		if item.Quantity == nil || *item.Quantity <= 0 {
			logger.Warn("Validation failed: quantity must be > 0", zap.Int("item_index", i))
			http.Error(w, fmt.Sprintf("Invalid payload: quantity must be > 0 in items[%d]", i), http.StatusBadRequest)
			return
		}
	}

	userID := *reqBody.UserID

	// Преобразуем HTTP DTO в service DTO
	serviceItems := make([]repository.OrderItem, 0, len(*reqBody.Items))
	for _, item := range *reqBody.Items {
		serviceItems = append(serviceItems, repository.OrderItem{
			ProductID: *item.ProductID,
			Quantity:  int32(*item.Quantity),
		})
	}

	// Вызываем service слой для создания заказа
	// Вся бизнес-логика теперь в service, а не в обработчике
	result, err := h.orderService.CreateOrder(ctx, service.CreateOrderInput{
		UserID: userID,
		Items:  serviceItems,
	})

	if err != nil {
		logger.Error("Order creation error", zap.Error(err))
		// Определяем HTTP статус на основе типа ошибки
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Формируем HTTP ответ из результата service
	// Преобразуем service DTO в HTTP DTO
	httpItems := make([]OrderItem, 0, len(result.Items))
	for _, item := range result.Items {
		productID := item.ProductID
		quantity := int(item.Quantity)
		httpItems = append(httpItems, OrderItem{
			ProductID: &productID,
			Quantity:  &quantity,
		})
	}

	resp := OrderResponse{
		ID:     &result.OrderID,
		UserID: &result.UserID,
		Status: &result.Status,
		Items:  &httpItems,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("Order created successfully", zap.String("order_id", result.OrderID))
}

// GetOrdersId обрабатывает GET /orders/{id} - получение заказа по ID
func (h *Handler) GetOrdersId(w http.ResponseWriter, r *http.Request, id string) {
	const op = "Handler.GetOrdersId"
	ctx := r.Context()

	logger := h.logger.With(zap.String("op", op), zap.String("order_id", id))
	logger.Info("Received request", zap.String("method", r.Method))

	// Вызываем service слой для получения заказа
	// Бизнес-логика теперь в service, а не в обработчике
	result, err := h.orderService.GetOrder(ctx, service.GetOrderInput{
		OrderID: id,
	})

	if err != nil {
		logger.Error("Get order error", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get order: %v", err), http.StatusInternalServerError)
		return
	}

	// Формируем HTTP ответ из результата service
	// Преобразуем service DTO (Items []) в HTTP DTO
	httpItems := make([]OrderItem, 0, len(result.Items))
	for _, item := range result.Items {
		productID := item.ProductID
		quantity := int(item.Quantity)
		httpItems = append(httpItems, OrderItem{
			ProductID: &productID,
			Quantity:  &quantity,
		})
	}

	resp := OrderResponse{
		ID:     &result.OrderID,
		UserID: &result.UserID,
		Status: &result.Status,
		Items:  &httpItems,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
