package gcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	cloudbuildpb "cloud.google.com/go/cloudbuild/apiv1/v2/cloudbuildpb"
	"github.com/googleapis/gax-go/v2"
	longrunningpb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djburkhart/kdlctl/pkg/types"
)

type fakeCloudBuildAPI struct {
	createBuildFn func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error)
	getBuildFn    func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error)
	closeFn       func() error
}

func (f *fakeCloudBuildAPI) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeCloudBuildAPI) CreateBuild(ctx context.Context, req *cloudbuildpb.CreateBuildRequest, opts ...gax.CallOption) (*longrunningpb.Operation, error) {
	return f.createBuildFn(ctx, req, opts...)
}

func (f *fakeCloudBuildAPI) GetBuild(ctx context.Context, req *cloudbuildpb.GetBuildRequest, opts ...gax.CallOption) (*cloudbuildpb.Build, error) {
	return f.getBuildFn(ctx, req, opts...)
}

func TestBuildDeployArgsIncludesAdvancedCloudRunFlags(t *testing.T) {
	t.Parallel()

	args := buildDeployArgs(BuildRequest{
		ProjectID:   "fixture-gcp-project",
		Region:      "us-central1",
		Environment: "production",
		Service: &types.DeploymentService{
			Kind:                 types.ServiceKindCloudRun,
			Name:                 "api-gateway",
			Image:                "image:latest",
			CPU:                  2,
			Memory:               "1Gi",
			MinInstances:         2,
			MaxInstances:         50,
			Concurrency:          80,
			Port:                 8080,
			Ingress:              "all",
			UseHTTP2:             true,
			AllowUnauthenticated: true,
			VPCConnector:         "serverless-connector",
			VPCEgress:            "private-ranges-only",
			CloudSQLInstances:    []string{"b", "a"},
			Labels:               map[string]string{"tier": "edge", "env": "prod"},
			Env: map[string]types.EnvVar{
				"LOG_LEVEL": {Value: "info"},
				"DB_DSN":    {Secret: "db-dsn"},
			},
		},
	})

	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "--use-http2")
	assert.Contains(t, joined, "--allow-unauthenticated")
	assert.Contains(t, joined, "--vpc-connector serverless-connector")
	assert.Contains(t, joined, "--vpc-egress private-ranges-only")
	assert.Contains(t, joined, "--add-cloudsql-instances a,b")
	assert.Contains(t, joined, "--labels env=prod,tier=edge")
	assert.Contains(t, joined, "--set-env-vars LOG_LEVEL=info")
	assert.Contains(t, joined, "--set-secrets DB_DSN=db-dsn:latest")
}

func TestBuildResourceScriptSupportsAllManagedKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  ResourceBuildRequest
		contains string
	}{
		{
			name: "cloud sql",
			request: ResourceBuildRequest{
				Region: "us-central1",
				Resource: &types.DeploymentResource{
					Kind:             types.ResourceKindCloudSQL,
					Name:             "primary-db",
					DatabaseVersion:  "POSTGRES_16",
					Tier:             "db-custom-2-7680",
					AvailabilityType: "REGIONAL",
					StorageSizeGB:    100,
				},
			},
			contains: "gcloud sql instances create 'primary-db'",
		},
		{
			name: "redis",
			request: ResourceBuildRequest{
				Region: "us-central1",
				Resource: &types.DeploymentResource{
					Kind:         types.ResourceKindRedis,
					Name:         "cache",
					Tier:         "STANDARD_HA",
					MemorySizeGB: 2,
					RedisVersion: "REDIS_7_0",
				},
			},
			contains: "gcloud redis instances create 'cache'",
		},
		{
			name: "pubsub",
			request: ResourceBuildRequest{
				Resource: &types.DeploymentResource{
					Kind:                     types.ResourceKindPubSubTopic,
					Name:                     "events",
					MessageRetentionDuration: "604800s",
					Labels:                   map[string]string{"env": "prod"},
				},
			},
			contains: "gcloud pubsub topics create 'events'",
		},
		{
			name: "logging bucket",
			request: ResourceBuildRequest{
				Resource: &types.DeploymentResource{
					Kind:          types.ResourceKindLoggingBucket,
					Name:          "logs",
					Location:      "global",
					RetentionDays: 30,
					Description:   "bucket",
				},
			},
			contains: "gcloud logging buckets create 'logs'",
		},
		{
			name: "logging sink",
			request: ResourceBuildRequest{
				Resource: &types.DeploymentResource{
					Kind:                 types.ResourceKindLoggingSink,
					Name:                 "errors",
					Destination:          "logging.googleapis.com/projects/p/locations/global/buckets/logs",
					Filter:               "severity>=ERROR",
					Description:          "sink",
					UniqueWriterIdentity: true,
				},
			},
			contains: "gcloud logging sinks create 'errors'",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, buildResourceScript(tt.request), tt.contains)
		})
	}
}

