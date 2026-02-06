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

// Config содержит конфигурацию IAM Service
type Config struct {
	AppEnv               Env
	GRPCAddr             string
	PostgresDSN          string
	RedisAddr            string        // для будущего использования
	RedisPassword        string        // для будущего использования
	SessionTTL           time.Duration // для будущего использования
	EnableGRPCReflection bool
	ShutdownTimeout      time.Duration

	// OpenTelemetry
	OTelEnabled       bool
	OTelEndpoint      string
	OTelSamplingRatio float64
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

	// GRPC_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.GRPCAddr = getString("GRPC_ADDR", "127.0.0.1:50053")
	} else {
		cfg.GRPCAddr = getString("GRPC_ADDR", "0.0.0.0:50053")
	}

	// IAM_POSTGRES_DSN
	if cfg.AppEnv == EnvLocal {
		cfg.PostgresDSN = getString("IAM_POSTGRES_DSN", "postgres://iam_user:iam_password@127.0.0.1:15433/iam?sslmode=disable")
	} else {
		cfg.PostgresDSN = getString("IAM_POSTGRES_DSN", "postgres://iam_user:iam_password@iam-postgres:5432/iam?sslmode=disable")
	}

	// Redis (для будущего использования)
	if cfg.AppEnv == EnvLocal {
		cfg.RedisAddr = getString("REDIS_ADDR", "127.0.0.1:16379")
	} else {
		cfg.RedisAddr = getString("REDIS_ADDR", "redis:6379")
	}
	cfg.RedisPassword = getString("REDIS_PASSWORD", "") // для будущего использования

	// SESSION_TTL (для будущего использования)
	sessionTTLStr := getString("SESSION_TTL", "24h")
	sessionTTL, err := time.ParseDuration(sessionTTLStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SESSION_TTL: %w", err)
	}
	cfg.SessionTTL = sessionTTL

	// ENABLE_GRPC_REFLECTION
	cfg.EnableGRPCReflection = getBool("ENABLE_GRPC_REFLECTION", false)

	// SHUTDOWN_TIMEOUT
	shutdownTimeoutStr := getString("SHUTDOWN_TIMEOUT", "5s")
	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	// OpenTelemetry
	cfg.OTelEnabled = getBool("OTEL_ENABLED", false)
	if cfg.AppEnv == EnvLocal {
		cfg.OTelEndpoint = getString("OTEL_EXPORTER_OTLP_ENDPOINT", "127.0.0.1:4317")
	} else {
		cfg.OTelEndpoint = getString("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	}
	cfg.OTelSamplingRatio = getFloat64("OTEL_SAMPLING_RATIO", 1.0)

	// Валидация
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate проверяет корректность конфигурации
func (c Config) Validate() error {
	if c.GRPCAddr == "" {
		return fmt.Errorf("GRPC_ADDR is required")
	}
	if c.PostgresDSN == "" {
		return fmt.Errorf("IAM_POSTGRES_DSN is required")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	if c.OTelEnabled && (c.OTelSamplingRatio < 0 || c.OTelSamplingRatio > 1) {
		return fmt.Errorf("OTEL_SAMPLING_RATIO must be in [0, 1]")
	}
	return nil
}

// Log выводит конфигурацию в лог (с маскировкой паролей)
func (c Config) Log() {
	log.Printf("Config loaded:")
	log.Printf("  APP_ENV: %s", c.AppEnv)
	log.Printf("  GRPC_ADDR: %s", c.GRPCAddr)
	log.Printf("  IAM_POSTGRES_DSN: %s", maskDSN(c.PostgresDSN))
	log.Printf("  REDIS_ADDR: %s", c.RedisAddr)
	log.Printf("  SESSION_TTL: %s", c.SessionTTL)
	log.Printf("  ENABLE_GRPC_REFLECTION: %v", c.EnableGRPCReflection)
	log.Printf("  SHUTDOWN_TIMEOUT: %s", c.ShutdownTimeout)
	log.Printf("  OTEL_ENABLED: %v", c.OTelEnabled)
	log.Printf("  OTEL_EXPORTER_OTLP_ENDPOINT: %s", c.OTelEndpoint)
	log.Printf("  OTEL_SAMPLING_RATIO: %f", c.OTelSamplingRatio)
}

func getFloat64(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	var f float64
	_, err := fmt.Sscanf(value, "%f", &f)
	if err != nil {
		return defaultValue
	}
	return f
}

// getString читает переменную окружения или возвращает дефолт
func getString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getBool читает булеву переменную окружения или возвращает дефолт
func getBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := parseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// parseBool парсит строку в bool
func parseBool(s string) (bool, error) {
	switch s {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool value: %s", s)
	}
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
