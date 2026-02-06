package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/notification/internal/telegram"
)

// Alertmanager webhook payload (Prometheus Alertmanager API)
// https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type alertmanagerPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"` // "firing" | "resolved"
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []alertItem       `json:"alerts"`
}

type alertItem struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// AlertmanagerHandler обрабатывает POST /alerts/alertmanager от Alertmanager и шлёт уведомления в Telegram.
type AlertmanagerHandler struct {
	logger         *zap.Logger
	telegramSender telegram.Sender
	alertChatID    string
}

// NewAlertmanagerHandler создаёт обработчик webhook алертов.
func NewAlertmanagerHandler(logger *zap.Logger, telegramSender telegram.Sender, alertChatID string) *AlertmanagerHandler {
	return &AlertmanagerHandler{
		logger:         logger,
		telegramSender: telegramSender,
		alertChatID:    alertChatID,
	}
}

// ServeHTTP принимает JSON от Alertmanager, форматирует сообщение и отправляет в Telegram.
func (h *AlertmanagerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload alertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("alertmanager webhook: decode failed", zap.Error(err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if h.alertChatID == "" {
		h.logger.Warn("alertmanager webhook: ALERT_TELEGRAM_CHAT_ID not set, skipping send")
		w.WriteHeader(http.StatusOK)
		return
	}

	text := h.formatMessage(&payload)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := h.telegramSender.Send(ctx, h.alertChatID, text); err != nil {
		h.logger.Error("alertmanager webhook: telegram send failed", zap.Error(err), zap.String("chat_id", h.alertChatID))
		http.Error(w, "failed to send alert", http.StatusInternalServerError)
		return
	}

	h.logger.Info("alertmanager webhook: alert sent to Telegram",
		zap.String("status", payload.Status),
		zap.Int("alerts", len(payload.Alerts)),
	)
	w.WriteHeader(http.StatusOK)
}

func (h *AlertmanagerHandler) formatMessage(p *alertmanagerPayload) string {
	var b strings.Builder
	emoji := "🔥"
	if p.Status == "resolved" {
		emoji = "✅"
	}
	b.WriteString(fmt.Sprintf("%s Alertmanager: %s\n", emoji, p.Status))
	b.WriteString(fmt.Sprintf("Receiver: %s\n", p.Receiver))
	if p.ExternalURL != "" {
		b.WriteString(fmt.Sprintf("URL: %s\n", p.ExternalURL))
	}
	for i, a := range p.Alerts {
		alertname := a.Labels["alertname"]
		if alertname == "" {
			alertname = "Alert"
		}
		b.WriteString(fmt.Sprintf("\n[%d] %s (%s)\n", i+1, alertname, a.Status))
		if summary := a.Annotations["summary"]; summary != "" {
			b.WriteString(fmt.Sprintf("Summary: %s\n", summary))
		}
		if desc := a.Annotations["description"]; desc != "" {
			b.WriteString(fmt.Sprintf("Description: %s\n", desc))
		}
		if a.StartsAt != "" {
			b.WriteString(fmt.Sprintf("StartsAt: %s\n", a.StartsAt))
		}
		if a.Status == "resolved" && a.EndsAt != "" {
			b.WriteString(fmt.Sprintf("EndsAt: %s\n", a.EndsAt))
		}
		for k, v := range a.Labels {
			if k != "alertname" {
				b.WriteString(fmt.Sprintf("%s=%s ", k, v))
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
