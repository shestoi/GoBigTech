package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformhealth "github.com/shestoi/GoBigTech/platform/health/http"
)

// NewRouter создаёт и настраивает HTTP роутер для Order Service
// Регистрирует все маршруты и возвращает готовый к использованию роутер
// readiness - функция для проверки готовности сервиса (например, проверка БД).
// Если readiness возвращает false, health endpoint вернёт 503 Service Unavailable.
func NewRouter(handler *Handler, readiness func() bool) chi.Router {
	router := chi.NewRouter()

	// Регистрируем обработчики заказов
	router.Post("/orders", handler.PostOrders)
	router.Get("/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		handler.GetOrdersId(w, r, id)
	})

	// Регистрируем health check (используем platform health handler с readiness)
	router.Get("/health", platformhealth.Handler(readiness))

	return router
}
