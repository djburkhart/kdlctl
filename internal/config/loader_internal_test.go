package config

import (
	"strings"
	"testing"

	kdl "github.com/calico32/kdl-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/pkg/types"
)

func TestMergeEnvironmentAndDefaults(t *testing.T) {
	t.Parallel()

	base := &types.EnvironmentConfig{
		Name: "base",
		CloudRunServices: map[string]*types.CloudRunService{
			"api": {
				Name:                 "api",
				Image:                "base-api",
				CPU:                  1,
				Memory:               "512Mi",
				MinInstances:         1,
				MaxInstances:         10,
				Concurrency:          20,
				Port:                 8080,
				Ingress:              "all",
				UseHTTP2:             false,
				AllowUnauthenticated: false,
				VPCConnector:         "base-vpc",
				VPCEgress:            "all-traffic",
				CloudSQLInstances:    []string{"base-sql"},
				Labels:               map[string]string{"base": "true"},
				Env:                  map[string]types.EnvVar{"BASE": {Value: "1"}},
				Traffic:              types.TrafficConfig{LatestPercent: 10},
			},
		},
		GRPCServers: map[string]*types.GRPCServer{
			"grpc": {Name: "grpc", Image: "base-grpc", Labels: map[string]string{"base": "true"}, Env: map[string]types.EnvVar{"BASE": {Value: "1"}}},
		},
		CaddyServers: map[string]*types.CaddyServer{
			"caddy": {Name: "caddy", Image: "base-caddy", Labels: map[string]string{"base": "true"}, Env: map[string]types.EnvVar{"BASE": {Value: "1"}}},
		},
		CloudSQL: map[string]*types.CloudSQLInstance{
			"db": {Name: "db", DatabaseVersion: "POSTGRES_15", Tier: "db-custom-1-3840", AvailabilityType: "ZONAL", StorageSizeGB: 50},
		},
		Redis: map[string]*types.RedisInstance{
			"cache": {Name: "cache", Tier: "BASIC", MemorySizeGB: 1, RedisVersion: "REDIS_6_X"},
		},
		PubSubTopics: map[string]*types.PubSubTopic{
			"events": {Name: "events", MessageRetentionDuration: "60s", Labels: map[string]string{"base": "true"}},
		},
		LoggingBuckets: map[string]*types.LoggingBucket{
			"logs": {Name: "logs", Location: "us", RetentionDays: 7, Description: "base"},
		},
		LoggingSinks: map[string]*types.LoggingSink{
			"sink": {Name: "sink", Destination: "dest-a", Filter: "severity>=ERROR", Description: "base", UniqueWriterIdentity: false},
		},
		NATS: &types.NATSConfig{Cluster: &types.NATSClusterConfig{Name: "base", Replicas: 1, StorageClass: "standard", Size: "10Gi"}},
	}

	override := &types.EnvironmentConfig{
		Name:    "override",
		Extends: "base",
		CloudRunServices: map[string]*types.CloudRunService{
			"api": {
				Image:                "override-api",
				CPU:                  2,
				Memory:               "1Gi",
				MinInstances:         2,
				MaxInstances:         20,
				Concurrency:          30,
				Port:                 9090,
				Ingress:              "internal",
				UseHTTP2:             true,
				AllowUnauthenticated: true,
				VPCConnector:         "override-vpc",
				VPCEgress:            "private-ranges-only",
				CloudSQLInstances:    []string{"override-sql"},
				Labels:               map[string]string{"override": "true"},
				Env:                  map[string]types.EnvVar{"OVERRIDE": {Secret: "secret"}},
				Traffic:              types.TrafficConfig{LatestPercent: 100},
			},
			"worker": {Name: "worker", Image: "worker"},
		},
		GRPCServers: map[string]*types.GRPCServer{
			"grpc":     {Image: "override-grpc", UseHTTP2: true, AllowUnauthenticated: true, VPCConnector: "grpc-vpc", VPCEgress: "private-ranges-only", CloudSQLInstances: []string{"grpc-sql"}, Labels: map[string]string{"override": "true"}, Env: map[string]types.EnvVar{"OVERRIDE": {Value: "1"}}, Traffic: types.TrafficConfig{LatestPercent: 100}},
			"grpc-new": {Name: "grpc-new", Image: "grpc-new"},
		},
		CaddyServers: map[string]*types.CaddyServer{
			"caddy":     {Image: "override-caddy", AllowUnauthenticated: true, Labels: map[string]string{"override": "true"}, Env: map[string]types.EnvVar{"OVERRIDE": {Value: "1"}}, Traffic: types.TrafficConfig{LatestPercent: 100}},
			"caddy-new": {Name: "caddy-new", Image: "caddy-new"},
		},
		CloudSQL: map[string]*types.CloudSQLInstance{
			"db":     {DatabaseVersion: "POSTGRES_16", Tier: "db-custom-2-7680", AvailabilityType: "REGIONAL", StorageSizeGB: 100},
			"db-new": {Name: "db-new"},
		},
		Redis: map[string]*types.RedisInstance{
			"cache":     {Tier: "STANDARD_HA", MemorySizeGB: 2, RedisVersion: "REDIS_7_0"},
			"cache-new": {Name: "cache-new"},
		},
		PubSubTopics: map[string]*types.PubSubTopic{
			"events":     {MessageRetentionDuration: "120s", Labels: map[string]string{"override": "true"}},
			"events-new": {Name: "events-new"},
		},
		LoggingBuckets: map[string]*types.LoggingBucket{
			"logs":     {Location: "global", RetentionDays: 30, Description: "override"},
			"logs-new": {Name: "logs-new"},
		},
		LoggingSinks: map[string]*types.LoggingSink{
			"sink":     {Destination: "dest-b", Filter: "severity>=WARNING", Description: "override", UniqueWriterIdentity: true},
			"sink-new": {Name: "sink-new", Destination: "dest-c"},
		},
		NATS: &types.NATSConfig{Cluster: &types.NATSClusterConfig{Name: "override", Replicas: 3, JetStream: true, StorageClass: "fast", Size: "20Gi"}},
	}

	merged := mergeEnvironment(base, override)

	assert.Equal(t, "override", merged.Name)
	assert.Equal(t, "base", merged.Extends)
	assert.Equal(t, "override-api", merged.CloudRunServices["api"].Image)
	assert.True(t, merged.CloudRunServices["api"].UseHTTP2)
	assert.True(t, merged.CloudRunServices["api"].AllowUnauthenticated)
	assert.Equal(t, "override-vpc", merged.CloudRunServices["api"].VPCConnector)
	assert.Equal(t, "private-ranges-only", merged.CloudRunServices["api"].VPCEgress)
	assert.Equal(t, []string{"override-sql"}, merged.CloudRunServices["api"].CloudSQLInstances)
	assert.Equal(t, "true", merged.CloudRunServices["api"].Labels["override"])
	assert.Equal(t, "1", merged.CloudRunServices["api"].Env["BASE"].Value)
	assert.Equal(t, "secret", merged.CloudRunServices["api"].Env["OVERRIDE"].Secret)
	assert.Equal(t, 100, merged.CloudRunServices["api"].Traffic.LatestPercent)
	assert.Equal(t, "worker", merged.CloudRunServices["worker"].Name)
	assert.True(t, merged.GRPCServers["grpc"].UseHTTP2)
	assert.True(t, merged.GRPCServers["grpc"].AllowUnauthenticated)
	assert.True(t, merged.CaddyServers["caddy"].AllowUnauthenticated)
	assert.Equal(t, "POSTGRES_16", merged.CloudSQL["db"].DatabaseVersion)
	assert.Equal(t, "STANDARD_HA", merged.Redis["cache"].Tier)
	assert.Equal(t, "120s", merged.PubSubTopics["events"].MessageRetentionDuration)
	assert.Equal(t, "global", merged.LoggingBuckets["logs"].Location)
	assert.True(t, merged.LoggingSinks["sink"].UniqueWriterIdentity)
	assert.Equal(t, "override", merged.NATS.Cluster.Name)
	assert.True(t, merged.NATS.Cluster.JetStream)
	assert.Equal(t, "20Gi", merged.NATS.Cluster.Size)
}

