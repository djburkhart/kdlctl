package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/internal/config"
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

func TestValidateProjectRejectsDuplicateTargetNames(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(fixturePath("invalid", "deploy.kdl"))
	require.NoError(t, err)

	err = config.ValidateProject(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, `defines "shared-name" in both cloud-run and grpc-server`)
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

func TestParseRejectsMissingProjectNode(t *testing.T) {
	t.Parallel()

	_, err := config.Parse([]byte(`environment "dev" {}`))
	require.Error(t, err)
	assert.ErrorContains(t, err, "top-level project node")
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
