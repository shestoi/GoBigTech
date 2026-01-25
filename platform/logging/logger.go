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
	var level zapcore.Level //Создаём переменную типа zapcore.Level.
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
	// encoder - это функция, которая преобразует данные в строку
	// encoderConfig - это конфигурация для encoder
	var encoder zapcore.Encoder             //создаём переменную типа zapcore.Encoder
	encoderConfig := zapcore.EncoderConfig{ //конфигурация для encoder
		TimeKey:        "ts",                           //ключ для времени
		LevelKey:       "level",                        //ключ для уровенля логирования
		NameKey:        "logger",                       //ключ для имени logger
		CallerKey:      "caller",                       //ключ для вызывающего кода
		MessageKey:     "msg",                          //ключ для сообщения
		StacktraceKey:  "stacktrace",                   //ключ для stacktrace
		LineEnding:     zapcore.DefaultLineEnding,      //конец строки
		EncodeLevel:    zapcore.LowercaseLevelEncoder,  //кодируем уровень логирования
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder, //кодируем время
		EncodeDuration: zapcore.SecondsDurationEncoder, //кодируем duration
		EncodeCaller:   zapcore.ShortCallerEncoder,     //кодируем вызывающий код
	}

	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig) //создаём json encoder
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig) //создаём console encoder
	}

	// Создаём core
	// core - это основная часть zap, которая собирает логи и отправляет их в writer
	//Это "сборка движка"
	core := zapcore.NewCore(
		encoder,                    //как форматировать
		zapcore.AddSync(os.Stderr), //куда отправлять
		level,                      //минимальный уровень логирования
	)

	// Создаём logger с опциями
	var opts []zap.Option
	if cfg.AddCaller { //если нужно добавлять информацию о вызывающем коде
		opts = append(opts, zap.AddCaller()) //добавляем опцию AddCaller
	}
	logger := zap.New(core, opts...) //создаём logger с опциями

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
	_ = log.Sync() //попытка дописать всё, что могло остаться в буфере
}
