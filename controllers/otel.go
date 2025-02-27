package controllers

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitOpenTelemetry(ctx context.Context) (*sdktrace.TracerProvider, *otellog.LoggerProvider, *metric.MeterProvider) {
	otelendpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	// Create resource.
	res, err := newResource()
	if err != nil {
		log.Panic("failed to create resource:", err)
	}

	tracer := initTracer(ctx, res, otelendpoint)
	logProvider := initLogProvider(ctx, res, otelendpoint)
	metricProvider := initMetricProvider(ctx, res, otelendpoint)

	return tracer, logProvider, metricProvider
}

func initTracer(ctx context.Context, resource *resource.Resource, otelendpoint string) *sdktrace.TracerProvider {
	exp, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(otelendpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Panic(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource),
		sdktrace.WithBatcher(exp),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

func newResource() (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("zeroshare-backend"),
			semconv.ServiceVersionKey.String("0.1.0"),
		))
}

func initLogProvider(ctx context.Context, res *resource.Resource, otelendpoint string) *otellog.LoggerProvider {
	exporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(otelendpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Panic("failed to create exporter:", err)
	}
	processor := otellog.NewBatchProcessor(exporter)
	provider := otellog.NewLoggerProvider(
		otellog.WithResource(res),
		otellog.WithProcessor(processor),
	)
	return provider
}

func initMetricProvider(ctx context.Context, res *resource.Resource, otelendpoint string) *metric.MeterProvider {
	exp, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(otelendpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Panic("failed to create metrics exporter:", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exp,
			metric.WithInterval(10*time.Second),
		)),
	)
	otel.SetMeterProvider(mp)
	return mp
}
