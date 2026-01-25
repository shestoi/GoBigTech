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