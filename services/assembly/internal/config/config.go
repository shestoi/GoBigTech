package config

import (
	"fmt"
	"log"
	"os"
	"strings"
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

// Config содержит конфигурацию Assembly Service
type Config struct {
	AppEnv          Env
	ShutdownTimeout time.Duration

	// Kafka
	KafkaBrokers           []string
	PaymentCompletedTopic  string // входной топик (order.payment.completed)
	AssemblyCompletedTopic string // выходной топик (order.assembly.completed)
	DLQTopic               string // топик для dead letter queue
	ConsumerGroupID        string

	// Retry
	RetryMaxAttempts int           // максимальное количество попыток
	RetryBackoffBase time.Duration // базовый интервал для backoff
}

// Load загружает конфигурацию из переменных окружения
func Load() (Config, error) {
	cfg := Config{}

	// Читаем APP_ENV
	appEnvStr := getString("APP_ENV", string(EnvLocal))
	appEnv := Env(appEnvStr)
	if appEnv != EnvLocal && appEnv != EnvDocker {
		return Config{}, fmt.Errorf("invalid APP_ENV: %s (must be 'local' or 'docker')", appEnvStr)
	}
	cfg.AppEnv = appEnv

	// SHUTDOWN_TIMEOUT
	shutdownTimeoutStr := getString("SHUTDOWN_TIMEOUT", "10s")
	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutStr) //парсим строку в duration
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	// Kafka Brokers
	brokersStr := getString("KAFKA_BROKERS", "")
	if brokersStr != "" {
		// Парсим список брокеров через запятую
		brokers := []string{}
		for _, broker := range strings.Split(brokersStr, ",") {
			broker = strings.TrimSpace(broker)
			if broker != "" {
				brokers = append(brokers, broker)
			}
		}
		if len(brokers) > 0 {
			cfg.KafkaBrokers = brokers
		}
	}
	// Если не задано, используем дефолт в зависимости от окружения
	if len(cfg.KafkaBrokers) == 0 {
		if cfg.AppEnv == EnvLocal {
			cfg.KafkaBrokers = []string{"localhost:19092"}
		} else {
			cfg.KafkaBrokers = []string{"kafka:9092"}
		}
	}

	// Kafka Topics
	cfg.PaymentCompletedTopic = getString("KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC", "order.payment.completed")
	cfg.AssemblyCompletedTopic = getString("KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC", "order.assembly.completed")
	cfg.DLQTopic = getString("KAFKA_ORDER_PAYMENT_COMPLETED_DLQ_TOPIC", "order.payment.completed.dlq")
	cfg.ConsumerGroupID = getString("KAFKA_ASSEMBLY_CONSUMER_GROUP_ID", "assembly-service")

	// Retry
	retryMaxAttemptsStr := getString("KAFKA_RETRY_MAX_ATTEMPTS", "3")
	retryMaxAttempts, err := parseInt(retryMaxAttemptsStr, 3)
	if err != nil {
		return Config{}, fmt.Errorf("invalid KAFKA_RETRY_MAX_ATTEMPTS: %w", err)
	}
	cfg.RetryMaxAttempts = retryMaxAttempts

	retryBackoffBaseStr := getString("KAFKA_RETRY_BACKOFF_BASE", "1s")
	retryBackoffBase, err := time.ParseDuration(retryBackoffBaseStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid KAFKA_RETRY_BACKOFF_BASE: %w", err)
	}
	cfg.RetryBackoffBase = retryBackoffBase

	// Валидация
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate проверяет корректность конфигурации
func (c Config) Validate() error {
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.PaymentCompletedTopic == "" {
		return fmt.Errorf("KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC is required")
	}
	if c.AssemblyCompletedTopic == "" {
		return fmt.Errorf("KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC is required")
	}
	if c.ConsumerGroupID == "" {
		return fmt.Errorf("KAFKA_ASSEMBLY_CONSUMER_GROUP_ID is required")
	}
	if c.DLQTopic == "" {
		return fmt.Errorf("KAFKA_ORDER_PAYMENT_COMPLETED_DLQ_TOPIC is required")
	}
	if c.RetryMaxAttempts <= 0 {
		return fmt.Errorf("KAFKA_RETRY_MAX_ATTEMPTS must be positive")
	}
	if c.RetryBackoffBase <= 0 {
		return fmt.Errorf("KAFKA_RETRY_BACKOFF_BASE must be positive")
	}
	return nil
}

// Log выводит конфигурацию в лог
func (c Config) Log() {
	log.Printf("Config loaded:")
	log.Printf("  APP_ENV: %s", c.AppEnv)
	log.Printf("  SHUTDOWN_TIMEOUT: %s", c.ShutdownTimeout)
	log.Printf("  KAFKA_BROKERS: %v", c.KafkaBrokers)
	log.Printf("  KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC: %s", c.PaymentCompletedTopic)
	log.Printf("  KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC: %s", c.AssemblyCompletedTopic)
	log.Printf("  KAFKA_ORDER_PAYMENT_COMPLETED_DLQ_TOPIC: %s", c.DLQTopic)
	log.Printf("  KAFKA_ASSEMBLY_CONSUMER_GROUP_ID: %s", c.ConsumerGroupID)
	log.Printf("  KAFKA_RETRY_MAX_ATTEMPTS: %d", c.RetryMaxAttempts)
	log.Printf("  KAFKA_RETRY_BACKOFF_BASE: %s", c.RetryBackoffBase)
}

// getString читает переменную окружения или возвращает дефолт
func getString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// parseInt парсит строку в int, при ошибке возвращает defaultValue
func parseInt(s string, defaultValue int) (int, error) {
	if s == "" {
		return defaultValue, nil
	}
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return defaultValue, err
	}
	return result, nil
}
