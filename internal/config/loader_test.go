package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/internal/config"
	"github.com/djburkhart/kdlctl/pkg/types"
)

func fixturePath(parts ...string) string {
	base := []string{"..", "..", "examples"}
	return filepath.Join(append(base, parts...)...)
}

func TestLoadFileComplexFixture(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(fixturePath("complex", "deploy.kdl"))
	require.NoError(t, err)
	assert.Equal(t, "fixture-gcp-project", cfg.ProjectID)
	assert.Equal(t, []string{"production", "staging"}, config.EnvironmentNames(cfg))

	env, err := config.ResolveEnvironment(cfg, "staging")
	require.NoError(t, err)

	api := env.CloudRunServices["api-gateway"]
	require.NotNil(t, api)
	assert.Equal(t, "us-central1-docker.pkg.dev/fixture-gcp-project/apps/api-gateway:staging", api.Image)
	assert.Equal(t, 5, api.MaxInstances)
	assert.Equal(t, "serverless-connector", api.VPCConnector)
	assert.Equal(t, "private-ranges-only", api.VPCEgress)
	assert.Equal(t, []string{"fixture-gcp-project:us-central1:primary-db"}, api.CloudSQLInstances)
	assert.Equal(t, "staging", api.Labels["env"])
	assert.Equal(t, "api-db-dsn", api.Env["API_DB_DSN"].Secret)

	grpc := env.GRPCServers["payments-grpc"]
	require.NotNil(t, grpc)
	assert.True(t, grpc.UseHTTP2)
	assert.Equal(t, "internal", grpc.Ingress)

	caddy := env.CaddyServers["edge-caddy"]
	require.NotNil(t, caddy)
	assert.True(t, caddy.AllowUnauthenticated)
	assert.Equal(t, "/etc/caddy/Caddyfile", caddy.Env["CADDY_CONFIG"].Value)

	require.Contains(t, env.CloudSQL, "primary-db")
	require.Contains(t, env.Redis, "sessions-cache")
	require.Contains(t, env.PubSubTopics, "domain-events")
	require.Contains(t, env.LoggingBuckets, "application-logs")
	require.Contains(t, env.LoggingSinks, "error-export")
	require.NotNil(t, env.NATS)
	require.NotNil(t, env.NATS.Cluster)
	assert.Equal(t, "severity>=WARNING", env.LoggingSinks["error-export"].Filter)
	assert.Equal(t, "nats-prod", env.NATS.Cluster.Name)
}

func TestLoadFileValidFixtureAndValidateProject(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(fixturePath("valid", "deploy.kdl"))
	require.NoError(t, err)
	require.NoError(t, config.ValidateProject(cfg))

	env, err := config.ResolveEnvironment(cfg, "dev")
	require.NoError(t, err)
	require.Contains(t, env.CloudRunServices, "api-service")
	assert.Equal(t, "debug", env.CloudRunServices["api-service"].Env["LOG_LEVEL"].Value)
}

func TestLoadFileMissingFile(t *testing.T) {
	t.Parallel()

	_, err := config.LoadFile(filepath.Join(t.TempDir(), "missing.kdl"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "read config file")
}

func TestLoadFileRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "deploy.kdl")
	require.NoError(t, os.WriteFile(path, []byte(`project "broken" {`), 0o644))

	_, err := config.LoadFile(path)
	require.Error(t, err)
	assert.ErrorContains(t, err, "parse kdl")
}

func TestValidateProjectRejectsDuplicateTargetNames(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(fixturePath("invalid", "deploy.kdl"))
	require.NoError(t, err)

	err = config.ValidateProject(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, `defines "shared-name" in both cloud-run and grpc-server`)
}

