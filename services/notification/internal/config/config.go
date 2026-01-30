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

// Config содержит конфигурацию Notification Service
type Config struct {
	AppEnv          Env
	ShutdownTimeout time.Duration
	PostgresDSN     string

	// Kafka
	KafkaBrokers                      []string
	PaymentCompletedTopic             string
	AssemblyCompletedTopic            string
	NotificationPaymentGroupID        string
	NotificationAssemblyGroupID       string
	NotificationKafkaRetryMaxAttempts int
	NotificationKafkaRetryBackoffBase time.Duration
	DLQTopic                          string

	// Telegram
	TelegramBotToken string
	TelegramChatID   string
	TelegramEnabled  bool

	// Templates
	TemplatesDir string

	// IAM
	IAMGRPCAddr string // адрес IAM Service для получения контактной информации пользователей
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
	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	// POSTGRES_DSN
	if cfg.AppEnv == EnvLocal {
		cfg.PostgresDSN = getString("NOTIFICATION_POSTGRES_DSN", "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable")
	} else {
		cfg.PostgresDSN = getString("NOTIFICATION_POSTGRES_DSN", "postgres://order_user:order_password@postgres:5432/orders?sslmode=disable")
	}

	// Kafka Brokers
	brokersStr := getString("KAFKA_BROKERS", "")
	if brokersStr != "" {
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

	// Consumer Group IDs
	cfg.NotificationPaymentGroupID = getString("KAFKA_NOTIFICATION_PAYMENT_GROUP_ID", "notification-payment")
	cfg.NotificationAssemblyGroupID = getString("KAFKA_NOTIFICATION_ASSEMBLY_GROUP_ID", "notification-assembly")

	// Retry настройки
	retryMaxAttemptsStr := getString("NOTIFICATION_KAFKA_RETRY_MAX_ATTEMPTS", "3")
	retryMaxAttempts, err := parseInt(retryMaxAttemptsStr, 3)
	if err != nil {
		return Config{}, fmt.Errorf("invalid NOTIFICATION_KAFKA_RETRY_MAX_ATTEMPTS: %w", err)
	}
	cfg.NotificationKafkaRetryMaxAttempts = retryMaxAttempts

	retryBackoffBaseStr := getString("NOTIFICATION_KAFKA_RETRY_BACKOFF_BASE", "1s")
	retryBackoffBase, err := time.ParseDuration(retryBackoffBaseStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid NOTIFICATION_KAFKA_RETRY_BACKOFF_BASE: %w", err)
	}
	cfg.NotificationKafkaRetryBackoffBase = retryBackoffBase

	// DLQ Topic
	cfg.DLQTopic = getString("KAFKA_NOTIFICATION_DLQ_TOPIC", "notification.dlq")

	// Telegram
	telegramEnabledStr := getString("TELEGRAM_ENABLED", "false")
	cfg.TelegramEnabled = telegramEnabledStr == "true" || telegramEnabledStr == "1"
	cfg.TelegramBotToken = getString("TELEGRAM_BOT_TOKEN", "8523796732:AAEkeA6oFQrQNBpl6DYekxK-wbn83bQL9Jg")
	cfg.TelegramChatID = getString("TELEGRAM_CHAT_ID", "6721014060")

	// Templates directory
	cfg.TemplatesDir = getString("TEMPLATES_DIR", "./templates")

	// IAM_GRPC_ADDR
	if cfg.AppEnv == EnvLocal {
		cfg.IAMGRPCAddr = getString("IAM_GRPC_ADDR", "127.0.0.1:50053")
	} else {
		cfg.IAMGRPCAddr = getString("IAM_GRPC_ADDR", "iam:50053")
	}

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
	if c.PostgresDSN == "" {
		return fmt.Errorf("NOTIFICATION_POSTGRES_DSN is required")
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
	if c.NotificationPaymentGroupID == "" {
		return fmt.Errorf("KAFKA_NOTIFICATION_PAYMENT_GROUP_ID is required")
	}
	if c.NotificationAssemblyGroupID == "" {
		return fmt.Errorf("KAFKA_NOTIFICATION_ASSEMBLY_GROUP_ID is required")
	}
	if c.NotificationKafkaRetryMaxAttempts <= 0 {
		return fmt.Errorf("NOTIFICATION_KAFKA_RETRY_MAX_ATTEMPTS must be positive")
	}
	if c.NotificationKafkaRetryBackoffBase <= 0 {
		return fmt.Errorf("NOTIFICATION_KAFKA_RETRY_BACKOFF_BASE must be positive")
	}
	if c.DLQTopic == "" {
		return fmt.Errorf("KAFKA_NOTIFICATION_DLQ_TOPIC is required")
	}
	// Валидация Telegram: если enabled, то token и chat_id обязательны
	if c.TelegramEnabled {
		if c.TelegramBotToken == "" {
			return fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_ENABLED=true")
		}
		if c.TelegramChatID == "" {
			return fmt.Errorf("TELEGRAM_CHAT_ID is required when TELEGRAM_ENABLED=true")
		}
	}
	if c.TemplatesDir == "" {
		return fmt.Errorf("TEMPLATES_DIR is required")
	}
	if c.IAMGRPCAddr == "" {
		return fmt.Errorf("IAM_GRPC_ADDR is required")
	}
	return nil
}

// Log выводит конфигурацию в лог
func (c Config) Log() {
	log.Printf("Config loaded:")
	log.Printf("  APP_ENV: %s", c.AppEnv)
	log.Printf("  SHUTDOWN_TIMEOUT: %s", c.ShutdownTimeout)
	log.Printf("  NOTIFICATION_POSTGRES_DSN: %s", maskDSN(c.PostgresDSN))
	log.Printf("  KAFKA_BROKERS: %v", c.KafkaBrokers)
	log.Printf("  KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC: %s", c.PaymentCompletedTopic)
	log.Printf("  KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC: %s", c.AssemblyCompletedTopic)
	log.Printf("  KAFKA_NOTIFICATION_PAYMENT_GROUP_ID: %s", c.NotificationPaymentGroupID)
	log.Printf("  KAFKA_NOTIFICATION_ASSEMBLY_GROUP_ID: %s", c.NotificationAssemblyGroupID)
	log.Printf("  NOTIFICATION_KAFKA_RETRY_MAX_ATTEMPTS: %d", c.NotificationKafkaRetryMaxAttempts)
	log.Printf("  NOTIFICATION_KAFKA_RETRY_BACKOFF_BASE: %s", c.NotificationKafkaRetryBackoffBase)
	log.Printf("  NOTIFICATION_DLQ_TOPIC: %s", c.DLQTopic)
	log.Printf("  TELEGRAM_ENABLED: %v", c.TelegramEnabled)
	if c.TelegramEnabled {
		log.Printf("  TELEGRAM_BOT_TOKEN: %s", maskToken(c.TelegramBotToken))
		log.Printf("  TELEGRAM_CHAT_ID: %s", c.TelegramChatID)
	}
	log.Printf("  TEMPLATES_DIR: %s", c.TemplatesDir)
	log.Printf("  IAM_GRPC_ADDR: %s", c.IAMGRPCAddr)
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

// maskDSN маскирует пароль в DSN для безопасного логирования
func maskDSN(dsn string) string {
	masked := dsn
	for i := 0; i < len(dsn)-1; i++ {
		if dsn[i] == ':' && i+1 < len(dsn) && dsn[i+1] != '/' {
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

// maskToken маскирует токен для безопасного логирования
func maskToken(token string) string {
	if len(token) == 0 {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
