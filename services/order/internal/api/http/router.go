package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformhealth "github.com/shestoi/GoBigTech/platform/health/http"
	platformobservability "github.com/shestoi/GoBigTech/platform/observability"

	"github.com/shestoi/GoBigTech/services/order/internal/api/http/middleware"
	"go.uber.org/zap"
)

// NewRouter создаёт и настраивает HTTP роутер для Order Service
// readiness - функция для проверки готовности сервиса (например, проверка БД).
// Если readiness возвращает false, health endpoint вернёт 503 Service Unavailable.
// logger используется для observability HTTP middleware (trace_id в логах).
func NewRouter(handler *Handler, readiness func() bool, logger *zap.Logger) chi.Router {
	router := chi.NewRouter()

	// Observability: trace context + span на каждый запрос, logger с trace_id в контексте
	if logger != nil {
		router.Use(platformobservability.HTTPMiddleware("order", logger))
	}

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
