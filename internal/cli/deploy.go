package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/djburkhart/kdlctl/internal/deploy"
	"github.com/djburkhart/kdlctl/internal/gcp"
	kdlnats "github.com/djburkhart/kdlctl/internal/nats"
)

type deployEvent struct {
	ProjectID   string   `json:"projectId"`
	Region      string   `json:"region"`
	Environment string   `json:"environment"`
	Services    []string `json:"services"`
}

func newDeployCmd() *cobra.Command {
	var environment string
	var service string
	var async bool
	var viaNATS bool
	var subject string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy services to Google Cloud",
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

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()

			if viaNATS {
				client, err := kdlnats.NewClient(viper.GetString("nats-url"))
				if err != nil {
					return err
				}
				defer client.Close()

				serviceNames := make([]string, 0, len(plan.Services))
				for _, svc := range plan.Services {
					serviceNames = append(serviceNames, svc.Name)
				}

				payload, err := json.Marshal(deployEvent{
					ProjectID:   plan.ProjectID,
					Region:      plan.Region,
					Environment: plan.Environment,
					Services:    serviceNames,
				})
				if err != nil {
					return fmt.Errorf("marshal deploy event: %w", err)
				}

				if err := client.Publish(ctx, subject, payload); err != nil {
					return err
				}

				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published deploy request to %s for %d service(s)\n", subject, len(plan.Services))
				return err
			}

			buildClient, err := gcp.NewCloudBuildClient(ctx)
			if err != nil {
				return err
			}
			defer buildClient.Close()

			for _, svc := range plan.Services {
				result, err := buildClient.SubmitCloudRunBuild(ctx, gcp.BuildRequest{
					ProjectID:   plan.ProjectID,
					Region:      plan.Region,
					Environment: plan.Environment,
					Service:     svc,
				}, !async)
				if err != nil {
					return err
				}

				if async {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Queued deploy for %s (operation: %s)\n", svc.Name, result.Operation); err != nil {
						return err
					}
					continue
				}

				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deployed %s with status %s", svc.Name, result.Status); err != nil {
					return err
				}
				if result.ID != "" {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), " (build: %s)", result.ID); err != nil {
						return err
					}
				}
				if result.LogURL != "" {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), " %s", result.LogURL); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&environment, "env", "", "Environment to deploy")
	cmd.Flags().StringVar(&service, "service", "", "Limit deployment to a single service")
	cmd.Flags().BoolVar(&async, "async", false, "Return after the Cloud Build operation is queued")
	cmd.Flags().BoolVar(&viaNATS, "via-nats", false, "Publish a deployment event instead of submitting Cloud Build directly")
	cmd.Flags().StringVar(&subject, "subject", "deploy.requested", "NATS subject to publish deploy events to")
	return cmd
}
