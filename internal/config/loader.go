package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	kdl "github.com/calico32/kdl-go"
	"github.com/djburkhart/kdlctl/pkg/types"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func LoadFile(path string) (*types.ProjectConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	return Parse(contents)
}

func Parse(data []byte) (*types.ProjectConfig, error) {
	doc, err := kdl.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse kdl: %w", err)
	}

	projectNode := doc.GetNode("project")
	if projectNode == nil {
		return nil, fmt.Errorf("deploy.kdl must define a top-level project node")
	}

	projectID, err := nodeArgumentString(projectNode, 0)
	if err != nil {
		return nil, fmt.Errorf("parse project name: %w", err)
	}

	cfg := &types.ProjectConfig{
		ProjectID:    projectID,
		Region:       propertyString(projectNode, "region"),
		Environments: map[string]*types.EnvironmentConfig{},
	}

	for _, node := range projectNode.Children().Nodes {
		switch node.Name() {
		case "environment":
			env, err := parseEnvironment(node)
			if err != nil {
				return nil, err
			}
			cfg.Environments[env.Name] = env
		}
	}

	if len(cfg.Environments) == 0 {
		return nil, fmt.Errorf("project must contain at least one environment")
	}

	for _, env := range cfg.Environments {
		for _, service := range env.CloudRunServices {
			applyCloudRunDefaults(service)
		}
		for _, service := range env.GRPCServers {
			applyGRPCServerDefaults(service)
		}
		for _, service := range env.CaddyServers {
			applyCaddyServerDefaults(service)
		}
		for _, resource := range env.CloudSQL {
			applyCloudSQLDefaults(resource)
		}
		for _, resource := range env.Redis {
			applyRedisDefaults(resource)
		}
		for _, resource := range env.PubSubTopics {
			applyPubSubDefaults(resource)
		}
		for _, resource := range env.LoggingBuckets {
			applyLoggingBucketDefaults(resource)
		}
		for _, resource := range env.LoggingSinks {
			applyLoggingSinkDefaults(resource)
		}
	}

	return cfg, nil
}

func ValidateProject(cfg *types.ProjectConfig) error {
	if err := validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate project: %w", err)
	}

	if len(cfg.Environments) == 0 {
		return fmt.Errorf("at least one environment must be defined")
	}

	for name := range cfg.Environments {
		if _, err := ResolveEnvironment(cfg, name); err != nil {
			return err
		}
	}

	return nil
}

func ValidateEnvironment(cfg *types.ProjectConfig, environment string) error {
	env, err := ResolveEnvironment(cfg, environment)
	if err != nil {
		return err
	}

	if env.Name == "" {
		return fmt.Errorf("environment %q is missing a name", environment)
	}

	if len(env.CloudRunServices) == 0 &&
		len(env.GRPCServers) == 0 &&
		len(env.CaddyServers) == 0 &&
		len(env.CloudSQL) == 0 &&
		len(env.Redis) == 0 &&
		len(env.PubSubTopics) == 0 &&
		len(env.LoggingBuckets) == 0 &&
		len(env.LoggingSinks) == 0 {
		return fmt.Errorf("environment %q has no deployable targets configured", environment)
	}
	if err := validateUniqueTargetNames(env, environment); err != nil {
		return err
	}

	for serviceName, service := range env.CloudRunServices {
		if err := validate.Struct(service); err != nil {
			return fmt.Errorf("validate cloud-run service %q in environment %q: %w", serviceName, environment, err)
		}
	}
	for serviceName, service := range env.GRPCServers {
		if err := validate.Struct(service); err != nil {
			return fmt.Errorf("validate grpc server %q in environment %q: %w", serviceName, environment, err)
		}
	}
	for serviceName, service := range env.CaddyServers {
		if err := validate.Struct(service); err != nil {
			return fmt.Errorf("validate caddy server %q in environment %q: %w", serviceName, environment, err)
		}
	}
	for name, resource := range env.CloudSQL {
		if err := validate.Struct(resource); err != nil {
			return fmt.Errorf("validate cloud-sql resource %q in environment %q: %w", name, environment, err)
		}
	}
	for name, resource := range env.Redis {
		if err := validate.Struct(resource); err != nil {
			return fmt.Errorf("validate redis resource %q in environment %q: %w", name, environment, err)
		}
	}
	for name, resource := range env.PubSubTopics {
		if err := validate.Struct(resource); err != nil {
			return fmt.Errorf("validate pubsub-topic resource %q in environment %q: %w", name, environment, err)
		}
	}
	for name, resource := range env.LoggingBuckets {
		if err := validate.Struct(resource); err != nil {
			return fmt.Errorf("validate logging-bucket resource %q in environment %q: %w", name, environment, err)
		}
	}
	for name, resource := range env.LoggingSinks {
		if err := validate.Struct(resource); err != nil {
			return fmt.Errorf("validate logging-sink resource %q in environment %q: %w", name, environment, err)
		}
	}

	return nil
}

