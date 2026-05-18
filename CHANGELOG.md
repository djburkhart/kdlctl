# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.1] - 2026-05-17

### Added

- Added `grpc-server` and `caddy-server` workload support alongside standard `cloud-run` services.
- Added managed infrastructure targets for `cloud-sql`, `redis`, `pubsub-topic`, `logging-bucket`, and `logging-sink`.
- Added richer Cloud Run deployment controls, including ingress, HTTP/2, unauthenticated access, VPC connector settings, Cloud SQL attachments, labels, secret-backed environment variables, and inherited environment configuration.
- Added NATS deployment event publishing support in the `deploy` command.
- Added a CodeQL workflow, Apache-2.0 licensing, and release automation for tagged builds.

### Changed

- Expanded the deployment planner and Cloud Build integration to render and apply both workload and infrastructure targets from `deploy.kdl`.
- Updated the example `deploy.kdl` template to cover gRPC, Caddy, data services, logging resources, labels, and NATS configuration.
- Refactored the deploy CLI flow to reduce cyclomatic complexity while preserving direct Cloud Build and NATS-driven deploy paths.

### Documentation

- Updated the README with the current badge set, release information, and the expanded feature set for `kdlctl`.