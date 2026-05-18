---
title: "Supported Targets"
description: "See which workload and managed resource types kdlctl supports and how each one maps onto Google Cloud deployment behavior."
summary: ""
date: 2026-05-18T10:00:00-04:00
lastmod: 2026-05-18T10:00:00-04:00
draft: false
weight: 300
toc: true
params:
  seo:
    title: ""
    description: ""
    canonical: ""
    robots: ""
---

## Workloads

### Cloud Run services

`cloud-run` is the default workload type for HTTP services. It supports sizing, scaling, networking, labels, secrets, Cloud SQL attachments, and traffic controls.

### gRPC servers

`grpc-server` uses the same deployment machinery with HTTP/2 enabled for gRPC transport.

### Caddy servers

`caddy-server` is intended for Caddy-based workloads that still deploy as Cloud Run services.

## Managed resources

### Cloud SQL

Use `cloud-sql` to provision or update a Cloud SQL instance, including version, tier, storage, and availability mode.

### Redis

Use `redis` for Memorystore instances, including tier, memory sizing, and Redis version.

### Pub/Sub topics

Use `pubsub-topic` for message retention and labels.

### Logging buckets

Use `logging-bucket` to define retention and storage location for log sinks.

### Logging sinks

Use `logging-sink` for destination, filter, description, and unique writer identity behavior.

## Operational notes

- repeated deploys patch resources in place where supported
- `status --env <env>` listens on `deploy.status.<env>.>` over NATS
- `rollback` changes Cloud Run traffic, not infrastructure state
- secrets in `env` are rendered to `gcloud run deploy --set-secrets`