func validateUniqueTargetNames(env *types.EnvironmentConfig, environment string) error {
	seen := map[string]string{}

	for name := range env.CloudRunServices {
		seen[name] = "cloud-run"
	}
	for name := range env.GRPCServers {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and grpc-server", environment, name, existing)
		}
		seen[name] = "grpc-server"
	}
	for name := range env.CaddyServers {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and caddy-server", environment, name, existing)
		}
		seen[name] = "caddy-server"
	}
	for name := range env.CloudSQL {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and cloud-sql", environment, name, existing)
		}
		seen[name] = "cloud-sql"
	}
	for name := range env.Redis {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and redis", environment, name, existing)
		}
		seen[name] = "redis"
	}
	for name := range env.PubSubTopics {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and pubsub-topic", environment, name, existing)
		}
		seen[name] = "pubsub-topic"
	}
	for name := range env.LoggingBuckets {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and logging-bucket", environment, name, existing)
		}
		seen[name] = "logging-bucket"
	}
	for name := range env.LoggingSinks {
		if existing, ok := seen[name]; ok {
			return fmt.Errorf("environment %q defines %q in both %s and logging-sink", environment, name, existing)
		}
	}

	return nil
}

func ResolveEnvironment(cfg *types.ProjectConfig, environment string) (*types.EnvironmentConfig, error) {
	seen := map[string]bool{}
	return resolveEnvironment(cfg, environment, seen)
}

