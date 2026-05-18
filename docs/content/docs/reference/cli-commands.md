---
title: "CLI Commands"
description: "Look up the full kdlctl command surface, the primary flags on each command, and the deployment workflows they support."
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

## Global flags and environment variables

Important environment variables:

- `KDLCTL_FILE`
- `KDLCTL_PROJECT_ID`
- `KDLCTL_REGION`
- `KDLCTL_NATS_URL`
- `KDLCTL_GITHUB_TOKEN`

## `init`

Create starter files for a new repository:

```powershell
kdlctl init
```

## `validate`

Validate a project or a specific environment:

```powershell
kdlctl validate --env prod
```

## `plan`

Render the effective deploy plan:

```powershell
kdlctl plan --env prod
kdlctl plan --env prod --json
```

## `deploy`

Submit workload and managed resource builds:

```powershell
kdlctl deploy --env prod
kdlctl deploy --env prod --service api-service
kdlctl deploy --env prod --service primary-db
kdlctl deploy --env prod --async
kdlctl deploy --env prod --via-nats
```

## `status`

Track a submitted build or listen on an environment-specific NATS subject:

```powershell
kdlctl status --build BUILD_ID
kdlctl status --env prod
```

## `rollback`

Shift Cloud Run traffic back to a known revision:

```powershell
kdlctl rollback --env prod --service api-service --revision api-service-00012-abc
```

## `nats`

Publish and subscribe directly:

```powershell
kdlctl nats publish --subject deploy.requested '{"env":"prod"}'
kdlctl nats subscribe --subject deploy.status.prod.>
```
