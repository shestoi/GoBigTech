package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Sender определяет интерфейс для отправки сообщений
type Sender interface {
	Send(ctx context.Context, chatID, text string) error
}

// TelegramSender реализует отправку сообщений через Telegram Bot API
type TelegramSender struct {
	logger   *zap.Logger
	botToken string
	apiURL   string
	client   *http.Client
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

	//Готовим payload (тело запроса)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	//Превращаем payload в JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	//Создаём HTTP-запрос с контекстом
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData)) //req для отправки запроса в Telegram
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	//устанавливаем заголовок Content-Type для отправки сообщения в JSON формате
	req.Header.Set("Content-Type", "application/json")

	//Отправляем запрос и получаем ответ
	resp, err := s.client.Do(req) //resp для получения ответа от Telegram
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// При не-200 читаем тело ответа для диагностики и не декодируем JSON
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Декодируем ответ от Telegram в формате JSON
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	//Телеграм обычно отвечает так: {"ok": true, "result": {"message_id": 1234567890}} или {"ok": false, "description": "Bad Request: chat not found"}
	if ok, _ := result["ok"].(bool); !ok { //ok для проверки успешности отправки сообщения
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
