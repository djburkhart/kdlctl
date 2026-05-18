# Contributing to kdlctl

Thanks for contributing to `kdlctl`.

## Getting started

1. Fork the repository and create a feature branch from `main`.
2. Install the Go version declared in `go.mod`.
3. Run the test suite before submitting changes:
   - `go test ./...`
   - `make test`
   - `make test-race`

## Development guidelines

- Keep changes focused and avoid unrelated refactors.
- Update `README.md`, examples, or workflow docs when behavior changes.
- Add or update tests for CLI, config parsing, deployment planning, or GCP integration behavior when relevant.
- Keep secrets, credentials, and local config out of commits.

## Release notes

If your change affects user-facing behavior, add an entry to `CHANGELOG.md` under `Unreleased` unless the release section has already been prepared.

## Pull requests

Pull requests should include:

- a clear summary of the change
- testing notes
- linked issues or deployment context when available

Small, focused pull requests are easiest to review and merge quickly.
