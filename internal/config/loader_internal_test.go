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

func TestParseEnvironmentWithExplicitValues(t *testing.T) {
	t.Parallel()

	doc, err := kdl.Parse(strings.NewReader(`
environment "prod" extends="base" {
  cloud-run "api" {
    image "image:latest"
    cpu 2
    memory "1Gi"
    min-instances 1
    max-instances 5
    concurrency 50
    port 8080
    ingress "internal"
    http2 #true
    allow-unauthenticated #true
    vpc-connector "run-vpc"
    vpc-egress "private-ranges-only"
    cloud-sql-instances {
      item "sql-a"
      item "sql-b"
    }
    labels {
      env "prod"
    }
    env {
      LOG_LEVEL "info"
      TOKEN secret="api-token"
    }
    traffic {
      latest 25
    }
  }
  grpc-server "grpc" {
    image "grpc:latest"
    cpu 1
    memory "512Mi"
    port 8443
  }
  caddy-server "edge" {
    image "caddy:latest"
    cpu 1
    memory "512Mi"
    port 8081
  }
  cloud-sql "db" {
    database-version "POSTGRES_16"
    tier "db-custom-2-7680"
    availability-type "REGIONAL"
    storage-gb 100
  }
  redis "cache" {
    tier "STANDARD_HA"
    memory-gb 2
    redis-version "REDIS_7_0"
  }
  pubsub-topic "events" {
    retention "120s"
    labels {
      app "api"
    }
  }
  logging-bucket "logs" {
    location "global"
    retention-days 30
    description "bucket"
  }
  logging-sink "sink" {
    destination "logging.googleapis.com/projects/p/locations/global/buckets/logs"
    filter "severity>=ERROR"
    description "sink"
    unique-writer-identity #true
  }
  nats {
    cluster "nats-prod" {
      replicas 3
      jetstream enabled=#true
      storage-class "fast"
      size "20Gi"
    }
  }
}`))
	require.NoError(t, err)

	env, err := parseEnvironment(doc.GetNode("environment"))
	require.NoError(t, err)

	api := env.CloudRunServices["api"]
	require.NotNil(t, api)
	assert.Equal(t, 2, api.CPU)
	assert.Equal(t, "1Gi", api.Memory)
	assert.Equal(t, 1, api.MinInstances)
	assert.Equal(t, 5, api.MaxInstances)
	assert.Equal(t, 50, api.Concurrency)
	assert.Equal(t, 8080, api.Port)
	assert.Equal(t, "internal", api.Ingress)
	assert.True(t, api.UseHTTP2)
	assert.True(t, api.AllowUnauthenticated)
	assert.Equal(t, "run-vpc", api.VPCConnector)
	assert.Equal(t, "private-ranges-only", api.VPCEgress)
	assert.Equal(t, []string{"sql-a", "sql-b"}, api.CloudSQLInstances)
	assert.Equal(t, "prod", api.Labels["env"])
	assert.Equal(t, "info", api.Env["LOG_LEVEL"].Value)
	assert.Equal(t, "api-token", api.Env["TOKEN"].Secret)
	assert.Equal(t, 25, api.Traffic.LatestPercent)

	require.NotNil(t, env.GRPCServers["grpc"])
	require.NotNil(t, env.CaddyServers["edge"])
	assert.Equal(t, "POSTGRES_16", env.CloudSQL["db"].DatabaseVersion)
	assert.Equal(t, "STANDARD_HA", env.Redis["cache"].Tier)
	assert.Equal(t, "120s", env.PubSubTopics["events"].MessageRetentionDuration)
	assert.Equal(t, "api", env.PubSubTopics["events"].Labels["app"])
	assert.Equal(t, "global", env.LoggingBuckets["logs"].Location)
	assert.Equal(t, "bucket", env.LoggingBuckets["logs"].Description)
	assert.Equal(t, "severity>=ERROR", env.LoggingSinks["sink"].Filter)
	assert.True(t, env.LoggingSinks["sink"].UniqueWriterIdentity)
	require.NotNil(t, env.NATS)
	require.NotNil(t, env.NATS.Cluster)
	assert.Equal(t, 3, env.NATS.Cluster.Replicas)
	assert.True(t, env.NATS.Cluster.JetStream)
	assert.Equal(t, "fast", env.NATS.Cluster.StorageClass)
	assert.Equal(t, "20Gi", env.NATS.Cluster.Size)
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

	t.Run("environment parse branches", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			snippet  string
			contains string
		}{
			{
				name: "environment name",
				snippet: `
environment {
}`,
				contains: "parse environment name",
			},
			{
				name: "grpc server",
				snippet: `
environment "prod" {
  grpc-server "grpc" {
    http2 "oops"
  }
}`,
				contains: `parse grpc-server in environment "prod"`,
			},
			{
				name: "caddy server",
				snippet: `
environment "prod" {
  caddy-server "edge" {
    allow-unauthenticated "oops"
  }
}`,
				contains: `parse caddy-server in environment "prod"`,
			},
			{
				name: "cloud sql",
				snippet: `
environment "prod" {
  cloud-sql "db" {
    storage-gb "oops"
  }
}`,
				contains: `parse cloud-sql in environment "prod"`,
			},
			{
				name: "redis",
				snippet: `
environment "prod" {
  redis "cache" {
    memory-gb "oops"
  }
}`,
				contains: `parse redis in environment "prod"`,
			},
			{
				name: "pubsub",
				snippet: `
environment "prod" {
  pubsub-topic "events" {
    labels {
      BAD
    }
  }
}`,
				contains: `parse pubsub-topic in environment "prod"`,
			},
			{
				name: "logging bucket",
				snippet: `
environment "prod" {
  logging-bucket "logs" {
    retention-days "oops"
  }
}`,
				contains: `parse logging-bucket in environment "prod"`,
			},
			{
				name: "logging sink",
				snippet: `
environment "prod" {
  logging-sink "sink" {
    unique-writer-identity "oops"
  }
}`,
				contains: `parse logging-sink in environment "prod"`,
			},
			{
				name: "nats",
				snippet: `
environment "prod" {
  nats {
    cluster "nats" {
      jetstream enabled="oops"
    }
  }
}`,
				contains: `parse nats config in environment "prod"`,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				doc, err := kdl.Parse(strings.NewReader(tt.snippet))
				require.NoError(t, err)

				_, err = parseEnvironment(doc.GetNode("environment"))
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.contains)
			})
		}
	})
}

