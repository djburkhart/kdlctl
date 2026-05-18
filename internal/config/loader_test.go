package config_test

import (
	"path/filepath"
	"testing"

	"github.com/djburkhart/kdlctl/internal/config"
)

func TestLoadFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "examples", "deploy.kdl")
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if cfg.ProjectID != "demo-gcp-project" {
		t.Fatalf("ProjectID = %q, want demo-gcp-project", cfg.ProjectID)
	}

	env, err := config.ResolveEnvironment(cfg, "staging")
	if err != nil {
		t.Fatalf("ResolveEnvironment() error = %v", err)
	}

	service, ok := env.CloudRunServices["api-service"]
	if !ok {
		t.Fatalf("staging environment missing api-service")
	}

	if service.Image != "us-central1-docker.pkg.dev/demo-gcp-project/apps/api-service:staging" {
		t.Fatalf("service.Image = %q", service.Image)
	}

	if service.Concurrency != 80 {
		t.Fatalf("service.Concurrency = %d, want 80", service.Concurrency)
	}

	grpcServer, ok := env.GRPCServers["payments-grpc"]
	if !ok {
		t.Fatalf("staging environment missing payments-grpc grpc-server")
	}

	if grpcServer.Port != 8443 {
		t.Fatalf("grpcServer.Port = %d, want 8443", grpcServer.Port)
	}

	caddyServer, ok := env.CaddyServers["edge-caddy"]
	if !ok {
		t.Fatalf("staging environment missing edge-caddy caddy-server")
	}

	if caddyServer.Image != "us-central1-docker.pkg.dev/demo-gcp-project/apps/caddy-edge:staging" {
		t.Fatalf("caddyServer.Image = %q", caddyServer.Image)
	}

	cloudSQL, ok := env.CloudSQL["primary-db"]
	if !ok {
		t.Fatalf("staging environment missing primary-db cloud-sql")
	}

	if cloudSQL.AvailabilityType != "REGIONAL" {
		t.Fatalf("cloudSQL.AvailabilityType = %q, want REGIONAL", cloudSQL.AvailabilityType)
	}

	redis, ok := env.Redis["sessions-cache"]
	if !ok {
		t.Fatalf("staging environment missing sessions-cache redis")
	}

	if redis.MemorySizeGB != 2 {
		t.Fatalf("redis.MemorySizeGB = %d, want 2", redis.MemorySizeGB)
	}

	topic, ok := env.PubSubTopics["app-events"]
	if !ok {
		t.Fatalf("staging environment missing app-events pubsub-topic")
	}

	if topic.Labels["env"] != "prod" {
		t.Fatalf("topic.Labels[env] = %q, want prod", topic.Labels["env"])
	}

	bucket, ok := env.LoggingBuckets["application-logs"]
	if !ok {
		t.Fatalf("staging environment missing application-logs logging-bucket")
	}

	if bucket.RetentionDays != 30 {
		t.Fatalf("bucket.RetentionDays = %d, want 30", bucket.RetentionDays)
	}

	sink, ok := env.LoggingSinks["error-export"]
	if !ok {
		t.Fatalf("staging environment missing error-export logging-sink")
	}

	if sink.Filter != "severity>=WARNING" {
		t.Fatalf("sink.Filter = %q, want severity>=WARNING", sink.Filter)
	}
}
