package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformhealth "github.com/shestoi/GoBigTech/platform/health/http"

	"github.com/shestoi/GoBigTech/services/order/internal/api/http/middleware"
)

// NewRouter создаёт и настраивает HTTP роутер для Order Service
// Регистрирует все маршруты и возвращает готовый к использованию роутер
// readiness - функция для проверки готовности сервиса (например, проверка БД).
// Если readiness возвращает false, health endpoint вернёт 503 Service Unavailable.
func NewRouter(handler *Handler, readiness func() bool) chi.Router {
	router := chi.NewRouter()

	// /orders* требуют x-session-id (middleware возвращает 401 при отсутствии)
	router.Route("/orders", func(r chi.Router) {
		r.Use(middleware.WithSessionID)
		r.Post("/", handler.PostOrders)
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			handler.GetOrdersId(w, r, id)
		})
	})

	// Health без middleware (не требует сессии)
	router.Get("/health", platformhealth.Handler(readiness))

	return router
}
