package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const serviceVersion = "1.0.0"

// Config holds OpenTelemetry configuration.
type Config struct {
	Enabled     bool
	Endpoint    string
	ServiceName string
	Insecure    bool
}

// Provider wraps the OTel TracerProvider and MeterProvider for clean shutdown.
type Provider struct {
	tp *trace.TracerProvider
	mp *metric.MeterProvider
}

// Init initializes OpenTelemetry exporters, providers, and propagators.
// Returns nil Provider if disabled.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	// gRPC dial options
	var dialOpts []grpc.DialOption
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, err
	}

	// Trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(10*time.Second))),
	)

	// Set global providers and propagators
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{tp: tp, mp: mp}, nil
}

// Shutdown flushes and shuts down the providers. Safe to call on nil.
func (p *Provider) Shutdown(ctx context.Context) {
	if p == nil {
		return
	}
	if p.tp != nil {
		_ = p.tp.Shutdown(ctx)
	}
	if p.mp != nil {
		_ = p.mp.Shutdown(ctx)
	}
}
