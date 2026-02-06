package observability

import (
	"context"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// httpHeaderCarrier адаптирует http.Header к propagation.TextMapCarrier
type httpHeaderCarrier struct {
	header http.Header
}

func (c httpHeaderCarrier) Get(key string) string {
	return c.header.Get(key)
}

func (c httpHeaderCarrier) Set(key, value string) {
	c.header.Set(key, value)
}

func (c httpHeaderCarrier) Keys() []string {
	out := make([]string, 0, len(c.header))
	for k := range c.header {
		out = append(out, k)
	}
	return out
}

// HTTPMiddleware возвращает chi/http middleware: извлекает trace context, создаёт span на запрос, пишет trace в контекст и logger.
// serviceName — имя сервиса для атрибутов.
func HTTPMiddleware(serviceName string, logger *zap.Logger) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)  // otel.Tracer - это функция из пакета otel, которая возвращает tracer для сервиса
	prop := otel.GetTextMapPropagator() // otel.GetTextMapPropagator - это функция из пакета otel, которая возвращает text map propagator
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := prop.Extract(r.Context(), httpHeaderCarrier{r.Header}) // extract - это функция из пакета prop, которая извлекает текст из запроса
			route := r.URL.Path                                           // route - это путь из запроса
			if r.URL.RawPath != "" {
				route = r.URL.RawPath
			}
			spanName := "HTTP " + r.Method + " " + route
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.route", route),
				),
			)
			defer span.End()

			// Логгер с trace_id/span_id в контексте запроса
			reqLogger := L(ctx, logger)
			ctx = withLogger(ctx, reqLogger)

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK} // response writer - это функция из пакета http, которая записывает статус код в ответ
			next.ServeHTTP(wrapped, r.WithContext(ctx))                              // serve http - это функция из пакета http, которая сервит запрос

			statusCode := wrapped.statusCode
			span.SetAttributes(attribute.Int("http.status_code", statusCode))
			if statusCode >= 400 {
				span.SetStatus(codes.Error, strconv.Itoa(statusCode))
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader записывает статус код в ответ
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type ctxKeyLogger struct{}

func withLogger(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, log)
}

// LoggerFromContext возвращает logger из контекста (если был положен HTTPMiddleware), иначе nil.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxKeyLogger{}).(*zap.Logger); ok {
		return l
	}
	return nil
}
