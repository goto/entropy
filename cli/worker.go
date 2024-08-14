package cli

import (
	"context"
	"sync"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/goto/entropy/core"
	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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

		wg := spawnWorkers(cmd.Context(), resourceService, cfg.Syncer.WorkerModules, cfg.Syncer.SyncInterval)
		defer func() {
			wg.Wait()
			zap.L().Info("all syncer workers exited")
		}()

		return nil
	})

	return cmd
}

func spawnWorkers(ctx context.Context, resourceService *core.Service, workerModules []workerModule, syncInterval time.Duration) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	if len(workerModules) == 0 {
		wg = resourceService.RunSyncer(ctx, 1, syncInterval, map[string][]string{}, wg)
	} else {
		for _, module := range workerModules {
			wg = resourceService.RunSyncer(ctx, module.Count, syncInterval, module.Scope, wg)
		}
	}
	return wg
}
