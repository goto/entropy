package cli

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
)

const (
	flagOutFormat   = "format"
	flagEntropyHost = "entropy"
	flagDialTimeout = "timeout"

	dialTimeout = 5 * time.Second
)

func ResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Entropy client with resource management commands",
		Example: heredoc.Doc(`
			$ entropy resource create
			$ entropy resource list
			$ entropy resource view <resource-urn>
			$ entropy resource delete <resource-urn>
			$ entropy resource edit <resource-urn>
			$ entropy resource revisions <resource-urn>
		`),
	}

	cfg, _ := loadClientConfig()

	cmd.PersistentFlags().StringP(flagEntropyHost, "h", cfg.Host, "Entropy host to connect to")
	cmd.PersistentFlags().DurationP(flagDialTimeout, "", dialTimeout, "Dial timeout")
	cmd.PersistentFlags().StringP(flagOutFormat, "o", "pretty", "output format (json, yaml, pretty)")

	cmd.AddCommand(
		cmdCreateResource(),
		cmdViewResource(),
		cmdEditResource(),
		cmdStreamLogs(),
		cmdApplyAction(),
		cmdDeleteResource(),
		cmdListRevisions(),
	)

	return cmd
}

func ModuleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "module",
		Short: "Entropy client with module management commands",
		Example: heredoc.Doc(`
			$ entropy resource create <file>
			$ entropy resource update <module-urn> <file>
			$ entropy resource view <module-urn>
		`),
	}

	cfg, _ := loadClientConfig()

	cmd.PersistentFlags().StringP(flagEntropyHost, "h", cfg.Host, "Entropy host to connect to")
	cmd.PersistentFlags().DurationP(flagDialTimeout, "", dialTimeout, "Dial timeout")
	cmd.PersistentFlags().StringP(flagOutFormat, "o", "pretty", "output format (json, yaml, pretty)")

	cmd.AddCommand(
		cmdModuleCreate(),
		cmdModuleUpdate(),
		cmdModuleView(),
	)

	return cmd
}

func createResourceServiceClient(cmd *cobra.Command) (entropyv1beta1.ResourceServiceClient, func(), error) {
	dialTimeoutVal, _ := cmd.Flags().GetDuration(flagDialTimeout)
	entropyAddr, _ := cmd.Flags().GetString(flagEntropyHost)

	dialCtx, dialCancel := context.WithTimeout(cmd.Context(), dialTimeoutVal)
	conn, err := grpc.DialContext(dialCtx, entropyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		dialCancel()
		return nil, nil, err
	}

	cancel := func() {
		dialCancel()
		_ = conn.Close()
	}
	return entropyv1beta1.NewResourceServiceClient(conn), cancel, nil
}

func createModuleServiceClient(cmd *cobra.Command) (entropyv1beta1.ModuleServiceClient, func(), error) {
	dialTimeoutVal, _ := cmd.Flags().GetDuration(flagDialTimeout)
	entropyAddr, _ := cmd.Flags().GetString(flagEntropyHost)

	dialCtx, dialCancel := context.WithTimeout(cmd.Context(), dialTimeoutVal)
	conn, err := grpc.DialContext(dialCtx, entropyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		dialCancel()
		return nil, nil, err
	}

	cancel := func() {
		dialCancel()
		_ = conn.Close()
	}
	return entropyv1beta1.NewModuleServiceClient(conn), cancel, nil
}
