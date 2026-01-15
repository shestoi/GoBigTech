# Payment Service

Сервис обработки платежей для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/grpc/`) - gRPC обработчики
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **In-memory реализация** (`internal/repository/memory/`) - для разработки

## DI Container / App Builder

Сервис использует явный builder паттерн для dependency injection через пакет `internal/app`.

### Структура

- **`internal/app/app.go`** - содержит структуру `App` и функцию `Build(cfg)` для создания всех зависимостей
- **`cmd/payment/main.go`** - минимальный entry point, который вызывает `app.Build()` и `app.Run()`

### Зависимости, собираемые в Build()

Функция `Build(cfg)` создаёт и настраивает следующие зависимости:

1. **Logger** - platform logger (zap) с конфигурацией из env
2. **Repository** - in-memory реализация PaymentRepository
3. **Service** - PaymentService с внедрённым repository
4. **gRPC handler** - gRPC обработчики с service
5. **gRPC server** - настроенный grpc.Server с reflection (если включено)
6. **Health check** - gRPC health service с начальным статусом SERVING (нет внешних зависимостей)
7. **Listener** - сетевой listener для gRPC сервера
8. **Shutdown manager** - platform shutdown manager с зарегистрированными функциями

Payment Service не имеет внешних зависимостей (БД), поэтому health check сразу устанавливается в SERVING.

## Запуск

```bash
go run ./cmd/payment
```

Сервис запускается на `127.0.0.1:50052` (gRPC).

## Health Check

Сервис использует стандартный gRPC health service (`grpc.health.v1.Health`) для проверки готовности.

### Проверка health через grpcurl

```bash
# Проверка health status
grpcurl -plaintext 127.0.0.1:50052 grpc.health.v1.Health/Check

# Ожидаемый ответ:
# {
#   "status": "SERVING"
# }
```

### Readiness и Liveness

- **Liveness**: Процесс жив (всегда SERVING после старта сервера)
- **Readiness**: Готов обслуживать запросы
  - Начальный статус: `SERVING` (нет внешних зависимостей)
  - При graceful shutdown: `NOT_SERVING` → остановка сервера

### Graceful Shutdown

При получении SIGINT/SIGTERM:
1. Readiness переключается в `NOT_SERVING`
2. gRPC сервер останавливается gracefully (GracefulStop)
3. Сервис завершает работу

## Тесты

### Запуск тестов

```bash
go test ./...
```

### Запуск с подробным выводом

```bash
go test ./... -v
```

## Coverage

### Получение coverage

```bash
# Генерация coverage файла
go test ./... -coverprofile=coverage.out

# Просмотр детального отчёта
go tool cover -func=coverage.out

# Просмотр HTML отчёта (опционально)
go tool cover -html=coverage.out
```

### Текущее покрытие

**Service слой**: 100.0% покрытие

- `internal/service/service.go` - полностью покрыт unit-тестами
- Все сценарии `ProcessPayment()` протестированы:
  - Валидация amount <= 0
  - Идемпотентность (возврат существующей транзакции)
  - Создание новой транзакции
  - Обработка ошибок repository

**Общее покрытие**: 12.2% (только service слой покрыт тестами)

> Примечание: Coverage включает все пакеты (cmd, api, repository, v1). 
> Service слой имеет 100% покрытие, что является основной целью для unit-тестов.

## Генерация моков

Моки для интерфейсов генерируются через [mockery](https://github.com/vektra/mockery).

### Установка mockery

```bash
go install github.com/vektra/mockery/v2@latest
```

### Генерация мока для PaymentRepository

```bash
go run github.com/vektra/mockery/v2@latest \
  --name=PaymentRepository \
  --dir=./internal/repository \
  --output=./internal/repository/mocks \
  --outpkg=mocks
```

Мок будет создан в `internal/repository/mocks/PaymentRepository.go`.


