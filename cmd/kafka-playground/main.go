// Package main содержит тестовый Kafka producer playground.
//
// Это пример того, как должен выглядеть Kafka producer в проекте GoBigTech:
//   - использует платформенный логгер (platform/logging) на основе zap
//   - использует платформенную конфигурацию Kafka (platform/kafka)
//   - корректно работает с context.Context и таймаутами
//   - правильно закрывает ресурсы (writer)
//
// По умолчанию подключается к localhost:19092 и топику test-topic.
// Это можно переопределить через переменные окружения:
//   - KAFKA_BROKERS (например, "localhost:19092" или "kafka:9092" для Docker)
//   - KAFKA_TOPIC (например, "test-topic" или доменный топик "order.paid")
package main

import (
	"context"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	platformkafka "github.com/shestoi/GoBigTech/platform/kafka"
	platformlogging "github.com/shestoi/GoBigTech/platform/logging"
)

func main() {
	// Инициализируем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Инициализируем платформенный логгер
	logger, err := platformlogging.New(platformlogging.Config{
		ServiceName: "kafka-playground",
		Env:         "local",
		Level:       "info",
		Format:      "console",
		AddCaller:   true,
	})
	if err != nil {
		// Если не удалось инициализировать логгер, используем стандартный вывод и выходим
		os.Stderr.WriteString("Failed to initialize logger: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer platformlogging.Sync(logger)

	// Загружаем конфигурацию Kafka из переменных окружения
	// Если переменные не заданы, используются дефолты (localhost:19092, test-topic)
	cfg := platformkafka.DefaultConfig()
	if err := platformkafka.LoadEnv(&cfg); err != nil {
		logger.Error("failed to load kafka config", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("kafka config loaded",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("topic", cfg.Topic),
	)

	// Создаём Kafka writer с использованием конфигурации
	// Все параметры берутся из cfg (либо дефолты, либо переменные окружения)
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{}, // алгоритм балансировки нагрузки
	}
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Error("failed to close kafka writer", zap.Error(err))
		}
	}()

	// Подготавливаем сообщение
	message := kafka.Message{
		Key:   []byte("demo"),
		Value: []byte("hello from Go"),
	}

	// Отправляем сообщение
	logger.Info("sending message to kafka",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("topic", cfg.Topic),
		zap.String("key", string(message.Key)),
		zap.String("value", string(message.Value)),
	)

	err = writer.WriteMessages(ctx, message)
	if err != nil {
		logger.Error("failed to send message",
			zap.Error(err),
			zap.Strings("brokers", cfg.Brokers),
			zap.String("topic", cfg.Topic),
			zap.String("key", string(message.Key)),
			zap.String("value", string(message.Value)),
		)
		os.Exit(1) //выход с кодом ошибки 1 - критическая ошибка
	}

	logger.Info("message sent successfully",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("topic", cfg.Topic),
		zap.String("key", string(message.Key)),
		zap.String("value", string(message.Value)),
	)
}
