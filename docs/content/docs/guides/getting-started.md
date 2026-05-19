---
title: "Getting Started"
description: "Install kdlctl, create your first deploy.kdl file, validate it, preview the plan, and launch your first Google Cloud deployment."
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

Use `kdlctl` when you want one KDL file to describe how your app should land on Google Cloud.

## Prerequisites

1. A Google Cloud project with Cloud Build and Cloud Run enabled.
2. An image already pushed to Artifact Registry.
3. Application Default Credentials configured locally:

```powershell
gcloud auth application-default login
```

## Install

```powershell
go install github.com/djburkhart/kdlctl/cmd/kdlctl@latest
```

## Create starter files

From your service repository, generate a starter config and supporting example file:

```powershell
kdlctl init
```

This creates:

- `deploy.kdl`
- `cloudbuild.yaml`
- `examples/deploy.kdl`

## Author your first deployment

At minimum, define a project, an environment, and one deployable service:

```kdl
project "my-gcp-project" region="us-central1" {
    environment "prod" {
        cloud-run "api-service" {
            image "us-central1-docker.pkg.dev/my-gcp-project/apps/api-service:latest"
            cpu 1
            memory "512Mi"
            min-instances 1
            max-instances 10
            concurrency 80
        }
    }
}
```

## Validate and plan

Check the config before you deploy:

```powershell
kdlctl validate --env prod
kdlctl plan --env prod
```

Use `--json` on `plan` if you want machine-readable output.

## Deploy

Submit the deployment through Cloud Build:

```powershell
kdlctl deploy --env prod
```

Target a single service or managed resource when you only want one change:

```powershell
kdlctl deploy --env prod --service api-service
```

## Watch status and roll back

Track a build directly:

```powershell
kdlctl status --build BUILD_ID
```

If you need to revert traffic on Cloud Run:

```powershell
kdlctl rollback --env prod --service api-service --revision api-service-00012-abc
```

## Next steps

- Read the [deploy.kdl schema reference](../reference/deploy-kdl-schema/)
- Review the [CLI command reference](../reference/cli-commands/)
- Start from the checked-in [`examples/deploy.kdl`](https://github.com/djburkhart/kdlctl/blob/main/examples/deploy.kdl)
