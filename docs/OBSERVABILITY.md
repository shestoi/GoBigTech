# Observability (Неделя 7)

Три столпа: **логи**, **метрики**, **трейсы**. Плюс алерты в Telegram через webhook в Notification Service.

---

## Режимы запуска

### Host-mode (сервисы на хосте)

- **Observability stack:** в Docker (`make obs-up`)
- **App services:** на хосте (`make *-run`)
- **Логи:** Filebeat видит только логи контейнеров (observability stack). Логи app-сервисов с хоста не попадают в Elasticsearch/Kibana автоматически.
- **Traces/Metrics:** App-сервисы шлют OTLP на `127.0.0.1:4317` → otel-collector в Docker (порт проброшен).
- **Alerts:** Alertmanager в Docker → `http://host.docker.internal:8081/alerts` → Notification на хосте.

**Использование:** разработка, отладка отдельных сервисов.

### Docker-mode (всё в Docker) ⭐

- **Observability stack:** в Docker (`make obs-up`)
- **App services:** в Docker (`make app-up` или `make all-up`)
- **Логи:** Filebeat видит логи **всех** контейнеров (включая app-сервисы) → Elasticsearch → Kibana.
- **Traces/Metrics:** App-сервисы шлют OTLP на `otel-collector:4317` (docker DNS).
- **Alerts:** Alertmanager в Docker → `http://notification:8081/alerts` → Notification в Docker.

**Использование:** Week 7 E2E проверка, полная интеграция, демо.

**По умолчанию в репозитории:** docker-mode (для Week 7 E2E).

---

## Аудит конфигов и потоков данных

### docker-compose.yml

- **Observability stack:** otel-collector, jaeger, prometheus, alertmanager, grafana, elasticsearch, kibana, filebeat.
- **App services (docker-mode):** iam, inventory, payment, order, assembly, notification.
- **Порты:** 4317/4318/8889/13133 (collector), 16686 (Jaeger), 9090 (Prometheus), 9093 (Alertmanager), 3000 (Grafana), 5601 (Kibana), 9200 (Elasticsearch), 8080 (order), 50051-50053 (gRPC), 8081 (notification alerts).

### deploy/otel/otel-collector-config.yaml

- **Receivers:** otlp (grpc 0.0.0.0:4317, http 0.0.0.0:4318).
- **Processors:** batch, memory_limiter, resource (deployment.environment=docker).
- **Exporters:** otlp → jaeger:4317 (traces), prometheus → 0.0.0.0:8889 (metrics).
- **Extensions:** health_check на 0.0.0.0:13133.
- **Pipelines:** traces [otlp→processors→otlp], metrics [otlp→processors→prometheus].

### deploy/prometheus/prometheus.yml и rules.yml

- **Scrape:** otel-collector:8889/metrics (канонично: метрики из сервисов идут в collector по OTLP, Prometheus скрейпит только collector).
- **Rules:** `rules.yml` — alert HighOrderRate: `increase(orders_created_total[1m]) > 10` (метрика из Order Service через OTLP).

### deploy/alertmanager/alertmanager.yml

- **Route:** receiver notification-webhook.
- **Receivers:** webhook URL `http://notification:8081/alerts` (docker-mode) или `http://host.docker.internal:8081/alerts` (host-mode).

### deploy/filebeat/filebeat.yml

- **Inputs:** container, paths `/var/lib/docker/containers/*/*.log`, add_docker_metadata.
- **Processors:** decode_json_fields для поля `message` с условием `when.contains: { message: "{" }` (map, не строка — иначе ошибка "wrong type, expect map").
- **Output:** Elasticsearch, индекс filebeat-%{+yyyy.MM.dd}.

### deploy/grafana/provisioning/datasources/datasources.yml

- **Prometheus:** url http://prometheus:9090, isDefault: true.
- **Elasticsearch:** url http://elasticsearch:9200, index pattern [filebeat-]YYYY.MM.DD, timeField @timestamp.

### Схема потоков

**Docker-mode:**
```
Traces:   order/inventory/payment/iam/assembly (OTLP gRPC otel-collector:4317) → otel-collector → jaeger
Metrics:  order/assembly (OTLP gRPC otel-collector:4317) → otel-collector → :8889/metrics ← prometheus scrape → grafana
Logs:     сервисы (stdout JSON с trace_id/span_id) → docker logs → filebeat → elasticsearch → kibana ✅ ВСЕ сервисы видны
Alerts:   prometheus rules (orders_created_total) → alertmanager → POST http://notification:8081/alerts → notification → Telegram Bot API
```

