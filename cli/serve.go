package cli

import (
	"context"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/goto/entropy/core"
	"github.com/goto/entropy/core/module"
	entropyserver "github.com/goto/entropy/internal/server"
	"github.com/goto/entropy/internal/store/postgres"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/dagger"
	"github.com/goto/entropy/modules/firehose"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/modules/job"
	"github.com/goto/entropy/modules/kafka"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/logger"
	"github.com/goto/entropy/pkg/telemetry"
)

func cmdServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start gRPC & HTTP servers and optionally workers",
		Aliases: []string{"server", "start"},
		Annotations: map[string]string{
			"group:other": "server",
		},
	}

	var migrate, spawnWorker bool
	cmd.Flags().BoolVar(&migrate, "migrate", false, "Run migrations before starting")
	cmd.Flags().BoolVar(&spawnWorker, "worker", false, "Run worker threads as well")

	cmd.RunE = handleErr(func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		return StartServer(cmd.Context(), cfg, migrate, spawnWorker)
	})

	return cmd
}

func StartServer(ctx context.Context, cfg Config, migrate, spawnWorker bool) error {
	err := logger.Setup(&cfg.Log)
	if err != nil {
		return err
	}

	telemetry.Init(ctx, cfg.Telemetry)
	nrApp, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.Telemetry.ServiceName),
		newrelic.ConfigLicense(cfg.Telemetry.NewRelicAPIKey),
	)

	store := setupStorage(cfg.PGConnStr, cfg.Syncer, cfg.Service)
	moduleService := module.NewService(setupRegistry(), store)
	resourceService := core.New(store, moduleService, time.Now, cfg.Syncer.SyncBackoffInterval, cfg.Syncer.MaxRetries, cfg.Telemetry.ServiceName)

	if migrate {
		if migrateErr := runMigrations(ctx, cfg); migrateErr != nil {
			return migrateErr
		}
	}

	if spawnWorker {
		eg := &errgroup.Group{}
		spawnWorkers(ctx, resourceService, cfg.Syncer.Workers, cfg.Syncer.SyncInterval, eg)
		go func() {
			if err := eg.Wait(); err != nil {
				zap.L().Error("syncer exited with error", zap.Error(err))
			}
		}()
	}

	return entropyserver.Serve(ctx,
		cfg.Service.httpAddr(), cfg.Service.grpcAddr(),
		nrApp, resourceService, moduleService,
	)
}

func setupRegistry() module.Registry {
	supported := []module.Descriptor{
		kubernetes.Module,
		firehose.Module,
		job.Module,
		kafka.Module,
		flink.Module,
		dagger.Module,
	}

	registry := &modules.Registry{}
	for _, desc := range supported {
		if err := registry.Register(desc); err != nil {
			zap.L().Fatal("failed to register module",
				zap.String("module_kind", desc.Kind),
				zap.Error(err),
			)
		}
	}
	return registry
}

func setupStorage(pgConStr string, syncCfg SyncerConf, serveCfg ServeConfig) *postgres.Store {
	store, err := postgres.Open(pgConStr, syncCfg.RefreshInterval, syncCfg.ExtendLockBy, serveCfg.PaginationSizeDefault, serveCfg.PaginationPageDefault)
	if err != nil {
		zap.L().Fatal("failed to connect to Postgres database",
			zap.Error(err), zap.String("conn_str", pgConStr))
	}
	return store
}