func EnvironmentNames(cfg *types.ProjectConfig) []string {
	names := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveEnvironment(cfg *types.ProjectConfig, environment string, seen map[string]bool) (*types.EnvironmentConfig, error) {
	current, ok := cfg.Environments[environment]
	if !ok {
		return nil, fmt.Errorf("environment %q not found", environment)
	}

	if current.Extends == "" {
		return current.Clone(), nil
	}

	if seen[environment] {
		return nil, fmt.Errorf("cyclic environment inheritance detected for %q", environment)
	}
	seen[environment] = true

	base, err := resolveEnvironment(cfg, current.Extends, seen)
	if err != nil {
		return nil, err
	}

	return mergeEnvironment(base, current), nil
}

func mergeEnvironment(base, override *types.EnvironmentConfig) *types.EnvironmentConfig {
	merged := base.Clone()
	merged.Name = override.Name
	merged.Extends = override.Extends

	for name, service := range override.CloudRunServices {
		existing, ok := merged.CloudRunServices[name]
		if !ok {
			merged.CloudRunServices[name] = service.Clone()
			continue
		}
		merged.CloudRunServices[name] = mergeService(existing, service)
	}
	for name, service := range override.GRPCServers {
		existing, ok := merged.GRPCServers[name]
		if !ok {
			merged.GRPCServers[name] = service.Clone()
			continue
		}
		merged.GRPCServers[name] = mergeGRPCServer(existing, service)
	}
	for name, service := range override.CaddyServers {
		existing, ok := merged.CaddyServers[name]
		if !ok {
			merged.CaddyServers[name] = service.Clone()
			continue
		}
		merged.CaddyServers[name] = mergeCaddyServer(existing, service)
	}
	for name, resource := range override.CloudSQL {
		existing, ok := merged.CloudSQL[name]
		if !ok {
			merged.CloudSQL[name] = resource.Clone()
			continue
		}
		merged.CloudSQL[name] = mergeCloudSQL(existing, resource)
	}
	for name, resource := range override.Redis {
		existing, ok := merged.Redis[name]
		if !ok {
			merged.Redis[name] = resource.Clone()
			continue
		}
		merged.Redis[name] = mergeRedis(existing, resource)
	}
	for name, resource := range override.PubSubTopics {
		existing, ok := merged.PubSubTopics[name]
		if !ok {
			merged.PubSubTopics[name] = resource.Clone()
			continue
		}
		merged.PubSubTopics[name] = mergePubSubTopic(existing, resource)
	}
	for name, resource := range override.LoggingBuckets {
		existing, ok := merged.LoggingBuckets[name]
		if !ok {
			merged.LoggingBuckets[name] = resource.Clone()
			continue
		}
		merged.LoggingBuckets[name] = mergeLoggingBucket(existing, resource)
	}
	for name, resource := range override.LoggingSinks {
		existing, ok := merged.LoggingSinks[name]
		if !ok {
			merged.LoggingSinks[name] = resource.Clone()
			continue
		}
		merged.LoggingSinks[name] = mergeLoggingSink(existing, resource)
	}

	if override.NATS != nil {
		if merged.NATS == nil {
			merged.NATS = override.NATS.Clone()
		} else {
			merged.NATS = mergeNATS(merged.NATS, override.NATS)
		}
	}

	return merged
}

func mergeService(base, override *types.CloudRunService) *types.CloudRunService {
	merged := base.Clone()

	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Image != "" {
		merged.Image = override.Image
	}
	if override.CPU != 0 {
		merged.CPU = override.CPU
	}
	if override.Memory != "" {
		merged.Memory = override.Memory
	}
	if override.MinInstances != 0 {
		merged.MinInstances = override.MinInstances
	}
	if override.MaxInstances != 0 {
		merged.MaxInstances = override.MaxInstances
	}
	if override.Concurrency != 0 {
		merged.Concurrency = override.Concurrency
	}
	if override.Port != 0 {
		merged.Port = override.Port
	}
	if override.Ingress != "" {
		merged.Ingress = override.Ingress
	}
	if override.UseHTTP2 {
		merged.UseHTTP2 = true
	}
	if override.AllowUnauthenticated {
		merged.AllowUnauthenticated = true
	}
	if override.VPCConnector != "" {
		merged.VPCConnector = override.VPCConnector
	}
	if override.VPCEgress != "" {
		merged.VPCEgress = override.VPCEgress
	}
	if len(override.CloudSQLInstances) > 0 {
		merged.CloudSQLInstances = append([]string(nil), override.CloudSQLInstances...)
	}
	for key, value := range override.Labels {
		if merged.Labels == nil {
			merged.Labels = map[string]string{}
		}
		merged.Labels[key] = value
	}
	if override.Traffic.LatestPercent != 0 {
		merged.Traffic.LatestPercent = override.Traffic.LatestPercent
	}
	for key, value := range override.Env {
		merged.Env[key] = value
	}

	applyCloudRunDefaults(merged)
	return merged
}

func mergeGRPCServer(base, override *types.GRPCServer) *types.GRPCServer {
	merged := base.Clone()

	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Image != "" {
		merged.Image = override.Image
	}
	if override.CPU != 0 {
		merged.CPU = override.CPU
	}
	if override.Memory != "" {
		merged.Memory = override.Memory
	}
	if override.MinInstances != 0 {
		merged.MinInstances = override.MinInstances
	}
	if override.MaxInstances != 0 {
		merged.MaxInstances = override.MaxInstances
	}
	if override.Concurrency != 0 {
		merged.Concurrency = override.Concurrency
	}
	if override.Port != 0 {
		merged.Port = override.Port
	}
	if override.Ingress != "" {
		merged.Ingress = override.Ingress
	}
	if override.UseHTTP2 {
		merged.UseHTTP2 = true
	}
	if override.AllowUnauthenticated {
		merged.AllowUnauthenticated = true
	}
	if override.VPCConnector != "" {
		merged.VPCConnector = override.VPCConnector
	}
	if override.VPCEgress != "" {
		merged.VPCEgress = override.VPCEgress
	}
	if len(override.CloudSQLInstances) > 0 {
		merged.CloudSQLInstances = append([]string(nil), override.CloudSQLInstances...)
	}
	for key, value := range override.Labels {
		if merged.Labels == nil {
			merged.Labels = map[string]string{}
		}
		merged.Labels[key] = value
	}
	if override.Traffic.LatestPercent != 0 {
		merged.Traffic.LatestPercent = override.Traffic.LatestPercent
	}
	for key, value := range override.Env {
		merged.Env[key] = value
	}

	applyGRPCServerDefaults(merged)
	return merged
}

func mergeCaddyServer(base, override *types.CaddyServer) *types.CaddyServer {
	merged := base.Clone()

	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Image != "" {
		merged.Image = override.Image
	}
	if override.CPU != 0 {
		merged.CPU = override.CPU
	}
	if override.Memory != "" {
		merged.Memory = override.Memory
	}
	if override.MinInstances != 0 {
		merged.MinInstances = override.MinInstances
	}
	if override.MaxInstances != 0 {
		merged.MaxInstances = override.MaxInstances
	}
	if override.Concurrency != 0 {
		merged.Concurrency = override.Concurrency
	}
	if override.Port != 0 {
		merged.Port = override.Port
	}
	if override.Ingress != "" {
		merged.Ingress = override.Ingress
	}
	if override.UseHTTP2 {
		merged.UseHTTP2 = true
	}
	if override.AllowUnauthenticated {
		merged.AllowUnauthenticated = true
	}
	if override.VPCConnector != "" {
		merged.VPCConnector = override.VPCConnector
	}
	if override.VPCEgress != "" {
		merged.VPCEgress = override.VPCEgress
	}
	if len(override.CloudSQLInstances) > 0 {
		merged.CloudSQLInstances = append([]string(nil), override.CloudSQLInstances...)
	}
	for key, value := range override.Labels {
		if merged.Labels == nil {
			merged.Labels = map[string]string{}
		}
		merged.Labels[key] = value
	}
	if override.Traffic.LatestPercent != 0 {
		merged.Traffic.LatestPercent = override.Traffic.LatestPercent
	}
	for key, value := range override.Env {
		merged.Env[key] = value
	}

	applyCaddyServerDefaults(merged)
	return merged
}

func mergeNATS(base, override *types.NATSConfig) *types.NATSConfig {
	merged := base.Clone()
	if override.Cluster == nil {
		return merged
	}
	if merged.Cluster == nil {
		merged.Cluster = override.Cluster
		return merged
	}

	if override.Cluster.Name != "" {
		merged.Cluster.Name = override.Cluster.Name
	}
	if override.Cluster.Replicas != 0 {
		merged.Cluster.Replicas = override.Cluster.Replicas
	}
	if override.Cluster.StorageClass != "" {
		merged.Cluster.StorageClass = override.Cluster.StorageClass
	}
	if override.Cluster.Size != "" {
		merged.Cluster.Size = override.Cluster.Size
	}
	if override.Cluster.JetStream {
		merged.Cluster.JetStream = true
	}

	return merged
}

func mergeCloudSQL(base, override *types.CloudSQLInstance) *types.CloudSQLInstance {
	merged := base.Clone()
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.DatabaseVersion != "" {
		merged.DatabaseVersion = override.DatabaseVersion
	}
	if override.Tier != "" {
		merged.Tier = override.Tier
	}
	if override.AvailabilityType != "" {
		merged.AvailabilityType = override.AvailabilityType
	}
	if override.StorageSizeGB != 0 {
		merged.StorageSizeGB = override.StorageSizeGB
	}
	applyCloudSQLDefaults(merged)
	return merged
}

func mergeRedis(base, override *types.RedisInstance) *types.RedisInstance {
	merged := base.Clone()
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Tier != "" {
		merged.Tier = override.Tier
	}
	if override.MemorySizeGB != 0 {
		merged.MemorySizeGB = override.MemorySizeGB
	}
	if override.RedisVersion != "" {
		merged.RedisVersion = override.RedisVersion
	}
	applyRedisDefaults(merged)
	return merged
}

func mergePubSubTopic(base, override *types.PubSubTopic) *types.PubSubTopic {
	merged := base.Clone()
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.MessageRetentionDuration != "" {
		merged.MessageRetentionDuration = override.MessageRetentionDuration
	}
	for key, value := range override.Labels {
		merged.Labels[key] = value
	}
	applyPubSubDefaults(merged)
	return merged
}

func mergeLoggingBucket(base, override *types.LoggingBucket) *types.LoggingBucket {
	merged := base.Clone()
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Location != "" {
		merged.Location = override.Location
	}
	if override.RetentionDays != 0 {
		merged.RetentionDays = override.RetentionDays
	}
	if override.Description != "" {
		merged.Description = override.Description
	}
	applyLoggingBucketDefaults(merged)
	return merged
}

func mergeLoggingSink(base, override *types.LoggingSink) *types.LoggingSink {
	merged := base.Clone()
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.Destination != "" {
		merged.Destination = override.Destination
	}
	if override.Filter != "" {
		merged.Filter = override.Filter
	}
	if override.Description != "" {
		merged.Description = override.Description
	}
	if override.UniqueWriterIdentity {
		merged.UniqueWriterIdentity = true
	}
	applyLoggingSinkDefaults(merged)
	return merged
}

func parseEnvironment(node *kdl.Node) (*types.EnvironmentConfig, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse environment name: %w", err)
	}

	env := &types.EnvironmentConfig{
		Name:             name,
		Extends:          propertyString(node, "extends"),
		CloudRunServices: map[string]*types.CloudRunService{},
		GRPCServers:      map[string]*types.GRPCServer{},
		CaddyServers:     map[string]*types.CaddyServer{},
		CloudSQL:         map[string]*types.CloudSQLInstance{},
		Redis:            map[string]*types.RedisInstance{},
		PubSubTopics:     map[string]*types.PubSubTopic{},
		LoggingBuckets:   map[string]*types.LoggingBucket{},
		LoggingSinks:     map[string]*types.LoggingSink{},
	}

	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "cloud-run":
			service, err := parseCloudRunService(child)
			if err != nil {
				return nil, fmt.Errorf("parse cloud-run in environment %q: %w", name, err)
			}
			env.CloudRunServices[service.Name] = service
		case "grpc-server":
			service, err := parseGRPCServer(child)
			if err != nil {
				return nil, fmt.Errorf("parse grpc-server in environment %q: %w", name, err)
			}
			env.GRPCServers[service.Name] = service
		case "caddy-server":
			service, err := parseCaddyServer(child)
			if err != nil {
				return nil, fmt.Errorf("parse caddy-server in environment %q: %w", name, err)
			}
			env.CaddyServers[service.Name] = service
		case "cloud-sql":
			resource, err := parseCloudSQL(child)
			if err != nil {
				return nil, fmt.Errorf("parse cloud-sql in environment %q: %w", name, err)
			}
			env.CloudSQL[resource.Name] = resource
		case "redis":
			resource, err := parseRedis(child)
			if err != nil {
				return nil, fmt.Errorf("parse redis in environment %q: %w", name, err)
			}
			env.Redis[resource.Name] = resource
		case "pubsub-topic":
			resource, err := parsePubSubTopic(child)
			if err != nil {
				return nil, fmt.Errorf("parse pubsub-topic in environment %q: %w", name, err)
			}
			env.PubSubTopics[resource.Name] = resource
		case "logging-bucket":
			resource, err := parseLoggingBucket(child)
			if err != nil {
				return nil, fmt.Errorf("parse logging-bucket in environment %q: %w", name, err)
			}
			env.LoggingBuckets[resource.Name] = resource
		case "logging-sink":
			resource, err := parseLoggingSink(child)
			if err != nil {
				return nil, fmt.Errorf("parse logging-sink in environment %q: %w", name, err)
			}
			env.LoggingSinks[resource.Name] = resource
		case "nats":
			natsConfig, err := parseNATS(child)
			if err != nil {
				return nil, fmt.Errorf("parse nats config in environment %q: %w", name, err)
			}
			env.NATS = natsConfig
		}
	}

	return env, nil
}

