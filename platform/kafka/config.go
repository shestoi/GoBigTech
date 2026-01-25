package kafka

// Config содержит конфигурацию для подключения к Kafka
type Config struct {
	// Brokers — список брокеров Kafka, через который будут подключаться Go-сервисы.
	// Значение зависит от среды выполнения:
	//   - локальная разработка (go run): localhost:19092
	//   - запуск в Docker: kafka:9092
	// Можно указать несколько брокеров через запятую: "broker1:9092,broker2:9092"
	Brokers []string `env:"KAFKA_BROKERS" envSeparator:","`
	// Topic — базовый топик по умолчанию (для playground-а и тестов).
	// В продакшене сервисы будут использовать доменные топики (например, order.paid, payment.completed).
	Topic string `env:"KAFKA_TOPIC" envDefault:"test-topic"`
}

// DefaultConfig возвращает конфигурацию с дефолтными значениями для локальной разработки.
// Сервисы должны получать актуальные значения через переменные окружения (KAFKA_BROKERS, KAFKA_TOPIC).
func DefaultConfig() Config {
	return Config{
		Brokers: []string{"localhost:19092"},
		Topic:   "test-topic",
	}
}

