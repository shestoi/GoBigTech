# Order Service

Сервис управления заказами для GoBigTech.

## Архитектура

Сервис построен по принципам чистой архитектуры:

- **API слой** (`internal/api/http/`) - HTTP обработчики (OpenAPI)
- **Service слой** (`internal/service/`) - бизнес-логика
- **Repository слой** (`internal/repository/`) - работа с данными через интерфейсы
- **Client слой** (`internal/client/grpc/`) - адаптеры для вызова других сервисов (Inventory, Payment)
- **In-memory реализация** (`internal/repository/memory/`) - для разработки

## DI Container / App Builder

Сервис использует явный builder паттерн для dependency injection через пакет `internal/app`.

### Структура

- **`internal/app/app.go`** - содержит структуру `App` и функцию `Build(cfg)` для создания всех зависимостей
- **`cmd/order/main.go`** - минимальный entry point, который вызывает `app.Build()` и `app.Run()`

### Зависимости, собираемые в Build()

Функция `Build(cfg)` создаёт и настраивает следующие зависимости:

1. **Logger** - platform logger (zap) с конфигурацией из env
2. **gRPC клиенты** - соединения с Inventory и Payment сервисами
3. **PostgreSQL pool** - connection pool для работы с БД
4. **Readiness функция** - проверка доступности PostgreSQL для health check
5. **Repository** - PostgreSQL реализация OrderRepository
6. **Service** - OrderService с внедрёнными зависимостями
7. **HTTP handler** - HTTP обработчики с logger
8. **HTTP router** - роутер с readiness функцией
9. **HTTP server** - настроенный http.Server
10. **Shutdown manager** - platform shutdown manager с зарегистрированными функциями

Все зависимости создаются в правильном порядке и с обработкой ошибок. При ошибке создания любой зависимости, уже созданные ресурсы корректно закрываются.

## Запуск

```bash
go run ./cmd/order
```

Сервис запускается на `127.0.0.1:8080` (HTTP).

## Health Check

Сервис предоставляет HTTP health check endpoint для проверки готовности.

### Проверка health через curl

```bash
# Проверка health status
curl -i http://127.0.0.1:8080/health

# Ожидаемый ответ при готовности (200 OK):
# HTTP/1.1 200 OK
# Content-Type: application/json
# 
# {"status":"ok"}

# Ожидаемый ответ при неготовности (503 Service Unavailable):
# HTTP/1.1 503 Service Unavailable
# Content-Type: application/json
# 
# {"status":"not ready"}
```

### Readiness и Liveness

- **Liveness**: Процесс жив (всегда 200 OK после старта сервера)
- **Readiness**: Готов обслуживать запросы
  - Проверяется через ping PostgreSQL при каждом запросе к `/health`
  - Если PostgreSQL недоступен: возвращается 503 Service Unavailable
  - Если PostgreSQL доступен: возвращается 200 OK

### Поведение при отсутствии PostgreSQL

Если PostgreSQL не поднята или недоступна:
- Сервис не стартует и логирует ошибку
- Health check недоступен (сервер не запущен)

### Graceful Shutdown

При получении SIGINT/SIGTERM:
1. Readiness проверка начинает возвращать false (PostgreSQL ping может не пройти)
2. HTTP сервер останавливается gracefully (Shutdown с таймаутом)
3. Соединение с PostgreSQL закрывается (pool.Close)
4. Сервис завершает работу

### Аутентификация (x-session-id)

Для маршрутов **POST /orders** и **GET /orders/{id}** клиент обязан передавать HTTP-заголовок **x-session-id** (session_id после Login в IAM). Без заголовка возвращается **401 Unauthorized** с текстом `session_id is required`. Order прокидывает session_id в gRPC metadata при вызовах Inventory. Endpoint **/health** не требует сессии. Если сессия истекла — клиент должен снова вызвать IAM Login и использовать новый session_id.

## База данных (PostgreSQL)

Order Service использует PostgreSQL для хранения заказов.

### Поднятие PostgreSQL через Docker Compose

Из корня проекта (`GoBigTech/`):

```bash
# Запустить PostgreSQL контейнер
docker compose up -d postgres

# Проверить статус
docker compose ps postgres

# Просмотр логов
docker compose logs postgres
```

PostgreSQL будет доступен на:
- **Host**: `localhost:5432`
- **Database**: `orders`
- **User**: `order_user`
- **Password**: `order_password`

### Миграции (goose)

Миграции находятся в папке `migrations/` и управляются через [goose](https://github.com/pressly/goose).

#### Накатка миграций

```bash
# Используя Makefile (рекомендуется)
make migrate-up

# Или напрямую через goose
go run github.com/pressly/goose/v3/cmd/goose@latest \
  -dir migrations \
  postgres "postgres://order_user:order_password@localhost:5432/orders?sslmode=disable" \
  up
```

#### Проверка статуса миграций

```bash
# Через Makefile
make migrate-status

# Или напрямую
go run github.com/pressly/goose/v3/cmd/goose@latest \
  -dir migrations \
  postgres "postgres://order_user:order_password@localhost:5432/orders?sslmode=disable" \
  status
```

#### Откат миграций

```bash
# Откатить последнюю миграцию
make migrate-down
```

#### Создание новой миграции

```bash
# Создать новую миграцию
make migrate-create NAME=add_some_feature

# Или напрямую
go run github.com/pressly/goose/v3/cmd/goose@latest \
  -dir migrations \
  create add_some_feature sql
```

### Проверка таблиц в PostgreSQL

После накатки миграций можно проверить, что таблицы созданы:

```bash
# Подключиться к PostgreSQL
docker compose exec postgres psql -U order_user -d orders

# В psql выполнить:
\dt                    # Список таблиц
\d orders              # Структура таблицы orders
\d order_items         # Структура таблицы order_items
\di                    # Список индексов
\q                     # Выход
```

Или через одну команду:

```bash
# Проверить таблицы
docker compose exec postgres psql -U order_user -d orders -c "\dt"

# Проверить структуру таблицы orders
docker compose exec postgres psql -U order_user -d orders -c "\d orders"
```

### Connection String (DSN)

Для подключения к базе данных используйте следующий DSN:

```
postgres://order_user:order_password@localhost:5432/orders?sslmode=disable
```

Для подключения из контейнера (внутри Docker сети):

```
postgres://order_user:order_password@postgres:5432/orders?sslmode=disable
```

## Тесты

### Запуск тестов

```bash
go test ./...
```

### Запуск с подробным выводом

```bash
go test ./... -v
```

### Интеграционные тесты

Интеграционные тесты используют [testcontainers-go](https://github.com/testcontainers/testcontainers-go) для поднятия PostgreSQL в Docker контейнере.

**Требования:**
- Docker должен быть запущен
- Тесты помечены build tag `integration` и не запускаются по умолчанию

**Запуск интеграционных тестов:**

```bash
# Запустить интеграционные тесты для PostgreSQL repository
go test -tags=integration ./internal/repository/postgres -v

# Запустить все интеграционные тесты
go test -tags=integration ./... -v

# С увеличенным таймаутом (если нужно)
go test -tags=integration ./internal/repository/postgres -v -timeout 5m
```

**Что делают интеграционные тесты:**
- Поднимают PostgreSQL контейнер (postgres:15-alpine) через testcontainers
- Автоматически накатывают миграции через goose
- Тестируют Save и GetByID методы repository
- Проверяют обработку ErrNotFound
- Автоматически удаляют контейнер после завершения теста

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

