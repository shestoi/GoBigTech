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

// Config содержит конфигурацию Payment Service
type Config struct {
	AppEnv              Env
	GRPCAddr            string
	EnableGRPCReflection bool
	ShutdownTimeout     time.Duration
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
		cfg.GRPCAddr = getString("GRPC_ADDR", "127.0.0.1:50052")
	} else {
		cfg.GRPCAddr = getString("GRPC_ADDR", "0.0.0.0:50052")
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
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

// Log выводит конфигурацию в лог
func (c Config) Log() {
	log.Printf("Config loaded:")
	log.Printf("  APP_ENV: %s", c.AppEnv)
	log.Printf("  GRPC_ADDR: %s", c.GRPCAddr)
	log.Printf("  ENABLE_GRPC_REFLECTION: %v", c.EnableGRPCReflection)
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

