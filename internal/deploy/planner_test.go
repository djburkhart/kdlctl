package deploy_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/internal/config"
	"github.com/djburkhart/kdlctl/internal/deploy"
)

func planFixturePath(parts ...string) string {
	base := []string{"..", "..", "examples"}
	return filepath.Join(append(base, parts...)...)
}

func TestBuildPlanIncludesServicesResourcesAndNATS(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(planFixturePath("complex", "deploy.kdl"))
	require.NoError(t, err)

	plan, err := deploy.BuildPlan(cfg, "production", "")
	require.NoError(t, err)
	assert.Equal(t, "fixture-gcp-project", plan.ProjectID)
	assert.Len(t, plan.Services, 3)
	assert.Len(t, plan.Resources, 5)
	require.NotNil(t, plan.NATS)
	require.NotNil(t, plan.NATS.Cluster)
	assert.Equal(t, "nats-prod", plan.NATS.Cluster.Name)

	rendered := plan.Render()
	assert.Contains(t, rendered, "cloud-run/api-gateway")
	assert.Contains(t, rendered, "cloudSqlInstances:")
	assert.Contains(t, rendered, "API_DB_DSN: secret:api-db-dsn")
	assert.Contains(t, rendered, "logging-sink/error-export")
	assert.Contains(t, rendered, "NATS cluster: nats-prod")
}

func TestBuildPlanFiltersSingleTarget(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(planFixturePath("complex", "deploy.kdl"))
	require.NoError(t, err)

	plan, err := deploy.BuildPlan(cfg, "production", "primary-db")
	require.NoError(t, err)
	assert.Empty(t, plan.Services)
	require.Len(t, plan.Resources, 1)
	assert.Equal(t, "primary-db", plan.Resources[0].Name)
}

func TestBuildPlanRejectsUnknownTargetFilter(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFile(planFixturePath("valid", "deploy.kdl"))
	require.NoError(t, err)

	_, err = deploy.BuildPlan(cfg, "dev", "missing-target")
	require.Error(t, err)
	assert.ErrorContains(t, err, `no targets matched environment "dev"`)
}
