package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// Init инициализирует OpenTelemetry: TracerProvider, MeterProvider, global propagator.
// Если cfg.Enabled == false — ставит noop providers и возвращает noop shutdown.
// Иначе создаёт OTLP exporters, BatchSpanProcessor, ParentBased(TraceIDRatioBased), устанавливает globals.
// shutdown нужно вызвать при остановке сервиса (например через platform/shutdown).
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled { // если observability не включено, то устанавливаем noop providers и возвращаем noop shutdown
		otel.SetTracerProvider(nooptrace.NewTracerProvider())
		otel.SetMeterProvider(noop.NewMeterProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator( // composite text map propagator - это propagator, который содержит trace context и baggage
			propagation.TraceContext{}, // trace context - это контекст, который содержит trace id и span id
			propagation.Baggage{},      // baggage - это контекст, который содержит baggage
		))
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("deployment.environment", cfg.DeploymentEnvironment),
		),
		resource.WithProcessRuntimeDescription(),
	)
	if err != nil {
		return nil, fmt.Errorf("observability resource: %w", err)
	}
	if cfg.ServiceVersion != "" {
		res, _ = resource.Merge(res, resource.NewWithAttributes("",
			attribute.String("service.version", cfg.ServiceVersion),
		))
	}

	// Trace exporter
	traceExp, err := otlptracegrpc.New(ctx, // otlptracegrpc.New() - это функция из пакета otlptracegrpc, которая создает новый trace exporter
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint), // with endpoint - это функция из пакета otlptracegrpc, которая устанавливает endpoint для trace exporter
		otlptracegrpc.WithInsecure(),                 // with insecure - это функция из пакета otlptracegrpc, которая устанавливает insecure для trace exporter
	)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRatio)) // parent based sampler - это sampler, который содержит parent based sampler
	tp := sdktrace.NewTracerProvider(                                              // new tracer provider - это функция из пакета sdktrace, которая создает новый tracer provider
		sdktrace.WithResource(res),     // with resource - это функция из пакета sdktrace, которая устанавливает resource для tracer provider
		sdktrace.WithBatcher(traceExp), // with batcher - это функция из пакета sdktrace, которая устанавливает batcher для tracer provider
		sdktrace.WithSampler(sampler),  // with sampler - это функция из пакета sdktrace, которая устанавливает sampler для tracer provider
	)
	otel.SetTracerProvider(tp)

	// MeterProvider с OTLP metrics exporter
	metricExp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		tp.Shutdown(context.Background())
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown = func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		if err := mp.Shutdown(ctx); err != nil {
			return err
		}
		return nil
	}
	return shutdown, nil
}
