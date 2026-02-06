package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceFields возвращает zap-поля trace_id и span_id из контекста, если span есть.
// Используется для корреляции логов с трейсами.
func TraceFields(ctx context.Context) []zap.Field {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}
	sc := span.SpanContext()
	return []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	}
}

// L возвращает logger с добавленными trace_id/span_id из ctx, если они есть.
// Использовать в хендлерах и сервисах: observability.L(ctx, logger).Info(...)
func L(ctx context.Context, base *zap.Logger) *zap.Logger {
	fields := TraceFields(ctx)
	if len(fields) == 0 {
		return base
	}
	return base.With(fields...)
}
