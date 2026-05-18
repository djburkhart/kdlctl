package templates

const ExampleDeployKDL = `project "demo-gcp-project" region="us-central1" {
    environment "prod" {
        cloud-run "api-service" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/api-service:latest"
            cpu 1
            memory "512Mi"
            min-instances 1
            max-instances 20
            concurrency 80
            env {
                LOG_LEVEL "info"
                GOOGLE_CLOUD_PROJECT "demo-gcp-project"
            }
            traffic {
                latest 100
            }
        }

        cloud-run "worker-service" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/worker-service:latest"
            cpu 1
            memory "512Mi"
            min-instances 1
            max-instances 10
            concurrency 20
            env {
                LOG_LEVEL "info"
                JOB_TOPIC "jobs.created"
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

    environment "staging" extends="prod" {
        cloud-run "api-service" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/api-service:staging"
            max-instances 5
        }
    }
}
`

const CloudBuildTemplate = `steps:
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
      - '--quiet'
options:
  substitutionOption: 'ALLOW_LOOSE'
substitutions:
  _IMAGE: 'us-central1-docker.pkg.dev/$PROJECT_ID/apps/api-service:latest'
  _SERVICE: 'api-service'
  _REGION: 'us-central1'
`
