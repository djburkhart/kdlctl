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

        grpc-server "payments-grpc" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/payments-grpc:latest"
            cpu 2
            memory "1Gi"
            port 8443
            min-instances 1
            max-instances 25
            concurrency 120
            env {
                LOG_LEVEL "info"
                GRPC_REFLECTION "enabled"
            }
        }

        caddy-server "edge-caddy" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/caddy-edge:latest"
            cpu 1
            memory "512Mi"
            port 8080
            min-instances 1
            max-instances 10
            concurrency 200
            env {
                CADDY_CONFIG "/etc/caddy/Caddyfile"
            }
        }

        cloud-sql "primary-db" {
            database-version "POSTGRES_16"
            tier "db-custom-1-3840"
            availability-type "REGIONAL"
            storage-gb 50
        }

        redis "sessions-cache" {
            tier "STANDARD_HA"
            memory-gb 2
            redis-version "REDIS_7_0"
        }

        pubsub-topic "app-events" {
            retention "604800s"
            labels {
                env "prod"
                service "api"
            }
        }

        logging-bucket "application-logs" {
            location "global"
            retention-days 30
            description "Application log retention bucket"
        }

        logging-sink "error-export" {
            destination "logging.googleapis.com/projects/demo-gcp-project/locations/global/buckets/application-logs"
            filter "severity>=ERROR"
            description "Export application errors"
            unique-writer-identity #true
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

        caddy-server "edge-caddy" {
            image "us-central1-docker.pkg.dev/demo-gcp-project/apps/caddy-edge:staging"
        }

        logging-sink "error-export" {
            filter "severity>=WARNING"
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
