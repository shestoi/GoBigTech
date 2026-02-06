# Kafka Setup

Kafka настроен в KRaft режиме (без ZooKeeper) для упрощения развёртывания и разработки.

## Запуск Kafka

```bash
# Убедитесь, что сеть gobigtech-network существует (создаётся docker-compose.yml)
docker network create gobigtech-network 2>/dev/null || true

# Запустить Kafka
docker compose -f docker-compose.kafka.yml up -d

# Проверить статус
docker compose -f docker-compose.kafka.yml ps

# Просмотр логов
docker logs -f gobigtech-kafka

# Остановить Kafka
docker compose -f docker-compose.kafka.yml down

# Остановить и удалить volumes (очистка данных)
docker compose -f docker-compose.kafka.yml down -v
```

## Адреса подключения

В зависимости от того, откуда вы подключаетесь:

- **Из Docker сети** (другие контейнеры): `kafka:9092` (DOCKER listener)
- **С хоста** (локальная разработка): `localhost:19092` (HOST listener)

## Конфигурация клиентов Kafka (Go-сервисы)

Все Go-сервисы и playground используют общий платформенный модуль `platform/kafka` для конфигурации подключения к Kafka.

### Переменные окружения

Конфигурация задаётся через переменные окружения:

- **KAFKA_BROKERS** — список адресов брокеров Kafka (разделитель: запятая)
- **KAFKA_TOPIC** — название топика (по умолчанию: `test-topic`)

### Примеры конфигурации

#### Локальная разработка (go run, GoLand)

При запуске приложений с хоста используйте HOST listener:

```bash
export KAFKA_BROKERS=localhost:19092
export KAFKA_TOPIC=test-topic

# Или для нескольких брокеров:
export KAFKA_BROKERS=localhost:19092,localhost:19093
```

#### Запуск сервисов в Docker

При запуске сервисов внутри docker-compose используйте DOCKER listener:

```bash
export KAFKA_BROKERS=kafka:9092
export KAFKA_TOPIC=test-topic
```

В будущем для доменных событий будут использоваться специфичные топики:
- `order.paid` — событие оплаты заказа
- `payment.completed` — событие завершения платежа
- `inventory.reserved` — событие резервирования товара

### Использование в коде

Все сервисы используют единый паттерн загрузки конфигурации:

```go
import (
    platformkafka "github.com/shestoi/GoBigTech/platform/kafka"
)

// Загружаем конфигурацию с дефолтами
cfg := platformkafka.DefaultConfig()

// Переопределяем из переменных окружения
if err := platformkafka.LoadEnv(&cfg); err != nil {
    logger.Error("failed to load kafka config", zap.Error(err))
    return err
}

// Используем cfg.Brokers и cfg.Topic для создания Kafka writer/reader
```

### Playground

Для тестирования подключения к Kafka используйте `cmd/kafka-playground`:

```bash
# С дефолтами (localhost:19092, test-topic)
go run ./cmd/kafka-playground

# С переопределением через env
KAFKA_BROKERS=localhost:19092 KAFKA_TOPIC=my-topic go run ./cmd/kafka-playground
```

## Работа с топиками

### Создание топика

```bash
docker exec -it gobigtech-kafka kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create \
  --topic test-topic \
  --partitions 1 \
  --replication-factor 1
```

### Список топиков

```bash
docker exec -it gobigtech-kafka kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --list
```

### Описание топика

```bash
docker exec -it gobigtech-kafka kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --describe \
  --topic test-topic
```

### Удаление топика

```bash
docker exec -it gobigtech-kafka kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --delete \
  --topic test-topic
```

## Отправка и получение сообщений

### Отправка сообщений (Producer)

В одном терминале:

```bash
docker exec -it gobigtech-kafka kafka-console-producer.sh \
  --bootstrap-server localhost:9092 \
  --topic test-topic
```

Введите сообщения и нажмите Enter после каждого. Для выхода: `Ctrl+C`.

### Получение сообщений (Consumer)

В другом терминале:

```bash
# Читать все сообщения с начала
docker exec -it gobigtech-kafka kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic test-topic \
  --from-beginning

# Читать только новые сообщения
docker exec -it gobigtech-kafka kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic test-topic
```