func TestBuildResourceScriptRejectsUnsupportedKinds(t *testing.T) {
	t.Parallel()

	script := buildResourceScript(ResourceBuildRequest{
		Resource: &types.DeploymentResource{
			Kind: "custom",
			Name: "thing",
		},
	})

	assert.Contains(t, script, "unsupported resource kind: custom")
}

func TestBuildIDFromOperationReadsMetadata(t *testing.T) {
	t.Parallel()

	metadata, err := anypb.New(&cloudbuildpb.BuildOperationMetadata{
		Build: &cloudbuildpb.Build{Id: "build-123"},
	})
	require.NoError(t, err)

	op := &longrunningpb.Operation{Metadata: metadata}
	assert.Equal(t, "build-123", buildIDFromOperation(op))
	assert.Empty(t, buildIDFromOperation(&longrunningpb.Operation{}))
}

func TestFormattingHelpersSortValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "A=1,B=2", formatEnvVars(map[string]types.EnvVar{
		"B": {Value: "2"},
		"A": {Value: "1"},
		"C": {Secret: "ignored"},
	}))
	assert.Equal(t, "DB_DSN=db:latest,TOKEN=token:latest", formatSecretVars(map[string]types.EnvVar{
		"TOKEN":  {Secret: "token"},
		"DB_DSN": {Secret: "db"},
	}))
	assert.Equal(t, "env=prod,tier=edge", formatResourceLabels(map[string]string{"tier": "edge", "env": "prod"}))
	assert.Equal(t, "a,b,c", formatStringList([]string{"c", "a", "b"}))
	assert.Equal(t, "'it'\\''s'", quoteShell("it's"))
}

func TestBuildStepsWrapExpectedCommands(t *testing.T) {
	t.Parallel()

	serviceSteps := serviceBuildSteps(BuildRequest{
		Region: "us-central1",
		Service: &types.DeploymentService{
			Name:         "api",
			Image:        "image:latest",
			CPU:          1,
			Memory:       "512Mi",
			MaxInstances: 1,
			Concurrency:  1,
			Port:         8080,
		},
	})
	require.Len(t, serviceSteps, 1)
	assert.Equal(t, "gcloud", serviceSteps[0].Entrypoint)
	assert.Contains(t, serviceSteps[0].Args, "deploy")

	resourceSteps := resourceBuildSteps(ResourceBuildRequest{
		Region: "us-central1",
		Resource: &types.DeploymentResource{
			Kind:         types.ResourceKindRedis,
			Name:         "cache",
			Tier:         "STANDARD_HA",
			MemorySizeGB: 2,
			RedisVersion: "REDIS_7_0",
		},
	})
	require.Len(t, resourceSteps, 1)
	assert.Equal(t, "bash", resourceSteps[0].Entrypoint)
	assert.Equal(t, "-ceu", resourceSteps[0].Args[0])
}

func TestCloudBuildClientSubmitCloudRunBuild(t *testing.T) {
	t.Parallel()

	metadata, err := anypb.New(&cloudbuildpb.BuildOperationMetadata{
		Build: &cloudbuildpb.Build{Id: "build-123"},
	})
	require.NoError(t, err)

	client := &CloudBuildClient{
		client: &fakeCloudBuildAPI{
			createBuildFn: func(_ context.Context, req *cloudbuildpb.CreateBuildRequest, _ ...gax.CallOption) (*longrunningpb.Operation, error) {
				require.Equal(t, "project", req.ProjectId)
				require.Len(t, req.Build.Steps, 1)
				return &longrunningpb.Operation{Name: "operations/build-123", Metadata: metadata}, nil
			},
			getBuildFn: func(_ context.Context, req *cloudbuildpb.GetBuildRequest, _ ...gax.CallOption) (*cloudbuildpb.Build, error) {
				require.Equal(t, "build-123", req.Id)
				return &cloudbuildpb.Build{Id: "build-123", Status: cloudbuildpb.Build_SUCCESS, LogUrl: "https://logs"}, nil
			},
		},
	}

	result, err := client.SubmitCloudRunBuild(context.Background(), BuildRequest{
		ProjectID:   "project",
		Region:      "us-central1",
		Environment: "prod",
		Service: &types.DeploymentService{
			Kind:         types.ServiceKindCloudRun,
			Name:         "api",
			Image:        "image:latest",
			CPU:          1,
			Memory:       "512Mi",
			MaxInstances: 1,
			Concurrency:  1,
			Port:         8080,
		},
	}, true)
	require.NoError(t, err)
	assert.Equal(t, "build-123", result.ID)
	assert.Equal(t, "SUCCESS", result.Status)
}

