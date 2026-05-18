---
title: "Automating Deployments"
description: "Integrate kdlctl into Cloud Build, GitHub Actions, and NATS-driven workflows so validation, planning, and deploys stay repeatable."
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

`kdlctl` works well in CI because the command surface is small and predictable.

## Cloud Build

Use the generated `cloudbuild.yaml` as the starting point for a repository-driven build:

```powershell
kdlctl validate --env prod
kdlctl plan --env prod
kdlctl deploy --env prod
```

This keeps your deploy logic in `deploy.kdl` rather than duplicating runtime flags in build steps.

## GitHub Actions

A practical Actions split looks like this:

1. validate and plan on pull requests
2. deploy on pushes to `main` or on release tags
3. keep test and docs workflows separate from production deployment

If you already use this repo’s release and test workflows as a template, the same pattern applies to downstream services.

## NATS-backed orchestration

When you want async coordination between systems, `kdlctl` can publish deployment events over NATS:

```powershell
kdlctl deploy --env prod --via-nats
kdlctl nats publish --subject deploy.requested '{\"env\":\"prod\"}'
kdlctl nats subscribe --subject deploy.status.prod.>
```

Use that path when another controller should observe or react to deploy state, not when a direct Cloud Build submission is simpler.

## Recommended pipeline order

1. `kdlctl validate --env <env>`
2. `kdlctl plan --env <env>`
3. `kdlctl deploy --env <env>`
4. `kdlctl status --build <build-id>` if you need explicit polling

## What to keep out of CI

Avoid restating the deploy shape in workflow YAML. Put service sizing, networking, and managed resource details in `deploy.kdl` so CI stays thin and auditable.
