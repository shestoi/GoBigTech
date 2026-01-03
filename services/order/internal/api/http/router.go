package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter создаёт и настраивает HTTP роутер для Order Service
// Регистрирует все маршруты и возвращает готовый к использованию роутер
func NewRouter(handler *Handler) chi.Router {
	router := chi.NewRouter()

	// Регистрируем обработчики заказов
	router.Post("/orders", handler.PostOrders)
	router.Get("/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		handler.GetOrdersId(w, r, id)
	})

	// Регистрируем health check
	router.Get("/health", handler.Health)

	return router
}
