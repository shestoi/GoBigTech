package kafka

import (
	"github.com/caarlos0/env/v10"
)

// LoadEnv загружает конфигурацию из переменных окружения
// Использует пакет caarlos0/env/v10 для парсинга env-тегов
func LoadEnv(cfg *Config) error {
	return env.Parse(cfg)
}