func TestHelperParsersAndDefaults(t *testing.T) {
	t.Parallel()

	doc, err := kdl.Parse(strings.NewReader(`
root enabled=#true bad="nope" {
  labels {
    app "api"
    count 2
  }
  list {
    item "one"
    item 2
  }
  env {
    LOG_LEVEL "info"
    DB_DSN secret="db-dsn"
  }
  traffic {
    latest 25
  }
}`))
	require.NoError(t, err)

	root := doc.GetNode("root")
	labels, err := parseStringMapBlock(root.Children().GetNode("labels"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"app": "api", "count": "2"}, labels)

	values, err := parseStringListBlock(root.Children().GetNode("list"))
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "2"}, values)

	env, err := parseEnvBlock(root.Children().GetNode("env"))
	require.NoError(t, err)
	assert.Equal(t, "info", env["LOG_LEVEL"].Value)
	assert.Equal(t, "db-dsn", env["DB_DSN"].Secret)

	traffic, err := parseTrafficBlock(root.Children().GetNode("traffic"))
	require.NoError(t, err)
	assert.Equal(t, 25, traffic.LatestPercent)

	assert.Equal(t, "true", propertyString(root, "enabled"))
	ok, err := propertyBool(root, "enabled")
	require.NoError(t, err)
	assert.True(t, ok)
	_, err = propertyBool(root, "bad")
	require.Error(t, err)
	assert.ErrorContains(t, err, "must be a boolean")

	argNode, err := kdl.Parse(strings.NewReader(`node "value" 10 #true 1.5`))
	require.NoError(t, err)
	node := argNode.GetNode("node")
	gotString, err := nodeArgumentString(node, 0)
	require.NoError(t, err)
	assert.Equal(t, "value", gotString)
	gotInt, err := nodeArgumentInt(node, 1)
	require.NoError(t, err)
	assert.Equal(t, 10, gotInt)
	gotBool, err := nodeArgumentBool(node, 2)
	require.NoError(t, err)
	assert.True(t, gotBool)
	_, err = nodeArgumentString(node, 9)
	require.Error(t, err)
	_, err = nodeArgumentInt(node, 9)
	require.Error(t, err)
	_, err = nodeArgumentBool(node, 9)
	require.Error(t, err)

	assert.Equal(t, "1.5", mustValueString(t, node.Arguments()[3]))
	boolValue, err := valueBool(node.Arguments()[2])
	require.NoError(t, err)
	assert.True(t, boolValue)
	boolFromString, err := valueBool(mustNode(t, `node "true"`).Arguments()[0])
	require.NoError(t, err)
	assert.True(t, boolFromString)
	_, err = valueBool(mustNode(t, `node 1`).Arguments()[0])
	require.Error(t, err)
	intValue, err := valueInt(node.Arguments()[1])
	require.NoError(t, err)
	assert.Equal(t, 10, intValue)
	intFromString, err := valueInt(mustNode(t, `node "42"`).Arguments()[0])
	require.NoError(t, err)
	assert.Equal(t, 42, intFromString)
	_, err = valueInt(mustNode(t, `node #true`).Arguments()[0])
	require.Error(t, err)

	cloudRun := &types.CloudRunService{}
	applyCloudRunDefaults(cloudRun)
	assert.Equal(t, 1, cloudRun.CPU)
	assert.Equal(t, "512Mi", cloudRun.Memory)
	assert.Equal(t, 80, cloudRun.Concurrency)
	assert.Equal(t, 100, cloudRun.MaxInstances)
	assert.Equal(t, 8080, cloudRun.Port)
	assert.Equal(t, "all", cloudRun.Ingress)
	assert.Equal(t, 100, cloudRun.Traffic.LatestPercent)
	assert.Empty(t, cloudRun.Env)

	grpc := &types.GRPCServer{}
	applyGRPCServerDefaults(grpc)
	assert.Equal(t, "internal", grpc.Ingress)
	assert.True(t, grpc.UseHTTP2)

	caddy := &types.CaddyServer{}
	applyCaddyServerDefaults(caddy)
	assert.Equal(t, "all", caddy.Ingress)

	cloudSQL := &types.CloudSQLInstance{}
	applyCloudSQLDefaults(cloudSQL)
	assert.Equal(t, "POSTGRES_16", cloudSQL.DatabaseVersion)
	assert.Equal(t, "db-custom-1-3840", cloudSQL.Tier)
	assert.Equal(t, "ZONAL", cloudSQL.AvailabilityType)
	assert.Equal(t, 50, cloudSQL.StorageSizeGB)

	redis := &types.RedisInstance{}
	applyRedisDefaults(redis)
	assert.Equal(t, "BASIC", redis.Tier)
	assert.Equal(t, 1, redis.MemorySizeGB)
	assert.Equal(t, "REDIS_7_0", redis.RedisVersion)

	topic := &types.PubSubTopic{}
	applyPubSubDefaults(topic)
	assert.Equal(t, "604800s", topic.MessageRetentionDuration)
	assert.Empty(t, topic.Labels)

	bucket := &types.LoggingBucket{}
	applyLoggingBucketDefaults(bucket)
	assert.Equal(t, "global", bucket.Location)
	assert.Equal(t, 30, bucket.RetentionDays)

	sink := &types.LoggingSink{}
	applyLoggingSinkDefaults(sink)
	assert.NotNil(t, sink)
}