**Host-mode:**
```
Traces:   order/inventory/payment/iam/assembly (OTLP gRPC 127.0.0.1:4317) → otel-collector → jaeger
Metrics:  order/assembly (OTLP gRPC 127.0.0.1:4317) → otel-collector → :8889/metrics ← prometheus scrape → grafana
Logs:     только observability stack (docker logs) → filebeat → elasticsearch → kibana ⚠️ app-сервисы не видны
Alerts:   prometheus rules (orders_created_total) → alertmanager → POST http://host.docker.internal:8081/alerts → notification (на хосте) → Telegram Bot API
```

---

## Запуск

### Host-mode (сервисы на хосте)

```bash
# 1. Поднять observability stack
make obs-up

# 2. Запустить app-сервисы на хосте (в отдельных терминалах)
export OTEL_ENABLED=1
export OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
export OTEL_SAMPLING_RATIO=1.0

make iam-run
make inventory-run
make payment-run
make order-run
make assembly-run
make notification-run  # для алертов на :8081
```

### Docker-mode (всё в Docker) ⭐

```bash
# Вариант 1: всё сразу
make all-up

# Вариант 2: по шагам
make stack-up   # kafka + observability
make app-up     # app-сервисы

# Остановка
make all-down   # или make app-down + make stack-down
```

Поднимаются: observability stack + app-сервисы (iam, inventory, payment, order, assembly, notification) в Docker.

**Проверка collector health:**

```bash
curl http://localhost:13133/health
```

Должен вернуть JSON со статусом (например `{"status":"Server healthy"}`).

**Проверка filebeat (без падения):**

```bash
docker logs filebeat --tail=50
```

Не должно быть ошибки `wrong type, expect map accessing decode_json_fields.when.contains`.

---

## UI и порты

| Сервис        | URL                        |
|---------------|----------------------------|
| Jaeger        | http://localhost:16686     |
| Grafana       | http://localhost:3000      |
| Prometheus    | http://localhost:9090      |
| Alertmanager  | http://localhost:9093      |
| Kibana        | http://localhost:5601      |
| Collector health | http://localhost:13133/health |

---

## Конфигурация OTEL

### Host-mode

```bash
export OTEL_ENABLED=1
export OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
export OTEL_SAMPLING_RATIO=1.0
```

Сервисы шлют traces и metrics в otel-collector на `127.0.0.1:4317` (порт проброшен из Docker).

### Docker-mode

В `docker-compose.yml` уже настроено:
- `OTEL_ENABLED=1`
- `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
- `OTEL_SAMPLING_RATIO=1.0`

Сервисы автоматически шлют traces и metrics в otel-collector через docker DNS.

---

## Логи и trace_id

- В коде: `observability.L(ctx, logger)` и `observability.TraceFields(ctx)` — добавляют trace_id/span_id в лог.
- Формат логов: JSON (env=docker или LOG_FORMAT=json), иначе console. Поля: service, env, trace_id, span_id (если span в контексте).
- **Docker-mode:** Filebeat видит логи **всех** контейнеров (observability + app-сервисы) → Elasticsearch → Kibana ✅
- **Host-mode:** Filebeat видит только логи контейнеров observability stack. Логи app-сервисов с хоста не попадают автоматически.

**Поиск в Kibana:** Discover → индекс filebeat-* → фильтр по полю `trace_id` (из Jaeger) или по `message`, `container.name`.

---

## Метрики

- метрики приходят через otel-collector, поэтому имя в Prometheus: otel_orders_created_total, и правило использует его.
- **Order:** `orders_created_total`, `order_revenue_total` (копейки) — экспорт OTLP в collector.
- **Assembly:** `assembly_duration_ms` (histogram) — время сборки.

Prometheus скрейпит только otel-collector:8889; метрики приложений попадают туда через OTLP. В Grafana: Explore → Prometheus → запросы `orders_created_total`, `order_revenue_total`, `assembly_duration_ms`.

---

## Алерты: Alertmanager → Notification → Telegram

Alertmanager **не поддерживает Telegram напрямую**. Используется webhook в **Notification Service** (POST /alerts принимает payload Alertmanager v4, форматирует сообщение и отправляет в Telegram).

### Docker-mode

- **Notification** в Docker слушает `0.0.0.0:8081` (через `ALERTS_HTTP_ADDR=0.0.0.0:8081` в compose).
- **Alertmanager** вызывает `http://notification:8081/alerts` (docker DNS).
- **Конфиг:** через env в `docker-compose.yml`: `TELEGRAM_BOT_TOKEN`, `ALERT_TELEGRAM_CHAT_ID`. `TELEGRAM_DISABLE=false` по умолчанию.

