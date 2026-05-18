package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/internal/deploy"
	"github.com/djburkhart/kdlctl/internal/gcp"
	"github.com/djburkhart/kdlctl/pkg/types"
)

type mockNATSClient struct {
	mock.Mock
}

func (m *mockNATSClient) Close() {
	m.Called()
}

func (m *mockNATSClient) Publish(ctx context.Context, subject string, payload []byte) error {
	args := m.Called(ctx, subject, payload)
	return args.Error(0)
}

func (m *mockNATSClient) Subscribe(ctx context.Context, subject string, count int, handler func(*nats.Msg) error) error {
	args := m.Called(ctx, subject, count, handler)
	return args.Error(0)
}

type mockCloudBuildClient struct {
	mock.Mock
}

func (m *mockCloudBuildClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCloudBuildClient) GetBuildStatus(ctx context.Context, projectID, buildID string) (*gcp.BuildResult, error) {
	args := m.Called(ctx, projectID, buildID)
	result, _ := args.Get(0).(*gcp.BuildResult)
	return result, args.Error(1)
}

type mockRunClient struct {
	mock.Mock
}

func (m *mockRunClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockRunClient) RollbackTraffic(ctx context.Context, projectID, region, serviceName, revision string) error {
	args := m.Called(ctx, projectID, region, serviceName, revision)
	return args.Error(0)
}

func (m *mockCloudBuildClient) SubmitCloudRunBuild(ctx context.Context, request gcp.BuildRequest, wait bool) (*gcp.BuildResult, error) {
	args := m.Called(ctx, request, wait)
	result, _ := args.Get(0).(*gcp.BuildResult)
	return result, args.Error(1)
}

func (m *mockCloudBuildClient) SubmitManagedResourceBuild(ctx context.Context, request gcp.ResourceBuildRequest, wait bool) (*gcp.BuildResult, error) {
	args := m.Called(ctx, request, wait)
	result, _ := args.Get(0).(*gcp.BuildResult)
	return result, args.Error(1)
}

func cliFixturePath(parts ...string) string {
	base := []string{"..", "..", "examples"}
	return filepath.Join(append(base, parts...)...)
}

func resetCLIState(t *testing.T) {
	t.Helper()

	originalNATSFactory := newNATSClient
	originalCloudBuildFactory := newCloudBuildClient
	originalRunFactory := newRunClient
	viper.Reset()
	t.Cleanup(func() {
		newNATSClient = originalNATSFactory
		newCloudBuildClient = originalCloudBuildFactory
		newRunClient = originalRunFactory
		viper.Reset()
	})
}

func executeCommand(t *testing.T, cmdArgs ...string) (string, error) {
	t.Helper()

	var stdout bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(cmdArgs)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestLoadProjectConfigAppliesOverrides(t *testing.T) {
	resetCLIState(t)

	viper.Set("file", cliFixturePath("valid", "deploy.kdl"))
	viper.Set("project-id", "override-project")
	viper.Set("region", "europe-west1")

	cfg, err := loadProjectConfig()
	require.NoError(t, err)
	assert.Equal(t, "override-project", cfg.ProjectID)
	assert.Equal(t, "europe-west1", cfg.Region)
}

func TestEnsureFileDoesNotExistAndWriteFile(t *testing.T) {
	resetCLIState(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "file.txt")
	require.NoError(t, writeFile(path, "hello"))
	assert.FileExists(t, path)

	err := ensureFileDoesNotExist(path, false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "already exists")
	require.NoError(t, ensureFileDoesNotExist(path, true))
}

func TestInitCommandCreatesStarterFiles(t *testing.T) {
	resetCLIState(t)

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	output, err := executeCommand(t, "init")
	require.NoError(t, err)
	assert.Contains(t, output, fmt.Sprintf("Created deploy.kdl, cloudbuild.yaml, and %s", filepath.Join("examples", "deploy.kdl")))
	assert.FileExists(t, filepath.Join(tempDir, "deploy.kdl"))
	assert.FileExists(t, filepath.Join(tempDir, "cloudbuild.yaml"))
	assert.FileExists(t, filepath.Join(tempDir, "examples", "deploy.kdl"))
}

func TestWriteJSON(t *testing.T) {
	resetCLIState(t)

	var stdout bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)

	require.NoError(t, writeJSON(cmd, map[string]string{"hello": "world"}))
	assert.JSONEq(t, `{"hello":"world"}`, stdout.String())
}

func TestPublishDeployRequestUsesMockNATSClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Publish", mock.Anything, "deploy.requested", mock.MatchedBy(func(payload []byte) bool {
		var event deployEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return false
		}
		return event.ProjectID == "fixture-gcp-project" &&
			event.Environment == "production" &&
			len(event.Targets) == 2
	})).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }
	viper.Set("nats-url", "nats://fixture:4222")

	var stdout bytes.Buffer
	err := publishDeployRequest(context.Background(), &stdout, &deploy.DeploymentPlan{
		ProjectID:   "fixture-gcp-project",
		Region:      "us-central1",
		Environment: "production",
		Services: []*types.DeploymentService{
			{Kind: types.ServiceKindCloudRun, Name: "api"},
		},
		Resources: []*types.DeploymentResource{
			{Kind: types.ResourceKindRedis, Name: "cache"},
		},
	}, "deploy.requested")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Published deploy request")
	mockClient.AssertExpectations(t)
}

func TestPublishDeployRequestErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("new client", func(t *testing.T) {
		newNATSClient = func(string) (natsClient, error) { return nil, errors.New("boom") }
		err := publishDeployRequest(context.Background(), &bytes.Buffer{}, &deploy.DeploymentPlan{}, "deploy.requested")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("publish", func(t *testing.T) {
		mockClient := &mockNATSClient{}
		mockClient.On("Publish", mock.Anything, "deploy.requested", mock.Anything).Return(errors.New("boom")).Once()
		mockClient.On("Close").Return().Once()
		newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

		err := publishDeployRequest(context.Background(), &bytes.Buffer{}, &deploy.DeploymentPlan{}, "deploy.requested")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
}

func TestSubmitDeployPlanUsesMockCloudBuildClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockCloudBuildClient{}
	mockClient.On("SubmitCloudRunBuild", mock.Anything, mock.MatchedBy(func(request gcp.BuildRequest) bool {
		return request.ProjectID == "fixture-gcp-project" && request.Service.Name == "api"
	}), true).Return(&gcp.BuildResult{
		ID:     "build-service",
		Status: "SUCCESS",
		LogURL: "https://logs/service",
	}, nil).Once()
	mockClient.On("SubmitManagedResourceBuild", mock.Anything, mock.MatchedBy(func(request gcp.ResourceBuildRequest) bool {
		return request.ProjectID == "fixture-gcp-project" && request.Resource.Name == "cache"
	}), true).Return(&gcp.BuildResult{
		ID:     "build-resource",
		Status: "SUCCESS",
		LogURL: "https://logs/resource",
	}, nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

	var stdout bytes.Buffer
	err := submitDeployPlan(context.Background(), &stdout, &deploy.DeploymentPlan{
		ProjectID:   "fixture-gcp-project",
		Region:      "us-central1",
		Environment: "production",
		Services: []*types.DeploymentService{
			{Kind: types.ServiceKindCloudRun, Name: "api"},
		},
		Resources: []*types.DeploymentResource{
			{Kind: types.ResourceKindRedis, Name: "cache"},
		},
	}, false)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deployed cloud-run/api with status SUCCESS")
	assert.Contains(t, stdout.String(), "Deployed redis/cache with status SUCCESS")
	mockClient.AssertExpectations(t)
}

func TestSubmitDeployPlanErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("new client", func(t *testing.T) {
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return nil, errors.New("boom") }
		err := submitDeployPlan(context.Background(), &bytes.Buffer{}, &deploy.DeploymentPlan{}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("service", func(t *testing.T) {
		mockClient := &mockCloudBuildClient{}
		mockClient.On("SubmitCloudRunBuild", mock.Anything, mock.Anything, true).Return((*gcp.BuildResult)(nil), errors.New("boom")).Once()
		mockClient.On("Close").Return(nil).Once()
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

		err := submitDeployPlan(context.Background(), &bytes.Buffer{}, &deploy.DeploymentPlan{
			ProjectID:   "fixture-gcp-project",
			Region:      "us-central1",
			Environment: "production",
			Services:    []*types.DeploymentService{{Kind: types.ServiceKindCloudRun, Name: "api"}},
		}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
}

func TestDeployCommandViaNATSUsesMocks(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Publish", mock.Anything, "deploy.requested", mock.Anything).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"deploy",
		"--env", "dev",
		"--service", "api-service",
		"--via-nats",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Published deploy request")
	mockClient.AssertExpectations(t)
}

func TestPlanCommandJSONOutput(t *testing.T) {
	resetCLIState(t)

	output, err := executeCommand(t,
		"--file", cliFixturePath("complex", "deploy.kdl"),
		"plan",
		"--env", "production",
		"--json",
	)
	require.NoError(t, err)
	assert.Contains(t, output, `"ProjectID": "fixture-gcp-project"`)
	assert.Contains(t, output, `"name": "api-gateway"`)
}

func TestValidateCommandWithFixture(t *testing.T) {
	resetCLIState(t)

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"validate",
		"--env", "dev",
	)
	require.NoError(t, err)
	assert.Contains(t, output, `Configuration is valid for environment "dev"`)
}

func TestValidateCommandWholeProject(t *testing.T) {
	resetCLIState(t)

	output, err := executeCommand(t,
		"--file", cliFixturePath("complex", "deploy.kdl"),
		"validate",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Configuration is valid for 2 environments")
}

func TestStatusCommandBuildUsesMockCloudBuildClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockCloudBuildClient{}
	mockClient.On("GetBuildStatus", mock.Anything, "valid-gcp-project", "build-123").Return(&gcp.BuildResult{
		ID:     "build-123",
		Status: "SUCCESS",
		LogURL: "https://logs/build-123",
	}, nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"status",
		"--build", "build-123",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Build build-123: SUCCESS https://logs/build-123")
	mockClient.AssertExpectations(t)
}

func TestStatusCommandEnvUsesMockNATSClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Subscribe", mock.Anything, "deploy.status.dev.>", 1, mock.Anything).Run(func(args mock.Arguments) {
		handler := args.Get(3).(func(*nats.Msg) error)
		_ = handler(&nats.Msg{
			Subject: "deploy.status.dev.api-service",
			Data:    []byte("ready"),
		})
	}).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }
	viper.Set("nats-url", "nats://fixture:4222")

	output, err := executeCommand(t, "status", "--env", "dev")
	require.NoError(t, err)
	assert.Contains(t, output, "[deploy.status.dev.api-service] ready")
	mockClient.AssertExpectations(t)
}

func TestDeployCommandAsyncUsesMockCloudBuildClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockCloudBuildClient{}
	mockClient.On("SubmitCloudRunBuild", mock.Anything, mock.Anything, false).Return(&gcp.BuildResult{
		Operation: "operations/build-1",
		Status:    "QUEUED",
	}, nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"deploy",
		"--env", "dev",
		"--service", "api-service",
		"--async",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Queued deploy for cloud-run/api-service")
	mockClient.AssertExpectations(t)
}

func TestStatusCommandRequiresBuildOrEnv(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t, "status")
	require.Error(t, err)
	assert.ErrorContains(t, err, "set either --build or --env")
}

func TestNATSPublishCommandUsesMockClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Publish", mock.Anything, "fixtures.publish", []byte("hello")).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	output, err := executeCommand(t, "nats", "publish", "--subject", "fixtures.publish", "hello")
	require.NoError(t, err)
	assert.Contains(t, output, "Published message to fixtures.publish")
	mockClient.AssertExpectations(t)
}

func TestNATSSubscribeCommandUsesMockClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Subscribe", mock.Anything, "fixtures.subscribe", 1, mock.Anything).Run(func(args mock.Arguments) {
		handler := args.Get(3).(func(*nats.Msg) error)
		_ = handler(&nats.Msg{Subject: "fixtures.subscribe", Data: []byte("payload")})
	}).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	output, err := executeCommand(t, "nats", "subscribe", "--subject", "fixtures.subscribe", "--count", "1")
	require.NoError(t, err)
	assert.Contains(t, output, "[fixtures.subscribe] payload")
	mockClient.AssertExpectations(t)
}

func TestRollbackCommandUsesMockRunClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockRunClient{}
	mockClient.On("RollbackTraffic", mock.Anything, "valid-gcp-project", "us-east1", "api-service", "api-service-0002").Return(nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newRunClient = func(context.Context) (runClient, error) { return mockClient, nil }

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"rollback",
		"--env", "dev",
		"--service", "api-service",
		"--revision", "api-service-0002",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Rolled back api-service in dev to revision api-service-0002")
	mockClient.AssertExpectations(t)
}

func TestRollbackCommandRequiresFlags(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t, "rollback")
	require.Error(t, err)
	assert.ErrorContains(t, err, "--env, --service, and --revision are required")
}

func TestHelperFunctions(t *testing.T) {
	resetCLIState(t)

	plan := &deploy.DeploymentPlan{
		Services: []*types.DeploymentService{{Kind: types.ServiceKindCloudRun, Name: "api"}},
		Resources: []*types.DeploymentResource{
			{Kind: types.ResourceKindRedis, Name: "cache"},
		},
	}
	assert.Equal(t, []string{"cloud-run/api", "redis/cache"}, deployTargetNames(plan))

	err := writeDeployResult(&bytes.Buffer{}, "cloud-run", "api", &gcp.BuildResult{
		ID:     "build-1",
		Status: "SUCCESS",
		LogURL: "https://logs",
	}, false)
	require.NoError(t, err)

	err = writeDeployResult(&bytes.Buffer{}, "cloud-run", "api", &gcp.BuildResult{
		Operation: "operations/build-1",
	}, true)
	require.NoError(t, err)
}

func TestWriteFileCreatesNestedDirectories(t *testing.T) {
	resetCLIState(t)

	root := t.TempDir()
	target := filepath.Join(root, "nested", "deeper", "output.txt")

	require.NoError(t, writeFile(target, "payload"))
	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(contents))
}