func TestCloudBuildClientSubmitManagedResourceBuildNoWait(t *testing.T) {
	t.Parallel()

	metadata, err := anypb.New(&cloudbuildpb.BuildOperationMetadata{
		Build: &cloudbuildpb.Build{Id: "build-456"},
	})
	require.NoError(t, err)

	client := &CloudBuildClient{
		client: &fakeCloudBuildAPI{
			createBuildFn: func(_ context.Context, req *cloudbuildpb.CreateBuildRequest, _ ...gax.CallOption) (*longrunningpb.Operation, error) {
				require.Equal(t, "project", req.ProjectId)
				return &longrunningpb.Operation{Name: "operations/build-456", Metadata: metadata}, nil
			},
			getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
				t.Fatal("unexpected poll")
				return nil, nil
			},
		},
	}

	result, err := client.SubmitManagedResourceBuild(context.Background(), ResourceBuildRequest{
		ProjectID:   "project",
		Region:      "us-central1",
		Environment: "prod",
		Resource: &types.DeploymentResource{
			Kind:         types.ResourceKindRedis,
			Name:         "cache",
			Tier:         "STANDARD_HA",
			MemorySizeGB: 2,
			RedisVersion: "REDIS_7_0",
		},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "QUEUED", result.Status)
	assert.Equal(t, "build-456", result.ID)
}

func TestCloudBuildClientErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("submit service build", func(t *testing.T) {
		t.Parallel()
		client := &CloudBuildClient{
			client: &fakeCloudBuildAPI{
				createBuildFn: func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error) {
					return nil, errors.New("boom")
				},
				getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
					t.Fatal("unexpected poll")
					return nil, nil
				},
			},
		}
		_, err := client.SubmitCloudRunBuild(context.Background(), BuildRequest{
			ProjectID:   "project",
			Region:      "us-central1",
			Environment: "prod",
			Service:     &types.DeploymentService{Name: "api", Image: "image", CPU: 1, Memory: "512Mi", Concurrency: 1, MaxInstances: 1, Port: 8080},
		}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "submit cloud build")
	})

	t.Run("missing build id", func(t *testing.T) {
		t.Parallel()
		client := &CloudBuildClient{
			client: &fakeCloudBuildAPI{
				createBuildFn: func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error) {
					return &longrunningpb.Operation{Name: "operations/build"}, nil
				},
				getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
					t.Fatal("unexpected poll")
					return nil, nil
				},
			},
		}
		_, err := client.SubmitManagedResourceBuild(context.Background(), ResourceBuildRequest{
			ProjectID:   "project",
			Region:      "us-central1",
			Environment: "prod",
			Resource:    &types.DeploymentResource{Kind: types.ResourceKindRedis, Name: "cache"},
		}, true)
		require.Error(t, err)
		assert.ErrorContains(t, err, "did not return a build ID")
	})

	t.Run("get build status", func(t *testing.T) {
		t.Parallel()
		client := &CloudBuildClient{
			client: &fakeCloudBuildAPI{
				createBuildFn: func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error) {
					t.Fatal("unexpected create")
					return nil, nil
				},
				getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
					return nil, errors.New("boom")
				},
			},
		}
		_, err := client.GetBuildStatus(context.Background(), "project", "build")
		require.Error(t, err)
		assert.ErrorContains(t, err, "get build status")
	})
}

func TestCloudBuildClientWaitForBuildErrors(t *testing.T) {
	t.Parallel()

	t.Run("poll error", func(t *testing.T) {
		t.Parallel()
		client := &CloudBuildClient{
			client: &fakeCloudBuildAPI{
				createBuildFn: func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error) {
					t.Fatal("unexpected create")
					return nil, nil
				},
				getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
					return nil, errors.New("boom")
				},
			},
		}
		_, err := client.waitForBuild(context.Background(), "project", "build", "operations/build")
		require.Error(t, err)
		assert.ErrorContains(t, err, "poll build build")
	})

	t.Run("context done", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := &CloudBuildClient{
			client: &fakeCloudBuildAPI{
				createBuildFn: func(context.Context, *cloudbuildpb.CreateBuildRequest, ...gax.CallOption) (*longrunningpb.Operation, error) {
					t.Fatal("unexpected create")
					return nil, nil
				},
				getBuildFn: func(context.Context, *cloudbuildpb.GetBuildRequest, ...gax.CallOption) (*cloudbuildpb.Build, error) {
					return &cloudbuildpb.Build{Id: "build", Status: cloudbuildpb.Build_WORKING}, nil
				},
			},
		}
		_, err := client.waitForBuild(ctx, "project", "build", "operations/build")
		require.ErrorIs(t, err, context.Canceled)
	})
}