## Проверка работы

1. Запустите Kafka: `docker compose -f docker-compose.kafka.yml up -d`
2. Создайте топик (команда выше)
3. В одном терминале запустите consumer с `--from-beginning`
4. В другом терминале запустите producer
5. Введите сообщение в producer - оно должно появиться в consumer

## Конфигурация

Kafka настроен для разработки с следующими параметрами:

- **KRaft режим**: без ZooKeeper, использует встроенный контроллер
- **Replication factor**: 1 (для single node)
- **Auto-create topics**: включено
- **Plaintext listener**: разрешён (для dev окружения)

Для production окружения рекомендуется:
- Увеличить replication factor
- Настроить SSL/TLS
- Отключить auto-create topics
- Настроить retention policies

## Troubleshooting

### Kafka не стартует

Проверьте логи:
```bash
docker logs gobigtech-kafka
```

Убедитесь, что порт 19092 не занят:
```bash
lsof -i :19092
```

### Не могу подключиться с хоста

Убедитесь, что используете правильный адрес:
- С хоста: `localhost:19092`
- Из Docker сети: `kafka:9092`

### Топик не создаётся автоматически

Если auto-create не сработал, создайте топик вручную (команда выше).

## Почему мы не полагаемся на auto.create.topics.enable

Хотя в dev-окружении у нас включена настройка `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"`, **мы не полагаемся на неё** в продакшене и не рекомендуем использовать для критичных сценариев. Вот почему:

- **auto.create.topics.enable не гарантирует создание топиков при Produce-запросе**: Топик создаётся только при первом metadata lookup, который может не произойти при отправке сообщения через kafka-go
- **kafka-go не инициирует metadata lookup**: В отличие от некоторых других клиентов, kafka-go не делает явный metadata запрос перед отправкой, что может привести к ситуации, когда топик не создаётся автоматически
- **Поведение зависит от клиента и версии брокера**: Разные версии Kafka и разные клиенты ведут себя по-разному, что делает поведение непредсказуемым
- **Фича нестабильная, в проде отключается почти всегда**: В production окружениях эта настройка почти всегда отключена из соображений безопасности и предсказуемости инфраструктуры
- **Топики должны создаваться заранее**: Правильный подход — создавать топики заранее через:
  - CLI-инструменты (`kafka-topics.sh`)
  - Infrastructure as Code (Terraform, Helm)
  - Init-скрипты при старте приложения
  - CI/CD пайплайны

В нашем проекте:
- Сейчас топики создаём вручную через `kafka-topics.sh` (см. раздел "Работа с топиками")
- В будущем добавим `kafka-init` сервис или init-скрипт, который будет создавать необходимые топики при старте инфраструктуры

## Почему в consumer делаем Commit после обработки (at-least-once)

В Assembly Service мы используем **at-least-once** семантику доставки сообщений. Это означает, что каждое сообщение будет обработано **минимум один раз**, но при сбоях может быть обработано повторно.

### Как это работает:

1. **FetchMessage вместо ReadMessage**: Используем `reader.FetchMessage(ctx)` для получения сообщения без автоматического commit offset'а
2. **Commit только после успешной обработки**: Вызываем `reader.CommitMessages(ctx, msg)` **только после** успешной обработки события или отправки в DLQ
3. **Retry при ошибках**: Если обработка не удалась, делаем retry (до 3 попыток с экспоненциальным backoff), и только после исчерпания попыток отправляем в DLQ и коммитим

### Почему не auto-commit:

- **Контроль над обработкой**: Auto-commit коммитит offset сразу после чтения, до обработки. Если сервис упадёт после commit, но до обработки, сообщение будет потеряно
- **Retry логика**: При auto-commit мы не можем сделать retry — offset уже закоммичен, и сообщение не будет прочитано повторно
- **DLQ для poison pills**: При auto-commit мы не можем отправить некорректные сообщения в DLQ и закоммитить их — они будут зацикливаться

### Гарантии:

