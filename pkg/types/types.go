package types

type ProjectConfig struct {
	ProjectID    string                        `validate:"required"`
	Region       string                        `validate:"required"`
	Environments map[string]*EnvironmentConfig `validate:"required"`
}

type EnvironmentConfig struct {
	Name             string `validate:"required"`
	Extends          string
	CloudRunServices map[string]*CloudRunService
	NATS             *NATSConfig
}

type CloudRunService struct {
	Name         string `validate:"required"`
	Image        string `validate:"required"`
	CPU          int    `validate:"gte=1"`
	Memory       string `validate:"required"`
	MinInstances int    `validate:"gte=0"`
	MaxInstances int    `validate:"gte=1"`
	Concurrency  int    `validate:"gte=1"`
	Env          map[string]EnvVar
	Traffic      TrafficConfig
}

type EnvVar struct {
	Value  string
	Secret string
}

type TrafficConfig struct {
	LatestPercent int
}

type NATSConfig struct {
	Cluster *NATSClusterConfig
}

type NATSClusterConfig struct {
	Name         string `validate:"required"`
	Replicas     int    `validate:"gte=1"`
	JetStream    bool
	StorageClass string
	Size         string
}

func (p *ProjectConfig) Clone() *ProjectConfig {
	if p == nil {
		return nil
	}

	cloned := &ProjectConfig{
		ProjectID:    p.ProjectID,
		Region:       p.Region,
		Environments: make(map[string]*EnvironmentConfig, len(p.Environments)),
	}

	for name, env := range p.Environments {
		cloned.Environments[name] = env.Clone()
	}

	return cloned
}

func (e *EnvironmentConfig) Clone() *EnvironmentConfig {
	if e == nil {
		return nil
	}

	cloned := &EnvironmentConfig{
		Name:             e.Name,
		Extends:          e.Extends,
		CloudRunServices: make(map[string]*CloudRunService, len(e.CloudRunServices)),
	}

	for name, svc := range e.CloudRunServices {
		cloned.CloudRunServices[name] = svc.Clone()
	}

	if e.NATS != nil {
		cloned.NATS = e.NATS.Clone()
	}

	return cloned
}

func (s *CloudRunService) Clone() *CloudRunService {
	if s == nil {
		return nil
	}

	cloned := &CloudRunService{
		Name:         s.Name,
		Image:        s.Image,
		CPU:          s.CPU,
		Memory:       s.Memory,
		MinInstances: s.MinInstances,
		MaxInstances: s.MaxInstances,
		Concurrency:  s.Concurrency,
		Traffic:      s.Traffic,
		Env:          make(map[string]EnvVar, len(s.Env)),
	}

	for key, value := range s.Env {
		cloned.Env[key] = value
	}

	return cloned
}

func (n *NATSConfig) Clone() *NATSConfig {
	if n == nil {
		return nil
	}

	cloned := &NATSConfig{}
	if n.Cluster != nil {
		cloned.Cluster = &NATSClusterConfig{
			Name:         n.Cluster.Name,
			Replicas:     n.Cluster.Replicas,
			JetStream:    n.Cluster.JetStream,
			StorageClass: n.Cluster.StorageClass,
			Size:         n.Cluster.Size,
		}
	}

	return cloned
}