func parseCloudRunService(node *kdl.Node) (*types.CloudRunService, error) {
	service, err := parseRuntimeService(node)
	if err != nil {
		return nil, err
	}

	cloudRunService := &types.CloudRunService{
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
		CloudSQLInstances:    service.CloudSQLInstances,
		Labels:               service.Labels,
		Env:                  service.Env,
		Traffic:              service.Traffic,
	}

	applyCloudRunDefaults(cloudRunService)
	return cloudRunService, nil
}

func parseGRPCServer(node *kdl.Node) (*types.GRPCServer, error) {
	service, err := parseRuntimeService(node)
	if err != nil {
		return nil, err
	}

	grpcServer := &types.GRPCServer{
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
		CloudSQLInstances:    service.CloudSQLInstances,
		Labels:               service.Labels,
		Env:                  service.Env,
		Traffic:              service.Traffic,
	}

	applyGRPCServerDefaults(grpcServer)
	return grpcServer, nil
}

func parseCaddyServer(node *kdl.Node) (*types.CaddyServer, error) {
	service, err := parseRuntimeService(node)
	if err != nil {
		return nil, err
	}

	caddyServer := &types.CaddyServer{
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
		CloudSQLInstances:    service.CloudSQLInstances,
		Labels:               service.Labels,
		Env:                  service.Env,
		Traffic:              service.Traffic,
	}

	applyCaddyServerDefaults(caddyServer)
	return caddyServer, nil
}