func TestValidateProjectRejectsInvalidProjectMetadata(t *testing.T) {
	t.Parallel()

	err := config.ValidateProject(&types.ProjectConfig{
		Environments: map[string]*types.EnvironmentConfig{
			"dev": {
				Name: "dev",
				CloudRunServices: map[string]*types.CloudRunService{
					"api": {Name: "api", Image: "example/api:latest"},
				},
			},
		},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "validate project")
}

func TestValidateEnvironmentAcceptsFullyPopulatedEnvironment(t *testing.T) {
	t.Parallel()

	err := config.ValidateEnvironment(&types.ProjectConfig{
		ProjectID: "fixture-project",
		Region:    "us-central1",
		Environments: map[string]*types.EnvironmentConfig{
			"dev": {
				Name: "dev",
				CloudRunServices: map[string]*types.CloudRunService{
					"api": {Name: "api", Image: "example/api:latest", CPU: 1, Memory: "512Mi", MaxInstances: 1, Concurrency: 1, Port: 8080},
				},
				GRPCServers: map[string]*types.GRPCServer{
					"grpc": {Name: "grpc", Image: "example/grpc:latest", CPU: 1, Memory: "512Mi", MaxInstances: 1, Concurrency: 1, Port: 8443},
				},
				CaddyServers: map[string]*types.CaddyServer{
					"edge": {Name: "edge", Image: "example/caddy:latest", CPU: 1, Memory: "512Mi", MaxInstances: 1, Concurrency: 1, Port: 8081},
				},
				CloudSQL: map[string]*types.CloudSQLInstance{
					"db": {Name: "db", DatabaseVersion: "POSTGRES_16", Tier: "db-custom-1-3840", AvailabilityType: "ZONAL", StorageSizeGB: 50},
				},
				Redis: map[string]*types.RedisInstance{
					"cache": {Name: "cache", Tier: "BASIC", MemorySizeGB: 1, RedisVersion: "REDIS_7_0"},
				},
				PubSubTopics: map[string]*types.PubSubTopic{
					"events": {Name: "events", MessageRetentionDuration: "60s"},
				},
				LoggingBuckets: map[string]*types.LoggingBucket{
					"logs": {Name: "logs", Location: "global", RetentionDays: 30},
				},
				LoggingSinks: map[string]*types.LoggingSink{
					"sink": {Name: "sink", Destination: "dest"},
				},
			},
		},
	}, "dev")
	require.NoError(t, err)
}

func TestValidateEnvironmentRejectsDuplicateTargetNamesAcrossKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      *types.EnvironmentConfig
		contains string
	}{
		{
			name: "caddy and cloud run",
			env: &types.EnvironmentConfig{
				Name:             "dev",
				CloudRunServices: map[string]*types.CloudRunService{"shared": {Name: "shared", Image: "image"}},
				CaddyServers:     map[string]*types.CaddyServer{"shared": {Name: "shared", Image: "image"}},
			},
			contains: `defines "shared" in both cloud-run and caddy-server`,
		},
		{
			name: "grpc and redis",
			env: &types.EnvironmentConfig{
				Name:        "dev",
				GRPCServers: map[string]*types.GRPCServer{"shared": {Name: "shared", Image: "image"}},
				Redis:       map[string]*types.RedisInstance{"shared": {Name: "shared", Tier: "BASIC", MemorySizeGB: 1, RedisVersion: "REDIS_7_0"}},
			},
			contains: `defines "shared" in both grpc-server and redis`,
		},
		{
			name: "caddy and pubsub",
			env: &types.EnvironmentConfig{
				Name:         "dev",
				CaddyServers: map[string]*types.CaddyServer{"shared": {Name: "shared", Image: "image"}},
				PubSubTopics: map[string]*types.PubSubTopic{"shared": {Name: "shared", MessageRetentionDuration: "60s"}},
			},
			contains: `defines "shared" in both caddy-server and pubsub-topic`,
		},
		{
			name: "cloud sql and logging bucket",
			env: &types.EnvironmentConfig{
				Name:           "dev",
				CloudSQL:       map[string]*types.CloudSQLInstance{"shared": {Name: "shared", DatabaseVersion: "POSTGRES_16", Tier: "db-custom-1-3840", AvailabilityType: "ZONAL", StorageSizeGB: 50}},
				LoggingBuckets: map[string]*types.LoggingBucket{"shared": {Name: "shared", Location: "global", RetentionDays: 30}},
			},
			contains: `defines "shared" in both cloud-sql and logging-bucket`,
		},
		{
			name: "logging bucket and logging sink",
			env: &types.EnvironmentConfig{
				Name:           "dev",
				LoggingBuckets: map[string]*types.LoggingBucket{"shared": {Name: "shared", Location: "global", RetentionDays: 30}},
				LoggingSinks:   map[string]*types.LoggingSink{"shared": {Name: "shared", Destination: "dest"}},
			},
			contains: `defines "shared" in both logging-bucket and logging-sink`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := config.ValidateEnvironment(&types.ProjectConfig{
				ProjectID: "fixture-project",
				Region:    "us-central1",
				Environments: map[string]*types.EnvironmentConfig{
					"dev": tt.env,
				},
			}, "dev")
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.contains)
		})
	}
}

func TestValidateEnvironmentRejectsEmptyTargets(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`
project "empty-targets" region="us-central1" {
    environment "dev" {
    }
}`))
	require.NoError(t, err)

	err = config.ValidateEnvironment(cfg, "dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, `has no deployable targets configured`)
}

