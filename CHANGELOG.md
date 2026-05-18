# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.3] - 2026-05-18

### Changed

- Expanded unit coverage again across the CLI, config loader, and GCP client packages, raising local package coverage to `internal/cli 96.3%`, `internal/config 97.5%`, and `internal/gcp 93.4%`, with `cmd/kdlctl`, `internal/github`, and `pkg/types` now covered at `100%`.
- Added small constructor and command-entry seams so CLI root execution and GCP client setup paths can be exercised directly in tests without changing runtime behavior.

### Testing

- Added targeted tests for CLI init, plan, status, rollback, NATS, and root command write/error branches, plus config validation, environment resolution, explicit parser-value coverage, and GCP client constructor and close/error paths.

## [v0.2.2] - 2026-05-17

### Added

- Added a repository health baseline with issue templates, a pull request template, contributing guidance, a code of conduct, support guidance, a security policy, and Dependabot updates for Go modules and GitHub Actions.
- Added a proper `.gitignore` for Go build outputs, coverage artifacts, local environment files, and common editor and OS noise.

### Fixed

- Fixed `kdlctl init` to create `examples/deploy.kdl` using platform-correct path handling so the starter file workflow works on Linux runners as well as Windows.

## [v0.2.1] - 2026-05-17

### Added

- Added `testify`-based unit tests across the CLI, planner, Cloud Build helpers, templates, and type cloning helpers.
- Added reusable example fixtures under `examples/valid`, `examples/invalid`, and `examples/complex`.
- Added a `Makefile` with `test` and `test-race` targets.
- Added a dedicated GitHub Actions test workflow with parallel test execution and a 70% coverage gate across `internal/...` and `pkg/...`.
- Added Coveralls reporting and pull request coverage feedback to the GitHub Actions test pipeline.

### Changed

- Refactored CLI integrations so NATS, Cloud Build, and Cloud Run rollback paths are mockable in tests without live credentials.
- Expanded internal test coverage across `internal/cli`, `internal/config`, `internal/deploy`, `internal/gcp`, `internal/github`, `internal/nats`, and `pkg/types`.

### Documentation

- Updated the README to reflect the current example fixture layout, testing workflow, coverage reporting, and release process for the `v0.2.x` line.

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