### Host-mode

- **Notification** на хосте слушает `:8081` (или `ALERTS_HTTP_ADDR=0.0.0.0:8081`).
- **Alertmanager** в Docker вызывает `http://host.docker.internal:8081/alerts`.
- **Конфиг:** `TELEGRAM_BOT_TOKEN`, `ALERT_TELEGRAM_CHAT_ID`. `TELEGRAM_DISABLE=true` — не слать в Telegram (webhook всё равно 200).

---

## End-to-End чеклист (Docker-mode) ⭐

### 1. Поднять всё

```bash
make all-up
```

Проверка:
```bash
curl http://localhost:13133/health
docker logs filebeat --tail=50
docker compose ps  # должны быть подняты все app-сервисы
```

### 2. Получить session_id и user_id через IAM

```bash
# Register
grpcurl -plaintext -d '{"email":"test@example.com","password":"password123"}' \
  localhost:50053 iam.IAMService/Register

# Login (получить session_id и user_id)
grpcurl -plaintext -d '{"email":"test@example.com","password":"password123"}' \
  localhost:50053 iam.IAMService/Login
```

### 3. Создать заказ

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "x-session-id: <SESSION_ID>" \
  -d '{"user_id":"<USER_ID>","items":[{"product_id":"p1","quantity":1}]}'
```

### 4. Проверить trace в Jaeger

- Открыть http://localhost:16686
- Service: `order` → Find Traces
- Должен быть trace с spans: HTTP POST /orders, CreateOrder, Inventory.ReserveStock, Payment.Charge и вызовы в inventory/payment.

### 5. Проверить логи в Kibana

- http://localhost:5601 → Discover → filebeat-*
- Поиск по полю `trace_id` (скопировать из Jaeger) или по `container.name: order`.
- **В docker-mode видны логи всех app-сервисов** ✅

### 6. Проверить метрики в Grafana

- http://localhost:3000 (admin/admin) → Explore → Prometheus
- Запросы: `orders_created_total`, `order_revenue_total`, `assembly_duration_ms`
- Targets: http://localhost:9090/targets — job otel-collector UP

### 7. Проверить алерт в Telegram

**Настроить Telegram (если ещё не настроено):**
```bash
# В .env или экспортировать перед make all-up:
export TELEGRAM_BOT_TOKEN="your_bot_token"
export ALERT_TELEGRAM_CHAT_ID="your_chat_id"
```

Сгенерировать >10 заказов за минуту:
```bash
SESSION_ID=<your_session> USER_ID=<your_user> make gen-orders
```

Или вручную цикл с curl POST /orders 11 раз с паузой несколько секунд. В Alertmanager (http://localhost:9093) появится HighOrderRate; в Telegram — сообщение от Notification.

**Проверка webhook вручную:**
```bash
curl -X POST http://localhost:8081/alerts \
  -H "Content-Type: application/json" \
  -d '{"version":"4","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"Test"},"annotations":{"summary":"Test"}}]}'
```

---

## End-to-End чеклист (Host-mode)

### 1. Поднять observability стек

```bash
make obs-up
curl http://localhost:13133/health
docker logs filebeat --tail=50
```

### 2. Запустить app-сервисы на хосте

```bash
export OTEL_ENABLED=1
export OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
export OTEL_SAMPLING_RATIO=1.0

make iam-run          # в отдельном терминале
make inventory-run
make payment-run
make order-run
make assembly-run
make notification-run # для приёма алертов на :8081
```

### 3-7. Аналогично docker-mode

⚠️ **Отличие:** в Kibana будут видны только логи observability stack, не app-сервисов (они на хосте, не в контейнерах).

---

## Пропагация и платформа

- **platform/observability:** extract/inject контекста (HTTP + gRPC), trace_id/span_id в zap, единый формат логов (service, env, version, trace_id, span_id).
- W3C Trace Context + Baggage; gRPC — через metadata.

## Локальный запуск без OTEL

При OTEL_ENABLED не задан или false — noop TracerProvider/MeterProvider, трассировка и экспорт метрик отключены.
