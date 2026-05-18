package deploy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/djburkhart/kdlctl/internal/config"
	"github.com/djburkhart/kdlctl/pkg/types"
)

type DeploymentPlan struct {
	ProjectID   string
	Region      string
	Environment string
	Services    []*types.CloudRunService
	NATS        *types.NATSConfig
}

func BuildPlan(cfg *types.ProjectConfig, environment string, serviceFilter string) (*DeploymentPlan, error) {
	if err := config.ValidateEnvironment(cfg, environment); err != nil {
		return nil, err
	}

	env, err := config.ResolveEnvironment(cfg, environment)
	if err != nil {
		return nil, err
	}

	serviceNames := make([]string, 0, len(env.CloudRunServices))
	for name := range env.CloudRunServices {
		if serviceFilter != "" && serviceFilter != name {
			continue
		}
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	if len(serviceNames) == 0 {
		return nil, fmt.Errorf("no cloud-run services matched environment %q", environment)
	}

	plan := &DeploymentPlan{
		ProjectID:   cfg.ProjectID,
		Region:      cfg.Region,
		Environment: environment,
		NATS:        env.NATS,
	}

	for _, name := range serviceNames {
		plan.Services = append(plan.Services, env.CloudRunServices[name].Clone())
	}

	return plan, nil
}

func (p *DeploymentPlan) Render() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Project: %s\n", p.ProjectID))
	builder.WriteString(fmt.Sprintf("Region: %s\n", p.Region))
	builder.WriteString(fmt.Sprintf("Environment: %s\n", p.Environment))
	builder.WriteString(fmt.Sprintf("Services: %d\n", len(p.Services)))

	for _, service := range p.Services {
		builder.WriteString(fmt.Sprintf(
			"\n- cloud-run/%s\n  image: %s\n  cpu: %d\n  memory: %s\n  instances: %d..%d\n  concurrency: %d\n",
			service.Name,
			service.Image,
			service.CPU,
			service.Memory,
			service.MinInstances,
			service.MaxInstances,
			service.Concurrency,
		))

		if len(service.Env) > 0 {
			keys := make([]string, 0, len(service.Env))
			for key := range service.Env {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			builder.WriteString("  env:\n")
			for _, key := range keys {
				value := service.Env[key]
				if value.Secret != "" {
					builder.WriteString(fmt.Sprintf("    %s: secret:%s\n", key, value.Secret))
					continue
				}
				builder.WriteString(fmt.Sprintf("    %s: %s\n", key, value.Value))
			}
		}
	}

	if p.NATS != nil && p.NATS.Cluster != nil {
		builder.WriteString(fmt.Sprintf(
			"\nNATS cluster: %s (replicas=%d, jetstream=%t, storageClass=%s, size=%s)\n",
			p.NATS.Cluster.Name,
			p.NATS.Cluster.Replicas,
			p.NATS.Cluster.JetStream,
			p.NATS.Cluster.StorageClass,
			p.NATS.Cluster.Size,
		))
	}

	return strings.TrimSpace(builder.String())
}
