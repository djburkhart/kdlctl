package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/pkg/types"
)

func TestProjectConfigCloneDeepCopiesNestedValues(t *testing.T) {
	t.Parallel()

	original := &types.ProjectConfig{
		ProjectID: "fixture-project",
		Region:    "us-central1",
		Environments: map[string]*types.EnvironmentConfig{
			"prod": {
				Name: "prod",
				CloudRunServices: map[string]*types.CloudRunService{
					"api": {
						Name:              "api",
						Image:             "image:latest",
						CPU:               2,
						Memory:            "1Gi",
						MinInstances:      1,
						MaxInstances:      10,
						Concurrency:       80,
						Port:              8080,
						CloudSQLInstances: []string{"project:region:db"},
						Labels:            map[string]string{"env": "prod"},
						Env:               map[string]types.EnvVar{"DB_DSN": {Secret: "db-dsn"}},
						Traffic:           types.TrafficConfig{LatestPercent: 100},
					},
				},
				GRPCServers: map[string]*types.GRPCServer{
					"grpc": {
						Name:         "grpc",
						Image:        "grpc:latest",
						CPU:          1,
						Memory:       "512Mi",
						MaxInstances: 5,
						Concurrency:  20,
						Port:         9090,
						Labels:       map[string]string{"tier": "grpc"},
						Env:          map[string]types.EnvVar{"TOKEN": {Value: "abc"}},
					},
				},
				CaddyServers: map[string]*types.CaddyServer{
					"caddy": {
						Name:         "caddy",
						Image:        "caddy:latest",
						CPU:          1,
						Memory:       "512Mi",
						MaxInstances: 2,
						Concurrency:  50,
						Port:         8081,
						Labels:       map[string]string{"tier": "edge"},
						Env:          map[string]types.EnvVar{"CONFIG": {Value: "/etc/caddy/Caddyfile"}},
					},
				},
				CloudSQL: map[string]*types.CloudSQLInstance{
					"db": {Name: "db", DatabaseVersion: "POSTGRES_16", Tier: "db-custom-2-7680", AvailabilityType: "REGIONAL", StorageSizeGB: 100},
				},
				Redis: map[string]*types.RedisInstance{
					"cache": {Name: "cache", Tier: "STANDARD_HA", MemorySizeGB: 2, RedisVersion: "REDIS_7_0"},
				},
				PubSubTopics: map[string]*types.PubSubTopic{
					"events": {Name: "events", MessageRetentionDuration: "604800s", Labels: map[string]string{"env": "prod"}},
				},
				LoggingBuckets: map[string]*types.LoggingBucket{
					"logs": {Name: "logs", Location: "global", RetentionDays: 30, Description: "logs"},
				},
				LoggingSinks: map[string]*types.LoggingSink{
					"sink": {Name: "sink", Destination: "logging.googleapis.com/projects/p/locations/global/buckets/logs", Filter: "severity>=ERROR", Description: "errors", UniqueWriterIdentity: true},
				},
				NATS: &types.NATSConfig{
					Cluster: &types.NATSClusterConfig{Name: "nats", Replicas: 3, JetStream: true, StorageClass: "standard", Size: "20Gi"},
				},
			},
		},
	}

	cloned := original.Clone()
	require.NotNil(t, cloned)
	require.NotSame(t, original, cloned)

	cloned.ProjectID = "changed-project"
	cloned.Environments["prod"].CloudRunServices["api"].Labels["env"] = "staging"
	cloned.Environments["prod"].CloudRunServices["api"].CloudSQLInstances[0] = "other"
	cloned.Environments["prod"].CloudRunServices["api"].Env["DB_DSN"] = types.EnvVar{Secret: "other-secret"}
	cloned.Environments["prod"].PubSubTopics["events"].Labels["env"] = "staging"
	cloned.Environments["prod"].NATS.Cluster.Name = "other-nats"

	assert.Equal(t, "fixture-project", original.ProjectID)
	assert.Equal(t, "prod", original.Environments["prod"].CloudRunServices["api"].Labels["env"])
	assert.Equal(t, "project:region:db", original.Environments["prod"].CloudRunServices["api"].CloudSQLInstances[0])
	assert.Equal(t, "db-dsn", original.Environments["prod"].CloudRunServices["api"].Env["DB_DSN"].Secret)
	assert.Equal(t, "prod", original.Environments["prod"].PubSubTopics["events"].Labels["env"])
	assert.Equal(t, "nats", original.Environments["prod"].NATS.Cluster.Name)
}

func TestCloneNilReceiversReturnNil(t *testing.T) {
	t.Parallel()

	var project *types.ProjectConfig
	var env *types.EnvironmentConfig
	var cloudRun *types.CloudRunService
	var grpc *types.GRPCServer
	var caddy *types.CaddyServer
	var cloudSQL *types.CloudSQLInstance
	var redis *types.RedisInstance
	var topic *types.PubSubTopic
	var bucket *types.LoggingBucket
	var sink *types.LoggingSink
	var natsCfg *types.NATSConfig

	assert.Nil(t, project.Clone())
	assert.Nil(t, env.Clone())
	assert.Nil(t, cloudRun.Clone())
	assert.Nil(t, grpc.Clone())
	assert.Nil(t, caddy.Clone())
	assert.Nil(t, cloudSQL.Clone())
	assert.Nil(t, redis.Clone())
	assert.Nil(t, topic.Clone())
	assert.Nil(t, bucket.Clone())
	assert.Nil(t, sink.Clone())
	assert.Nil(t, natsCfg.Clone())
}
