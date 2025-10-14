# syntax=docker/dockerfile:1

# -----------------------------------------------------------------------------
# Builder stage: compile the Yamato API and worker binaries.
# GHCR tagging strategy: publish artifacts as ghcr.io/${GITHUB_REPOSITORY}/api:${GIT_SHA}
# and ghcr.io/${GITHUB_REPOSITORY}/worker:${GIT_SHA}, while keeping `latest`
# tags for the default branch.
# -----------------------------------------------------------------------------
FROM golang:1.22 AS builder

# 1.- Define build arguments so Docker's cross-compilation features can be used.
ARG TARGETOS
ARG TARGETARCH

# 2.- Enable reproducible builds and configure the working directory.
ENV CGO_ENABLED=0
WORKDIR /src

# 3.- Restore Go modules before copying the full workspace for better caching.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 4.- Copy the remaining sources and leverage build caching for dependencies.
COPY . .

# 5.- Compile the API server binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /out/api ./main.go

# 6.- Compile the worker binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /out/worker ./cmd/worker

# -----------------------------------------------------------------------------
# Runtime image for the API server.
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim AS api

# 1.- Install runtime dependencies required for TLS and health checks.
RUN --mount=type=cache,target=/var/cache/apt --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# 2.- Establish the working directory used by the container.
WORKDIR /app

# 3.- Copy the API binary built in the previous stage.
COPY --from=builder /out/api /usr/local/bin/api

# 4.- Expose the HTTP port consumed by docker-compose.
EXPOSE 8080

# 5.- Run the API binary as the container entrypoint.
ENTRYPOINT ["/usr/local/bin/api"]

# -----------------------------------------------------------------------------
# Runtime image for the background worker.
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim AS worker

# 1.- Install runtime dependencies required for TLS communication.
RUN --mount=type=cache,target=/var/cache/apt --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 2.- Establish the working directory used by the container.
WORKDIR /app

# 3.- Copy the worker binary built in the shared builder stage.
COPY --from=builder /out/worker /usr/local/bin/worker

# 4.- Run the worker process on container startup.
ENTRYPOINT ["/usr/local/bin/worker"]
