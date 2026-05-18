package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/djburkhart/kdlctl/internal/deploy"
)

func newPlanCmd() *cobra.Command {
	var environment string
	var service string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Render a workload and infrastructure plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if environment == "" {
				return fmt.Errorf("--env is required")
			}

			cfg, err := loadProjectConfig()
			if err != nil {
				return err
			}

			plan, err := deploy.BuildPlan(cfg, environment, service)
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(cmd, plan)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), plan.Render())
			return err
		},
	}

	cmd.Flags().StringVar(&environment, "env", "", "Environment to plan")
	cmd.Flags().StringVar(&service, "service", "", "Limit the plan to a single named target")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Render the plan as JSON")
	return cmd
}
