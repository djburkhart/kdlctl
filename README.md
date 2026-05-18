# kdlctl

[![CodeQL](https://github.com/djburkhart/kdlctl/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/djburkhart/kdlctl/actions/workflows/codeql.yml)
[![License: Apache-2.0](https://img.shields.io/github/license/djburkhart/kdlctl)](https://github.com/djburkhart/kdlctl/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/djburkhart/kdlctl)](https://github.com/djburkhart/kdlctl/blob/main/go.mod)
[![GitHub release](https://img.shields.io/github/v/release/djburkhart/kdlctl)](https://github.com/djburkhart/kdlctl/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/djburkhart/kdlctl)](https://goreportcard.com/report/github.com/djburkhart/kdlctl)
[![Go Reference](https://pkg.go.dev/badge/github.com/djburkhart/kdlctl.svg)](https://pkg.go.dev/github.com/djburkhart/kdlctl)

`kdlctl` is a Go CLI for deploying apps and microservices to Google Cloud from a KDL-based deployment file. The initial release focuses on Cloud Run deployments through Cloud Build, with built-in NATS utilities and GitHub integration hooks for GitOps workflows.

- Changelog: [`CHANGELOG.md`](./CHANGELOG.md)
- Releases: [GitHub Releases](https://github.com/djburkhart/kdlctl/releases)

## Features

- `deploy.kdl` as the source of truth for project, environment, Cloud Run services, gRPC servers, Caddy servers, databases, caches, messaging, logging, and NATS configuration
- Cobra-based CLI with `init`, `validate`, `plan`, `deploy`, `status`, `rollback`, and `nats` commands
- Direct Cloud Build submission for workloads and managed infrastructure, including Cloud SQL, Redis, Pub/Sub topics, and Cloud Logging resources
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

## Install

```powershell
go install github.com/djburkhart/kdlctl/cmd/kdlctl@latest
```

## Example config

See `examples/deploy.kdl` for a complete sample. `kdlctl` supports workload blocks like `cloud-run`, `grpc-server`, and `caddy-server`, plus managed infrastructure blocks such as `cloud-sql`, `redis`, `pubsub-topic`, `logging-bucket`, and `logging-sink`.

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

        grpc-server "payments-grpc" {
            image "us-central1-docker.pkg.dev/my-gcp-project/apps/payments-grpc:latest"
            port 8443
        }

        caddy-server "edge-caddy" {
            image "us-central1-docker.pkg.dev/my-gcp-project/apps/caddy-edge:latest"
            port 8080
        }

        cloud-sql "primary-db" {
            database-version "POSTGRES_16"
            tier "db-custom-1-3840"
            availability-type "REGIONAL"
            storage-gb 50
        }

        redis "sessions-cache" {
            tier "STANDARD_HA"
            memory-gb 2
            redis-version "REDIS_7_0"
        }

        pubsub-topic "app-events" {
            retention "604800s"
        }

        logging-bucket "application-logs" {
            retention-days 30
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
go run ./cmd/kdlctl deploy --env prod --service payments-grpc
go run ./cmd/kdlctl deploy --env prod --service primary-db
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
- `grpc-server` targets are deployed with Cloud Run HTTP/2 enabled so gRPC services can be reached through managed ingress.
- `caddy-server` targets are deployed as Cloud Run services as well; Cloud Run still terminates external TLS, so Caddy should listen on the configured internal port.
- `cloud-sql` provisions Cloud SQL instances; repeated deploys patch tier, storage, and availability settings when the instance already exists.
- `redis` provisions Memorystore Redis instances; repeated deploys update size and Redis version where supported.
- `pubsub-topic`, `logging-bucket`, and `logging-sink` resources are created or updated in place through `gcloud`.
- `status --env <env>` listens on `deploy.status.<env>.>` over NATS.
- Environment inheritance is supported with `extends="<base-env>"`.
- Pushing a `v*` tag triggers the GitHub release workflow, which runs tests, builds release archives, and publishes a GitHub Release using the matching `CHANGELOG.md` entry.
