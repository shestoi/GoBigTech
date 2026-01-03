# Order Service

Сервис управления заказами для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/http/`) - HTTP обработчики (OpenAPI)
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **Client слой** (`internal/client/grpc/`) - адаптеры для вызова других сервисов (Inventory, Payment)
- **In-memory реализация** (`internal/repository/memory/`) - для разработки

## Запуск

```bash
go run ./cmd/order
```

Сервис запускается на `127.0.0.1:8080` (HTTP).

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

- `internal/service/order.go` - полностью покрыт unit-тестами
- Все сценарии `CreateOrder()` и `GetOrder()` протестированы:
  - Успешное создание заказа (все шаги: inventory, payment, save)
  - Ошибка при резервировании товара (inventory)
  - Ошибка при обработке оплаты (payment)
  - Ошибка при сохранении заказа (repository)
  - Успешное получение заказа с товарами
  - Ошибка при заказе не найден (ErrNotFound)
  - Получение заказа без товаров

**Общее покрытие**: 12.0% (только service слой покрыт тестами)

> Примечание: Coverage включает все пакеты (cmd, api, repository, client). 
> Service слой имеет 100% покрытие, что является основной целью для unit-тестов.

## Генерация моков

Моки для интерфейсов генерируются через [mockery](https://github.com/vektra/mockery) с использованием `go:generate`.

### Генерация моков

```bash
go generate ./...
```

Или для конкретных пакетов:

```bash
go generate ./internal/repository/...
go generate ./internal/service/...
```

Моки будут созданы в:
- `internal/repository/mocks/OrderRepository.go`
- `internal/service/mocks/InventoryClient.go`
- `internal/service/mocks/PaymentClient.go`

Директивы `//go:generate` находятся:
- В `internal/repository/repository.go` рядом с интерфейсом `OrderRepository`
- В `internal/service/interfaces.go` рядом с интерфейсами `InventoryClient` и `PaymentClient`