func parseCloudSQL(node *kdl.Node) (*types.CloudSQLInstance, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse cloud-sql name: %w", err)
	}

	resource := &types.CloudSQLInstance{Name: name}
	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "database-version":
			resource.DatabaseVersion, err = nodeArgumentString(child, 0)
		case "tier":
			resource.Tier, err = nodeArgumentString(child, 0)
		case "availability-type":
			resource.AvailabilityType, err = nodeArgumentString(child, 0)
		case "storage-gb":
			resource.StorageSizeGB, err = nodeArgumentInt(child, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for cloud-sql %q: %w", child.Name(), name, err)
		}
	}

	applyCloudSQLDefaults(resource)
	return resource, nil
}

func parseRedis(node *kdl.Node) (*types.RedisInstance, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse redis name: %w", err)
	}

	resource := &types.RedisInstance{Name: name}
	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "tier":
			resource.Tier, err = nodeArgumentString(child, 0)
		case "memory-gb":
			resource.MemorySizeGB, err = nodeArgumentInt(child, 0)
		case "redis-version":
			resource.RedisVersion, err = nodeArgumentString(child, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for redis %q: %w", child.Name(), name, err)
		}
	}

	applyRedisDefaults(resource)
	return resource, nil
}

func parsePubSubTopic(node *kdl.Node) (*types.PubSubTopic, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse pubsub-topic name: %w", err)
	}

	resource := &types.PubSubTopic{Name: name, Labels: map[string]string{}}
	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "retention":
			resource.MessageRetentionDuration, err = nodeArgumentString(child, 0)
		case "labels":
			resource.Labels, err = parseStringMapBlock(child)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for pubsub-topic %q: %w", child.Name(), name, err)
		}
	}

	applyPubSubDefaults(resource)
	return resource, nil
}

