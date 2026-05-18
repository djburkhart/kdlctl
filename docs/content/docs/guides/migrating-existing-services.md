---
title: "Migrating Existing Services"
description: "Move an existing Cloud Run or microservice deployment workflow onto kdlctl without losing environment-specific runtime settings."
summary: ""
date: 2026-05-18T10:00:00-04:00
lastmod: 2026-05-18T10:00:00-04:00
draft: false
weight: 200
toc: true
params:
  seo:
    title: ""
    description: ""
    canonical: ""
    robots: ""
---

Use this guide when you already deploy with handwritten `gcloud` commands, Cloud Build YAML, or GitHub Actions and want to consolidate that logic into `deploy.kdl`.

## 1. Inventory the current runtime settings

Capture the values your service already depends on:

- image location
- region
- CPU and memory
- min/max instances
- concurrency
- ingress and authentication rules
- VPC connector and egress settings
- Cloud SQL attachments
- environment variables and secrets

If the existing service uses gRPC or Caddy, map it to `grpc-server` or `caddy-server` instead of plain `cloud-run`.

## 2. Move infrastructure declarations into KDL

`kdlctl` can provision and update more than just services. Put dependent managed resources into the same environment where possible:

- `cloud-sql`
- `redis`
- `pubsub-topic`
- `logging-bucket`
- `logging-sink`

This keeps the deployment plan readable and avoids scattering infrastructure state across multiple scripts.

## 3. Keep environments explicit

Use one environment per deployment target and inherit shared defaults with `extends` where it reduces duplication.

```kdl
environment "base" {
    cloud-run "api-service" {
        cpu 1
        memory "512Mi"
    }
}

environment "prod" extends="base" {
    cloud-run "api-service" {
        image "us-central1-docker.pkg.dev/my-project/apps/api-service:latest"
        min-instances 2
        max-instances 20
    }
}
```

## 4. Replace ad-hoc CI steps

Once the file validates locally, CI can reduce to a few predictable steps:

1. install `kdlctl`
2. run `kdlctl validate --env <env>`
3. run `kdlctl plan --env <env>`
4. run `kdlctl deploy --env <env>` on your protected branch or release flow

The repo includes a `cloudbuild.yaml` template and a docs site workflow you can adapt alongside the main test and release workflows.

## 5. Roll out one target at a time

You do not need to migrate everything in one shot. A practical sequence is:

1. move one Cloud Run workload into `deploy.kdl`
2. validate and plan it
3. deploy just that service with `--service`
4. add databases, caches, and logging resources after the workload flow is stable

## 6. Keep a rollback path ready

`kdlctl rollback` only shifts Cloud Run traffic. Keep the last known-good revision name handy during your first few production deployments so you can revert quickly if needed.
