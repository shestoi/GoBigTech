# Payment Service

Сервис обработки платежей для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/grpc/`) - gRPC обработчики
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **In-memory реализация** (`internal/repository/memory/`) - для разработки

## Запуск

```bash
go run ./cmd/payment
```

Сервис запускается на `127.0.0.1:50052` (gRPC).

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

