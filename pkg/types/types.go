package types

type ProjectConfig struct {
	ProjectID    string                        `validate:"required"`
	Region       string                        `validate:"required"`
	Environments map[string]*EnvironmentConfig `validate:"required"`
}

type ServiceKind string

const (
	ServiceKindCloudRun ServiceKind = "cloud-run"
	ServiceKindGRPC     ServiceKind = "grpc-server"
	ServiceKindCaddy    ServiceKind = "caddy-server"
)

type ResourceKind string

const (
	ResourceKindCloudSQL      ResourceKind = "cloud-sql"
	ResourceKindRedis         ResourceKind = "redis"
	ResourceKindPubSubTopic   ResourceKind = "pubsub-topic"
	ResourceKindLoggingBucket ResourceKind = "logging-bucket"
	ResourceKindLoggingSink   ResourceKind = "logging-sink"
)

type EnvironmentConfig struct {
	Name             string `validate:"required"`
	Extends          string
	CloudRunServices map[string]*CloudRunService
	GRPCServers      map[string]*GRPCServer
	CaddyServers     map[string]*CaddyServer
	CloudSQL         map[string]*CloudSQLInstance
	Redis            map[string]*RedisInstance
	PubSubTopics     map[string]*PubSubTopic
	LoggingBuckets   map[string]*LoggingBucket
	LoggingSinks     map[string]*LoggingSink
	NATS             *NATSConfig
}

type CloudRunService struct {
	Name                 string `validate:"required"`
	Image                string `validate:"required"`
	CPU                  int    `validate:"gte=1"`
	Memory               string `validate:"required"`
	MinInstances         int    `validate:"gte=0"`
	MaxInstances         int    `validate:"gte=1"`
	Concurrency          int    `validate:"gte=1"`
	Port                 int    `validate:"gte=1,lte=65535"`
	Ingress              string
	UseHTTP2             bool
	AllowUnauthenticated bool
	VPCConnector         string
	VPCEgress            string
	CloudSQLInstances    []string
	Labels               map[string]string
	Env                  map[string]EnvVar
	Traffic              TrafficConfig
}

type GRPCServer struct {
	Name                 string `validate:"required"`
	Image                string `validate:"required"`
	CPU                  int    `validate:"gte=1"`
	Memory               string `validate:"required"`
	MinInstances         int    `validate:"gte=0"`
	MaxInstances         int    `validate:"gte=1"`
	Concurrency          int    `validate:"gte=1"`
	Port                 int    `validate:"gte=1,lte=65535"`
	Ingress              string
	UseHTTP2             bool
	AllowUnauthenticated bool
	VPCConnector         string
	VPCEgress            string
	CloudSQLInstances    []string
	Labels               map[string]string
	Env                  map[string]EnvVar
	Traffic              TrafficConfig
}

type CaddyServer struct {
	Name                 string `validate:"required"`
	Image                string `validate:"required"`
	CPU                  int    `validate:"gte=1"`
	Memory               string `validate:"required"`
	MinInstances         int    `validate:"gte=0"`
	MaxInstances         int    `validate:"gte=1"`
	Concurrency          int    `validate:"gte=1"`
	Port                 int    `validate:"gte=1,lte=65535"`
	Ingress              string
	UseHTTP2             bool
	AllowUnauthenticated bool
	VPCConnector         string
	VPCEgress            string
	CloudSQLInstances    []string
	Labels               map[string]string
	Env                  map[string]EnvVar
	Traffic              TrafficConfig
}

