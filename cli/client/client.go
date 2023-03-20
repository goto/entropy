package client

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
	entropyHostFlag = "entropy"
	dialTimeoutFlag = "timeout"
)

func Command() *cobra.Command {
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

	cmd.PersistentFlags().StringP(entropyHostFlag, "E", "", "Entropy host to connect to")
	cmd.PersistentFlags().DurationP(dialTimeoutFlag, "T", 5*time.Second, "Dial timeout")

	cmd.AddCommand(
		cmdStreamLogs(),
		cmdApplyAction(),
		cmdCreateResource(),
		cmdListResources(),
		cmdViewResource(),
		cmdEditResource(),
		cmdDeleteResource(),
		cmdGetGetRevisions(),
	)

	return cmd
}

func createConnection(ctx context.Context, host string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	return grpc.DialContext(ctx, host, opts...)
}

func createClient(cmd *cobra.Command) (entropyv1beta1.ResourceServiceClient, func(), error) {
	dialTimeout, _ := cmd.Flags().GetDuration(dialTimeoutFlag)
	entropyAddr, _ := cmd.Flags().GetString(entropyHostFlag)

	dialTimeoutCtx, dialCancel := context.WithTimeout(cmd.Context(), dialTimeout)
	conn, err := createConnection(dialTimeoutCtx, entropyAddr)
	if err != nil {
		dialCancel()
		return nil, nil, err
	}

	cancel := func() {
		dialCancel()
		_ = conn.Close()
	}

	client := entropyv1beta1.NewResourceServiceClient(conn)
	return client, cancel, nil
}
