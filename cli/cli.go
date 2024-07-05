package cli

import (
	"context"
	"fmt"

	"github.com/goto/salt/cmdx"
	"github.com/spf13/cobra"

	"github.com/goto/entropy/cli/client"
)

var rootCmd = &cobra.Command{
	Use: "entropy",
	Long: `Entropy is a framework to safely and predictably create, change, 
and improve modern cloud applications and infrastructure using 
familiar languages, tools, and engineering practices.`,
}

func Execute(ctx context.Context) {
	cfg, err := loadConfig(rootCmd)
	if err != nil {
		panic(err)
	}
	clientCfg := client.Config{
		Host: fmt.Sprintf("%s:%d", cfg.Service.Host, cfg.Service.Port),
	}

	rootCmd.PersistentFlags().StringP(configFlag, "c", "", "Override config file")
	rootCmd.AddCommand(
		cmdServe(),
		cmdMigrate(),
		cmdVersion(),
		cmdShowConfigs(),
		cmdInitConfig(),
		client.ResourceCommand(&clientCfg),
		client.ModuleCommand(&clientCfg),
	)

	cmdx.SetHelp(rootCmd)
	_ = rootCmd.ExecuteContext(ctx)
}