func TestParseEnvironmentAndNATS(t *testing.T) {
	t.Parallel()

	doc, err := kdl.Parse(strings.NewReader(`
environment "prod" extends="base" {
  cloud-run "api" {
    image "image:latest"
  }
  grpc-server "grpc" {
    image "grpc:latest"
  }
  caddy-server "caddy" {
    image "caddy:latest"
  }
  cloud-sql "db" {}
  redis "cache" {}
  pubsub-topic "events" {}
  logging-bucket "logs" {}
  logging-sink "sink" {
    destination "dest"
  }
  nats {
    cluster "nats-prod" {
      replicas 3
      jetstream enabled=#true
      storage-class "standard"
      size "20Gi"
    }
  }
}`))
	require.NoError(t, err)

	env, err := parseEnvironment(doc.GetNode("environment"))
	require.NoError(t, err)
	assert.Equal(t, "prod", env.Name)
	assert.Equal(t, "base", env.Extends)
	require.Contains(t, env.CloudRunServices, "api")
	require.Contains(t, env.GRPCServers, "grpc")
	require.Contains(t, env.CaddyServers, "caddy")
	require.Contains(t, env.CloudSQL, "db")
	require.Contains(t, env.Redis, "cache")
	require.Contains(t, env.PubSubTopics, "events")
	require.Contains(t, env.LoggingBuckets, "logs")
	require.Contains(t, env.LoggingSinks, "sink")
	require.NotNil(t, env.NATS)
	require.NotNil(t, env.NATS.Cluster)
	assert.Equal(t, "nats-prod", env.NATS.Cluster.Name)
	assert.True(t, env.NATS.Cluster.JetStream)
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	t.Run("env block", func(t *testing.T) {
		t.Parallel()
		_, err := Parse([]byte(`
project "bad" region="us-central1" {
  environment "prod" {
    cloud-run "api" {
      labels {
        KEY
      }
    }
  }
}`))
		require.Error(t, err)
		assert.ErrorContains(t, err, `read "KEY" map entry`)
	})

	t.Run("traffic block", func(t *testing.T) {
		t.Parallel()
		_, err := Parse([]byte(`
project "bad" region="us-central1" {
  environment "prod" {
    cloud-run "api" {
      image "image"
      traffic {
        latest "oops"
      }
    }
  }
}`))
		require.Error(t, err)
		assert.ErrorContains(t, err, "parse latest traffic")
	})

	t.Run("nats property", func(t *testing.T) {
		t.Parallel()
		node := mustNode(t, `node enabled="nope"`)
		_, err := propertyBool(node, "enabled")
		require.Error(t, err)
	})
}

func mustNode(t *testing.T, snippet string) *kdl.Node {
	t.Helper()
	doc, err := kdl.Parse(strings.NewReader(snippet))
	require.NoError(t, err)
	node := doc.GetNode("node")
	require.NotNil(t, node)
	return node
}

func mustValueString(t *testing.T, value kdl.Value) string {
	t.Helper()
	str, err := valueString(value)
	require.NoError(t, err)
	return str
}
