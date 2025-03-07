package telemetry

import (
	"context"
	"net/http"
	"net/http/pprof"

	"go.uber.org/zap"
)

type Config struct {
	// Debug sets the bind address for pprof & zpages server.
	Debug string `mapstructure:"debug_addr" default:"localhost:8090"`

	// OpenTelemetry trace & metrics configurations.
	EnableRuntimeMetrics bool `mapstructure:"enable_runtime_metrics"`

	// OpenTelemetry exporter configurations.
	ServiceName string `mapstructure:"service_name"`

	// NewRelic configs.
	NewRelicAPIKey string `mapstructure:"newrelic_api_key"`

	// OpenTelemetry Agent exporter.
	EnableOtelAgent  bool   `mapstructure:"enable_otel_agent"`
	OpenTelAgentAddr string `mapstructure:"otel_agent_addr"`
}

// Init initialises OpenTelemetry based async-telemetry processes and
// returns (i.e., it does not block).
func Init(ctx context.Context, cfg Config) {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))

	if err := setupOpenTelemetry(ctx, mux, cfg); err != nil {
		zap.L().Error("failed to setup OpenTelemetry", zap.Error(err))
	}

	zap.L().Info("telemetry server started", zap.String("debug_addr", cfg.Debug))

	if cfg.Debug != "" {
		go func() {
			//nolint:gosec
			if err := http.ListenAndServe(cfg.Debug, mux); err != nil {
				zap.L().Error("debug server exited due to error", zap.Error(err))
			}
		}()
	}
}
