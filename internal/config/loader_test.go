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
}