func TestMergeHelpersHandleEmptyMapsAndNilClusters(t *testing.T) {
	t.Parallel()

	grpcMerged := mergeGRPCServer(&types.GRPCServer{
		Name:  "grpc",
		Image: "base",
	}, &types.GRPCServer{
		Labels: map[string]string{"tier": "api"},
		Env:    map[string]types.EnvVar{"TOKEN": {Secret: "token"}},
	})
	assert.Equal(t, "api", grpcMerged.Labels["tier"])
	assert.Equal(t, "token", grpcMerged.Env["TOKEN"].Secret)

	caddyMerged := mergeCaddyServer(&types.CaddyServer{
		Name:  "caddy",
		Image: "base",
	}, &types.CaddyServer{
		Labels: map[string]string{"tier": "edge"},
		Env:    map[string]types.EnvVar{"CONFIG": {Value: "/etc/caddy/Caddyfile"}},
	})
	assert.Equal(t, "edge", caddyMerged.Labels["tier"])
	assert.Equal(t, "/etc/caddy/Caddyfile", caddyMerged.Env["CONFIG"].Value)

	natsMerged := mergeNATS(&types.NATSConfig{}, &types.NATSConfig{
		Cluster: &types.NATSClusterConfig{Name: "nats", Replicas: 3},
	})
	require.NotNil(t, natsMerged.Cluster)
	assert.Equal(t, "nats", natsMerged.Cluster.Name)

	natsBase := &types.NATSConfig{Cluster: &types.NATSClusterConfig{Name: "base"}}
	assert.Equal(t, "base", mergeNATS(natsBase, &types.NATSConfig{}).Cluster.Name)
}