func parseLoggingBucket(node *kdl.Node) (*types.LoggingBucket, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse logging-bucket name: %w", err)
	}

	resource := &types.LoggingBucket{Name: name}
	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "location":
			resource.Location, err = nodeArgumentString(child, 0)
		case "retention-days":
			resource.RetentionDays, err = nodeArgumentInt(child, 0)
		case "description":
			resource.Description, err = nodeArgumentString(child, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for logging-bucket %q: %w", child.Name(), name, err)
		}
	}

	applyLoggingBucketDefaults(resource)
	return resource, nil
}

func parseLoggingSink(node *kdl.Node) (*types.LoggingSink, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse logging-sink name: %w", err)
	}

	resource := &types.LoggingSink{Name: name}
	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "destination":
			resource.Destination, err = nodeArgumentString(child, 0)
		case "filter":
			resource.Filter, err = nodeArgumentString(child, 0)
		case "description":
			resource.Description, err = nodeArgumentString(child, 0)
		case "unique-writer-identity":
			resource.UniqueWriterIdentity, err = nodeArgumentBool(child, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for logging-sink %q: %w", child.Name(), name, err)
		}
	}

	applyLoggingSinkDefaults(resource)
	return resource, nil
}

type runtimeService struct {
	Name                 string
	Image                string
	CPU                  int
	Memory               string
	MinInstances         int
	MaxInstances         int
	Concurrency          int
	Port                 int
	Ingress              string
	UseHTTP2             bool
	AllowUnauthenticated bool
	VPCConnector         string
	VPCEgress            string
	CloudSQLInstances    []string
	Labels               map[string]string
	Env                  map[string]types.EnvVar
	Traffic              types.TrafficConfig
}

func parseRuntimeService(node *kdl.Node) (*runtimeService, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse service name: %w", err)
	}

	service := &runtimeService{
		Name:   name,
		Env:    map[string]types.EnvVar{},
		Labels: map[string]string{},
	}

	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "image":
			service.Image, err = nodeArgumentString(child, 0)
		case "cpu":
			service.CPU, err = nodeArgumentInt(child, 0)
		case "memory":
			service.Memory, err = nodeArgumentString(child, 0)
		case "min-instances":
			service.MinInstances, err = nodeArgumentInt(child, 0)
		case "max-instances":
			service.MaxInstances, err = nodeArgumentInt(child, 0)
		case "concurrency":
			service.Concurrency, err = nodeArgumentInt(child, 0)
		case "port":
			service.Port, err = nodeArgumentInt(child, 0)
		case "ingress":
			service.Ingress, err = nodeArgumentString(child, 0)
		case "http2":
			service.UseHTTP2, err = nodeArgumentBool(child, 0)
		case "allow-unauthenticated":
			service.AllowUnauthenticated, err = nodeArgumentBool(child, 0)
		case "vpc-connector":
			service.VPCConnector, err = nodeArgumentString(child, 0)
		case "vpc-egress":
			service.VPCEgress, err = nodeArgumentString(child, 0)
		case "cloud-sql-instances":
			service.CloudSQLInstances, err = parseStringListBlock(child)
		case "labels":
			service.Labels, err = parseStringMapBlock(child)
		case "env":
			service.Env, err = parseEnvBlock(child)
		case "traffic":
			service.Traffic, err = parseTrafficBlock(child)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for service %q: %w", child.Name(), name, err)
		}
	}

	return service, nil
}

