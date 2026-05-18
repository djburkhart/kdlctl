package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/djburkhart/kdlctl/internal/config"
)

func newValidateCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate deploy.kdl",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadProjectConfig()
			if err != nil {
				return err
			}

			if environment == "" {
				if err := config.ValidateProject(cfg); err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Configuration is valid for %d environments\n", len(cfg.Environments))
				return err
			}

			if err := config.ValidateEnvironment(cfg, environment); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Configuration is valid for environment %q\n", environment)
			return err
		},
	}

	cmd.Flags().StringVar(&environment, "env", "", "Validate a single environment")
	return cmd
}
