.PHONY: dev test migrate-up migrate-down seed run-worker gen-openapi docker-build docker-push ensure-ghcr-vars

REGISTRY ?= ghcr.io
IMAGE_TAG ?= $(shell git rev-parse --short=12 HEAD)

## dev: Run the HTTP API using the local Go toolchain.
dev:
	go run ./main.go

## test: Execute the entire Go test suite.
test:
	go test ./...

## migrate-up: Apply all database migrations using the embedded SQL bundle.
migrate-up:
	go run ./cmd/tools/migrate -direction up

## migrate-down: Attempt to roll back migrations (no-op while down scripts are missing).
migrate-down:
	go run ./cmd/tools/migrate -direction down

## seed: Populate the database with baseline roles, permissions, and settings.
seed:
	go run ./cmd/tools/seeder

## run-worker: Start the background job worker for queues and scheduled jobs.
run-worker:
	go run ./cmd/worker

## gen-openapi: Generate Go types from the OpenAPI specification using oapi-codegen.
gen-openapi:
	mkdir -p internal/http/openapi
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --generate types --package openapi --output internal/http/openapi/theyamato.gen.go docs/openapi/theyamato/theyamato.yaml

## docker-build: Build local API and worker images tagged for GHCR testing.
docker-build: ensure-ghcr-vars
	docker buildx build --target api --tag $(REGISTRY)/$(GITHUB_REPOSITORY)/api:$(IMAGE_TAG) --load .
	docker buildx build --target worker --tag $(REGISTRY)/$(GITHUB_REPOSITORY)/worker:$(IMAGE_TAG) --load .

## docker-push: Build and push multi-architecture images to GHCR.
docker-push: ensure-ghcr-vars
	docker buildx build --target api --platform linux/amd64,linux/arm64 --tag $(REGISTRY)/$(GITHUB_REPOSITORY)/api:$(IMAGE_TAG) --push .
	docker buildx build --target worker --platform linux/amd64,linux/arm64 --tag $(REGISTRY)/$(GITHUB_REPOSITORY)/worker:$(IMAGE_TAG) --push .

## ensure-ghcr-vars: Validate environment requirements for GHCR operations.
ensure-ghcr-vars:
	@:$(if $(GITHUB_REPOSITORY),,$(error GITHUB_REPOSITORY is required (for example owner/repo)))
