package main

import (
	"log"

	"github.com/shestoi/GoBigTech/services/notification/internal/app"
	"github.com/shestoi/GoBigTech/services/notification/internal/config"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Выводим конфигурацию в лог
	cfg.Log()

	// Создаём и настраиваем приложение через DI container
	application, err := app.Build(cfg)
	if err != nil {
		log.Fatalf("Failed to build app: %v", err)
	}

	// Запускаем сервис
	if err := application.Run(); err != nil {
		log.Fatalf("Service error: %v", err)
	}
}
