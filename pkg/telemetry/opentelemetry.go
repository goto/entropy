package telemetry

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func setupOpenTelemetry(ctx context.Context, mux *http.ServeMux, cfg Config) error {
	var options []sdkmetric.Option
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
	if cfg.EnableOtelAgent {
		opt, err := setupOTELMetrics(ctx, cfg)
		if err != nil {
			return err
		}
		options = append(options, opt...)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		append([]sdkmetric.Option{sdkmetric.WithResource(res)}, options...)...,
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

func setupOTELMetrics(ctx context.Context, cfg Config) ([]sdkmetric.Option, error) {
	var sdkMetricOptions []sdkmetric.Option
	var periodicReaderOptions []sdkmetric.PeriodicReaderOption

	promExporter, err := prometheus.New(prometheus.WithNamespace(cfg.ServiceName))
	if err != nil {
		return nil, err
	}

	otlpExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(cfg.OpenTelAgentAddr),
	)
	if err != nil {
		return nil, err
	}

	periodicReaderOptions = append(periodicReaderOptions, sdkmetric.WithInterval(10*time.Second))

	if cfg.EnableRuntimeMetrics {
		periodicReaderOptions = append(periodicReaderOptions, sdkmetric.WithProducer(runtime.NewProducer()))
	}

	sdkMetricOptions = append(sdkMetricOptions, sdkmetric.WithReader(promExporter))
	sdkMetricOptions = append(sdkMetricOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
		otlpExporter,
		periodicReaderOptions...,
	)))

	return sdkMetricOptions, nil

}

func GetMeter(name string) metric.Meter {
	return otel.GetMeterProvider().Meter(name)
}
