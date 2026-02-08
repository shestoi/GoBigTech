package httpapi

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/iam/internal/service"
)

const sessionIDHeader = "x-session-id"

// ValidateHandler обрабатывает POST /internal/validate: проверка сессии по заголовку x-session-id.
// Использует существующую логику IAM (service.ValidateSession). 401 при отсутствии заголовка или невалидной сессии.
type ValidateHandler struct {
	iamService *service.Service
	logger     *zap.Logger
}

// NewValidateHandler создаёт обработчик валидации сессии.
func NewValidateHandler(iamService *service.Service, logger *zap.Logger) *ValidateHandler {
	return &ValidateHandler{iamService: iamService, logger: logger}
}

// ServeHTTP реализует http.Handler.
func (h *ValidateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get(sessionIDHeader)
	if sessionID == "" {
		h.logger.Debug("validate: missing x-session-id header")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	_, err := h.iamService.ValidateSession(r.Context(), service.ValidateSessionInput{SessionID: sessionID})
	if err != nil {
		h.logger.Debug("validate: session invalid or expired", zap.String("session_id", sessionID), zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
