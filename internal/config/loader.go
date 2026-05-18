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

	if len(env.CloudRunServices) == 0 {
		return fmt.Errorf("environment %q has no cloud-run services configured", environment)
	}

	for serviceName, service := range env.CloudRunServices {
		if err := validate.Struct(service); err != nil {
			return fmt.Errorf("validate service %q in environment %q: %w", serviceName, environment, err)
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
	if override.Traffic.LatestPercent != 0 {
		merged.Traffic.LatestPercent = override.Traffic.LatestPercent
	}
	for key, value := range override.Env {
		merged.Env[key] = value
	}

	applyCloudRunDefaults(merged)
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

func parseEnvironment(node *kdl.Node) (*types.EnvironmentConfig, error) {
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse environment name: %w", err)
	}

	env := &types.EnvironmentConfig{
		Name:             name,
		Extends:          propertyString(node, "extends"),
		CloudRunServices: map[string]*types.CloudRunService{},
	}

	for _, child := range node.Children().Nodes {
		switch child.Name() {
		case "cloud-run":
			service, err := parseCloudRunService(child)
			if err != nil {
				return nil, fmt.Errorf("parse cloud-run in environment %q: %w", name, err)
			}
			env.CloudRunServices[service.Name] = service
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
	name, err := nodeArgumentString(node, 0)
	if err != nil {
		return nil, fmt.Errorf("parse cloud-run service name: %w", err)
	}

	service := &types.CloudRunService{
		Name: name,
		Env:  map[string]types.EnvVar{},
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
		case "env":
			service.Env, err = parseEnvBlock(child)
		case "traffic":
			service.Traffic, err = parseTrafficBlock(child)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %q for service %q: %w", child.Name(), name, err)
		}
	}

	applyCloudRunDefaults(service)
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
	if service.Traffic.LatestPercent == 0 {
		service.Traffic.LatestPercent = 100
	}
	if service.Env == nil {
		service.Env = map[string]types.EnvVar{}
	}
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
