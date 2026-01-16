package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Health представляет обёртку над стандартным gRPC health service.
// Позволяет управлять статусом readiness/liveness для сервиса.
type Health struct {
	srv *health.Server
}

// New создаёт новый экземпляр Health с указанным начальным статусом.
// По умолчанию рекомендуется использовать NOT_SERVING для readiness,
// чтобы сервис не считался готовым до проверки зависимостей (БД и т.д.).
func New(initialStatus grpc_health_v1.HealthCheckResponse_ServingStatus) *Health {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", initialStatus)
	return &Health{srv: healthServer}
}

// Register регистрирует health service на gRPC сервере.
// Должно вызываться до запуска сервера (grpcSrv.Serve).
func (h *Health) Register(grpcSrv *grpc.Server) {
	grpc_health_v1.RegisterHealthServer(grpcSrv, h.srv)
}

// SetServing устанавливает статус health check на SERVING для указанного сервиса.
// Если serviceName пустая строка, устанавливается статус для всего сервера (overall).
// Используется для переключения readiness в SERVING после успешной проверки зависимостей.
func (h *Health) SetServing(serviceName string) {
	h.srv.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
}

// SetNotServing устанавливает статус health check на NOT_SERVING для указанного сервиса.
// Если serviceName пустая строка, устанавливается статус для всего сервера (overall).
// Используется для переключения readiness в NOT_SERVING при graceful shutdown
// или при потере соединения с зависимостями.
func (h *Health) SetNotServing(serviceName string) {
	h.srv.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
}

// RegisterHealthServer - устаревшая функция, оставлена для обратной совместимости.
// Используйте New() и Register() вместо неё.
// Deprecated: используйте Health.New() и Health.Register().
func RegisterHealthServer(s *grpc.Server, initialStatus grpc_health_v1.HealthCheckResponse_ServingStatus) *health.Server {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", initialStatus)
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	return healthServer
}
