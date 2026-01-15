package http

import (
	"encoding/json"
	"net/http"
)

// Handler возвращает HTTP handler для health check endpoint.
// Возвращает 200 OK с JSON телом {"status":"ok"} если readiness функция не указана
// или если readiness функция возвращает true.
// Возвращает 503 Service Unavailable если readiness функция указана и возвращает false.
func Handler(readiness func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Если readiness функция указана, проверяем её
		if readiness != nil && !readiness() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
			return
		}

		// По умолчанию возвращаем 200 OK
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
