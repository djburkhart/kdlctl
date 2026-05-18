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
	Services    []*types.DeploymentService
	Resources   []*types.DeploymentResource
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

	plan := &DeploymentPlan{
		ProjectID:   cfg.ProjectID,
		Region:      cfg.Region,
		Environment: environment,
		NATS:        env.NATS,
	}

	appendCloudRunServices(plan, env.CloudRunServices, serviceFilter)
	appendGRPCServers(plan, env.GRPCServers, serviceFilter)
	appendCaddyServers(plan, env.CaddyServers, serviceFilter)
	appendCloudSQLResources(plan, env.CloudSQL, serviceFilter)
	appendRedisResources(plan, env.Redis, serviceFilter)
	appendPubSubResources(plan, env.PubSubTopics, serviceFilter)
	appendLoggingBucketResources(plan, env.LoggingBuckets, serviceFilter)
	appendLoggingSinkResources(plan, env.LoggingSinks, serviceFilter)

	sort.Slice(plan.Services, func(i, j int) bool {
		if plan.Services[i].Kind == plan.Services[j].Kind {
			return plan.Services[i].Name < plan.Services[j].Name
		}
		return plan.Services[i].Kind < plan.Services[j].Kind
	})
	sort.Slice(plan.Resources, func(i, j int) bool {
		if plan.Resources[i].Kind == plan.Resources[j].Kind {
			return plan.Resources[i].Name < plan.Resources[j].Name
		}
		return plan.Resources[i].Kind < plan.Resources[j].Kind
	})

	if len(plan.Services) == 0 && len(plan.Resources) == 0 {
		return nil, fmt.Errorf("no targets matched environment %q", environment)
	}

	return plan, nil
}

