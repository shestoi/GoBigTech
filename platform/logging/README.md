# Platform Logging

Единый structured logging на основе [zap](https://github.com/uber-go/zap) для всех сервисов GoBigTech.

## Использование

### Создание logger

```go
import (
    platformlogging "github.com/shestoi/GoBigTech/platform/logging"
    "go.uber.org/zap"
)

// В main.go после загрузки config
cfg := config.Load()
logger, err := platformlogging.New(platformlogging.Config{
    ServiceName: "order",  // или "inventory", "payment"
    Env:         cfg.AppEnv, // "local" или "docker"
    Level:       "info",     // debug/info/warn/error, default "info"
    Format:      "",          // "json"|"console", default: local=console, docker=json
    AddCaller:   true,        // default: local=true, docker=false
})
if err != nil {
    log.Fatalf("Failed to create logger: %v", err)
}
defer platformlogging.Sync(logger)
```

### Базовое логирование

```go
logger.Info("Service started", zap.String("addr", "127.0.0.1:8080"))
logger.Error("Failed to connect", zap.Error(err))
logger.Debug("Processing request", zap.String("user_id", userID))
logger.Warn("Deprecated feature used")
```

### Использование op паттерна

Для отслеживания операций используйте константу `op`:

```go
const op = "OrderService.CreateOrder"

func (s *OrderService) CreateOrder(ctx context.Context, input CreateOrderInput) error {
    logger := logger.With(zap.String("op", op))
    logger.Info("Creating order", zap.String("user_id", input.UserID))
    
    // ... бизнес-логика ...
    
    if err != nil {
        logger.Error("Failed to create order", zap.Error(err))
        return err
    }
    
    logger.Info("Order created successfully", zap.String("order_id", orderID))
    return nil
}
```

### Передача logger в зависимости

```go
// В main.go
handler := httpapi.NewHandler(orderService, logger)

// В handler
type Handler struct {
    orderService *service.OrderService
    logger       *zap.Logger
}

func (h *Handler) PostOrders(w http.ResponseWriter, r *http.Request) {
    const op = "Handler.PostOrders"
    logger := h.logger.With(zap.String("op", op))
    
    logger.Info("Received request", zap.String("method", r.Method))
    // ...
}
```

## Конфигурация через переменные окружения

Рекомендуется использовать переменные окружения для настройки:

- `LOG_LEVEL` - уровень логирования (debug/info/warn/error), default "info"
- `LOG_FORMAT` - формат вывода (json/console), default: local=console, docker=json
- `LOG_ADD_CALLER` - добавлять caller info (true/false), default: local=true, docker=false

## Формат логов

### Console формат (local)
```
2026-01-09T12:00:00.000000000Z	info	order/main.go:27	Service started	{"service": "order", "env": "local", "addr": "127.0.0.1:8080"}
```

### JSON формат (docker)
```json
{"ts":"2026-01-09T12:00:00.000000000Z","level":"info","caller":"order/main.go:27","msg":"Service started","service":"order","env":"docker","addr":"127.0.0.1:8080"}
```

Все логи автоматически содержат поля `service` и `env`.

