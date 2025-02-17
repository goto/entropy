package telemetry

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

	err = setupCommonMetrics(GetMeter(cfg.ServiceName))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			otel.Handle(err)
		}
	}()

	return nil
}

func setupCommonMetrics(meter metric.Meter) error {
	pending_count, err := meter.Int64UpDownCounter(
		"pending_count",
		metric.WithDescription("Total number of resources with status PENDING"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64UpDownCounter(
		"completed_count",
		metric.WithDescription("Total number of resources with status COMPLETED"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64UpDownCounter(
		"error_counter",
		metric.WithDescription("Total number of resources with status ERROR"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			// Simulate request count increment
			pending_count.Add(context.Background(), 1,
				metric.WithAttributes(attribute.String("status", "PENDING")),
			)
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

func GetMeter(name string) metric.Meter {
	return otel.GetMeterProvider().Meter(name)
}
