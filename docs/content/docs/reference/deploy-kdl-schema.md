---
title: "deploy.kdl Schema"
description: "Reference the top-level deploy.kdl structure, supported blocks, and the main runtime knobs kdlctl understands."
summary: ""
date: 2026-05-18T10:00:00-04:00
lastmod: 2026-05-18T10:00:00-04:00
draft: false
weight: 100
toc: true
params:
  seo:
    title: ""
    description: ""
    canonical: ""
    robots: ""
---

`deploy.kdl` is the source of truth for a `kdlctl` deployment.

## Top-level structure

```kdl
project "my-gcp-project" region="us-central1" {
    environment "prod" {
        // workloads and managed resources
    }
}
```

## Environment blocks

An environment can define its own targets or inherit from another environment:

```kdl
environment "staging" extends="base" {
}
```

## Supported workload blocks

### `cloud-run`

General-purpose Cloud Run service deployment.

Common fields:

- `image`
- `cpu`
- `memory`
- `min-instances`
- `max-instances`
- `concurrency`
- `port`
- `ingress`
- `http2`
- `allow-unauthenticated`
- `vpc-connector`
- `vpc-egress`
- `cloud-sql-instances`
- `labels`
- `env`
- `traffic`

### `grpc-server`

Cloud Run deployment with HTTP/2 enabled for gRPC-oriented services.

### `caddy-server`

Cloud Run deployment intended for Caddy-based edge or proxy workloads.

## Supported managed resource blocks

- `cloud-sql`
- `redis`
- `pubsub-topic`
- `logging-bucket`
- `logging-sink`

These resources are planned and submitted through the same deployment workflow as services.

## NATS block

Optional NATS configuration lives under:

```kdl
nats {
    cluster "nats-prod" {
        replicas 3
        jetstream enabled=#true
        storage-class "fast"
        size "20Gi"
    }
}
```

## Validation rules

`kdlctl validate` checks:

- required project and region values
- environment inheritance resolution
- duplicate target names across workloads and resources
- resource-specific required fields
- invalid environment references

Use `kdlctl plan --env <env>` after validation to confirm the effective, merged environment is what you expect.
