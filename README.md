# GoBigTech
GoBigTech — учебный production-like проект микросервисной backend-системы на Go.
Проект моделирует процесс оформления заказа: создание заказа, проверка наличия товара, оплата, сборка заказа, уведомления пользователя и проверка доступа через IAM.

## Стек
- Go
- HTTP, gRPC, protobuf
- PostgreSQL, MongoDB, Redis
- Kafka KRaft
- Docker, Docker Compose
- Envoy Gateway
- OpenAPI, gRPC-Gateway, buf, protoc-gen-validate
- Zap, OpenTelemetry, Prometheus, Grafana, Jaeger
- Testify, mockery, testcontainers-go
- CI/CD: GitHub Actions

## Архитектура
В проекте реализованы сервисы:
- Order Service — создание и управление заказами
- Inventory Service — проверка остатков
- Payment Service — обработка оплаты
- IAM Service — пользователи, сессии, авторизация
- Assembly Service — асинхронная сборка заказа
- Notification Service — уведомления пользователя
  Сервисы взаимодействуют через HTTP/gRPC и Kafka.

## Что реализовано
- микросервисная архитектура
- HTTP и gRPC API
- кодогенерация из OpenAPI и protobuf
- PostgreSQL и MongoDB репозитории
- Redis-сессии с TTL
- Kafka producer/consumer
- DI-контейнер
- unit, integration и e2e тесты
- observability: logs, metrics, traces
- Envoy как единая точка входа
- Docker Compose для инфраструктуры

## Быстрый запуск
```
make up
```

Тесты
```
make test
make test-integration
make test-e2e
```
Kafka

Kafka работает в KRaft-режиме без ZooKeeper.
```
docker compose -f docker-compose.kafka.yml up -d
```
Авторизация

Защищённые gRPC-методы требуют session_id в metadata:

```-H "x-session-id: ${SESSION_ID}"```

Получить session_id можно через IAM Service: Register → Login.

Цель проекта

Цель проекта — не просто реализовать CRUD, а собрать backend-систему, приближенную к реальной production-разработке: с контрактами API, слоями, тестами, инфраструктурой, асинхронным взаимодействием, авторизацией и наблюдаемостью.

