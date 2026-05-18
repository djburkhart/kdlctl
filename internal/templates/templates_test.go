package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExampleDeployKDLIncludesSupportedTargets(t *testing.T) {
	t.Parallel()

	assert.Contains(t, ExampleDeployKDL, `cloud-run "api-service"`)
	assert.Contains(t, ExampleDeployKDL, `grpc-server "payments-grpc"`)
	assert.Contains(t, ExampleDeployKDL, `caddy-server "edge-caddy"`)
	assert.Contains(t, ExampleDeployKDL, `cloud-sql "primary-db"`)
	assert.Contains(t, ExampleDeployKDL, `redis "sessions-cache"`)
	assert.Contains(t, ExampleDeployKDL, `logging-sink "error-export"`)
	assert.True(t, strings.Contains(ExampleDeployKDL, `nats {`))
}

func TestCloudBuildTemplateHasExpectedSubstitutions(t *testing.T) {
	t.Parallel()

	assert.Contains(t, CloudBuildTemplate, `_IMAGE`)
	assert.Contains(t, CloudBuildTemplate, `_SERVICE`)
	assert.Contains(t, CloudBuildTemplate, `_REGION`)
}
