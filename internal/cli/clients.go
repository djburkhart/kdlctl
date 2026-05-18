package cli

import (
	"context"

	"github.com/nats-io/nats.go"

	"github.com/djburkhart/kdlctl/internal/gcp"
	kdlnats "github.com/djburkhart/kdlctl/internal/nats"
)

type natsClient interface {
	Close()
	Publish(ctx context.Context, subject string, payload []byte) error
	Subscribe(ctx context.Context, subject string, count int, handler func(*nats.Msg) error) error
}

type cloudBuildClient interface {
	Close() error
	GetBuildStatus(ctx context.Context, projectID, buildID string) (*gcp.BuildResult, error)
	SubmitCloudRunBuild(ctx context.Context, request gcp.BuildRequest, wait bool) (*gcp.BuildResult, error)
	SubmitManagedResourceBuild(ctx context.Context, request gcp.ResourceBuildRequest, wait bool) (*gcp.BuildResult, error)
}

type runClient interface {
	Close() error
	RollbackTraffic(ctx context.Context, projectID, region, serviceName, revision string) error
}

var newNATSClient = func(url string) (natsClient, error) {
	return kdlnats.NewClient(url)
}

var newCloudBuildClient = func(ctx context.Context) (cloudBuildClient, error) {
	return gcp.NewCloudBuildClient(ctx)
}

var newRunClient = func(ctx context.Context) (runClient, error) {
	return gcp.NewRunClient(ctx)
}
