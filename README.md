# GoBigTech

## Testing

```bash
make test
make test-integration
make test-e2e
```

## Infrastructure

### Kafka

Kafka настроен в KRaft режиме (без ZooKeeper) для разработки и тестирования.

**Быстрый старт:**
```bash
# Запустить Kafka
docker compose -f docker-compose.kafka.yml up -d

# Проверить статус
docker compose -f docker-compose.kafka.yml ps

# Просмотр логов
docker logs -f gobigtech-kafka
```

**Адреса подключения:**
- Из Docker сети: `kafka:9092`
- С хоста: `localhost:19092`

Подробная документация: [docs/kafka.md](docs/kafka.md)

## Аутентификация через сессии

Все защищённые gRPC сервисы (например, Inventory) требуют передачи `session_id` в gRPC metadata.

### Как получить session_id

1. Зарегистрировать пользователя через `IAM.Register`
2. Выполнить вход через `IAM.Login` - в ответе будет `session_id`

### Передача session_id в запросах

Клиент обязан передавать `x-session-id` в gRPC metadata при каждом запросе к защищённым сервисам.

**Пример с grpcurl:**
```bash
# Получить session_id
SESSION_ID=$(grpcurl -plaintext \
  -d '{"login":"user","password":"pass"}' \
  127.0.0.1:50053 iam.v1.IAMService/Login | jq -r '.session_id')

# Использовать session_id в запросе
grpcurl -plaintext \
  -H "x-session-id: ${SESSION_ID}" \
  -d '{"product_id":"product-1"}' \
  127.0.0.1:50051 inventory.v1.InventoryService/GetStock
```

### Обработка истёкших сессий

Если TTL сессии истёк (сессия удалена из Redis), сервис вернёт ошибку:
```
rpc error: code = Unauthenticated desc = invalid or expired session
```

В этом случае клиент должен:
1. Вызвать `IAM.Login` снова для получения нового `session_id`
2. Повторить исходный запрос с новым `session_id`

### Исключения

Следующие методы не требуют аутентификации:
- `/grpc.health.v1.Health/Check` - health check
- `/grpc.health.v1.Health/Watch` - health watch
- `/grpc.reflection.*` - gRPC reflection (для разработки)

Подробная документация: [docs/IAM_SESSIONS.md](docs/IAM_SESSIONS.md)