- ✅ **At-least-once**: Каждое сообщение будет обработано минимум один раз
- ⚠️ **Возможны дубликаты**: При сбое после обработки, но до commit, сообщение может быть обработано повторно
- ✅ **Нет потерь**: Сообщения не теряются при сбоях (offset коммитится только после обработки)

### Обработка дубликатов:

В production рекомендуется делать обработчики **идемпотентными** (например, проверять, не обработан ли уже заказ по `order_id`), чтобы повторная обработка не вызывала проблем.

## DLQ: зачем и как смотреть

**Dead Letter Queue (DLQ)** — это специальный топик для сообщений, которые не удалось обработать после всех попыток.

### Когда сообщение попадает в DLQ:

1. **Poison pill (некорректный формат)**: Если сообщение не удалось распарсить (невалидный JSON, отсутствуют обязательные поля) → сразу в DLQ
2. **Исчерпаны все retry**: Если обработка не удалась после всех попыток (по умолчанию 3 попытки) → в DLQ

### Что содержится в DLQ сообщении:

- `original_topic`, `original_partition`, `original_offset` — откуда пришло сообщение
- `original_key`, `original_value` — оригинальные ключ и значение (base64 encoded)
- `error_message` — причина ошибки
- `failed_at` — время отправки в DLQ (RFC3339)
- `event_type`, `event_id`, `order_id` — если удалось извлечь из оригинального сообщения

### Как посмотреть DLQ:

```bash
# Просмотр сообщений в DLQ
make kafka-consume-dlq
```

### Что делать с сообщениями в DLQ:

1. **Анализ**: Изучить `error_message` и `original_value` для понимания причины ошибки
2. **Исправление**: Исправить проблему (например, обновить код обработчика, исправить данные)
3. **Репроцессинг**: Вручную отправить исправленное сообщение обратно в основной топик (или создать скрипт для автоматического репроцессинга)

### Мониторинг DLQ:

В production рекомендуется:
- Настроить алерты на появление сообщений в DLQ
- Регулярно мониторить размер DLQ топика
- Автоматизировать репроцессинг исправленных сообщений

## Order consumer: order.assembly.completed → status assembled

Order Service слушает топик `order.assembly.completed` и обновляет статус заказа с `paid` на `assembled` при получении события.

### Как это работает:

1. **Consumer читает события**: Order Service использует Kafka consumer group `order-service` для чтения из топика `order.assembly.completed`
2. **Idempotency через inbox**: Каждое событие сохраняется в таблицу `order_inbox_events` (по `event_id`). Если событие уже обработано (duplicate), оно пропускается
3. **Обновление статуса**: Если событие впервые обработано, выполняется `UPDATE orders SET status='assembled' WHERE id=$1 AND status='paid'`
4. **At-least-once**: Offset коммитится только после успешной обработки (FetchMessage + CommitMessages)

### Конфигурация:

Переменные окружения для Order Service:
- `KAFKA_BROKERS` (default: `localhost:19092` для local, `kafka:9092` для docker)
- `KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC` (default: `order.assembly.completed`)
- `KAFKA_ORDER_CONSUMER_GROUP_ID` (default: `order-service`)

### Запуск локально:

```bash
# Поднять Kafka
make kafka-up

# Создать топики
make kafka-topics-create

# Запустить Order Service
cd services/order
APP_ENV=local go run ./cmd/order
```

### Запуск в Docker:

```bash
# В docker-compose.yml или через переменные окружения
APP_ENV=docker
KAFKA_BROKERS=kafka:9092
KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC=order.assembly.completed
KAFKA_ORDER_CONSUMER_GROUP_ID=order-service
```

### Проверка работы:

1. **Создать заказ через Order Service** (он опубликует `order.payment.completed`)
2. **Assembly Service обработает событие** и опубликует `order.assembly.completed`
3. **Order Service получит событие** и обновит статус заказа
4. **Проверить через API**: `GET /orders/{order_id}` должен вернуть `status: "assembled"`

### Мониторинг:

```bash
# Просмотр событий в топике
make kafka-consume-assembly

# Проверка inbox таблицы (PostgreSQL)
psql -h localhost -p 15432 -U order_user -d orders -c "SELECT * FROM order_inbox_events ORDER BY received_at DESC LIMIT 10;"
```

