# GHCR Deployment Guide

This guide documents how the Yamato backend publishes container images to GitHub Container Registry (GHCR) for production deployments.

## Automated publishing via GitHub Actions

1. The [`Publish Containers`](../../.github/workflows/ghcr.yml) workflow builds the API and worker images on pushes to the `main` branch, version tags (`v*`), and manual invocations.
2. Images are published to `ghcr.io/<owner>/<repo>/<component>` where `<component>` is `api` or `worker`.
3. Tags include the commit SHA, the branch name, semantic-version aliases, and `latest` for the default branch. Multi-architecture manifests for `linux/amd64` and `linux/arm64` are generated.
4. Authentication uses the ephemeral `GITHUB_TOKEN`; no additional secrets are required.

## Local validation before pushing

1. Ensure Docker Buildx is installed (`docker buildx version`).
2. Export `GITHUB_REPOSITORY=owner/repo` to match the GitHub namespace.
3. Run `make docker-build` to compile and load API and worker images locally using the current commit SHA as the tag.
4. Execute `make docker-push` to publish multi-architecture images to GHCR once you are satisfied with local validation.

## Image naming helpers

The package [`internal/tooling/container`](../../internal/tooling/container/tagger.go) includes reusable helpers to compute GHCR-compliant repository names and tag sets. These functions are covered by unit tests in [`internal/tooling/container/tagger_test.go`](../../internal/tooling/container/tagger_test.go).
