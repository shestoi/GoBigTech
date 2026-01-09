# Inventory Service

Сервис управления инвентарём для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/grpc/`) - gRPC обработчики
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **MongoDB реализация** (`internal/repository/mongo/`) - production реализация
- **In-memory реализация** (`internal/repository/memory/`) - для тестирования

## База данных (MongoDB)

Inventory Service использует MongoDB для хранения данных об инвентаре.

### Поднятие MongoDB через Docker Compose

Из корня проекта (`GoBigTech/`):

```bash
# Запустить MongoDB контейнер
docker compose up -d mongo

# Проверить статус
docker compose ps mongo

# Просмотр логов
docker compose logs mongo
```

MongoDB будет доступна на:
- **Host**: `127.0.0.1:15417` (порт на хосте)
- **Container**: `27017` (внутри Docker сети)
- **Username**: `inventory_user`
- **Password**: `inventory_password`
- **Database**: `inventory` (по умолчанию)

### Переменные окружения

Сервис использует следующие переменные окружения:

- `INVENTORY_MONGO_URI` - URI для подключения к MongoDB
  - Дефолт: `mongodb://inventory_user:inventory_password@127.0.0.1:15417/?authSource=admin`
  - Для Docker сети: `mongodb://inventory_user:inventory_password@mongo:27017/?authSource=admin`

- `INVENTORY_MONGO_DB` - имя базы данных
  - Дефолт: `inventory`

### Проверка подключения

```bash
# Подключиться к MongoDB через mongosh
docker compose exec mongo mongosh -u inventory_user -p inventory_password --authenticationDatabase admin

# Или через команду извне (если установлен mongosh)
mongosh "mongodb://inventory_user:inventory_password@127.0.0.1:15417/?authSource=admin"
```

### Структура данных

**Коллекция:** `inventory`

**Документ:**
```json
{
  "product_id": "product-123",
  "stock": 42,
  "updated_at": ISODate("2026-01-08T12:00:00Z")
}
```

**Индексы:**
- Уникальный индекс на `product_id` (создаётся автоматически при старте)

### Проверка данных в MongoDB

```bash
# Подключиться к MongoDB
docker compose exec mongo mongosh -u inventory_user -p inventory_password --authenticationDatabase admin

# В mongosh выполнить:
use inventory
db.inventory.find()
db.inventory.findOne({ product_id: "product-123" })
db.inventory.getIndexes()
```

### Примеры команд

```bash
# Запуск сервиса с дефолтными настройками
go run ./cmd/inventory

# Запуск с кастомным URI
INVENTORY_MONGO_URI="mongodb://inventory_user:inventory_password@127.0.0.1:15417/?authSource=admin" \
INVENTORY_MONGO_DB="inventory" \
go run ./cmd/inventory
```

## Запуск

```bash
go run ./cmd/inventory
```

Сервис запускается на `127.0.0.1:50051` (gRPC).

**Важно:** Перед запуском убедитесь, что MongoDB поднята через `docker compose up -d mongo`.

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

