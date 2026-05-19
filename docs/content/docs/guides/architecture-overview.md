---
title: "Architecture Overview"
description: "See how kdlctl, deploy.kdl, Cloud Build, Google Cloud resources, and GitOps integrations fit together in one deployment flow."
summary: ""
date: 2026-05-19T09:47:55-04:00
lastmod: 2026-05-19T09:47:55-04:00
draft: false
weight: 250
toc: true
params:
  seo:
    title: ""
    description: ""
    canonical: ""
    robots: ""
---

Use this page as the high-level map of how `kdlctl` turns a `deploy.kdl` file into Google Cloud workloads and managed resources, while still fitting into GitOps and event-driven automation.

## Overview diagram

<img src="../../../images/Overview.png" alt="kdlctl deployment overview" class="img-fluid" />

## What the diagram shows

- `kdlctl` sits in the middle of the flow and acts as the entry point for deploy orchestration.
- `deploy.kdl` provides the source configuration that the parser and validator turn into deployable targets.
- The resource mapper splits that configuration into platform resources and compute workloads.
- Cloud Build is used to trigger deployment work for supported workload types.
- External systems can trigger or observe deploy activity through GitHub Actions, webhooks, and NATS messaging.
