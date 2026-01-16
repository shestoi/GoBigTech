package logging

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config содержит конфигурацию для создания logger
type Config struct {
	// ServiceName имя сервиса (order/inventory/payment)
	ServiceName string
	// Env окружение (local/docker)
	Env string
	// Level уровень логирования (debug/info/warn/error), default "info"
	Level string
	// Format формат вывода ("json"|"console"), default: local=console, docker=json
	Format string
	// AddCaller добавлять ли информацию о вызывающем коде, default: local=true, docker=false
	AddCaller bool
}

// New создаёт новый zap.Logger с указанной конфигурацией
// Всегда добавляет поля service и env ко всем логам
func New(cfg Config) (*zap.Logger, error) {
	// Устанавливаем значения по умолчанию
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Format == "" {
		if cfg.Env == "docker" {
			cfg.Format = "json"
		} else {
			cfg.Format = "console"
		}
	}
	// AddCaller по умолчанию: true для local, false для docker
	if cfg.Env == "docker" && !cfg.AddCaller {
		cfg.AddCaller = false
	} else if cfg.Env == "local" && !cfg.AddCaller {
		cfg.AddCaller = true
	}

	// Парсим уровень логирования
	var level zapcore.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		return nil, fmt.Errorf("invalid log level: %s (must be debug/info/warn/error)", cfg.Level)
	}

	// Настраиваем encoder в зависимости от формата
	var encoder zapcore.Encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Создаём core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stderr),
		level,
	)

	// Создаём logger с опциями
	var opts []zap.Option
	if cfg.AddCaller {
		opts = append(opts, zap.AddCaller())
	}
	logger := zap.New(core, opts...)

	// Добавляем service и env ко всем логам
	logger = logger.With(
		zap.String("service", cfg.ServiceName),
		zap.String("env", cfg.Env),
	)

	return logger, nil
}

// Sync безопасно вызывает log.Sync(), игнорируя harmless ошибки
// (например, "sync /dev/stderr: invalid argument" на некоторых системах)
func Sync(log *zap.Logger) {
	_ = log.Sync()
}