### Idempotency гарантии:

- ✅ **Дубликаты игнорируются**: Если одно и то же событие приходит повторно (по `event_id`), оно не обрабатывается повторно
- ✅ **Транзакционность**: Insert в inbox и update статуса выполняются в одной транзакции
- ✅ **Безопасность**: Если заказ уже `assembled` или не найден, update не выполняется, но это не ошибка

## Notification service

Notification Service слушает события `order.payment.completed` и `order.assembly.completed` и отправляет уведомления пользователям.

### Архитектура:

- **Два независимых consumer**: один для payment events, другой для assembly events
- **Idempotency через inbox**: каждое событие сохраняется в `notification_inbox_events` (по `event_id`)
- **At-least-once**: Offset коммитится только после успешной обработки (FetchMessage + CommitMessages)
- **Retry**: 3 попытки с экспоненциальным backoff (1s, 2s, 4s)

### Конфигурация:

Переменные окружения:
- `APP_ENV` (local/docker)
- `SHUTDOWN_TIMEOUT` (default: `10s`)
- `NOTIFICATION_POSTGRES_DSN` (default: использует тот же postgres что и order)
- `KAFKA_BROKERS` (default: `localhost:19092` для local, `kafka:9092` для docker)
- `KAFKA_ORDER_PAYMENT_COMPLETED_TOPIC` (default: `order.payment.completed`)
- `KAFKA_ORDER_ASSEMBLY_COMPLETED_TOPIC` (default: `order.assembly.completed`)
- `KAFKA_NOTIFICATION_PAYMENT_GROUP_ID` (default: `notification-payment`)
- `KAFKA_NOTIFICATION_ASSEMBLY_GROUP_ID` (default: `notification-assembly`)
- `NOTIFICATION_KAFKA_RETRY_MAX_ATTEMPTS` (default: `3`)
- `NOTIFICATION_KAFKA_RETRY_BACKOFF_BASE` (default: `1s`)

### Запуск локально:

```bash
# Поднять инфраструктуру
make kafka-up
make kafka-create-topics

# Применить миграции (если нужно)
cd services/notification
goose -dir migrations postgres "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable" up

# Запустить Notification Service
make notification-run
# или
cd services/notification
APP_ENV=local go run ./cmd/notification
```

### Проверка работы:

1. **Создать заказ через Order Service** → Order опубликует `order.payment.completed`
2. **Assembly Service обработает** → опубликует `order.assembly.completed`
3. **Notification Service получит оба события** и отправит уведомления (пока логирует)

### Мониторинг:

```bash
# Проверка inbox таблицы
psql -h localhost -p 15432 -U order_user -d orders -c \
  "SELECT * FROM notification_inbox_events ORDER BY processed_at DESC LIMIT 10;"
```

### Telegram интеграция:

Для работы с Telegram необходимо:

1. **Создать бота через @BotFather** в Telegram:
   - Отправить `/newbot`
   - Получить `TELEGRAM_BOT_TOKEN`

2. **Получить Chat ID**:
   - Написать боту любое сообщение
   - Отправить GET запрос: `https://api.telegram.org/bot<TOKEN>/getUpdates`
   - Найти `chat.id` в ответе

3. **Переменные окружения**:
```bash
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_ID=your_chat_id_here
TELEGRAM_ENABLED=true  # или false для отключения
```

4. **Запуск с Telegram**:
```bash
cd services/notification
APP_ENV=local \
TELEGRAM_BOT_TOKEN=your_token \
TELEGRAM_CHAT_ID=your_chat_id \
TELEGRAM_ENABLED=true \
go run ./cmd/notification
```

### DLQ:

При ошибках обработки сообщения отправляются в DLQ топик `order.notification.dlq`:
- JSON/parse ошибки → DLQ + commit
- Исчерпание retry → DLQ + commit
- Ошибка публикации в DLQ → не commit (Kafka повторит)

Просмотр DLQ:
```bash
make kafka-consume-dlq
# или
docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic order.notification.dlq \
  --from-beginning
```
