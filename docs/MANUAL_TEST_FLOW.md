# Manual Test Flow для Event-Driven Pipeline

Этот документ описывает пошаговый процесс для ручного тестирования event-driven пайплайна.

## Предварительные требования

- Docker и Docker Compose установлены
- Go 1.21+ установлен
- Kafka, PostgreSQL должны быть доступны

## Шаг 1: Поднять инфраструктуру

```bash
# Поднять Kafka
make kafka-up

# Поднять PostgreSQL (если не запущен)
docker compose up -d postgres

# Создать Kafka топики
make kafka-topics-create
```

## Шаг 2: Применить миграции

```bash
# Миграции для Order Service (включая outbox таблицу)
make migrate-up-order

# Проверить что таблица order_outbox_events создана
psql -h localhost -p 15432 -U order_user -d orders -c \
  "\d order_outbox_events"

# Миграции для Notification Service (включая inbox таблицу)
make migrate-up-notification
```

## Шаг 3: Запустить сервисы

В отдельных терминалах:

```bash
# Terminal 1: Order Service
make order-run

# Terminal 2: Assembly Service
make assembly-run

# Terminal 3: Notification Service
# С Telegram (если настроен):
cd services/notification && \
  APP_ENV=local \
  TELEGRAM_ENABLED=true \
  TELEGRAM_BOT_TOKEN=your_token \
  TELEGRAM_CHAT_ID=your_chat_id \
  go run ./cmd/notification

# Без Telegram:
make notification-run
```

## Шаг 4: Создать заказ

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "items": [
      {"product_id": "product-1", "quantity": 2}
    ]
  }'
```

Запомните `order_id` из ответа.

## Шаг 5: Проверить цепочку событий

### 5.1 Проверить Order Service

- В логах Order Service должно быть:
  - "Order saved successfully with outbox event"
  - Outbox dispatcher должен опубликовать событие в Kafka

### 5.2 Проверить Assembly Service

- В логах Assembly Service должно быть:
  - "received order paid event"
  - "order assembly completed event published"

### 5.3 Проверить Order Service (обновление статуса)

- В логах Order Service должно быть:
  - "received order assembly completed event"
  - "order status updated to assembled"

### 5.4 Проверить Notification Service

- В логах Notification Service должно быть:
  - "notification sent for order paid"
  - "notification sent for order assembly completed"
- Если Telegram enabled, должно прийти сообщение в Telegram

## Шаг 6: Проверить базу данных

### Order Service

```bash
# Проверить заказ
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT id, user_id, status FROM orders WHERE id = 'order-xxx';"

# Проверить outbox события (должны быть со статусом 'sent' после публикации)
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT event_id, event_type, topic, status, attempts, sent_at FROM order_outbox_events ORDER BY created_at DESC LIMIT 5;"

# Убедиться что статус = 'sent' и sent_at заполнен
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT COUNT(*) as pending_count FROM order_outbox_events WHERE status = 'pending';"

# Проверить inbox события
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT event_id, event_type, order_id FROM order_inbox_events ORDER BY received_at DESC LIMIT 5;"
```

### Notification Service

```bash
# Проверить inbox события
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT event_id, event_type, order_id, processed_at FROM notification_inbox_events ORDER BY processed_at DESC LIMIT 5;"
```

## Шаг 7: Проверить Kafka топики

```bash
# Просмотр событий оплаты
make kafka-tail-payment

# Просмотр событий сборки
make kafka-tail-assembly

# Просмотр DLQ (если есть ошибки)
make kafka-tail-notification-dlq
```

## Шаг 8: Тестирование ошибок

### Тест idempotency

Отправить то же событие дважды (через Kafka producer) - должно обработаться только один раз.

### Тест DLQ

Отправить невалидное сообщение в Kafka - должно попасть в DLQ.

### Тест Outbox

Остановить Kafka, создать заказ - событие должно остаться в outbox со статусом `pending`. После запуска Kafka dispatcher должен отправить событие.

## Команды для отладки

```bash
# Список всех топиков
make kafka-topics-list

# Просмотр логов сервисов
# (в терминалах где запущены сервисы)

# Проверка health check
curl http://localhost:8080/health
```

