package main

import (
	//"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"
	paymentpb "github.com/shestoi/GoBigTech/services/payment/v1"
	//orderapi    "github.com/shestoi/GoBigTech/services/order/api"
)

// Вспомогательные функции для создания указателей
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// Структура для хранения состояния сервера
type OrderServer struct {
	inventoryClient inventorypb.InventoryServiceClient
	paymentClient   paymentpb.PaymentServiceClient
}

// Структуры запросов
type OrderItem struct {
	ProductID *string `json:"product_id"`
	Quantity  *int    `json:"quantity"`
}

type OrderRequest struct {
	UserID *string      `json:"user_id"`
	Items  *[]OrderItem `json:"items"`
}

// Структура для ответа
type OrderResponse struct {
	ID     *string      `json:"id"`
	UserID *string      `json:"user_id"`
	Status *string      `json:"status"`
	Items  *[]OrderItem `json:"items"`
}

// Обработчик POST /orders
func (s *OrderServer) PostOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	log.Println("Received POST /orders request")

	// Декодируем JSON тело запроса
	var reqBody OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("JSON decode error: %v", err)
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Валидация входных данных
	if reqBody.UserID == nil || reqBody.Items == nil || len(*reqBody.Items) == 0 {
		log.Printf("Validation failed: missing required fields")
		http.Error(w, "Invalid payload: user_id and items are required", http.StatusBadRequest)
		return
	}

	firstItem := (*reqBody.Items)[0]
	if firstItem.ProductID == nil || firstItem.Quantity == nil {
		log.Printf("Validation failed: missing product_id or quantity")
		http.Error(w, "Invalid payload: product_id and quantity are required in items", http.StatusBadRequest)
		return
	}

	productID := *firstItem.ProductID
	quantity := int32(*firstItem.Quantity)
	userID := *reqBody.UserID

	log.Printf("Creating order for user %s: product=%s, quantity=%d", userID, productID, quantity)

	// 1. Резервируем товар через Inventory сервис
	_, err := s.inventoryClient.ReserveStock(ctx, &inventorypb.ReserveStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})

	if err != nil {
		log.Printf("Inventory ReserveStock error: %v", err)
		http.Error(w, fmt.Sprintf("Inventory service error: %v", err), http.StatusServiceUnavailable)
		return
	}

	log.Printf("Inventory reserved successfully")

	// 2. Обрабатываем оплату через Payment сервис
	paymentResp, err := s.paymentClient.ProcessPayment(ctx, &paymentpb.ProcessPaymentRequest{
		OrderId: "order-123",
		UserId:  userID,
		Amount:  100.0, // Фиксированная сумма для упрощения
		Method:  "card",
	})

	if err != nil {
		log.Printf("Payment ProcessPayment error: %v", err)
		http.Error(w, fmt.Sprintf("Payment service error: %v", err), http.StatusServiceUnavailable)
		return
	}

	log.Printf("Payment processed successfully, transaction ID: %s", paymentResp.TransactionId)

	// 3. Формируем ответ
	id := fmt.Sprintf("order-%s", paymentResp.TransactionId)
	status := "paid"

	resp := OrderResponse{
		ID:     &id,
		UserID: &userID,
		Status: &status,
		Items:  reqBody.Items,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Order created successfully: %s", id)
}

// Обработчик GET /orders/{id}
func (s *OrderServer) GetOrdersId(w http.ResponseWriter, r *http.Request, id string) {
	log.Printf("Received GET /orders/%s request", id)

	status := "paid"
	userID := "u1"
	productID := "p1"
	quantity := 2

	items := []OrderItem{{
		ProductID: &productID,
		Quantity:  &quantity,
	}}

	resp := OrderResponse{
		ID:     &id,
		UserID: &userID,
		Status: &status,
		Items:  &items,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Адаптер для совместимости с сгенерированным интерфейсом
type ServerWrapper struct {
	server *OrderServer
}

// PostOrders адаптер
func (sw *ServerWrapper) PostOrders(w http.ResponseWriter, r *http.Request) {
	sw.server.PostOrders(w, r)
}

// GetOrdersId адаптер
func (sw *ServerWrapper) GetOrdersId(w http.ResponseWriter, r *http.Request, id string) {
	sw.server.GetOrdersId(w, r, id)
}

func main() {
	log.Println("Starting Order service...")

	// Подключаемся к Inventory сервису
	inventoryAddr := "127.0.0.1:50051"
	log.Printf("Connecting to Inventory service at %s", inventoryAddr)

	inventoryConn, err := grpc.NewClient(inventoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Inventory gRPC client: %v", err)
	}
	defer inventoryConn.Close()

	inventoryClient := inventorypb.NewInventoryServiceClient(inventoryConn)

	// Подключаемся к Payment сервису
	paymentAddr := "127.0.0.1:50052"
	log.Printf("Connecting to Payment service at %s", paymentAddr)

	paymentConn, err := grpc.NewClient(paymentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Payment gRPC client: %v", err)
	}
	defer paymentConn.Close()

	paymentClient := paymentpb.NewPaymentServiceClient(paymentConn)

	// Создаем сервер
	orderServer := &OrderServer{
		inventoryClient: inventoryClient,
		paymentClient:   paymentClient,
	}

	// Создаем адаптер
	wrapper := &ServerWrapper{server: orderServer}

	// Настраиваем роутер
	router := chi.NewRouter()

	// Регистрируем обработчики
	router.Post("/orders", wrapper.PostOrders)
	router.Get("/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		wrapper.GetOrdersId(w, r, id)
	})

	// Добавляем health check
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Запускаем сервер
	port := ":8080"
	log.Printf("Order HTTP server starting on %s", port)
	log.Printf("Health check available at http://localhost%s/health", port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
