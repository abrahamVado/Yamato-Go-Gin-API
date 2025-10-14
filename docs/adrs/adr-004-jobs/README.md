# ADR 004: Background Jobs Infrastructure

## Context

Our application increasingly relies on asynchronous work such as sending notifications, performing data imports, and executing scheduled maintenance tasks. The current synchronous request/response flow cannot support these workloads without causing latency spikes and reliability concerns. We therefore need a consistent background job platform that allows producers to enqueue work, workers to process the queue predictably, and the system to withstand transient failures.

## Queue Backend

We will use Redis as the backbone for the job queue because it is already part of our stack and offers the primitives required for high-throughput processing. Two Redis data structures will be used in tandem:

- **Redis Streams** capture ordered event data, enable message acknowledgements, and simplify horizontal scaling via consumer groups.
- **Redis Lists** act as an emergency failover buffer that preserves job order during maintenance and offers straightforward observability with existing tooling.

Streams are the primary transport for new jobs. The list acts as a staging or recovery queue when we need to drain or back up jobs without losing them. Both structures live under a `jobs:` namespace so we can monitor and manage them independently from other Redis keys.

## Worker Binary Responsibilities

The worker binary is a dedicated executable separate from the web server. Its responsibilities include:

1. **Polling Streams:** Consume jobs from Redis Streams using consumer groups to balance load among worker replicas.
2. **Job Dispatching:** Deserialize payloads, choose the appropriate handler, and execute units of work within a managed context that enforces deadlines and cancellation.
3. **Result Reporting:** Acknowledge successful jobs, append failures to the retry list, and emit metrics/logs for observability.
4. **Graceful Shutdown:** Respond to termination signals by draining in-flight jobs and persisting unfinished work back to the Redis List to avoid loss.

Configuration for the worker binary mirrors the application configuration style (environment variables layered with YAML), allowing deployment automation to reuse existing patterns.

## Scheduler Triggers

Recurring work is orchestrated by a lightweight scheduler that publishes jobs into Redis Streams. The scheduler triggers originate from three sources:

- **Cron Expressions:** Declarative schedules stored in configuration that describe periodic maintenance, billing runs, or sync jobs.
- **API-Initiated Triggers:** Administrative endpoints allow operators to enqueue ad-hoc jobs, ensuring we can backfill or replay tasks on demand.
- **Domain Events:** Critical business events (e.g., order state changes) push follow-up jobs synchronously to Streams, decoupling long-running processing from the main request flow.

Schedulers only enqueue payloads; they never perform long-running work themselves. This keeps the scheduling component stateless and resilient.

## Retry Policies

Resilience is a first-class requirement. Failed jobs follow a structured retry lifecycle:

- **Immediate Retries:** Each job receives up to three rapid retries with exponential backoff (e.g., 1s, 5s, 30s) handled by the worker before being considered failed.
- **Deferred Retries:** Once immediate retries are exhausted, jobs are pushed onto the Redis List with metadata describing the next retry window. A separate retry worker rehydrates the job back into the Stream when the delay expires.
- **Dead Letter Queue:** Jobs that fail a configurable number of total attempts move to a `jobs:dead_letter` Stream along with failure context for manual inspection and replay.

Operators can inspect retry states via existing Redis inspection tooling, and alerts are raised when the dead letter count breaches thresholds.

## Consequences

This architecture standardizes the asynchronous processing model, reduces coupling between web requests and background tasks, and provides a roadmap for scaling job throughput. Redis Streams offer horizontal scalability, while Lists give us an operational escape hatch. Explicit retry handling ensures transient issues do not cause permanent job loss, and the separation of concerns between scheduler, worker, and queue provides clarity for future enhancements.
