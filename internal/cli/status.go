package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/djburkhart/kdlctl/internal/gcp"
	kdlnats "github.com/djburkhart/kdlctl/internal/nats"
)

func newStatusCmd() *cobra.Command {
	var buildID string
	var environment string
	var follow bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get deployment status from Cloud Build or NATS",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case buildID != "":
				cfg, err := loadProjectConfig()
				if err != nil {
					return err
				}

				client, err := gcp.NewCloudBuildClient(cmd.Context())
				if err != nil {
					return err
				}
				defer client.Close()

				result, err := client.GetBuildStatus(cmd.Context(), cfg.ProjectID, buildID)
				if err != nil {
					return err
				}

				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Build %s: %s %s\n", result.ID, result.Status, result.LogURL)
				return err
			case environment != "":
				client, err := kdlnats.NewClient(viper.GetString("nats-url"))
				if err != nil {
					return err
				}
				defer client.Close()

				subject := fmt.Sprintf("deploy.status.%s.>", environment)
				ctx := cmd.Context()
				if !follow {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
				}

				count := 0
				if !follow {
					count = 1
				}

				return client.Subscribe(ctx, subject, count, func(msg *nats.Msg) error {
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", msg.Subject, string(msg.Data))
					return err
				})
			default:
				return fmt.Errorf("set either --build or --env")
			}
		},
	}

	cmd.Flags().StringVar(&buildID, "build", "", "Cloud Build ID")
	cmd.Flags().StringVar(&environment, "env", "", "Environment to watch over NATS")
	cmd.Flags().BoolVar(&follow, "follow", false, "Keep streaming NATS status messages")
	return cmd
}
