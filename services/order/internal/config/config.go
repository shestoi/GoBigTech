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

	// Kafka
	Brokers                          []string      //список брокеров Kafka
	PaymentCompletedTopic            string        //топик для оплаты заказа
	AssemblyCompletedTopic           string        //топик для событий завершения сборки заказа
	OrderConsumerGroupID             string        //consumer group ID для Order Service
	AssemblyConsumerRetryMaxAttempts int           //максимальное количество попыток retry для assembly consumer
	AssemblyConsumerRetryBackoffBase time.Duration //базовый интервал для backoff retry
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

	// Kafka
	brokersStr := getString("KAFKA_BROKERS", "") //получаем список брокеров из переменных окружения
	if brokersStr != "" {
		// Парсим список брокеров через запятую
		brokers := []string{}                                 //создаём пустой слайс для брокеров
		for _, broker := range splitString(brokersStr, ",") { //разбиваем строку на брокеры по запятой
			broker = trimSpace(broker) //убираем пробелы в начале и конце строки
			if broker != "" {
				brokers = append(brokers, broker)
			}
		}
		if len(brokers) > 0 {
			cfg.Brokers = brokers
		}
	}
	// Если не задано, используем дефолт в зависимости от окружения
	if len(cfg.Brokers) == 0 {
		if cfg.AppEnv == EnvLocal {
			cfg.Brokers = []string{"localhost:19092"}
		} else {
			cfg.Brokers = []string{"kafka:9092"}
		}
	}
	cfg.PaymentCompletedTopic = getString("KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC", "order.payment.completed")
	cfg.AssemblyCompletedTopic = getString("KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC", "order.assembly.completed")
	cfg.OrderConsumerGroupID = getString("KAFKA_ORDER_CONSUMER_GROUP_ID", "order-service")

	// Retry настройки для assembly consumer (order <- order.assembly.completed)
	retryMaxAttemptsStr := getString("ORDER_KAFKA_RETRY_MAX_ATTEMPTS", "3")
	retryMaxAttempts, err := parseInt(retryMaxAttemptsStr, 3)
	if err != nil {
		return Config{}, fmt.Errorf("invalid ORDER_KAFKA_RETRY_MAX_ATTEMPTS: %w", err)
	}
	cfg.AssemblyConsumerRetryMaxAttempts = retryMaxAttempts

	retryBackoffBaseStr := getString("ORDER_KAFKA_RETRY_BACKOFF_BASE", "1s")
	retryBackoffBase, err := time.ParseDuration(retryBackoffBaseStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid ORDER_KAFKA_RETRY_BACKOFF_BASE: %w", err)
	}
	cfg.AssemblyConsumerRetryBackoffBase = retryBackoffBase

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
	if len(c.Brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.PaymentCompletedTopic == "" {
		return fmt.Errorf("KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC is required")
	}
	if c.AssemblyCompletedTopic == "" {
		return fmt.Errorf("KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC is required")
	}
	if c.OrderConsumerGroupID == "" {
		return fmt.Errorf("KAFKA_ORDER_CONSUMER_GROUP_ID is required")
	}
	if c.AssemblyConsumerRetryMaxAttempts <= 0 {
		return fmt.Errorf("ORDER_KAFKA_RETRY_MAX_ATTEMPTS must be positive")
	}
	if c.AssemblyConsumerRetryBackoffBase <= 0 {
		return fmt.Errorf("ORDER_KAFKA_RETRY_BACKOFF_BASE must be positive")
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
	log.Printf("  KAFKA_BROKERS: %v", c.Brokers)
	log.Printf("  KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC: %s", c.PaymentCompletedTopic)
	log.Printf("  KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC: %s", c.AssemblyCompletedTopic)
	log.Printf("  KAFKA_ORDER_CONSUMER_GROUP_ID: %s", c.OrderConsumerGroupID)
	log.Printf("  ORDER_KAFKA_RETRY_MAX_ATTEMPTS: %d", c.AssemblyConsumerRetryMaxAttempts)
	log.Printf("  ORDER_KAFKA_RETRY_BACKOFF_BASE: %s", c.AssemblyConsumerRetryBackoffBase)
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

// splitString разбивает строку по разделителю
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep { //если текущая позиция + длина разделителя меньше или равна длине строки и текущая подстрока равна разделителю
			result = append(result, s[start:i]) //добавляем подстроку в результат
			start = i + len(sep)
			i += len(sep) - 1 //увеличиваем i на длину разделителя - 1
		}
	}
	result = append(result, s[start:]) //добавляем последнюю подстроку в результат
	return result                      //возвращаем результат
}

// trimSpace удаляет пробелы в начале и конце строки
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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