func parseEnvBlock(node *kdl.Node) (map[string]types.EnvVar, error) {
	envVars := make(map[string]types.EnvVar, len(node.Children().Nodes))
	for _, child := range node.Children().Nodes {
		value := types.EnvVar{}
		if len(child.Arguments()) > 0 {
			str, err := valueString(child.Arguments()[0])
			if err != nil {
				return nil, fmt.Errorf("read env var %q: %w", child.Name(), err)
			}
			value.Value = str
		}
		if secretValue, ok := child.Properties()["secret"]; ok {
			str, err := valueString(secretValue)
			if err != nil {
				return nil, fmt.Errorf("read secret for env var %q: %w", child.Name(), err)
			}
			value.Secret = str
		}
		envVars[child.Name()] = value
	}
	return envVars, nil
}

func parseTrafficBlock(node *kdl.Node) (types.TrafficConfig, error) {
	traffic := types.TrafficConfig{LatestPercent: 100}
	for _, child := range node.Children().Nodes {
		if child.Name() != "latest" {
			continue
		}
		percent, err := nodeArgumentInt(child, 0)
		if err != nil {
			return types.TrafficConfig{}, fmt.Errorf("parse latest traffic: %w", err)
		}
		traffic.LatestPercent = percent
	}
	return traffic, nil
}

