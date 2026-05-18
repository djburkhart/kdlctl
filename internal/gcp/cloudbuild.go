package gcp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	cloudbuildpb "cloud.google.com/go/cloudbuild/apiv1/v2/cloudbuildpb"
	longrunningpb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/djburkhart/kdlctl/pkg/types"
)

type BuildRequest struct {
	ProjectID   string
	Region      string
	Environment string
	Service     *types.CloudRunService
}

type BuildResult struct {
	ID        string
	Operation string
	Status    string
	LogURL    string
}

type CloudBuildClient struct {
	client *cloudbuild.Client
}

func NewCloudBuildClient(ctx context.Context) (*CloudBuildClient, error) {
	client, err := cloudbuild.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud build client: %w", err)
	}

	return &CloudBuildClient{client: client}, nil
}

func (c *CloudBuildClient) Close() error {
	return c.client.Close()
}

func (c *CloudBuildClient) SubmitCloudRunBuild(ctx context.Context, request BuildRequest, wait bool) (*BuildResult, error) {
	build := &cloudbuildpb.Build{
		Steps: []*cloudbuildpb.BuildStep{
			{
				Name:       "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Entrypoint: "gcloud",
				Args:       buildDeployArgs(request),
			},
		},
		Tags: []string{"kdlctl", request.Environment, request.Service.Name},
		Substitutions: map[string]string{
			"_KDLCTL_ENV":     request.Environment,
			"_KDLCTL_SERVICE": request.Service.Name,
		},
		Timeout: durationpb.New(20 * time.Minute),
	}

	op, err := c.client.CreateBuild(ctx, &cloudbuildpb.CreateBuildRequest{
		ProjectId: request.ProjectID,
		Build:     build,
	})
	if err != nil {
		return nil, fmt.Errorf("submit cloud build: %w", err)
	}

	result := &BuildResult{
		Operation: op.GetName(),
		Status:    "QUEUED",
	}
	result.ID = buildIDFromOperation(op)

	if !wait {
		return result, nil
	}

	if result.ID == "" {
		return nil, fmt.Errorf("cloud build operation did not return a build ID")
	}

	return c.waitForBuild(ctx, request.ProjectID, result.ID, result.Operation)
}

func (c *CloudBuildClient) GetBuildStatus(ctx context.Context, projectID, buildID string) (*BuildResult, error) {
	build, err := c.client.GetBuild(ctx, &cloudbuildpb.GetBuildRequest{
		ProjectId: projectID,
		Id:        buildID,
	})
	if err != nil {
		return nil, fmt.Errorf("get build status: %w", err)
	}

	return &BuildResult{
		ID:     build.GetId(),
		Status: build.GetStatus().String(),
		LogURL: build.GetLogUrl(),
	}, nil
}

func buildDeployArgs(request BuildRequest) []string {
	service := request.Service
	args := []string{
		"run",
		"deploy",
		service.Name,
		"--image", service.Image,
		"--region", request.Region,
		"--platform", "managed",
		"--cpu", strconv.Itoa(service.CPU),
		"--memory", service.Memory,
		"--concurrency", strconv.Itoa(service.Concurrency),
		"--min-instances", strconv.Itoa(service.MinInstances),
		"--max-instances", strconv.Itoa(service.MaxInstances),
		"--quiet",
	}

	if envVars := formatEnvVars(service.Env); envVars != "" {
		args = append(args, "--set-env-vars", envVars)
	}

	if secretVars := formatSecretVars(service.Env); secretVars != "" {
		args = append(args, "--set-secrets", secretVars)
	}

	return args
}

func formatEnvVars(values map[string]types.EnvVar) string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		if value.Secret != "" || value.Value == "" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value.Value))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func (c *CloudBuildClient) waitForBuild(ctx context.Context, projectID, buildID, operationName string) (*BuildResult, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		build, err := c.client.GetBuild(ctx, &cloudbuildpb.GetBuildRequest{
			ProjectId: projectID,
			Id:        buildID,
		})
		if err != nil {
			return nil, fmt.Errorf("poll build %s: %w", buildID, err)
		}

		status := build.GetStatus().String()
		if status != "QUEUED" && status != "WORKING" && status != "PENDING" {
			return &BuildResult{
				ID:        build.GetId(),
				Operation: operationName,
				Status:    status,
				LogURL:    build.GetLogUrl(),
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func buildIDFromOperation(op *longrunningpb.Operation) string {
	if op.GetMetadata() == nil {
		return ""
	}

	var metadata cloudbuildpb.BuildOperationMetadata
	if err := op.GetMetadata().UnmarshalTo(&metadata); err != nil {
		return ""
	}

	if metadata.GetBuild() == nil {
		return ""
	}

	return metadata.GetBuild().GetId()
}

func formatSecretVars(values map[string]types.EnvVar) string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		if value.Secret == "" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s:latest", key, value.Secret))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}
