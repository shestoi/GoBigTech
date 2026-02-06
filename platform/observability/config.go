package observability

// Config конфигурация OpenTelemetry (traces + metrics + propagator)
type Config struct {
	// Enabled включить экспорт в OTLP collector
	Enabled bool
	// OTLPEndpoint адрес OTLP gRPC (traces + metrics), например "127.0.0.1:4317" или "otel-collector:4317"
	OTLPEndpoint string
	// SamplingRatio доля трасс для семплирования (0..1), 1.0 = все
	SamplingRatio float64
	// ServiceName имя сервиса (order, inventory, payment, iam, notification, assembly)
	ServiceName string
	// DeploymentEnvironment окружение (local, docker)
	DeploymentEnvironment string
	// ServiceVersion опционально, например из build
	ServiceVersion string
}
