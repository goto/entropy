package telemetry

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func setupOpenTelemetry(ctx context.Context, mux *http.ServeMux, cfg Config) error {
	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return err
	}

	// Setup metrics if enabled
	if cfg.EnableMetrics {
		if err := setupMetrics(ctx, cfg, res, mux); err != nil {
			return err
		}
	}

	return nil
}

func setupMetrics(ctx context.Context, cfg Config, res *resource.Resource, mux *http.ServeMux) error {
	promExporter, err := prometheus.New(prometheus.WithNamespace(cfg.ServiceName))
	if err != nil {
		return err
	}

	otlpExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(cfg.OpenTelAgentAddr),
	)
	if err != nil {
		return err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				otlpExporter,
				sdkmetric.WithInterval(10*time.Second),
			),
		),
	)

	otel.SetMeterProvider(meterProvider)

	go func() {
		<-ctx.Done()
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			otel.Handle(err)
		}
	}()

	return nil
}

func GetMeter(name string) metric.Meter {
	return otel.GetMeterProvider().Meter(name)
}
