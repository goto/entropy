package cli

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/goto/salt/cmdx"
	"github.com/goto/salt/config"
	"github.com/goto/salt/printer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/logger"
	"github.com/goto/entropy/pkg/telemetry"
)

const configFlag = "config"

// Config contains the application configuration.
type Config struct {
	Log       logger.LogConfig `mapstructure:"log"`
	Syncer    SyncerConf       `mapstructure:"syncer"`
	Service   ServeConfig      `mapstructure:"service"`
	PGConnStr string           `mapstructure:"pg_conn_str" default:"postgres://postgres@localhost:5432/entropy?sslmode=disable"`
	Telemetry telemetry.Config `mapstructure:"telemetry"`
}

type SyncerConf struct {
	SyncInterval        time.Duration           `mapstructure:"sync_interval" default:"1s"`
	RefreshInterval     time.Duration           `mapstructure:"refresh_interval" default:"3s"`
	ExtendLockBy        time.Duration           `mapstructure:"extend_lock_by" default:"5s"`
	SyncBackoffInterval time.Duration           `mapstructure:"sync_backoff_interval" default:"5s"`
	MaxRetries          int                     `mapstructure:"max_retries" default:"5"`
	Workers             map[string]WorkerConfig `mapstructure:"workers" default:"[]"`
}

type WorkerConfig struct {
	Count int                 `mapstructure:"count" default:"1"`
	Scope map[string][]string `mapstructure:"labels"`
}

type ServeConfig struct {
	Host string `mapstructure:"host" default:""`
	Port int    `mapstructure:"port" default:"8080"`

	HTTPAddr              string `mapstructure:"http_addr" default:":8081"`
	PaginationSizeDefault int32  `mapstructure:"pagination_size_default" default:"0"`
	PaginationPageDefault int32  `mapstructure:"pagination_page_default" default:"1"`
}

type clientConfig struct {
	Host string `mapstructure:"host" default:"localhost:8080"`
}

func (serveCfg ServeConfig) httpAddr() string { return serveCfg.HTTPAddr }

func (serveCfg ServeConfig) grpcAddr() string {
	return fmt.Sprintf("%s:%d", serveCfg.Host, serveCfg.Port)
}

func cmdConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Example: heredoc.Doc(`
			$ entropy config init
			$ entropy config view
		`),
	}

	cmd.AddCommand(cmdInitConfig(), cmdShowConfigs())

	return cmd
}

func cmdInitConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize client configuration",
		RunE: handleErr(func(cmd *cobra.Command, args []string) error {
			cfg := cmdx.SetConfig("entropy")

			if err := cfg.Init(&clientConfig{}); err != nil {
				return err
			}

			fmt.Printf("config created: %v\n", cfg.File())
			return nil
		}),
	}
}

func cmdShowConfigs() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Display configurations currently loaded",
		RunE: handleErr(func(cmd *cobra.Command, args []string) error {
			clientCfg := cmdx.SetConfig("entropy")
			data, err := clientCfg.Read()
			if err != nil {
				fatalExitf("failed to read client configs: %v", err)
			}
			printer.Textln("Client config")
			yaml.NewEncoder(os.Stdout).Encode(data)

			cfg, err := loadConfig(cmd)
			if err != nil {
				fatalExitf("failed to read configs: %v", err)
			}
			printer.Textln("Config")
			return yaml.NewEncoder(os.Stdout).Encode(cfg)
		}),
	}
}

func loadConfig(cmd *cobra.Command) (Config, error) {
	var opts []config.LoaderOption

	cfgFile, _ := cmd.Flags().GetString(configFlag)
	if cfgFile != "" {
		opts = append(opts, config.WithFile(cfgFile))
	} else {
		opts = append(opts,
			config.WithPath("./"),
			config.WithName("entropy"),
		)
	}

	var cfg Config
	err := config.NewLoader(opts...).Load(&cfg)
	if errors.As(err, &config.ConfigFileNotFoundError{}) {
		log.Println(err)
	} else {
		return cfg, err
	}

	PaginationSizeDefault = cfg.Service.PaginationSizeDefault
	PaginationPageDefault = cfg.Service.PaginationPageDefault

	return cfg, nil
}

func loadClientConfig() (*clientConfig, error) {
	var config clientConfig

	cfg := cmdx.SetConfig("entropy")
	err := cfg.Load(&config)

	return &config, err
}