func TestValidateProjectRejectsMissingEnvironments(t *testing.T) {
	t.Parallel()

	err := config.ValidateProject(&types.ProjectConfig{
		ProjectID:    "fixture-project",
		Region:       "us-central1",
		Environments: map[string]*types.EnvironmentConfig{},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "at least one environment must be defined")
}

func TestValidateEnvironmentRejectsMissingName(t *testing.T) {
	t.Parallel()

	err := config.ValidateEnvironment(&types.ProjectConfig{
		ProjectID: "fixture-project",
		Region:    "us-central1",
		Environments: map[string]*types.EnvironmentConfig{
			"dev": {
				CloudRunServices: map[string]*types.CloudRunService{
					"api": {Name: "api", Image: "example/api:latest"},
				},
			},
		},
	}, "dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, `environment "dev" is missing a name`)
}

func TestValidateEnvironmentRejectsInvalidTargetsByKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      *types.EnvironmentConfig
		contains string
	}{
		{
			name: "grpc",
			env: &types.EnvironmentConfig{
				Name:        "dev",
				GRPCServers: map[string]*types.GRPCServer{"grpc": {Name: "grpc"}},
			},
			contains: `validate grpc server "grpc"`,
		},
		{
			name: "caddy",
			env: &types.EnvironmentConfig{
				Name:         "dev",
				CaddyServers: map[string]*types.CaddyServer{"edge": {Name: "edge"}},
			},
			contains: `validate caddy server "edge"`,
		},
		{
			name: "cloud-sql",
			env: &types.EnvironmentConfig{
				Name:     "dev",
				CloudSQL: map[string]*types.CloudSQLInstance{"db": {Name: "db"}},
			},
			contains: `validate cloud-sql resource "db"`,
		},
		{
			name: "redis",
			env: &types.EnvironmentConfig{
				Name:  "dev",
				Redis: map[string]*types.RedisInstance{"cache": {Name: "cache"}},
			},
			contains: `validate redis resource "cache"`,
		},
		{
			name: "pubsub",
			env: &types.EnvironmentConfig{
				Name:         "dev",
				PubSubTopics: map[string]*types.PubSubTopic{"events": {Name: "events"}},
			},
			contains: `validate pubsub-topic resource "events"`,
		},
		{
			name: "logging-bucket",
			env: &types.EnvironmentConfig{
				Name:           "dev",
				LoggingBuckets: map[string]*types.LoggingBucket{"logs": {Name: "logs"}},
			},
			contains: `validate logging-bucket resource "logs"`,
		},
		{
			name: "logging-sink",
			env: &types.EnvironmentConfig{
				Name:         "dev",
				LoggingSinks: map[string]*types.LoggingSink{"sink": {Name: "sink"}},
			},
			contains: `validate logging-sink resource "sink"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateEnvironment(&types.ProjectConfig{
				ProjectID: "fixture-project",
				Region:    "us-central1",
				Environments: map[string]*types.EnvironmentConfig{
					"dev": tt.env,
				},
			}, "dev")
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.contains)
		})
	}
}

func TestValidateEnvironmentRejectsInvalidService(t *testing.T) {
	t.Parallel()

	err := config.ValidateEnvironment(&types.ProjectConfig{
		ProjectID: "fixture-project",
		Region:    "us-central1",
		Environments: map[string]*types.EnvironmentConfig{
			"dev": {
				Name: "dev",
				CloudRunServices: map[string]*types.CloudRunService{
					"api": {Name: "api"},
				},
			},
		},
	}, "dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, `validate cloud-run service "api"`)
}

func TestResolveEnvironmentRejectsCycles(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`
project "cycle-project" region="us-central1" {
    environment "a" extends="b" {
        cloud-run "svc-a" {
            image "example/a:latest"
        }
    }

    environment "b" extends="a" {
        cloud-run "svc-b" {
            image "example/b:latest"
        }
    }
}`))
	require.NoError(t, err)

	_, err = config.ResolveEnvironment(cfg, "a")
	require.Error(t, err)
	assert.ErrorContains(t, err, `cyclic environment inheritance`)
}

func TestResolveEnvironmentRejectsMissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(fixturePath("valid", "deploy.kdl"))
	require.NoError(t, err)

	_, err = config.ResolveEnvironment(cfg, "missing")
	require.Error(t, err)
	assert.ErrorContains(t, err, `environment "missing" not found`)
}

func TestParseRejectsMissingProjectNode(t *testing.T) {
	t.Parallel()

	_, err := config.Parse([]byte(`environment "dev" {}`))
	require.Error(t, err)
	assert.ErrorContains(t, err, "top-level project node")
}

func TestParseRejectsProjectWithoutEnvironments(t *testing.T) {
	t.Parallel()

	_, err := config.Parse([]byte(`project "empty" region="us-central1" {}`))
	require.Error(t, err)
	assert.ErrorContains(t, err, "at least one environment")
}

func TestLoadFileRoundTripFromTempFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "deploy.kdl")
	require.NoError(t, os.WriteFile(path, []byte(`
project "temp-project" region="us-central1" {
    environment "dev" {
        cloud-run "api" {
            image "example/api:latest"
        }
    }
}`), 0o644))

	cfg, err := config.LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "temp-project", cfg.ProjectID)
}

func TestEnvironmentNamesAreSorted(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`
project "sorting-project" region="us-central1" {
    environment "prod" {
        cloud-run "api" {
            image "example/api:latest"
        }
    }
    environment "dev" {
        cloud-run "api" {
            image "example/api:dev"
        }
    }
}`))
	require.NoError(t, err)

	assert.Equal(t, []string{"dev", "prod"}, config.EnvironmentNames(cfg))
}
