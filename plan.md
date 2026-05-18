# GCP KDL Deploy CLI - Implementation Plan

**Project**: Automated GCP Deployments using KDL Configuration + Cloud Build + NATS + GitHub  
**Language**: Go  
**Owner**: Resolvora LLC  
**Date**: May 17, 2026

---

## 1. Overview & Goals

Build a powerful, developer-friendly CLI tool (`kdlctl`) that:

- Uses **KDL** (KDL Document Language) as the primary configuration format for deployments.
- Automates deployments to Google Cloud Platform via **Google Cloud Build**.
- Integrates with **GitHub** for source control and GitOps workflows.
- Leverages **NATS.io** for event-driven orchestration, real-time status updates, and notifications.
- Provides a clean, fast, and reliable experience for deploying to Cloud Run, GKE, Artifact Registry, etc.

### Key Benefits
- Human-readable, structured configuration with KDL (cleaner than YAML/JSON for this use case).
- GitOps-friendly workflow.
- Event-driven architecture via NATS (decouples CLI from long-running operations).
- Full automation of the Cloud Build pipeline.
- Real-time feedback and status via NATS subscriptions.

---

## 2. High-Level Architecture

```
User (Terminal)
   │
   ▼
┌──────────────────────────────┐
│        kdlctl CLI (Go)       │
│  - Cobra commands            │
│  - KDL parsing (calico32/kdl-go) │
│  - Validation & Planning     │
└──────────────┬───────────────┘
               │
    ┌──────────┼──────────┐
    │          │          │
    ▼          ▼          ▼
GitHub API   NATS.io   Cloud Build API
(go-github)  (nats.go) (cloud.google.com/go/cloudbuild/apiv1)
    │          │          │
    └──────────┴──────────┘
               │
               ▼
        Google Cloud Platform
        - Cloud Build (triggers & builds)
        - Artifact Registry
        - Cloud Run / GKE
        - Optional: NATS cluster on GKE
```

**Core Flow**:
1. Edit `deploy.kdl` in the repository.
2. Run `kdlctl deploy --env prod`.
3. CLI validates KDL → triggers Cloud Build (or publishes NATS event).
4. Cloud Build executes the pipeline.
5. Status and logs stream back via NATS or Cloud Build API.

---

## 3. Recommended Tech Stack

| Component            | Library                                      | Notes |
|----------------------|----------------------------------------------|-------|
| CLI Framework        | `github.com/spf13/cobra` + `viper`          | Standard, powerful |
| KDL Parser           | `github.com/calico32/kdl-go`                | Best Go implementation (struct tags, encoding/decoding) |
| GCP Cloud Build      | `cloud.google.com/go/cloudbuild/apiv1`      | Official client |
| GCP Other Services   | `cloud.google.com/go/run`, `artifactregistry` | As needed |
| GitHub Integration   | `github.com/google/go-github/vXX`           | Official GitHub client |
| NATS Client          | `github.com/nats-io/nats.go` + JetStream    | Full-featured |
| Validation           | `github.com/go-playground/validator/v10`    | Struct validation |
| Auth                 | Google ADC + `golang.org/x/oauth2`          | Service accounts + GitHub PAT/App |

---

## 4. KDL Configuration Example (`deploy.kdl`)

```kdl
project "my-gcp-project" region="us-central1" {
    environment "prod" {
        cloud-run "api-service" {
            image "us-central1-docker.pkg.dev/my-gcp-project/api:latest"
            cpu 2
            memory "1Gi"
            min-instances 2
            max-instances 200
            concurrency 80
            env {
                LOG_LEVEL "info"
                DATABASE_URL secret="prod-db-connection"
            }
            traffic {
                latest 100
            }
        }

        nats {
            cluster "nats-prod" {
                replicas 3
                jetstream enabled=#true
                storage-class "standard"
                size "20Gi"
            }
        }
    }

    environment "staging" {
        // Can inherit or override values
        cloud-run "api-service" {
            min-instances 1
            max-instances 50
        }
    }
}
```

The CLI will unmarshal this directly into typed Go structs.

---

## 5. CLI Commands

```bash
# Initialize project
kdlctl init

# Validate configuration
kdlctl validate --env prod

# Dry-run / Plan
kdlctl plan --env prod

# Execute deployment
kdlctl deploy --env prod --async

# Check status
kdlctl status --build <BUILD_ID>
kdlctl status --env prod          # Live via NATS

# Rollback
kdlctl rollback --env prod --revision 5

# NATS utilities
kdlctl nats publish deploy.requested
kdlctl nats subscribe deploy.status
```

---

## 6. Cloud Build Integration

The CLI will support two modes:

1. **Direct Build Submission** (fastest for one-off deploys)
2. **Trigger Management** (recommended for GitOps)

**Example `cloudbuild.yaml`** (generated or templated by CLI):

```yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', '$_IMAGE', '.']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', '$_IMAGE']
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    entrypoint: 'gcloud'
    args:
      - 'run'
      - 'deploy'
      - '$_SERVICE'
      - '--image'
      - '$_IMAGE'
      - '--region'
      - '$_REGION'
      - '--platform'
      - 'managed'
options:
  substitutionOption: 'ALLOW_LOOSE'
substitutions:
  _IMAGE: 'us-central1-docker.pkg.dev/$PROJECT_ID/api:latest'
  _SERVICE: 'api-prod'
  _REGION: 'us-central1'
```

The Go code will use `cloudbuildpb.CreateBuildRequest` to submit builds programmatically.

---

## 7. NATS Integration Strategy

**Primary Roles**:
- **Event Bus**: CLI publishes `deploy.requested` events. A worker (Cloud Run / GKE) consumes and triggers Cloud Build.
- **Real-time Status**: Cloud Build steps publish progress to NATS subjects (`deploy.status.{env}.{service}`).
- **Notifications**: Slack / email / webhook integrations via NATS.

**Deployment Options for NATS**:
- Recommended: NATS + JetStream on **GKE** using official Helm chart.
- Alternative: Single-node NATS on Compute Engine (for simpler setups).

The CLI can also automate the deployment of the NATS cluster itself using the same KDL-driven flow.

---

## 8. GitHub Integration

- Connect repository to Cloud Build (2nd generation triggers).
- CLI capabilities:
  - Create deployment branches or PRs with updated `deploy.kdl`.
  - Manage GitHub repository secrets.
  - Trigger builds on specific labels or comments.
  - Support for GitOps workflow (merge to `main` → auto-deploy).

---

## 9. Recommended Project Structure

```
kdlctl/
├── cmd/
│   └── deploy/
│       └── main.go
├── internal/
│   ├── config/           # KDL parsing, validation, unmarshaling
│   ├── gcp/              # Cloud Build, Run, Artifact Registry clients
│   ├── github/           # GitHub API wrappers
│   ├── nats/             # Publisher / Subscriber helpers + JetStream
│   └── deploy/           # Core deployment orchestration logic
├── pkg/
│   └── types/            # Deployment structs (KDL-mapped)
├── examples/
│   └── deploy.kdl
├── cloudbuild.yaml
├── go.mod
├── go.sum
└── README.md
```

---

## 10. Implementation Roadmap

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| **Phase 1** | 1–2 weeks | Cobra CLI skeleton, KDL parsing + validation, `init` / `validate` / `plan` commands |
| **Phase 2** | 1 week     | GCP Cloud Build client, direct build submission, basic `deploy` command |
| **Phase 3** | 1 week     | GitHub integration, trigger management, improved auth |
| **Phase 4** | 1–2 weeks  | NATS event bus, real-time status, rollback support, multi-environment handling |
| **Phase 5** | Ongoing    | Polish, testing, documentation, NATS cluster automation, advanced features (approvals, canary) |

---

## 11. Initial Setup Steps

### GCP
1. Create GCP project and enable required APIs.
2. Create Artifact Registry repository.
3. Create Service Account with appropriate roles (`Cloud Build Editor`, `Cloud Run Admin`, etc.).
4. Configure Application Default Credentials (`gcloud auth application-default login`).

### GitHub
1. Create repository.
2. Connect repository to Cloud Build (Console → Cloud Build → Triggers).

### NATS (Development)
```bash
docker run -p 4222:4222 nats:latest
```

### Local Development
```bash
go mod init github.com/djburkhart/kdlctl
go get github.com/spf13/cobra github.com/calico32/kdl-go cloud.google.com/go/cloudbuild/apiv1 github.com/nats-io/nats.go
```

---

## 12. Next Steps & Open Questions

**Immediate Next Steps**:
- Generate the initial Go project skeleton.
- Create the KDL struct definitions and parsing layer.
- Implement the first working `deploy` command that triggers a Cloud Build.

**Open Questions** (please answer to refine the plan):
1. Primary deployment target: **Cloud Run** or **GKE** (or both)?
2. Should the CLI support **multi-project** / **multi-region** deployments out of the box?
3. Do you want **approval gates** before production deployments?
4. Preferred NATS deployment model (GKE cluster vs single instance)?
5. Any specific services beyond Cloud Run / NATS (e.g., Cloud Functions, App Engine, Compute Engine)?

---

**Status**: Ready for implementation.  
This document will be updated as the project progresses.

---

*Generated for Resolvora LLC – May 2026*