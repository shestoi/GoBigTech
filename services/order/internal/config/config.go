package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Env представляет окружение приложения
type Env string

const (
	// EnvLocal - локальное окружение (для разработки на хосте)
	EnvLocal Env = "local"
	// EnvDocker - Docker окружение (для запуска в контейнерах)
	EnvDocker Env = "docker"
)

// Config содержит конфигурацию Order Service
type Config struct {
	AppEnv            Env
	HTTPAddr          string
	PostgresDSN       string
	InventoryGRPCAddr string
	PaymentGRPCAddr   string
	ShutdownTimeout   time.Duration
}

// Load загружает конфигурацию из переменных окружения
// Читает APP_ENV и устанавливает дефолты в зависимости от окружения
func Load() (Config, error) {
	cfg := Config{}

	// Читаем APP_ENV
	appEnvStr := getString("APP_ENV", string(EnvLocal))
	appEnv := Env(appEnvStr)
	if appEnv != EnvLocal && appEnv != EnvDocker {
		return Config{}, fmt.Errorf("invalid APP_ENV: %s (must be 'local' or 'docker')", appEnvStr)
	}
	cfg.AppEnv = appEnv

	// HTTP_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.HTTPAddr = getString("HTTP_ADDR", "127.0.0.1:8080")
	} else {
		cfg.HTTPAddr = getString("HTTP_ADDR", "0.0.0.0:8080")
	}

	// ORDER_POSTGRES_DSN
	if cfg.AppEnv == EnvLocal {
		cfg.PostgresDSN = getString("ORDER_POSTGRES_DSN", "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable")
	} else {
		cfg.PostgresDSN = getString("ORDER_POSTGRES_DSN", "postgres://order_user:order_password@postgres:5432/orders?sslmode=disable")
	}

	// INVENTORY_GRPC_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.InventoryGRPCAddr = getString("INVENTORY_GRPC_ADDR", "127.0.0.1:50051")
	} else {
		cfg.InventoryGRPCAddr = getString("INVENTORY_GRPC_ADDR", "inventory:50051")
	}

	// PAYMENT_GRPC_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.PaymentGRPCAddr = getString("PAYMENT_GRPC_ADDR", "127.0.0.1:50052")
	} else {
		cfg.PaymentGRPCAddr = getString("PAYMENT_GRPC_ADDR", "payment:50052")
	}

	// SHUTDOWN_TIMEOUT
	shutdownTimeoutStr := getString("SHUTDOWN_TIMEOUT", "5s")
	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	// Валидация
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate проверяет корректность конфигурации
func (c Config) Validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	if c.PostgresDSN == "" {
		return fmt.Errorf("ORDER_POSTGRES_DSN is required")
	}
	if c.InventoryGRPCAddr == "" {
		return fmt.Errorf("INVENTORY_GRPC_ADDR is required")
	}
	if c.PaymentGRPCAddr == "" {
		return fmt.Errorf("PAYMENT_GRPC_ADDR is required")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

// Log выводит конфигурацию в лог (с маскировкой паролей)
func (c Config) Log() {
	log.Printf("Config loaded:")
	log.Printf("  APP_ENV: %s", c.AppEnv)
	log.Printf("  HTTP_ADDR: %s", c.HTTPAddr)
	log.Printf("  ORDER_POSTGRES_DSN: %s", maskDSN(c.PostgresDSN))
	log.Printf("  INVENTORY_GRPC_ADDR: %s", c.InventoryGRPCAddr)
	log.Printf("  PAYMENT_GRPC_ADDR: %s", c.PaymentGRPCAddr)
	log.Printf("  SHUTDOWN_TIMEOUT: %s", c.ShutdownTimeout)
}

// getString читает переменную окружения или возвращает дефолт
func getString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// maskDSN маскирует пароль в DSN для безопасного логирования
func maskDSN(dsn string) string {
	// Формат: postgres://user:password@host:port/db
	masked := dsn
	for i := 0; i < len(dsn)-1; i++ {
		if dsn[i] == ':' && i+1 < len(dsn) && dsn[i+1] != '/' {
			// Нашли начало пароля, ищем @
			for j := i + 1; j < len(dsn); j++ {
				if dsn[j] == '@' {
					masked = dsn[:i+1] + "***" + dsn[j:]
					break
				}
			}
			break
		}
	}
	return masked
}