func (p *DeploymentPlan) Render() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Project: %s\n", p.ProjectID))
	builder.WriteString(fmt.Sprintf("Region: %s\n", p.Region))
	builder.WriteString(fmt.Sprintf("Environment: %s\n", p.Environment))
	builder.WriteString(fmt.Sprintf("Services: %d\n", len(p.Services)))
	builder.WriteString(fmt.Sprintf("Resources: %d\n", len(p.Resources)))

	for _, service := range p.Services {
		builder.WriteString(fmt.Sprintf(
			"\n- %s/%s\n  image: %s\n  cpu: %d\n  memory: %s\n  port: %d\n  ingress: %s\n  http2: %t\n  instances: %d..%d\n  concurrency: %d\n",
			service.Kind,
			service.Name,
			service.Image,
			service.CPU,
			service.Memory,
			service.Port,
			service.Ingress,
			service.UseHTTP2,
			service.MinInstances,
			service.MaxInstances,
			service.Concurrency,
		))

		if service.VPCConnector != "" {
			builder.WriteString(fmt.Sprintf("  vpcConnector: %s\n", service.VPCConnector))
		}
		if service.VPCEgress != "" {
			builder.WriteString(fmt.Sprintf("  vpcEgress: %s\n", service.VPCEgress))
		}
		if len(service.CloudSQLInstances) > 0 {
			builder.WriteString("  cloudSqlInstances:\n")
			for _, instance := range service.CloudSQLInstances {
				builder.WriteString(fmt.Sprintf("    - %s\n", instance))
			}
		}

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

	for _, resource := range p.Resources {
		builder.WriteString(fmt.Sprintf("\n- %s/%s\n", resource.Kind, resource.Name))
		switch resource.Kind {
		case types.ResourceKindCloudSQL:
			builder.WriteString(fmt.Sprintf(
				"  databaseVersion: %s\n  tier: %s\n  availabilityType: %s\n  storageGb: %d\n",
				resource.DatabaseVersion,
				resource.Tier,
				resource.AvailabilityType,
				resource.StorageSizeGB,
			))
		case types.ResourceKindRedis:
			builder.WriteString(fmt.Sprintf(
				"  tier: %s\n  memoryGb: %d\n  redisVersion: %s\n",
				resource.Tier,
				resource.MemorySizeGB,
				resource.RedisVersion,
			))
		case types.ResourceKindPubSubTopic:
			builder.WriteString(fmt.Sprintf("  retention: %s\n", resource.MessageRetentionDuration))
			if len(resource.Labels) > 0 {
				keys := make([]string, 0, len(resource.Labels))
				for key := range resource.Labels {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				builder.WriteString("  labels:\n")
				for _, key := range keys {
					builder.WriteString(fmt.Sprintf("    %s: %s\n", key, resource.Labels[key]))
				}
			}
		case types.ResourceKindLoggingBucket:
			builder.WriteString(fmt.Sprintf(
				"  location: %s\n  retentionDays: %d\n",
				resource.Location,
				resource.RetentionDays,
			))
			if resource.Description != "" {
				builder.WriteString(fmt.Sprintf("  description: %s\n", resource.Description))
			}
		case types.ResourceKindLoggingSink:
			builder.WriteString(fmt.Sprintf("  destination: %s\n", resource.Destination))
			if resource.Filter != "" {
				builder.WriteString(fmt.Sprintf("  filter: %s\n", resource.Filter))
			}
			if resource.Description != "" {
				builder.WriteString(fmt.Sprintf("  description: %s\n", resource.Description))
			}
			builder.WriteString(fmt.Sprintf("  uniqueWriterIdentity: %t\n", resource.UniqueWriterIdentity))
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

func appendCloudRunServices(plan *DeploymentPlan, services map[string]*types.CloudRunService, filter string) {
	for name, service := range services {
		if filter != "" && filter != name {
			continue
		}
		plan.Services = append(plan.Services, &types.DeploymentService{
			Kind:                 types.ServiceKindCloudRun,
			Name:                 service.Name,
			Image:                service.Image,
			CPU:                  service.CPU,
			Memory:               service.Memory,
			MinInstances:         service.MinInstances,
			MaxInstances:         service.MaxInstances,
			Concurrency:          service.Concurrency,
			Port:                 service.Port,
			Ingress:              service.Ingress,
			UseHTTP2:             service.UseHTTP2,
			AllowUnauthenticated: service.AllowUnauthenticated,
			VPCConnector:         service.VPCConnector,
			VPCEgress:            service.VPCEgress,
			CloudSQLInstances:    append([]string(nil), service.CloudSQLInstances...),
			Labels:               cloneStringLabels(service.Labels),
			Env:                  cloneEnv(service.Env),
			Traffic:              service.Traffic,
		})
	}
}

func appendGRPCServers(plan *DeploymentPlan, services map[string]*types.GRPCServer, filter string) {
	for name, service := range services {
		if filter != "" && filter != name {
			continue
		}
		plan.Services = append(plan.Services, &types.DeploymentService{
			Kind:                 types.ServiceKindGRPC,
			Name:                 service.Name,
			Image:                service.Image,
			CPU:                  service.CPU,
			Memory:               service.Memory,
			MinInstances:         service.MinInstances,
			MaxInstances:         service.MaxInstances,
			Concurrency:          service.Concurrency,
			Port:                 service.Port,
			Ingress:              service.Ingress,
			UseHTTP2:             service.UseHTTP2,
			AllowUnauthenticated: service.AllowUnauthenticated,
			VPCConnector:         service.VPCConnector,
			VPCEgress:            service.VPCEgress,
			CloudSQLInstances:    append([]string(nil), service.CloudSQLInstances...),
			Labels:               cloneStringLabels(service.Labels),
			Env:                  cloneEnv(service.Env),
			Traffic:              service.Traffic,
		})
	}
}

func appendCaddyServers(plan *DeploymentPlan, services map[string]*types.CaddyServer, filter string) {
	for name, service := range services {
		if filter != "" && filter != name {
			continue
		}
		plan.Services = append(plan.Services, &types.DeploymentService{
			Kind:                 types.ServiceKindCaddy,
			Name:                 service.Name,
			Image:                service.Image,
			CPU:                  service.CPU,
			Memory:               service.Memory,
			MinInstances:         service.MinInstances,
			MaxInstances:         service.MaxInstances,
			Concurrency:          service.Concurrency,
			Port:                 service.Port,
			Ingress:              service.Ingress,
			UseHTTP2:             service.UseHTTP2,
			AllowUnauthenticated: service.AllowUnauthenticated,
			VPCConnector:         service.VPCConnector,
			VPCEgress:            service.VPCEgress,
			CloudSQLInstances:    append([]string(nil), service.CloudSQLInstances...),
			Labels:               cloneStringLabels(service.Labels),
			Env:                  cloneEnv(service.Env),
			Traffic:              service.Traffic,
		})
	}
}

func cloneEnv(values map[string]types.EnvVar) map[string]types.EnvVar {
	cloned := make(map[string]types.EnvVar, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStringLabels(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func appendCloudSQLResources(plan *DeploymentPlan, resources map[string]*types.CloudSQLInstance, filter string) {
	for name, resource := range resources {
		if filter != "" && filter != name {
			continue
		}
		plan.Resources = append(plan.Resources, &types.DeploymentResource{
			Kind:             types.ResourceKindCloudSQL,
			Name:             resource.Name,
			DatabaseVersion:  resource.DatabaseVersion,
			Tier:             resource.Tier,
			AvailabilityType: resource.AvailabilityType,
			StorageSizeGB:    resource.StorageSizeGB,
		})
	}
}

func appendRedisResources(plan *DeploymentPlan, resources map[string]*types.RedisInstance, filter string) {
	for name, resource := range resources {
		if filter != "" && filter != name {
			continue
		}
		plan.Resources = append(plan.Resources, &types.DeploymentResource{
			Kind:         types.ResourceKindRedis,
			Name:         resource.Name,
			Tier:         resource.Tier,
			MemorySizeGB: resource.MemorySizeGB,
			RedisVersion: resource.RedisVersion,
		})
	}
}

func appendPubSubResources(plan *DeploymentPlan, resources map[string]*types.PubSubTopic, filter string) {
	for name, resource := range resources {
		if filter != "" && filter != name {
			continue
		}
		plan.Resources = append(plan.Resources, &types.DeploymentResource{
			Kind:                     types.ResourceKindPubSubTopic,
			Name:                     resource.Name,
			MessageRetentionDuration: resource.MessageRetentionDuration,
			Labels:                   cloneStringLabels(resource.Labels),
		})
	}
}

func appendLoggingBucketResources(plan *DeploymentPlan, resources map[string]*types.LoggingBucket, filter string) {
	for name, resource := range resources {
		if filter != "" && filter != name {
			continue
		}
		plan.Resources = append(plan.Resources, &types.DeploymentResource{
			Kind:          types.ResourceKindLoggingBucket,
			Name:          resource.Name,
			Location:      resource.Location,
			RetentionDays: resource.RetentionDays,
			Description:   resource.Description,
		})
	}
}

func appendLoggingSinkResources(plan *DeploymentPlan, resources map[string]*types.LoggingSink, filter string) {
	for name, resource := range resources {
		if filter != "" && filter != name {
			continue
		}
		plan.Resources = append(plan.Resources, &types.DeploymentResource{
			Kind:                 types.ResourceKindLoggingSink,
			Name:                 resource.Name,
			Destination:          resource.Destination,
			Filter:               resource.Filter,
			Description:          resource.Description,
			UniqueWriterIdentity: resource.UniqueWriterIdentity,
		})
	}
}
