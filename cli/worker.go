package cli

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/goto/entropy/core"
	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/logger"
	"github.com/goto/entropy/pkg/telemetry"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func cmdWorker() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start workers",
		Example: heredoc.Doc(`
			$ entropy worker
		`),
		Annotations: map[string]string{
			"group:other": "server",
		},
	}

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		err = logger.Setup(&cfg.Log)
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		telemetry.Init(ctx, cfg.Telemetry)
		_, err = newrelic.NewApplication(
			newrelic.ConfigAppName(cfg.Telemetry.ServiceName),
			newrelic.ConfigLicense(cfg.Telemetry.NewRelicAPIKey),
		)
		if err != nil {
			zap.L().Error("error initializing opentelemetry", zap.Error(err))
		}

		return StartWorkers(ctx, cfg)
	})

	return cmd
}

func StartWorkers(ctx context.Context, cfg Config) error {
	store := setupStorage(cfg.PGConnStr, cfg.Syncer, cfg.Service)
	moduleService := module.NewService(setupRegistry(), store)
	resourceService := core.New(store, moduleService, time.Now, cfg.Syncer.SyncBackoffInterval, cfg.Syncer.MaxRetries, cfg.Telemetry.ServiceName)

	eg := &errgroup.Group{}
	spawnWorkers(ctx, resourceService, cfg.Syncer.Workers, cfg.Syncer.SyncInterval, eg)
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func spawnWorkers(ctx context.Context, resourceService *core.Service, workerModules map[string]WorkerConfig, syncInterval time.Duration, eg *errgroup.Group) {
	if len(workerModules) == 0 {
		resourceService.RunSyncer(ctx, 1, syncInterval, map[string][]string{}, eg)
	} else {
		for _, module := range workerModules {
			resourceService.RunSyncer(ctx, module.Count, syncInterval, module.Scope, eg)
		}
	}
}
