# Backend Reconstruction Blueprint

## Overview
This document summarizes the core subsystems required to recreate the backend service. Source references point to Go files under `backend/internal` and `backend/cmd` that illustrate the wiring.

## Application Assembly
- Entry point: `backend/cmd/server/main.go` bootstraps logging, loads configuration via `app.LoadConfig`, and builds the HTTP server through `app.Build` before starting `http.Server`.
- Dependency wiring: `backend/internal/app/app.go` constructs shared services and registers both REST and WebSocket routes on a single Gin engine.
  - Database access is optional; when `Config.DBDSN` is empty, repositories operate in-memory.
  - Cleanup logic stops the WebSocket hub and closes the SQL connection if one was opened.

## Service Components
- **Authentication (`internal/auth`)**
  - `auth.Validator` parses bearer tokens and exposes `ValidateToken`. It is injected into the API server to guard sensitive endpoints.
- **Bidding (`internal/bidding`)**
  - `bidding.Arbiter` performs constraint checks (`MaxPrice`, `MaxETA`, geofence radius) and persists the accepted bid through a `bidding.Repository` implementation such as `NewSQLRepository`.
- **Trip Management (`internal/trip`)**
  - `trip.Manager` tracks lifecycle transitions (`StartTrip`, `PauseTrip`, `ResumeTrip`, `CancelTrip`, `CompleteTrip`) and records `trip.TripEvent` instances through its `EventRepository`.
  - `trip.Metrics` aggregates timing data exposed through `/trips/:id/metrics`.
- **Real-time Hub (`internal/ws`)**
  - `ws.Hub` coordinates WebSocket clients, multiplexing riders and drivers per room via broadcast channels.

## HTTP & WebSocket Interfaces
- `internal/api/server.go` registers the REST endpoints documented alongside this file (health, bid evaluation, trip state, trip metrics).
- `internal/ws/hub.go` registers the WebSocket upgrade and occupancy inspection routes.

## Recreation Checklist
1. Implement the configuration loader and builder that produce a Gin engine plus cleanup hook.
2. Instantiate and inject `auth.Validator`, `bidding.Arbiter`, and `trip.Manager` into `api.Server` so that handlers have access to their dependencies.
3. Mount the WebSocket hub routes on the same router to share state between HTTP and real-time features.
4. Ensure graceful shutdown propagates through `Hub.Shutdown` and closes the SQL connection to avoid leaks.
