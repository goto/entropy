package cli

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/goto/entropy/core"
	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/logger"
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

		store := setupStorage(cfg.PGConnStr, cfg.Syncer, cfg.Service)
		moduleService := module.NewService(setupRegistry(), store)
		resourceService := core.New(store, moduleService, time.Now, cfg.Syncer.SyncBackoffInterval, cfg.Syncer.MaxRetries)

		eg := &errgroup.Group{}
		spawnWorkers(cmd.Context(), resourceService, cfg.Syncer.Workers, cfg.Syncer.SyncInterval, eg)
		if err := eg.Wait(); err != nil {
			zap.L().Error("syncer exited with error", zap.Error(err))
			return err
		}

		return nil
	})

	return cmd
}

func spawnWorkers(ctx context.Context, resourceService *core.Service, workerModules []workerConfig, syncInterval time.Duration, eg *errgroup.Group) {
	if len(workerModules) == 0 {
		resourceService.RunSyncer(ctx, 1, syncInterval, map[string][]string{}, eg)
	} else {
		for _, module := range workerModules {
			resourceService.RunSyncer(ctx, module.Count, syncInterval, module.Scope, eg)
		}
	}
}
