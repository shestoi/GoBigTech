package authctx

import (
	"context"
)

type ctxKeySessionID struct{}

var sessionIDKey = ctxKeySessionID{}

// WithSessionID сохраняет session_id в контексте (используется HTTP middleware и gRPC клиентами)
func WithSessionID(ctx context.Context, sid string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sid)
}

// SessionIDFromContext возвращает session_id из контекста, если он был установлен
func SessionIDFromContext(ctx context.Context) (string, bool) {
	sid, ok := ctx.Value(sessionIDKey).(string)
	return sid, ok
}