type DeploymentService struct {
	Kind                 ServiceKind       `json:"kind"`
	Name                 string            `json:"name"`
	Image                string            `json:"image"`
	CPU                  int               `json:"cpu"`
	Memory               string            `json:"memory"`
	MinInstances         int               `json:"minInstances"`
	MaxInstances         int               `json:"maxInstances"`
	Concurrency          int               `json:"concurrency"`
	Port                 int               `json:"port"`
	Ingress              string            `json:"ingress,omitempty"`
	UseHTTP2             bool              `json:"useHttp2,omitempty"`
	AllowUnauthenticated bool              `json:"allowUnauthenticated,omitempty"`
	VPCConnector         string            `json:"vpcConnector,omitempty"`
	VPCEgress            string            `json:"vpcEgress,omitempty"`
	CloudSQLInstances    []string          `json:"cloudSqlInstances,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Env                  map[string]EnvVar `json:"env,omitempty"`
	Traffic              TrafficConfig     `json:"traffic"`
}

type CloudSQLInstance struct {
	Name             string `validate:"required"`
	DatabaseVersion  string `validate:"required"`
	Tier             string `validate:"required"`
	AvailabilityType string `validate:"required,oneof=REGIONAL ZONAL"`
	StorageSizeGB    int    `validate:"gte=10"`
}

type RedisInstance struct {
	Name         string `validate:"required"`
	Tier         string `validate:"required,oneof=BASIC STANDARD_HA"`
	MemorySizeGB int    `validate:"gte=1"`
	RedisVersion string `validate:"required"`
}

type PubSubTopic struct {
	Name                     string `validate:"required"`
	MessageRetentionDuration string `validate:"required"`
	Labels                   map[string]string
}

type LoggingBucket struct {
	Name          string `validate:"required"`
	Location      string `validate:"required"`
	RetentionDays int    `validate:"gte=1"`
	Description   string
}

type LoggingSink struct {
	Name                 string `validate:"required"`
	Destination          string `validate:"required"`
	Filter               string
	Description          string
	UniqueWriterIdentity bool
}

type DeploymentResource struct {
	Kind                     ResourceKind      `json:"kind"`
	Name                     string            `json:"name"`
	DatabaseVersion          string            `json:"databaseVersion,omitempty"`
	Tier                     string            `json:"tier,omitempty"`
	AvailabilityType         string            `json:"availabilityType,omitempty"`
	StorageSizeGB            int               `json:"storageSizeGb,omitempty"`
	MemorySizeGB             int               `json:"memorySizeGb,omitempty"`
	RedisVersion             string            `json:"redisVersion,omitempty"`
	MessageRetentionDuration string            `json:"messageRetentionDuration,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty"`
	Location                 string            `json:"location,omitempty"`
	RetentionDays            int               `json:"retentionDays,omitempty"`
	Description              string            `json:"description,omitempty"`
	Destination              string            `json:"destination,omitempty"`
	Filter                   string            `json:"filter,omitempty"`
	UniqueWriterIdentity     bool              `json:"uniqueWriterIdentity,omitempty"`
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
		GRPCServers:      make(map[string]*GRPCServer, len(e.GRPCServers)),
		CaddyServers:     make(map[string]*CaddyServer, len(e.CaddyServers)),
		CloudSQL:         make(map[string]*CloudSQLInstance, len(e.CloudSQL)),
		Redis:            make(map[string]*RedisInstance, len(e.Redis)),
		PubSubTopics:     make(map[string]*PubSubTopic, len(e.PubSubTopics)),
		LoggingBuckets:   make(map[string]*LoggingBucket, len(e.LoggingBuckets)),
		LoggingSinks:     make(map[string]*LoggingSink, len(e.LoggingSinks)),
	}

	for name, svc := range e.CloudRunServices {
		cloned.CloudRunServices[name] = svc.Clone()
	}
	for name, svc := range e.GRPCServers {
		cloned.GRPCServers[name] = svc.Clone()
	}
	for name, svc := range e.CaddyServers {
		cloned.CaddyServers[name] = svc.Clone()
	}
	for name, resource := range e.CloudSQL {
		cloned.CloudSQL[name] = resource.Clone()
	}
	for name, resource := range e.Redis {
		cloned.Redis[name] = resource.Clone()
	}
	for name, resource := range e.PubSubTopics {
		cloned.PubSubTopics[name] = resource.Clone()
	}
	for name, resource := range e.LoggingBuckets {
		cloned.LoggingBuckets[name] = resource.Clone()
	}
	for name, resource := range e.LoggingSinks {
		cloned.LoggingSinks[name] = resource.Clone()
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
		Name:                 s.Name,
		Image:                s.Image,
		CPU:                  s.CPU,
		Memory:               s.Memory,
		MinInstances:         s.MinInstances,
		MaxInstances:         s.MaxInstances,
		Concurrency:          s.Concurrency,
		Port:                 s.Port,
		Ingress:              s.Ingress,
		UseHTTP2:             s.UseHTTP2,
		AllowUnauthenticated: s.AllowUnauthenticated,
		VPCConnector:         s.VPCConnector,
		VPCEgress:            s.VPCEgress,
		CloudSQLInstances:    append([]string(nil), s.CloudSQLInstances...),
		Labels:               cloneStringMap(s.Labels),
		Traffic:              s.Traffic,
		Env:                  make(map[string]EnvVar, len(s.Env)),
	}

	for key, value := range s.Env {
		cloned.Env[key] = value
	}

	return cloned
}

func (s *GRPCServer) Clone() *GRPCServer {
	if s == nil {
		return nil
	}

	cloned := &GRPCServer{
		Name:                 s.Name,
		Image:                s.Image,
		CPU:                  s.CPU,
		Memory:               s.Memory,
		MinInstances:         s.MinInstances,
		MaxInstances:         s.MaxInstances,
		Concurrency:          s.Concurrency,
		Port:                 s.Port,
		Ingress:              s.Ingress,
		UseHTTP2:             s.UseHTTP2,
		AllowUnauthenticated: s.AllowUnauthenticated,
		VPCConnector:         s.VPCConnector,
		VPCEgress:            s.VPCEgress,
		CloudSQLInstances:    append([]string(nil), s.CloudSQLInstances...),
		Labels:               cloneStringMap(s.Labels),
		Traffic:              s.Traffic,
		Env:                  make(map[string]EnvVar, len(s.Env)),
	}

	for key, value := range s.Env {
		cloned.Env[key] = value
	}

	return cloned
}

func (s *CaddyServer) Clone() *CaddyServer {
	if s == nil {
		return nil
	}

	cloned := &CaddyServer{
		Name:                 s.Name,
		Image:                s.Image,
		CPU:                  s.CPU,
		Memory:               s.Memory,
		MinInstances:         s.MinInstances,
		MaxInstances:         s.MaxInstances,
		Concurrency:          s.Concurrency,
		Port:                 s.Port,
		Ingress:              s.Ingress,
		UseHTTP2:             s.UseHTTP2,
		AllowUnauthenticated: s.AllowUnauthenticated,
		VPCConnector:         s.VPCConnector,
		VPCEgress:            s.VPCEgress,
		CloudSQLInstances:    append([]string(nil), s.CloudSQLInstances...),
		Labels:               cloneStringMap(s.Labels),
		Traffic:              s.Traffic,
		Env:                  make(map[string]EnvVar, len(s.Env)),
	}

	for key, value := range s.Env {
		cloned.Env[key] = value
	}

	return cloned
}

func (r *CloudSQLInstance) Clone() *CloudSQLInstance {
	if r == nil {
		return nil
	}

	return &CloudSQLInstance{
		Name:             r.Name,
		DatabaseVersion:  r.DatabaseVersion,
		Tier:             r.Tier,
		AvailabilityType: r.AvailabilityType,
		StorageSizeGB:    r.StorageSizeGB,
	}
}

func (r *RedisInstance) Clone() *RedisInstance {
	if r == nil {
		return nil
	}

	return &RedisInstance{
		Name:         r.Name,
		Tier:         r.Tier,
		MemorySizeGB: r.MemorySizeGB,
		RedisVersion: r.RedisVersion,
	}
}

func (r *PubSubTopic) Clone() *PubSubTopic {
	if r == nil {
		return nil
	}

	return &PubSubTopic{
		Name:                     r.Name,
		MessageRetentionDuration: r.MessageRetentionDuration,
		Labels:                   cloneStringMap(r.Labels),
	}
}

func (r *LoggingBucket) Clone() *LoggingBucket {
	if r == nil {
		return nil
	}

	return &LoggingBucket{
		Name:          r.Name,
		Location:      r.Location,
		RetentionDays: r.RetentionDays,
		Description:   r.Description,
	}
}

func (r *LoggingSink) Clone() *LoggingSink {
	if r == nil {
		return nil
	}

	return &LoggingSink{
		Name:                 r.Name,
		Destination:          r.Destination,
		Filter:               r.Filter,
		Description:          r.Description,
		UniqueWriterIdentity: r.UniqueWriterIdentity,
	}
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

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
