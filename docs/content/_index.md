---
title: "kdlctl"
description: "Deploy Cloud Run services and managed Google Cloud resources from a single deploy.kdl file with a repeatable, KDL-driven workflow."
lead: "A KDL-driven Google Cloud deployment CLI for Cloud Run, gRPC, Caddy, Cloud SQL, Redis, Pub/Sub, Logging, and NATS-backed workflows."
date: 2026-05-18T10:00:00-04:00
lastmod: 2026-05-18T10:00:00-04:00
draft: false
params:
  seo:
    title: "" # custom title (optional)
    description: "" # custom description (recommended)
    canonical: "" # custom canonical URL (optional)
    robots: "" # custom robot tags (optional)
---

`kdlctl` turns a `deploy.kdl` file into a repeatable Google Cloud deployment workflow. Use it to validate configs, preview plans, submit Cloud Build jobs, deploy Cloud Run workloads, provision managed resources, publish NATS events, and roll back services when you need to recover quickly.

The docs site covers installation, first deploys, migration guidance, the supported KDL schema, command reference, and the repo links you need for releases, coverage, CI, and API reference. For the system-level diagram, see the [Architecture Overview]({{< relref "/docs/guides/architecture-overview.md" >}}) guide.
