package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Manager управляет graceful shutdown сервиса
// Перехватывает SIGINT/SIGTERM и последовательно выполняет зарегистрированные shutdown функции
type Manager struct {
	timeout time.Duration
	logger  *zap.Logger
	funcs   []shutdownFunc
	mu      sync.Mutex
}

type shutdownFunc struct {
	name string
	fn   func(context.Context) error
}

// New создаёт новый Manager с указанным таймаутом и logger
func New(timeout time.Duration, logger *zap.Logger) *Manager {
	return &Manager{
		timeout: timeout,
		logger:  logger,
		funcs:   make([]shutdownFunc, 0),
	}
}

// Add регистрирует shutdown функцию с указанным именем
// Функции будут выполнены в порядке регистрации при получении сигнала
func (m *Manager) Add(name string, fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.funcs = append(m.funcs, shutdownFunc{name: name, fn: fn})
}

// Wait блокирует выполнение до получения SIGINT или SIGTERM,
// затем последовательно выполняет все зарегистрированные shutdown функции
// Каждая функция выполняется с context.WithTimeout
func (m *Manager) Wait() {
	// Создаём канал для сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Ожидаем сигнал
	<-sigChan
	m.logger.Info("Received shutdown signal, starting graceful shutdown")

	// Выполняем все зарегистрированные функции последовательно
	m.mu.Lock()
	funcs := make([]shutdownFunc, len(m.funcs))
	copy(funcs, m.funcs)
	m.mu.Unlock()

	for i := len(funcs) - 1; i >= 0; i-- {
		fn := funcs[i]
		m.logger.Info("Executing shutdown function", zap.String("name", fn.name))

		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		start := time.Now()

		err := fn.fn(ctx) //выполняем shutdown функцию
		cancel()          //отменяем контекст

		duration := time.Since(start) //время выполнения
		if err != nil {
			m.logger.Error("Shutdown function failed",
				zap.String("name", fn.name),
				zap.Error(err),
				zap.Duration("duration", duration))
		} else {
			m.logger.Info("Shutdown function completed",
				zap.String("name", fn.name),
				zap.Duration("duration", duration))
		}
	}

	m.logger.Info("Graceful shutdown completed")
}

// ShutdownHTTPServer возвращает shutdown функцию для http.Server
func ShutdownHTTPServer(srv interface {
	Shutdown(context.Context) error
}) func(context.Context) error {
	return func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
}

// ShutdownGRPCServer возвращает shutdown функцию для gRPC сервера
// Выполняет GracefulStop с таймаутом, при превышении таймаута вызывает Stop()
func ShutdownGRPCServer(srv interface {
	GracefulStop()
	Stop()
}) func(context.Context) error {
	return func(ctx context.Context) error {
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			srv.Stop()
			return fmt.Errorf("graceful stop timeout exceeded, forced stop")
		}
	}
}

// DisconnectMongo возвращает shutdown функцию для MongoDB клиента
func DisconnectMongo(client interface {
	Disconnect(context.Context) error
}) func(context.Context) error {
	return func(ctx context.Context) error {
		return client.Disconnect(ctx)
	}
}

// ClosePool возвращает shutdown функцию для закрытия connection pool
func ClosePool(pool interface {
	Close()
}) func(context.Context) error {
	return func(ctx context.Context) error {
		pool.Close()
		return nil
	}
}

// SetHealthNotServing возвращает shutdown функцию для установки health в NOT_SERVING
func SetHealthNotServing(health interface {
	SetNotServing(string)
}) func(context.Context) error {
	return func(ctx context.Context) error {
		health.SetNotServing("")
		return nil
	}
}
