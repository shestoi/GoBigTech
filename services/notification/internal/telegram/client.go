package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Sender определяет интерфейс для отправки сообщений
type Sender interface {
	Send(ctx context.Context, chatID, text string) error
}

// TelegramSender реализует отправку сообщений через Telegram Bot API
type TelegramSender struct {
	logger    *zap.Logger
	botToken  string
	apiURL    string
	client    *http.Client
}

// NewTelegramSender создаёт новый Telegram sender
func NewTelegramSender(logger *zap.Logger, botToken string) *TelegramSender {
	return &TelegramSender{
		logger:   logger,
		botToken: botToken,
		apiURL:   "https://api.telegram.org/bot" + botToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send отправляет сообщение в Telegram
func (s *TelegramSender) Send(ctx context.Context, chatID, text string) error {
	url := fmt.Sprintf("%s/sendMessage", s.apiURL)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if ok, _ := result["ok"].(bool); !ok {
		description, _ := result["description"].(string)
		return fmt.Errorf("telegram API error: %s", description)
	}

	s.logger.Debug("telegram message sent successfully",
		zap.String("chat_id", chatID),
	)

	return nil
}

// NoOpSender - no-op реализация Sender (для тестов или когда Telegram отключён)
type NoOpSender struct {
	logger *zap.Logger
}

// NewNoOpSender создаёт no-op sender
func NewNoOpSender(logger *zap.Logger) *NoOpSender {
	return &NoOpSender{
		logger: logger,
	}
}

// Send ничего не делает, только логирует
func (s *NoOpSender) Send(ctx context.Context, chatID, text string) error {
	s.logger.Debug("no-op sender: message not sent",
		zap.String("chat_id", chatID),
		zap.String("text_preview", truncate(text, 50)),
	)
	return nil
}

// truncate обрезает строку до указанной длины
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