func TestMergeHelpersOverrideAllFields(t *testing.T) {
	t.Parallel()

	grpc := mergeGRPCServer(&types.GRPCServer{
		Name:         "base",
		Image:        "base-image",
		CPU:          1,
		Memory:       "512Mi",
		MinInstances: 1,
		MaxInstances: 2,
		Concurrency:  10,
		Port:         8080,
		Ingress:      "internal",
		Labels:       map[string]string{},
		Env:          map[string]types.EnvVar{},
	}, &types.GRPCServer{
		Name:                 "override",
		Image:                "override-image",
		CPU:                  2,
		Memory:               "1Gi",
		MinInstances:         3,
		MaxInstances:         4,
		Concurrency:          20,
		Port:                 8443,
		Ingress:              "all",
		UseHTTP2:             true,
		AllowUnauthenticated: true,
		VPCConnector:         "grpc-vpc",
		VPCEgress:            "private-ranges-only",
		CloudSQLInstances:    []string{"grpc-sql"},
		Labels:               map[string]string{"tier": "api"},
		Env:                  map[string]types.EnvVar{"TOKEN": {Secret: "grpc-token"}},
		Traffic:              types.TrafficConfig{LatestPercent: 100},
	})
	assert.Equal(t, "override", grpc.Name)
	assert.Equal(t, "override-image", grpc.Image)
	assert.Equal(t, 2, grpc.CPU)
	assert.Equal(t, "1Gi", grpc.Memory)
	assert.Equal(t, 3, grpc.MinInstances)
	assert.Equal(t, 4, grpc.MaxInstances)
	assert.Equal(t, 20, grpc.Concurrency)
	assert.Equal(t, 8443, grpc.Port)
	assert.Equal(t, "all", grpc.Ingress)
	assert.True(t, grpc.UseHTTP2)
	assert.True(t, grpc.AllowUnauthenticated)
	assert.Equal(t, "grpc-vpc", grpc.VPCConnector)
	assert.Equal(t, "private-ranges-only", grpc.VPCEgress)
	assert.Equal(t, []string{"grpc-sql"}, grpc.CloudSQLInstances)
	assert.Equal(t, "api", grpc.Labels["tier"])
	assert.Equal(t, "grpc-token", grpc.Env["TOKEN"].Secret)
	assert.Equal(t, 100, grpc.Traffic.LatestPercent)

	caddy := mergeCaddyServer(&types.CaddyServer{
		Name:         "base",
		Image:        "base-image",
		CPU:          1,
		Memory:       "512Mi",
		MinInstances: 1,
		MaxInstances: 2,
		Concurrency:  10,
		Port:         8080,
		Ingress:      "internal",
		Labels:       map[string]string{},
		Env:          map[string]types.EnvVar{},
	}, &types.CaddyServer{
		Name:                 "override",
		Image:                "override-image",
		CPU:                  2,
		Memory:               "1Gi",
		MinInstances:         3,
		MaxInstances:         4,
		Concurrency:          20,
		Port:                 8443,
		Ingress:              "all",
		UseHTTP2:             true,
		AllowUnauthenticated: true,
		VPCConnector:         "caddy-vpc",
		VPCEgress:            "all-traffic",
		CloudSQLInstances:    []string{"caddy-sql"},
		Labels:               map[string]string{"tier": "edge"},
		Env:                  map[string]types.EnvVar{"CONFIG": {Value: "Caddyfile"}},
		Traffic:              types.TrafficConfig{LatestPercent: 100},
	})
	assert.Equal(t, "override", caddy.Name)
	assert.Equal(t, "override-image", caddy.Image)
	assert.Equal(t, 2, caddy.CPU)
	assert.Equal(t, "1Gi", caddy.Memory)
	assert.Equal(t, 3, caddy.MinInstances)
	assert.Equal(t, 4, caddy.MaxInstances)
	assert.Equal(t, 20, caddy.Concurrency)
	assert.Equal(t, 8443, caddy.Port)
	assert.Equal(t, "all", caddy.Ingress)
	assert.True(t, caddy.UseHTTP2)
	assert.True(t, caddy.AllowUnauthenticated)
	assert.Equal(t, "caddy-vpc", caddy.VPCConnector)
	assert.Equal(t, "all-traffic", caddy.VPCEgress)
	assert.Equal(t, []string{"caddy-sql"}, caddy.CloudSQLInstances)
	assert.Equal(t, "edge", caddy.Labels["tier"])
	assert.Equal(t, "Caddyfile", caddy.Env["CONFIG"].Value)
	assert.Equal(t, 100, caddy.Traffic.LatestPercent)

	sql := mergeCloudSQL(&types.CloudSQLInstance{Name: "base", DatabaseVersion: "POSTGRES_15", Tier: "db-custom-1-3840", AvailabilityType: "ZONAL", StorageSizeGB: 50}, &types.CloudSQLInstance{
		Name:             "override",
		DatabaseVersion:  "POSTGRES_16",
		Tier:             "db-custom-2-7680",
		AvailabilityType: "REGIONAL",
		StorageSizeGB:    100,
	})
	assert.Equal(t, "override", sql.Name)
	assert.Equal(t, "POSTGRES_16", sql.DatabaseVersion)
	assert.Equal(t, "db-custom-2-7680", sql.Tier)
	assert.Equal(t, "REGIONAL", sql.AvailabilityType)
	assert.Equal(t, 100, sql.StorageSizeGB)

	redis := mergeRedis(&types.RedisInstance{Name: "base", Tier: "BASIC", MemorySizeGB: 1, RedisVersion: "REDIS_6_X"}, &types.RedisInstance{
		Name:         "override",
		Tier:         "STANDARD_HA",
		MemorySizeGB: 2,
		RedisVersion: "REDIS_7_0",
	})
	assert.Equal(t, "override", redis.Name)
	assert.Equal(t, "STANDARD_HA", redis.Tier)
	assert.Equal(t, 2, redis.MemorySizeGB)
	assert.Equal(t, "REDIS_7_0", redis.RedisVersion)

	topic := mergePubSubTopic(&types.PubSubTopic{Name: "base", MessageRetentionDuration: "60s", Labels: map[string]string{}}, &types.PubSubTopic{
		Name:                     "override",
		MessageRetentionDuration: "120s",
		Labels:                   map[string]string{"env": "prod"},
	})
	assert.Equal(t, "override", topic.Name)
	assert.Equal(t, "120s", topic.MessageRetentionDuration)
	assert.Equal(t, "prod", topic.Labels["env"])

	bucket := mergeLoggingBucket(&types.LoggingBucket{Name: "base", Location: "us", RetentionDays: 7, Description: "base"}, &types.LoggingBucket{
		Name:          "override",
		Location:      "global",
		RetentionDays: 30,
		Description:   "override",
	})
	assert.Equal(t, "override", bucket.Name)
	assert.Equal(t, "global", bucket.Location)
	assert.Equal(t, 30, bucket.RetentionDays)
	assert.Equal(t, "override", bucket.Description)

	sink := mergeLoggingSink(&types.LoggingSink{Name: "base", Destination: "dest-a", Filter: "severity>=ERROR", Description: "base"}, &types.LoggingSink{
		Name:                 "override",
		Destination:          "dest-b",
		Filter:               "severity>=WARNING",
		Description:          "override",
		UniqueWriterIdentity: true,
	})
	assert.Equal(t, "override", sink.Name)
	assert.Equal(t, "dest-b", sink.Destination)
	assert.Equal(t, "severity>=WARNING", sink.Filter)
	assert.Equal(t, "override", sink.Description)
	assert.True(t, sink.UniqueWriterIdentity)
}

