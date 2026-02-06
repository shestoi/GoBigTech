package middleware

import (
	"net/http"

	"github.com/shestoi/GoBigTech/services/order/internal/authctx"
)

// WithSessionID — HTTP middleware: читает заголовок x-session-id, при отсутствии возвращает 401, иначе кладёт sid в context
func WithSessionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sid := r.Header.Get("x-session-id")
		if sid == "" {
			http.Error(w, "session_id is required", http.StatusUnauthorized)
			return
		}
		ctx := authctx.WithSessionID(r.Context(), sid) // добавляем session_id в контекст
		next.ServeHTTP(w, r.WithContext(ctx)) // вызываем следующий handler
	})
}
