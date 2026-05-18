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
	"github.com/googleapis/gax-go/v2"
	longrunningpb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/djburkhart/kdlctl/pkg/types"
)

type BuildRequest struct {
	ProjectID   string
	Region      string
	Environment string
	Service     *types.DeploymentService
}

type ResourceBuildRequest struct {
	ProjectID   string
	Region      string
	Environment string
	Resource    *types.DeploymentResource
}

type BuildResult struct {
	ID        string
	Operation string
	Status    string
	LogURL    string
}

type CloudBuildClient struct {
	client cloudBuildAPI
}

type cloudBuildAPI interface {
	Close() error
	CreateBuild(ctx context.Context, req *cloudbuildpb.CreateBuildRequest, opts ...gax.CallOption) (*longrunningpb.Operation, error)
	GetBuild(ctx context.Context, req *cloudbuildpb.GetBuildRequest, opts ...gax.CallOption) (*cloudbuildpb.Build, error)
}

var newCloudBuildAPI = func(ctx context.Context) (cloudBuildAPI, error) {
	return cloudbuild.NewClient(ctx)
}

func NewCloudBuildClient(ctx context.Context) (*CloudBuildClient, error) {
	client, err := newCloudBuildAPI(ctx)
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
		Steps: serviceBuildSteps(request),
		Tags:  []string{"kdlctl", request.Environment, request.Service.Name},
		Substitutions: map[string]string{
			"_KDLCTL_ENV":     request.Environment,
			"_KDLCTL_SERVICE": request.Service.Name,
			"_KDLCTL_KIND":    string(request.Service.Kind),
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

func (c *CloudBuildClient) SubmitManagedResourceBuild(ctx context.Context, request ResourceBuildRequest, wait bool) (*BuildResult, error) {
	build := &cloudbuildpb.Build{
		Steps: resourceBuildSteps(request),
		Tags:  []string{"kdlctl", request.Environment, request.Resource.Name},
		Substitutions: map[string]string{
			"_KDLCTL_ENV":      request.Environment,
			"_KDLCTL_RESOURCE": request.Resource.Name,
			"_KDLCTL_KIND":     string(request.Resource.Kind),
		},
		Timeout: durationpb.New(20 * time.Minute),
	}

	op, err := c.client.CreateBuild(ctx, &cloudbuildpb.CreateBuildRequest{
		ProjectId: request.ProjectID,
		Build:     build,
	})
	if err != nil {
		return nil, fmt.Errorf("submit managed resource build: %w", err)
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
		"--port", strconv.Itoa(service.Port),
		"--concurrency", strconv.Itoa(service.Concurrency),
		"--min-instances", strconv.Itoa(service.MinInstances),
		"--max-instances", strconv.Itoa(service.MaxInstances),
		"--quiet",
	}

	if service.Ingress != "" {
		args = append(args, "--ingress", service.Ingress)
	}

	if service.UseHTTP2 {
		args = append(args, "--use-http2")
	}

	if service.AllowUnauthenticated {
		args = append(args, "--allow-unauthenticated")
	}

	if service.VPCConnector != "" {
		args = append(args, "--vpc-connector", service.VPCConnector)
	}

	if service.VPCEgress != "" {
		args = append(args, "--vpc-egress", service.VPCEgress)
	}

	if cloudSQLInstances := formatStringList(service.CloudSQLInstances); cloudSQLInstances != "" {
		args = append(args, "--add-cloudsql-instances", cloudSQLInstances)
	}

	if labels := formatResourceLabels(service.Labels); labels != "" {
		args = append(args, "--labels", labels)
	}

	if envVars := formatEnvVars(service.Env); envVars != "" {
		args = append(args, "--set-env-vars", envVars)
	}

	if secretVars := formatSecretVars(service.Env); secretVars != "" {
		args = append(args, "--set-secrets", secretVars)
	}

	return args
}

func serviceBuildSteps(request BuildRequest) []*cloudbuildpb.BuildStep {
	return []*cloudbuildpb.BuildStep{
		{
			Name:       "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Entrypoint: "gcloud",
			Args:       buildDeployArgs(request),
		},
	}
}

func resourceBuildSteps(request ResourceBuildRequest) []*cloudbuildpb.BuildStep {
	return []*cloudbuildpb.BuildStep{
		{
			Name:       "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Entrypoint: "bash",
			Args:       []string{"-ceu", buildResourceScript(request)},
		},
	}
}

func buildResourceScript(request ResourceBuildRequest) string {
	resource := request.Resource
	switch resource.Kind {
	case types.ResourceKindCloudSQL:
		return fmt.Sprintf(`if gcloud sql instances describe %s >/dev/null 2>&1; then
  gcloud sql instances patch %s --tier=%s --storage-size=%d --availability-type=%s --quiet
else
  gcloud sql instances create %s --database-version=%s --tier=%s --region=%s --storage-size=%d --availability-type=%s --quiet
fi`,
			quoteShell(resource.Name),
			quoteShell(resource.Name),
			quoteShell(resource.Tier),
			resource.StorageSizeGB,
			quoteShell(resource.AvailabilityType),
			quoteShell(resource.Name),
			quoteShell(resource.DatabaseVersion),
			quoteShell(resource.Tier),
			quoteShell(request.Region),
			resource.StorageSizeGB,
			quoteShell(resource.AvailabilityType),
		)
	case types.ResourceKindRedis:
		return fmt.Sprintf(`if gcloud redis instances describe %s --region=%s >/dev/null 2>&1; then
  gcloud redis instances update %s --region=%s --size=%d --redis-version=%s --quiet
else
  gcloud redis instances create %s --region=%s --tier=%s --size=%d --redis-version=%s --quiet
fi`,
			quoteShell(resource.Name),
			quoteShell(request.Region),
			quoteShell(resource.Name),
			quoteShell(request.Region),
			resource.MemorySizeGB,
			quoteShell(resource.RedisVersion),
			quoteShell(resource.Name),
			quoteShell(request.Region),
			quoteShell(resource.Tier),
			resource.MemorySizeGB,
			quoteShell(resource.RedisVersion),
		)
	case types.ResourceKindPubSubTopic:
		labels := formatResourceLabels(resource.Labels)
		createArgs := ""
		updateArgs := ""
		if labels != "" {
			createArgs += fmt.Sprintf(" --labels=%s", quoteShell(labels))
			updateArgs += fmt.Sprintf(" --update-labels=%s", quoteShell(labels))
		}
		if resource.MessageRetentionDuration != "" {
			createArgs += fmt.Sprintf(" --message-retention-duration=%s", quoteShell(resource.MessageRetentionDuration))
			updateArgs += fmt.Sprintf(" --message-retention-duration=%s", quoteShell(resource.MessageRetentionDuration))
		}
		return fmt.Sprintf(`if gcloud pubsub topics describe %s >/dev/null 2>&1; then
  gcloud pubsub topics update %s%s --quiet
else
  gcloud pubsub topics create %s%s --quiet
fi`,
			quoteShell(resource.Name),
			quoteShell(resource.Name),
			updateArgs,
			quoteShell(resource.Name),
			createArgs,
		)
	case types.ResourceKindLoggingBucket:
		descriptionArg := ""
		if resource.Description != "" {
			descriptionArg = fmt.Sprintf(" --description=%s", quoteShell(resource.Description))
		}
		return fmt.Sprintf(`if gcloud logging buckets describe %s --location=%s >/dev/null 2>&1; then
  gcloud logging buckets update %s --location=%s --retention-days=%d%s --quiet
else
  gcloud logging buckets create %s --location=%s --retention-days=%d%s --quiet
fi`,
			quoteShell(resource.Name),
			quoteShell(resource.Location),
			quoteShell(resource.Name),
			quoteShell(resource.Location),
			resource.RetentionDays,
			descriptionArg,
			quoteShell(resource.Name),
			quoteShell(resource.Location),
			resource.RetentionDays,
			descriptionArg,
		)
	case types.ResourceKindLoggingSink:
		filterArg := ""
		if resource.Filter != "" {
			filterArg = fmt.Sprintf(" --log-filter=%s", quoteShell(resource.Filter))
		}
		descriptionArg := ""
		if resource.Description != "" {
			descriptionArg = fmt.Sprintf(" --description=%s", quoteShell(resource.Description))
		}
		identityArg := ""
		if resource.UniqueWriterIdentity {
			identityArg = " --unique-writer-identity"
		}
		return fmt.Sprintf(`if gcloud logging sinks describe %s >/dev/null 2>&1; then
  gcloud logging sinks update %s --destination=%s%s%s --quiet
else
  gcloud logging sinks create %s %s%s%s%s --quiet
fi`,
			quoteShell(resource.Name),
			quoteShell(resource.Name),
			quoteShell(resource.Destination),
			filterArg,
			descriptionArg,
			quoteShell(resource.Name),
			quoteShell(resource.Destination),
			filterArg,
			descriptionArg,
			identityArg,
		)
	default:
		return fmt.Sprintf("echo %s && exit 1", quoteShell("unsupported resource kind: "+string(resource.Kind)))
	}
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

func formatResourceLabels(values map[string]string) string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	cloned := append([]string(nil), values...)
	sort.Strings(cloned)
	return strings.Join(cloned, ",")
}

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
