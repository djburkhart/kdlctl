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
	"github.com/spf13/cobra"
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

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

type failAfterWriter struct {
	writes int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, errors.New("boom")
	}
	return len(p), nil
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
	originalExitFunc := exitFunc
	originalStderrWriter := stderrWriter
	originalRootCommandFactory := rootCommandFactory
	viper.Reset()
	t.Cleanup(func() {
		newNATSClient = originalNATSFactory
		newCloudBuildClient = originalCloudBuildFactory
		newRunClient = originalRunFactory
		exitFunc = originalExitFunc
		stderrWriter = originalStderrWriter
		rootCommandFactory = originalRootCommandFactory
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

func executeSpecificCommand(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()

	cmd.SetOut(errWriter{})
	cmd.SetErr(errWriter{})
	cmd.SetArgs(args)
	return cmd.Execute()
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

func TestWriteFileReturnsErrorWhenParentIsAFile(t *testing.T) {
	resetCLIState(t)

	root := t.TempDir()
	parentFile := filepath.Join(root, "parent")
	require.NoError(t, os.WriteFile(parentFile, []byte("not-a-directory"), 0o644))

	err := writeFile(filepath.Join(parentFile, "child.txt"), "payload")
	require.Error(t, err)
	assert.ErrorContains(t, err, "create directory")
}

func TestWriteFileReturnsErrorWhenTargetIsDirectory(t *testing.T) {
	resetCLIState(t)

	root := t.TempDir()
	targetDir := filepath.Join(root, "output")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	err := writeFile(targetDir, "payload")
	require.Error(t, err)
	assert.ErrorContains(t, err, "write")
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

func TestInitCommandForceOverwritesStarterFiles(t *testing.T) {
	resetCLIState(t)

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	require.NoError(t, os.WriteFile("deploy.kdl", []byte("old"), 0o644))
	require.NoError(t, os.WriteFile("cloudbuild.yaml", []byte("old"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join("examples"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("examples", "deploy.kdl"), []byte("old"), 0o644))

	_, err = executeCommand(t, "init", "--force")
	require.NoError(t, err)

	contents, err := os.ReadFile("deploy.kdl")
	require.NoError(t, err)
	assert.NotEqual(t, "old", string(contents))
}

func TestInitCommandFailsWithoutForceWhenStarterFilesExist(t *testing.T) {
	resetCLIState(t)

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	require.NoError(t, os.WriteFile("deploy.kdl", []byte("old"), 0o644))

	_, err = executeCommand(t, "init")
	require.Error(t, err)
	assert.ErrorContains(t, err, "already exists")
}

func TestInitCommandReturnsWriteError(t *testing.T) {
	resetCLIState(t)

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	require.NoError(t, os.WriteFile("examples", []byte("not-a-directory"), 0o644))

	_, err = executeCommand(t, "init")
	require.Error(t, err)
	assert.ErrorContains(t, err, "create directory")
}

func TestInitCommandOutputWriteError(t *testing.T) {
	resetCLIState(t)

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	err = executeSpecificCommand(t, newInitCmd())
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestExecute(t *testing.T) {
	resetCLIState(t)

	t.Run("success", func(t *testing.T) {
		called := false
		rootCommandFactory = func() *cobra.Command {
			return &cobra.Command{
				RunE: func(*cobra.Command, []string) error {
					called = true
					return nil
				},
			}
		}
		exitFunc = func(int) { t.Fatal("unexpected exit") }

		Execute()
		assert.True(t, called)
	})

	t.Run("failure", func(t *testing.T) {
		var stderr bytes.Buffer
		var exitCode int
		stderrWriter = &stderr
		rootCommandFactory = func() *cobra.Command {
			return &cobra.Command{
				RunE: func(*cobra.Command, []string) error {
					return errors.New("boom")
				},
			}
		}
		exitFunc = func(code int) { exitCode = code }

		Execute()
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, stderr.String(), "boom")
	})
}

func TestMustBindFlagPanicsOnInvalidFlag(t *testing.T) {
	resetCLIState(t)

	assert.Panics(t, func() {
		mustBindFlag("broken", nil)
	})
}

func TestWriteJSON(t *testing.T) {
	resetCLIState(t)

	var stdout bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)

	require.NoError(t, writeJSON(cmd, map[string]string{"hello": "world"}))
	assert.JSONEq(t, `{"hello":"world"}`, stdout.String())
}

func TestWriteJSONError(t *testing.T) {
	resetCLIState(t)

	cmd := newRootCmd()
	cmd.SetOut(errWriter{})

	err := writeJSON(cmd, map[string]string{"hello": "world"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
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

func TestDeployCommandRequiresEnvironment(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t, "deploy")
	require.Error(t, err)
	assert.ErrorContains(t, err, "--env is required")
}

func TestDeployCommandDirectUsesMockCloudBuildClient(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockCloudBuildClient{}
	mockClient.On("SubmitCloudRunBuild", mock.Anything, mock.Anything, true).Return(&gcp.BuildResult{
		ID:     "build-service",
		Status: "SUCCESS",
	}, nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"deploy",
		"--env", "dev",
		"--service", "api-service",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Deployed cloud-run/api-service with status SUCCESS")
	mockClient.AssertExpectations(t)
}

func TestLoadDeployPlan(t *testing.T) {
	resetCLIState(t)

	viper.Set("file", cliFixturePath("valid", "deploy.kdl"))

	plan, err := loadDeployPlan("dev", "api-service")
	require.NoError(t, err)
	require.Len(t, plan.Services, 1)
	assert.Equal(t, "api-service", plan.Services[0].Name)
}

func TestLoadDeployPlanErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("missing env", func(t *testing.T) {
		_, err := loadDeployPlan("", "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "--env is required")
	})

	t.Run("missing file", func(t *testing.T) {
		viper.Set("file", filepath.Join(t.TempDir(), "missing.kdl"))
		_, err := loadDeployPlan("dev", "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "read config file")
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

	t.Run("resource", func(t *testing.T) {
		mockClient := &mockCloudBuildClient{}
		mockClient.On("SubmitManagedResourceBuild", mock.Anything, mock.Anything, true).Return((*gcp.BuildResult)(nil), errors.New("boom")).Once()
		mockClient.On("Close").Return(nil).Once()
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

		err := submitDeployPlan(context.Background(), &bytes.Buffer{}, &deploy.DeploymentPlan{
			ProjectID:   "fixture-gcp-project",
			Region:      "us-central1",
			Environment: "production",
			Resources:   []*types.DeploymentResource{{Kind: types.ResourceKindRedis, Name: "cache"}},
		}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
}

func TestSubmitDeployHelpersReturnWriteErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("service", func(t *testing.T) {
		mockClient := &mockCloudBuildClient{}
		mockClient.On("SubmitCloudRunBuild", mock.Anything, mock.Anything, true).Return(&gcp.BuildResult{
			ID:     "build-service",
			Status: "SUCCESS",
		}, nil).Once()

		err := submitServiceDeploys(context.Background(), errWriter{}, mockClient, &deploy.DeploymentPlan{
			ProjectID:   "fixture-gcp-project",
			Region:      "us-central1",
			Environment: "production",
			Services:    []*types.DeploymentService{{Kind: types.ServiceKindCloudRun, Name: "api"}},
		}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})

	t.Run("resource", func(t *testing.T) {
		mockClient := &mockCloudBuildClient{}
		mockClient.On("SubmitManagedResourceBuild", mock.Anything, mock.Anything, true).Return(&gcp.BuildResult{
			ID:     "build-resource",
			Status: "SUCCESS",
		}, nil).Once()

		err := submitResourceDeploys(context.Background(), errWriter{}, mockClient, &deploy.DeploymentPlan{
			ProjectID:   "fixture-gcp-project",
			Region:      "us-central1",
			Environment: "production",
			Resources:   []*types.DeploymentResource{{Kind: types.ResourceKindRedis, Name: "cache"}},
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

func TestPlanCommandTextOutput(t *testing.T) {
	resetCLIState(t)

	output, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"plan",
		"--env", "dev",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "Project: valid-gcp-project")
	assert.Contains(t, output, "- cloud-run/api-service")
}

func TestPlanCommandWriteError(t *testing.T) {
	resetCLIState(t)
	viper.Set("file", cliFixturePath("valid", "deploy.kdl"))

	err := executeSpecificCommand(t, newPlanCmd(), "--env", "dev")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestPlanCommandJSONWriteError(t *testing.T) {
	resetCLIState(t)
	viper.Set("file", cliFixturePath("complex", "deploy.kdl"))

	err := executeSpecificCommand(t, newPlanCmd(), "--env", "production", "--json")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestPlanCommandRequiresEnvironment(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"plan",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "--env is required")
}

func TestPlanCommandUnknownService(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"plan",
		"--env", "dev",
		"--service", "missing",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, `no targets matched environment "dev"`)
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

func TestValidateCommandWriteErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("single environment", func(t *testing.T) {
		viper.Set("file", cliFixturePath("valid", "deploy.kdl"))
		err := executeSpecificCommand(t, newValidateCmd(), "--env", "dev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("whole project", func(t *testing.T) {
		viper.Set("file", cliFixturePath("complex", "deploy.kdl"))
		err := executeSpecificCommand(t, newValidateCmd())
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
}

func TestValidateCommandInvalidEnvironment(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"validate",
		"--env", "missing",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, `environment "missing" not found`)
}

func TestValidateCommandMissingFile(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", filepath.Join(t.TempDir(), "missing.kdl"),
		"validate",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "read config file")
}

func TestValidateCommandInvalidFixture(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", cliFixturePath("invalid", "deploy.kdl"),
		"validate",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, `defines "shared-name" in both cloud-run and grpc-server`)
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

func TestStatusCommandWriteErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("build output", func(t *testing.T) {
		viper.Set("file", cliFixturePath("valid", "deploy.kdl"))
		mockClient := &mockCloudBuildClient{}
		mockClient.On("GetBuildStatus", mock.Anything, "valid-gcp-project", "build-123").Return(&gcp.BuildResult{
			ID:     "build-123",
			Status: "SUCCESS",
			LogURL: "https://logs/build-123",
		}, nil).Once()
		mockClient.On("Close").Return(nil).Once()
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

		err := executeSpecificCommand(t, newStatusCmd(), "--build", "build-123")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})

	t.Run("nats output", func(t *testing.T) {
		mockClient := &mockNATSClient{}
		handlerErr := errors.New("boom")
		returnedErr := handlerErr
		mockClient.On("Subscribe", mock.Anything, "deploy.status.dev.>", 1, mock.Anything).Run(func(args mock.Arguments) {
			handler := args.Get(3).(func(*nats.Msg) error)
			err := handler(&nats.Msg{Subject: "deploy.status.dev.api", Data: []byte("ready")})
			require.Error(t, err)
			assert.ErrorContains(t, err, "boom")
			returnedErr = err
		}).Return(func(context.Context, string, int, func(*nats.Msg) error) error {
			return returnedErr
		}).Once()
		mockClient.On("Close").Return().Once()
		newNATSClient = func(string) (natsClient, error) { return mockClient, nil }
		viper.Set("nats-url", "nats://fixture:4222")

		err := executeSpecificCommand(t, newStatusCmd(), "--env", "dev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
}

func TestStatusCommandBuildErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("new client", func(t *testing.T) {
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return nil, errors.New("boom") }

		_, err := executeCommand(t,
			"--file", cliFixturePath("valid", "deploy.kdl"),
			"status",
			"--build", "build-123",
		)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("get status", func(t *testing.T) {
		mockClient := &mockCloudBuildClient{}
		mockClient.On("GetBuildStatus", mock.Anything, "valid-gcp-project", "build-123").Return((*gcp.BuildResult)(nil), errors.New("boom")).Once()
		mockClient.On("Close").Return(nil).Once()
		newCloudBuildClient = func(context.Context) (cloudBuildClient, error) { return mockClient, nil }

		_, err := executeCommand(t,
			"--file", cliFixturePath("valid", "deploy.kdl"),
			"status",
			"--build", "build-123",
		)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
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

func TestStatusCommandFollowUsesContinuousSubscription(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Subscribe", mock.Anything, "deploy.status.dev.>", 0, mock.Anything).Run(func(args mock.Arguments) {
		handler := args.Get(3).(func(*nats.Msg) error)
		_ = handler(&nats.Msg{
			Subject: "deploy.status.dev.worker",
			Data:    []byte("streaming"),
		})
	}).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }
	viper.Set("nats-url", "nats://fixture:4222")

	output, err := executeCommand(t, "status", "--env", "dev", "--follow")
	require.NoError(t, err)
	assert.Contains(t, output, "[deploy.status.dev.worker] streaming")
	mockClient.AssertExpectations(t)
}

func TestStatusCommandEnvErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("new client", func(t *testing.T) {
		newNATSClient = func(string) (natsClient, error) { return nil, errors.New("boom") }

		_, err := executeCommand(t, "status", "--env", "dev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("subscribe", func(t *testing.T) {
		mockClient := &mockNATSClient{}
		mockClient.On("Subscribe", mock.Anything, "deploy.status.dev.>", 1, mock.Anything).Return(errors.New("boom")).Once()
		mockClient.On("Close").Return().Once()
		newNATSClient = func(string) (natsClient, error) { return mockClient, nil }
		viper.Set("nats-url", "nats://fixture:4222")

		_, err := executeCommand(t, "status", "--env", "dev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		mockClient.AssertExpectations(t)
	})
}

func TestNATSCommandsReturnClientErrors(t *testing.T) {
	resetCLIState(t)

	t.Run("publish new client", func(t *testing.T) {
		newNATSClient = func(string) (natsClient, error) { return nil, errors.New("boom") }
		_, err := executeCommand(t, "nats", "publish", "--subject", "fixtures.publish", "hello")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("subscribe new client", func(t *testing.T) {
		newNATSClient = func(string) (natsClient, error) { return nil, errors.New("boom") }
		_, err := executeCommand(t, "nats", "subscribe", "--subject", "fixtures.subscribe", "--count", "1")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
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

func TestNATSPublishCommandWriteError(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Publish", mock.Anything, "fixtures.publish", []byte("hello")).Return(nil).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	err := executeSpecificCommand(t, newNATSPublishCmd(), "--subject", "fixtures.publish", "hello")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	mockClient.AssertExpectations(t)
}

func TestNATSPublishCommandReturnsPublishError(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Publish", mock.Anything, "fixtures.publish", []byte("hello")).Return(errors.New("boom")).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	_, err := executeCommand(t, "nats", "publish", "--subject", "fixtures.publish", "hello")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
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

func TestNATSSubscribeCommandHandlerWriteError(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Subscribe", mock.Anything, "fixtures.subscribe", 1, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string, _ int, handler func(*nats.Msg) error) error {
			err := handler(&nats.Msg{Subject: "fixtures.subscribe", Data: []byte("payload")})
			require.Error(t, err)
			return err
		},
	).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	err := executeSpecificCommand(t, newNATSSubscribeCmd(), "--subject", "fixtures.subscribe", "--count", "1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "write")
	mockClient.AssertExpectations(t)
}

func TestNATSSubscribeCommandReturnsSubscribeError(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockNATSClient{}
	mockClient.On("Subscribe", mock.Anything, "fixtures.subscribe", 1, mock.Anything).Return(errors.New("boom")).Once()
	mockClient.On("Close").Return().Once()
	newNATSClient = func(string) (natsClient, error) { return mockClient, nil }

	_, err := executeCommand(t, "nats", "subscribe", "--subject", "fixtures.subscribe", "--count", "1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
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

func TestRollbackCommandWriteError(t *testing.T) {
	resetCLIState(t)
	viper.Set("file", cliFixturePath("valid", "deploy.kdl"))

	mockClient := &mockRunClient{}
	mockClient.On("RollbackTraffic", mock.Anything, "valid-gcp-project", "us-east1", "api-service", "api-service-0002").Return(nil).Once()
	mockClient.On("Close").Return(nil).Once()
	newRunClient = func(context.Context) (runClient, error) { return mockClient, nil }

	err := executeSpecificCommand(t, newRollbackCmd(), "--env", "dev", "--service", "api-service", "--revision", "api-service-0002")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	mockClient.AssertExpectations(t)
}

func TestRollbackCommandReturnsClientError(t *testing.T) {
	resetCLIState(t)

	newRunClient = func(context.Context) (runClient, error) { return nil, errors.New("boom") }

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"rollback",
		"--env", "dev",
		"--service", "api-service",
		"--revision", "api-service-0002",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestRollbackCommandReturnsRollbackError(t *testing.T) {
	resetCLIState(t)

	mockClient := &mockRunClient{}
	mockClient.On("RollbackTraffic", mock.Anything, "valid-gcp-project", "us-east1", "api-service", "api-service-0002").Return(errors.New("boom")).Once()
	mockClient.On("Close").Return(nil).Once()
	newRunClient = func(context.Context) (runClient, error) { return mockClient, nil }

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"rollback",
		"--env", "dev",
		"--service", "api-service",
		"--revision", "api-service-0002",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	mockClient.AssertExpectations(t)
}

func TestRollbackCommandRequiresFlags(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t, "rollback")
	require.Error(t, err)
	assert.ErrorContains(t, err, "--env, --service, and --revision are required")
}

func TestRollbackCommandInvalidEnvironment(t *testing.T) {
	resetCLIState(t)

	_, err := executeCommand(t,
		"--file", cliFixturePath("valid", "deploy.kdl"),
		"rollback",
		"--env", "missing",
		"--service", "api-service",
		"--revision", "api-service-0002",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, `environment "missing" not found`)
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

	err = writeDeployResult(&bytes.Buffer{}, "cloud-run", "api", &gcp.BuildResult{
		Status: "SUCCESS",
	}, false)
	require.NoError(t, err)

	err = writeDeployResult(errWriter{}, "cloud-run", "api", &gcp.BuildResult{
		Status: "SUCCESS",
	}, false)
	require.Error(t, err)

	err = writeDeployResult(&failAfterWriter{}, "cloud-run", "api", &gcp.BuildResult{
		ID:     "build-1",
		Status: "SUCCESS",
	}, false)
	require.Error(t, err)
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
