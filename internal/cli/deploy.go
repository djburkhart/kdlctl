package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/djburkhart/kdlctl/internal/deploy"
	"github.com/djburkhart/kdlctl/internal/gcp"
)

type deployEvent struct {
	ProjectID   string   `json:"projectId"`
	Region      string   `json:"region"`
	Environment string   `json:"environment"`
	Targets     []string `json:"targets"`
}

func newDeployCmd() *cobra.Command {
	var environment string
	var service string
	var async bool
	var viaNATS bool
	var subject string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy workloads and managed resources to Google Cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := loadDeployPlan(environment, service)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()

			if viaNATS {
				return publishDeployRequest(ctx, cmd.OutOrStdout(), plan, subject)
			}

			return submitDeployPlan(ctx, cmd.OutOrStdout(), plan, async)
		},
	}

	cmd.Flags().StringVar(&environment, "env", "", "Environment to deploy")
	cmd.Flags().StringVar(&service, "service", "", "Limit deployment to a single named target")
	cmd.Flags().BoolVar(&async, "async", false, "Return after the Cloud Build operation is queued")
	cmd.Flags().BoolVar(&viaNATS, "via-nats", false, "Publish a deployment event instead of submitting Cloud Build directly")
	cmd.Flags().StringVar(&subject, "subject", "deploy.requested", "NATS subject to publish deploy events to")
	return cmd
}

func loadDeployPlan(environment, service string) (*deploy.DeploymentPlan, error) {
	if environment == "" {
		return nil, fmt.Errorf("--env is required")
	}

	cfg, err := loadProjectConfig()
	if err != nil {
		return nil, err
	}

	return deploy.BuildPlan(cfg, environment, service)
}

func publishDeployRequest(ctx context.Context, out io.Writer, plan *deploy.DeploymentPlan, subject string) error {
	client, err := newNATSClient(viper.GetString("nats-url"))
	if err != nil {
		return err
	}
	defer client.Close()

	targetNames := deployTargetNames(plan)
	payload, err := json.Marshal(deployEvent{
		ProjectID:   plan.ProjectID,
		Region:      plan.Region,
		Environment: plan.Environment,
		Targets:     targetNames,
	})
	if err != nil {
		return fmt.Errorf("marshal deploy event: %w", err)
	}

	if err := client.Publish(ctx, subject, payload); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "Published deploy request to %s for %d target(s)\n", subject, len(targetNames))
	return err
}

func deployTargetNames(plan *deploy.DeploymentPlan) []string {
	targetNames := make([]string, 0, len(plan.Services)+len(plan.Resources))
	for _, svc := range plan.Services {
		targetNames = append(targetNames, fmt.Sprintf("%s/%s", svc.Kind, svc.Name))
	}
	for _, resource := range plan.Resources {
		targetNames = append(targetNames, fmt.Sprintf("%s/%s", resource.Kind, resource.Name))
	}
	return targetNames
}

func submitDeployPlan(ctx context.Context, out io.Writer, plan *deploy.DeploymentPlan, async bool) error {
	buildClient, err := newCloudBuildClient(ctx)
	if err != nil {
		return err
	}
	defer buildClient.Close()

	if err := submitServiceDeploys(ctx, out, buildClient, plan, async); err != nil {
		return err
	}

	return submitResourceDeploys(ctx, out, buildClient, plan, async)
}

func submitServiceDeploys(ctx context.Context, out io.Writer, buildClient cloudBuildClient, plan *deploy.DeploymentPlan, async bool) error {
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

		if err := writeDeployResult(out, string(svc.Kind), svc.Name, result, async); err != nil {
			return err
		}
	}

	return nil
}

func submitResourceDeploys(ctx context.Context, out io.Writer, buildClient cloudBuildClient, plan *deploy.DeploymentPlan, async bool) error {
	for _, resource := range plan.Resources {
		result, err := buildClient.SubmitManagedResourceBuild(ctx, gcp.ResourceBuildRequest{
			ProjectID:   plan.ProjectID,
			Region:      plan.Region,
			Environment: plan.Environment,
			Resource:    resource,
		}, !async)
		if err != nil {
			return err
		}

		if err := writeDeployResult(out, string(resource.Kind), resource.Name, result, async); err != nil {
			return err
		}
	}

	return nil
}

func writeDeployResult(out io.Writer, kind, name string, result *gcp.BuildResult, async bool) error {
	if async {
		_, err := fmt.Fprintf(out, "Queued deploy for %s/%s (operation: %s)\n", kind, name, result.Operation)
		return err
	}

	if _, err := fmt.Fprintf(out, "Deployed %s/%s with status %s", kind, name, result.Status); err != nil {
		return err
	}
	if result.ID != "" {
		if _, err := fmt.Fprintf(out, " (build: %s)", result.ID); err != nil {
			return err
		}
	}
	if result.LogURL != "" {
		if _, err := fmt.Fprintf(out, " %s", result.LogURL); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out)
	return err
}
