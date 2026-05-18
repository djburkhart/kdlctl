# kdlctl

`kdlctl` is a Go CLI for deploying apps and microservices to Google Cloud from a KDL-based deployment file. The initial release focuses on Cloud Run deployments through Cloud Build, with built-in NATS utilities and GitHub integration hooks for GitOps workflows.

## Features

- `deploy.kdl` as the source of truth for project, environment, service, and NATS configuration
- Cobra-based CLI with `init`, `validate`, `plan`, `deploy`, `status`, `rollback`, and `nats` commands
- Direct Cloud Build submission for Cloud Run deploys using an already-built Artifact Registry image
- Optional NATS event publishing for async orchestration
- Cloud Run rollback support via the Cloud Run Admin API

## Project layout

```text
kdlctl/
├── cmd/kdlctl
├── internal/cli
├── internal/config
├── internal/deploy
├── internal/gcp
├── internal/github
├── internal/nats
├── internal/templates
├── pkg/types
├── examples/deploy.kdl
├── cloudbuild.yaml
└── README.md
```

## Getting started

1. Enable the Cloud Build and Cloud Run APIs in your GCP project.
2. Authenticate with Application Default Credentials:
   `gcloud auth application-default login`
3. Make sure your Cloud Run image already exists in Artifact Registry.
4. Create a starter config:
   `go run ./cmd/kdlctl init`

## Example config

See `examples/deploy.kdl` for a complete sample. A minimal Cloud Run service looks like this:

```kdl
project "my-gcp-project" region="us-central1" {
    environment "prod" {
        cloud-run "api-service" {
            image "us-central1-docker.pkg.dev/my-gcp-project/apps/api-service:latest"
            cpu 1
            memory "512Mi"
            min-instances 1
            max-instances 20
            concurrency 80
        }
    }
}
```

## Commands

```bash
go run ./cmd/kdlctl init
go run ./cmd/kdlctl validate --env prod
go run ./cmd/kdlctl plan --env prod
go run ./cmd/kdlctl deploy --env prod
go run ./cmd/kdlctl deploy --env prod --service api-service --async
go run ./cmd/kdlctl deploy --env prod --via-nats
go run ./cmd/kdlctl status --build BUILD_ID
go run ./cmd/kdlctl rollback --env prod --service api-service --revision api-service-00012-abc
go run ./cmd/kdlctl nats publish --subject deploy.requested '{"env":"prod"}'
go run ./cmd/kdlctl nats subscribe --subject deploy.status.prod.>
```

## Environment variables

- `KDLCTL_FILE`
- `KDLCTL_PROJECT_ID`
- `KDLCTL_REGION`
- `KDLCTL_NATS_URL`
- `KDLCTL_GITHUB_TOKEN`

## Notes

- `deploy` submits a Cloud Build job that runs `gcloud run deploy` against the image specified in `deploy.kdl`.
- `status --env <env>` listens on `deploy.status.<env>.>` over NATS.
- Environment inheritance is supported with `extends="<base-env>"`.
