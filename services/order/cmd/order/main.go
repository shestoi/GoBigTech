package main

import (
	"log"

	"github.com/shestoi/GoBigTech/services/order/internal/app"
	"github.com/shestoi/GoBigTech/services/order/internal/config"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Создаём и настраиваем приложение через DI container
	application, err := app.Build(cfg) //Build собирает граф зависимостей и инициализирует все компоненты
	if err != nil {
		log.Fatalf("Failed to build app: %v", err)
	}

	// Запускаем сервис
	if err := application.Run(); err != nil {
		log.Fatalf("Service error: %v", err)
	} //Run запускает сервис и блокируется до graceful shutdown.
}