func parseNATS(node *kdl.Node) (*types.NATSConfig, error) {
	cfg := &types.NATSConfig{}
	clusterNode := node.Children().GetNode("cluster")
	if clusterNode == nil {
		return cfg, nil
	}

	name, err := nodeArgumentString(clusterNode, 0)
	if err != nil {
		return nil, fmt.Errorf("parse nats cluster name: %w", err)
	}

	cluster := &types.NATSClusterConfig{Name: name}
	for _, child := range clusterNode.Children().Nodes {
		switch child.Name() {
		case "replicas":
			cluster.Replicas, err = nodeArgumentInt(child, 0)
		case "jetstream":
			cluster.JetStream, err = propertyBool(child, "enabled")
		case "storage-class":
			cluster.StorageClass, err = nodeArgumentString(child, 0)
		case "size":
			cluster.Size, err = nodeArgumentString(child, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("parse nats %q: %w", child.Name(), err)
		}
	}

	if cluster.Replicas == 0 {
		cluster.Replicas = 1
	}
	cfg.Cluster = cluster
	return cfg, nil
}

func applyCloudRunDefaults(service *types.CloudRunService) {
	if service.CPU == 0 {
		service.CPU = 1
	}
	if service.Memory == "" {
		service.Memory = "512Mi"
	}
	if service.Concurrency == 0 {
		service.Concurrency = 80
	}
	if service.MaxInstances == 0 {
		service.MaxInstances = 100
	}
	if service.Port == 0 {
		service.Port = 8080
	}
	if service.Ingress == "" {
		service.Ingress = "all"
	}
	if service.Labels == nil {
		service.Labels = map[string]string{}
	}
	if service.Traffic.LatestPercent == 0 {
		service.Traffic.LatestPercent = 100
	}
	if service.Env == nil {
		service.Env = map[string]types.EnvVar{}
	}
}

func applyGRPCServerDefaults(service *types.GRPCServer) {
	if service.CPU == 0 {
		service.CPU = 1
	}
	if service.Memory == "" {
		service.Memory = "512Mi"
	}
	if service.Concurrency == 0 {
		service.Concurrency = 80
	}
	if service.MaxInstances == 0 {
		service.MaxInstances = 100
	}
	if service.Port == 0 {
		service.Port = 8080
	}
	if service.Ingress == "" {
		service.Ingress = "internal"
	}
	if !service.UseHTTP2 {
		service.UseHTTP2 = true
	}
	if service.Labels == nil {
		service.Labels = map[string]string{}
	}
	if service.Traffic.LatestPercent == 0 {
		service.Traffic.LatestPercent = 100
	}
	if service.Env == nil {
		service.Env = map[string]types.EnvVar{}
	}
}

func applyCaddyServerDefaults(service *types.CaddyServer) {
	if service.CPU == 0 {
		service.CPU = 1
	}
	if service.Memory == "" {
		service.Memory = "512Mi"
	}
	if service.Concurrency == 0 {
		service.Concurrency = 80
	}
	if service.MaxInstances == 0 {
		service.MaxInstances = 100
	}
	if service.Port == 0 {
		service.Port = 8080
	}
	if service.Ingress == "" {
		service.Ingress = "all"
	}
	if service.Labels == nil {
		service.Labels = map[string]string{}
	}
	if service.Traffic.LatestPercent == 0 {
		service.Traffic.LatestPercent = 100
	}
	if service.Env == nil {
		service.Env = map[string]types.EnvVar{}
	}
}

func applyCloudSQLDefaults(resource *types.CloudSQLInstance) {
	if resource.DatabaseVersion == "" {
		resource.DatabaseVersion = "POSTGRES_16"
	}
	if resource.Tier == "" {
		resource.Tier = "db-custom-1-3840"
	}
	if resource.AvailabilityType == "" {
		resource.AvailabilityType = "ZONAL"
	}
	if resource.StorageSizeGB == 0 {
		resource.StorageSizeGB = 50
	}
}

func applyRedisDefaults(resource *types.RedisInstance) {
	if resource.Tier == "" {
		resource.Tier = "BASIC"
	}
	if resource.MemorySizeGB == 0 {
		resource.MemorySizeGB = 1
	}
	if resource.RedisVersion == "" {
		resource.RedisVersion = "REDIS_7_0"
	}
}

func applyPubSubDefaults(resource *types.PubSubTopic) {
	if resource.MessageRetentionDuration == "" {
		resource.MessageRetentionDuration = "604800s"
	}
	if resource.Labels == nil {
		resource.Labels = map[string]string{}
	}
}

func applyLoggingBucketDefaults(resource *types.LoggingBucket) {
	if resource.Location == "" {
		resource.Location = "global"
	}
	if resource.RetentionDays == 0 {
		resource.RetentionDays = 30
	}
}

func applyLoggingSinkDefaults(resource *types.LoggingSink) {
	_ = resource
}

func nodeArgumentString(node *kdl.Node, index int) (string, error) {
	arguments := node.Arguments()
	if len(arguments) <= index {
		return "", fmt.Errorf("missing argument %d", index)
	}
	return valueString(arguments[index])
}

func nodeArgumentInt(node *kdl.Node, index int) (int, error) {
	arguments := node.Arguments()
	if len(arguments) <= index {
		return 0, fmt.Errorf("missing argument %d", index)
	}
	return valueInt(arguments[index])
}

func nodeArgumentBool(node *kdl.Node, index int) (bool, error) {
	arguments := node.Arguments()
	if len(arguments) <= index {
		return false, fmt.Errorf("missing argument %d", index)
	}
	return valueBool(arguments[index])
}

func parseStringMapBlock(node *kdl.Node) (map[string]string, error) {
	values := make(map[string]string, len(node.Children().Nodes))
	for _, child := range node.Children().Nodes {
		value, err := nodeArgumentString(child, 0)
		if err != nil {
			return nil, fmt.Errorf("read %q map entry: %w", child.Name(), err)
		}
		values[child.Name()] = value
	}
	return values, nil
}

func parseStringListBlock(node *kdl.Node) ([]string, error) {
	values := make([]string, 0, len(node.Children().Nodes))
	for _, child := range node.Children().Nodes {
		value, err := nodeArgumentString(child, 0)
		if err != nil {
			return nil, fmt.Errorf("read %q list entry: %w", child.Name(), err)
		}
		values = append(values, value)
	}
	return values, nil
}

func propertyString(node *kdl.Node, key string) string {
	value, ok := node.Properties()[key]
	if !ok {
		return ""
	}

	str, err := valueString(value)
	if err != nil {
		return ""
	}

	return str
}

func propertyBool(node *kdl.Node, key string) (bool, error) {
	value, ok := node.Properties()[key]
	if !ok {
		return false, nil
	}
	if value.Kind() != kdl.Bool {
		return false, fmt.Errorf("property %q must be a boolean", key)
	}
	return value.Bool(), nil
}

func valueString(value kdl.Value) (string, error) {
	switch value.Kind() {
	case kdl.String:
		return value.String(), nil
	case kdl.Int:
		return strconv.Itoa(value.Int()), nil
	case kdl.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case kdl.Float:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported value kind %s", value.Kind())
	}
}

func valueBool(value kdl.Value) (bool, error) {
	switch value.Kind() {
	case kdl.Bool:
		return value.Bool(), nil
	case kdl.String:
		boolValue, err := strconv.ParseBool(value.String())
		if err != nil {
			return false, fmt.Errorf("parse bool %q: %w", value.String(), err)
		}
		return boolValue, nil
	default:
		return false, fmt.Errorf("expected boolean, got %s", value.Kind())
	}
}

func valueInt(value kdl.Value) (int, error) {
	switch value.Kind() {
	case kdl.Int:
		return value.Int(), nil
	case kdl.String:
		number, err := strconv.Atoi(value.String())
		if err != nil {
			return 0, fmt.Errorf("parse integer %q: %w", value.String(), err)
		}
		return number, nil
	default:
		return 0, fmt.Errorf("expected integer, got %s", value.Kind())
	}
}
