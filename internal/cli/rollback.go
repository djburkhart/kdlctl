package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/djburkhart/kdlctl/internal/config"
)

func newRollbackCmd() *cobra.Command {
	var environment string
	var service string
	var revision string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back a Cloud Run service to a prior revision",
		RunE: func(cmd *cobra.Command, args []string) error {
			if environment == "" || service == "" || revision == "" {
				return fmt.Errorf("--env, --service, and --revision are required")
			}

			cfg, err := loadProjectConfig()
			if err != nil {
				return err
			}

			if err := config.ValidateEnvironment(cfg, environment); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
			defer cancel()

			client, err := newRunClient(ctx)
			if err != nil {
				return err
			}
			defer client.Close()

			if err := client.RollbackTraffic(ctx, cfg.ProjectID, cfg.Region, service, revision); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Rolled back %s in %s to revision %s\n", service, environment, revision)
			return err
		},
	}

	cmd.Flags().StringVar(&environment, "env", "", "Environment to roll back")
	cmd.Flags().StringVar(&service, "service", "", "Cloud Run service name")
	cmd.Flags().StringVar(&revision, "revision", "", "Revision name to send 100% of traffic to")
	return cmd
}
