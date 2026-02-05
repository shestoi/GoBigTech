package http

import (
	"net/http"
)

// NewAlertRouter возвращает роутер для webhook алертов: POST /alerts и POST /alerts/alertmanager (Alertmanager v4 payload).
func NewAlertRouter(alertHandler *AlertmanagerHandler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/alerts", alertHandler)
	mux.Handle("/alerts/alertmanager", alertHandler)
	return mux
}
