package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
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

// Config содержит конфигурацию Inventory Service
type Config struct {
	AppEnv               Env
	GRPCAddr             string
	MongoURI             string
	MongoDBName          string
	IAMGRPCAddr          string // адрес IAM Service для проверки сессий
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
		cfg.GRPCAddr = getString("GRPC_ADDR", "127.0.0.1:50051")
	} else {
		cfg.GRPCAddr = getString("GRPC_ADDR", "0.0.0.0:50051")
	}

	// INVENTORY_MONGO_URI
	if cfg.AppEnv == EnvLocal {
		cfg.MongoURI = getString("INVENTORY_MONGO_URI", "mongodb://inventory_user:inventory_password@127.0.0.1:15417/?authSource=admin")
	} else {
		cfg.MongoURI = getString("INVENTORY_MONGO_URI", "mongodb://inventory_user:inventory_password@mongo:27017/?authSource=admin")
	}

	// INVENTORY_MONGO_DB
	cfg.MongoDBName = getString("INVENTORY_MONGO_DB", "inventory")

	// IAM_GRPC_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.IAMGRPCAddr = getString("IAM_GRPC_ADDR", "127.0.0.1:50053")
	} else {
		cfg.IAMGRPCAddr = getString("IAM_GRPC_ADDR", "iam:50053")
	}

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
	if c.MongoURI == "" {
		return fmt.Errorf("INVENTORY_MONGO_URI is required")
	}
	if c.MongoDBName == "" {
		return fmt.Errorf("INVENTORY_MONGO_DB is required")
	}
	if c.IAMGRPCAddr == "" {
		return fmt.Errorf("IAM_GRPC_ADDR is required")
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
	log.Printf("  INVENTORY_MONGO_URI: %s", maskMongoURI(c.MongoURI))
	log.Printf("  INVENTORY_MONGO_DB: %s", c.MongoDBName)
	log.Printf("  IAM_GRPC_ADDR: %s", c.IAMGRPCAddr)
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
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// maskMongoURI маскирует пароль в MongoDB URI для безопасного логирования
func maskMongoURI(uri string) string {
	// Формат: mongodb://user:password@host:port/...
	masked := uri
	for i := 0; i < len(uri)-1; i++ {
		if uri[i] == ':' && i+1 < len(uri) && uri[i+1] != '/' {
			// Нашли начало пароля, ищем @
			for j := i + 1; j < len(uri); j++ {
				if uri[j] == '@' {
					masked = uri[:i+1] + "***" + uri[j:]
					break
				}
			}
			break
		}
	}
	return masked
}
