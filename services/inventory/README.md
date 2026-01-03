# Inventory Service

Сервис управления инвентарём для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/grpc/`) - gRPC обработчики
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **In-memory реализация** (`internal/repository/memory/`) - для разработки

## Запуск

```bash
go run ./cmd/inventory
```

Сервис запускается на `127.0.0.1:50051` (gRPC).

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

# Просмотр итогового процента
go tool cover -func=coverage.out | tail -1

# Просмотр HTML отчёта (опционально)
go tool cover -html=coverage.out
```

### Текущее покрытие

**Service слой**: 100.0% покрытие

- `internal/service/service.go` - полностью покрыт unit-тестами
- Все сценарии `GetStock()` и `ReserveStock()` протестированы:
  - Успешное получение остатка
  - Обработка ErrNotFound (возврат default=42)
  - Обработка произвольных ошибок
  - Успешное резервирование
  - Недостаточно товара
  - Ошибки при резервировании

**Общее покрытие**: 12.2% (только service слой покрыт тестами)

> Примечание: Coverage включает все пакеты (cmd, api, repository, v1). 
> Service слой имеет 100% покрытие, что является основной целью для unit-тестов.

## Генерация моков

Моки для интерфейсов генерируются через [mockery](https://github.com/vektra/mockery) с использованием `go:generate`.

### Генерация моков

```bash
go generate ./...
```

Или для конкретного пакета:

```bash
go generate ./internal/repository/...
```

Мок для `InventoryRepository` будет создан в `internal/repository/mocks/InventoryRepository.go`.

Директива `//go:generate` находится в файле `internal/repository/repository.go` рядом с определением интерфейса.

