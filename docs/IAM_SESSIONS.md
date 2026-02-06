# IAM Sessions и защита Inventory через gRPC Interceptor

## Архитектура

### Сессии в Redis

Сессии хранятся в Redis как **hash** по ключу `session:<session_id>`, TTL задаётся на ключ целиком (`EXPIRE`):
- **Ключ**: `session:<session_id>`
- **Поля hash**: `user_id` (обязательно), `created_at` (RFC3339), `last_seen_at` (RFC3339)
- **TTL**: 24 часа (настраивается через `SESSION_TTL` в config)
- Session ID: UUID v4 (генерируется через `uuid.NewString()`)

### Защита Inventory Service

Inventory Service защищён через gRPC unary interceptor, который:
1. Извлекает `x-session-id` из gRPC metadata
2. Вызывает IAM.ValidateSession для проверки сессии
3. Добавляет `user_id` в context для использования в handlers
4. Пропускает health check и reflection методы без проверки

## Команды для запуска

### 1. Поднять инфраструктуру

```bash
docker compose up -d iam-postgres redis
```

### 2. Запустить IAM Service

```bash
make iam-run
```

Или вручную:
```bash
cd services/iam && APP_ENV=local go run ./cmd/iam
```

### 3. Запустить Inventory Service

```bash
make inventory-run
```

Или вручную:
```bash
cd services/inventory && APP_ENV=local go run ./cmd/inventory
```

## Примеры использования с grpcurl

### 1. Регистрация пользователя

```bash
grpcurl -plaintext \
  -d '{"login":"testuser","password":"testpass123"}' \
  127.0.0.1:50053 iam.v1.IAMService/Register
```

Ответ:
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 2. Вход (получение session_id)

```bash
grpcurl -plaintext \
  -d '{"login":"testuser","password":"testpass123"}' \
  127.0.0.1:50053 iam.v1.IAMService/Login
```

Ответ:
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 3. Вызов Inventory с сессией

```bash
# Сохраните session_id из предыдущего шага
SESSION_ID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"

grpcurl -plaintext \
  -H "x-session-id: ${SESSION_ID}" \
  -d '{"product_id":"product-1"}' \
  127.0.0.1:50051 inventory.v1.InventoryService/GetStock
```

### 4. Вызов Inventory без сессии (должен вернуть ошибку)

```bash
grpcurl -plaintext \
  -d '{"product_id":"product-1"}' \
  127.0.0.1:50051 inventory.v1.InventoryService/GetStock
```

Ожидаемая ошибка:
```
rpc error: code = Unauthenticated desc = session_id is required
```

### 5. Health check (без сессии, должен работать)

```bash
grpcurl -plaintext \
  127.0.0.1:50051 grpc.health.v1.Health/Check
```

## Структура хранения сессий

### Redis (hash)

- **Ключ**: `session:<session_id>`
- **Тип**: hash (HSET/HGET), TTL на ключе (EXPIRE)
- **Поля**: `user_id` (UUID пользователя), `created_at` (RFC3339), `last_seen_at` (RFC3339)
- **TTL**: 24 часа (настраивается через `SESSION_TTL`)

Пример:
```
session:a1b2c3d4-e5f6-7890-abcd-ef1234567890
  user_id      -> "550e8400-e29b-41d4-a716-446655440000"
  created_at   -> "2025-01-28T12:00:00Z"
  last_seen_at -> "2025-01-28T14:30:00Z"
```

### Проверка сессии в Redis

```bash
# Подключиться к Redis
docker exec -it iam-redis redis-cli

# Получить user_id сессии (hash)
HGET session:a1b2c3d4-e5f6-7890-abcd-ef1234567890 user_id

# Все поля hash
HGETALL session:a1b2c3d4-e5f6-7890-abcd-ef1234567890

# Проверить TTL ключа
TTL session:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

## Как работает Interceptor

1. **Извлечение session_id**: Interceptor читает gRPC metadata и ищет заголовок `x-session-id`
2. **Валидация**: Вызывает `IAM.ValidateSession(session_id)` через gRPC клиент
3. **Добавление в context**: Если сессия валидна, добавляет `user_id` в context с ключом `user_id`
4. **Пропуск публичных методов**: Health check и reflection методы не требуют аутентификации

## Переменные окружения

### IAM Service

- `IAM_POSTGRES_DSN` - PostgreSQL DSN для IAM
- `REDIS_ADDR` - адрес Redis (по умолчанию: `127.0.0.1:16379` для local)
- `REDIS_PASSWORD` - пароль Redis (опционально)
- `SESSION_TTL` - TTL сессий (по умолчанию: `24h`)
- `GRPC_ADDR` - адрес gRPC сервера (по умолчанию: `127.0.0.1:50053` для local)

### Inventory Service

- `IAM_GRPC_ADDR` - адрес IAM Service (по умолчанию: `127.0.0.1:50053` для local)
- `GRPC_ADDR` - адрес gRPC сервера (по умолчанию: `127.0.0.1:50051` для local)

## Важные замечания

1. **Proto файлы**: После изменения proto файлов необходимо сгенерировать код:
   ```bash
   # Если buf.build доступен:
   buf generate --template tools/buf.gen.yaml api/proto
   
   # Или используя protoc напрямую (если установлен):
   protoc --go_out=services --go-grpc_out=services \
     --go_opt=paths=source_relative \
     --go-grpc_opt=paths=source_relative \
     api/proto/iam/v1/iam.proto
   ```
   
   **ВАЖНО**: После добавления ValidateSession RPC необходимо перегенерировать proto файлы!

2. **Health check**: Методы `/grpc.health.v1.Health/*` и `/grpc.reflection.*` не требуют аутентификации

3. **x-session-id**: Клиент обязан передавать заголовок `x-session-id` в gRPC metadata для защищённых сервисов (например Inventory). Без него interceptor вернёт Unauthenticated.

4. **Sliding TTL**: TTL сессии продлевается на `SESSION_TTL` при каждом успешном вызове `ValidateSession`. Пока клиент периодически вызывает защищённые методы, сессия не истекает. Если сессия истекла — клиент должен снова вызвать `Login` и получить новый `session_id`.

5. **Истёкшая сессия**: Если сессия не найдена или истекла, IAM возвращает `codes.Unauthenticated`. Клиент должен выполнить повторный Login и использовать новый `session_id`.

6. **Order Service (HTTP)**: Клиент при вызове POST /orders и GET /orders/{id} обязан передавать HTTP-заголовок `x-session-id` (иначе 401 Unauthorized). Order прокидывает его в gRPC metadata при вызовах Inventory. Endpoint /health не требует сессии.

7. **Контекст**: `user_id` доступен в handlers через `ctx.Value(interceptor.UserIDContextKey)`

8. **telegram_id при регистрации**: Поле `telegram_id` в IAM.Register опционально. Если не указано — уведомления Notification не отправляются (см. [NOTIFICATIONS.md](NOTIFICATIONS.md)).