func TestHelperParsersAdditionalBranches(t *testing.T) {
	t.Parallel()

	node := mustNode(t, `node bad=#null`)
	assert.Empty(t, propertyString(node, "missing"))
	assert.Empty(t, propertyString(node, "bad"))

	value, err := valueString(node.Properties()["bad"])
	require.Error(t, err)
	assert.Empty(t, value)

	listDoc, err := kdl.Parse(strings.NewReader(`
node {
  items {
    entry
  }
}`))
	require.NoError(t, err)
	_, err = parseStringListBlock(listDoc.GetNode("node").Children().GetNode("items"))
	require.Error(t, err)
	assert.ErrorContains(t, err, `read "entry" list entry`)

	natsDoc, err := kdl.Parse(strings.NewReader(`
node {
}`))
	require.NoError(t, err)
	natsCfg, err := parseNATS(natsDoc.GetNode("node"))
	require.NoError(t, err)
	assert.NotNil(t, natsCfg)
	assert.Nil(t, natsCfg.Cluster)

	clusterDoc, err := kdl.Parse(strings.NewReader(`
node {
  cluster "nats" {
    storage-class "fast"
    size "20Gi"
  }
}`))
	require.NoError(t, err)
	natsCfg, err = parseNATS(clusterDoc.GetNode("node"))
	require.NoError(t, err)
	require.NotNil(t, natsCfg.Cluster)
	assert.Equal(t, "nats", natsCfg.Cluster.Name)
	assert.Equal(t, 1, natsCfg.Cluster.Replicas)
	assert.Equal(t, "fast", natsCfg.Cluster.StorageClass)
	assert.Equal(t, "20Gi", natsCfg.Cluster.Size)
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
