package observability

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// 1.- MetricsOption customizes the metrics collector during construction.
type MetricsOption func(*Metrics)

// 1.- Metrics aggregates Prometheus collectors for HTTP, database, Redis, WebSocket, and job telemetry.
type Metrics struct {
	registry *prometheus.Registry
	handler  http.Handler
	tracer   trace.Tracer

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	dbQueries  *prometheus.CounterVec
	dbDuration *prometheus.HistogramVec

	redisCommands *prometheus.CounterVec
	redisLatency  *prometheus.HistogramVec

	wsConnections *prometheus.GaugeVec
	wsMessages    *prometheus.CounterVec

	jobRuns     *prometheus.CounterVec
	jobDuration *prometheus.HistogramVec
}

// 1.- WithTracer configures an OpenTelemetry tracer used to wrap metric recordings with spans.
func WithTracer(tracer trace.Tracer) MetricsOption {
	return func(m *Metrics) {
		m.tracer = tracer
	}
}

// 1.- NewMetrics constructs a Metrics instance wired to a dedicated Prometheus registry.
func NewMetrics(service string, opts ...MetricsOption) (*Metrics, error) {
	// 1.- Normalize the service label used by the collectors.
	trimmedService := strings.TrimSpace(service)
	if trimmedService == "" {
		trimmedService = "Larago API"
	}

	// 2.- Create a standalone registry so unit tests can safely inspect collector output.
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(prometheus.Labels{"service": trimmedService}, registry)

	// 3.- Instantiate every collector required by the application.
	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests handled by the service.",
	}, []string{"method", "route", "status"})
	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Latency distribution for HTTP handlers.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	dbQueries := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "db_queries_total",
		Help: "Total number of database queries executed.",
	}, []string{"operation", "resource", "success"})
	dbDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Latency distribution for database queries.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "resource"})

	redisCommands := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "redis_commands_total",
		Help: "Total number of Redis commands executed.",
	}, []string{"command", "success"})
	redisLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "redis_command_duration_seconds",
		Help:    "Latency distribution for Redis commands.",
		Buckets: prometheus.DefBuckets,
	}, []string{"command"})

	wsConnections := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "websocket_connections",
		Help: "Number of active WebSocket connections.",
	}, []string{"scope"})
	wsMessages := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "websocket_messages_total",
		Help: "Total number of WebSocket messages processed.",
	}, []string{"scope", "direction"})

	jobRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "job_runs_total",
		Help: "Total number of background job executions.",
	}, []string{"name", "status"})
	jobDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Latency distribution for background jobs.",
		Buckets: prometheus.DefBuckets,
	}, []string{"name"})

	// 4.- Register every collector with the wrapped registerer.
	collectors := []prometheus.Collector{
		httpRequests, httpDuration,
		dbQueries, dbDuration,
		redisCommands, redisLatency,
		wsConnections, wsMessages,
		jobRuns, jobDuration,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, err
		}
	}

	// 5.- Build the Metrics value with the registered collectors and configurable options.
	m := &Metrics{
		registry:      registry,
		handler:       promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		httpRequests:  httpRequests,
		httpDuration:  httpDuration,
		dbQueries:     dbQueries,
		dbDuration:    dbDuration,
		redisCommands: redisCommands,
		redisLatency:  redisLatency,
		wsConnections: wsConnections,
		wsMessages:    wsMessages,
		jobRuns:       jobRuns,
		jobDuration:   jobDuration,
	}

	// 6.- Apply optional configuration hooks such as tracing integration.
	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// 1.- Handler exposes the Prometheus HTTP handler used to scrape metrics.
func (m *Metrics) Handler() http.Handler {
	return m.handler
}

// 1.- RecordHTTP captures request counts and latencies for HTTP handlers.
func (m *Metrics) RecordHTTP(ctx context.Context, method, route string, status int, duration time.Duration) {
	// 1.- Start a tracing span when a tracer has been provided.
	end := m.startSpan(ctx, "observability.metrics.http",
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.String("http.status", strconv.Itoa(status)))
	defer end()

	// 2.- Sanitize label values so Prometheus cardinality remains controlled.
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "UNKNOWN"
	}
	route = sanitizeRoute(route)
	statusLabel := strconv.Itoa(status)
	if statusLabel == "0" {
		statusLabel = "unknown"
	}

	// 3.- Record both the request counter and the observed latency.
	m.httpRequests.WithLabelValues(method, route, statusLabel).Inc()
	if duration < 0 {
		duration = 0
	}
	m.httpDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// 1.- RecordDBQuery tracks database activity including latency and success state.
func (m *Metrics) RecordDBQuery(ctx context.Context, operation, resource string, success bool, duration time.Duration) {
	end := m.startSpan(ctx, "observability.metrics.db",
		attribute.String("db.operation", operation),
		attribute.String("db.resource", resource),
		attribute.Bool("db.success", success))
	defer end()

	operation = strings.ToUpper(strings.TrimSpace(operation))
	if operation == "" {
		operation = "UNKNOWN"
	}
	resource = sanitizeResource(resource)
	outcome := strconv.FormatBool(success)

	m.dbQueries.WithLabelValues(operation, resource, outcome).Inc()
	if duration < 0 {
		duration = 0
	}
	m.dbDuration.WithLabelValues(operation, resource).Observe(duration.Seconds())
}

// 1.- RecordRedisOperation records Redis command counts and durations.
func (m *Metrics) RecordRedisOperation(ctx context.Context, command string, success bool, duration time.Duration) {
	end := m.startSpan(ctx, "observability.metrics.redis",
		attribute.String("redis.command", command),
		attribute.Bool("redis.success", success))
	defer end()

	command = strings.ToUpper(strings.TrimSpace(command))
	if command == "" {
		command = "UNKNOWN"
	}
	outcome := strconv.FormatBool(success)

	m.redisCommands.WithLabelValues(command, outcome).Inc()
	if duration < 0 {
		duration = 0
	}
	m.redisLatency.WithLabelValues(command).Observe(duration.Seconds())
}

// 1.- IncrementWebsocketConnections increments the active WebSocket connection gauge.
func (m *Metrics) IncrementWebsocketConnections(ctx context.Context, scope string) {
	end := m.startSpan(ctx, "observability.metrics.ws.connect", attribute.String("ws.scope", scope))
	defer end()

	scope = sanitizeResource(scope)
	m.wsConnections.WithLabelValues(scope).Inc()
}

// 1.- DecrementWebsocketConnections decrements the active WebSocket connection gauge.
func (m *Metrics) DecrementWebsocketConnections(ctx context.Context, scope string) {
	end := m.startSpan(ctx, "observability.metrics.ws.disconnect", attribute.String("ws.scope", scope))
	defer end()

	scope = sanitizeResource(scope)
	m.wsConnections.WithLabelValues(scope).Dec()
}

// 1.- RecordWebsocketMessage counts inbound or outbound WebSocket messages.
func (m *Metrics) RecordWebsocketMessage(ctx context.Context, scope, direction string) {
	end := m.startSpan(ctx, "observability.metrics.ws.message",
		attribute.String("ws.scope", scope),
		attribute.String("ws.direction", direction))
	defer end()

	scope = sanitizeResource(scope)
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		direction = "unknown"
	}

	m.wsMessages.WithLabelValues(scope, direction).Inc()
}

// 1.- RecordJobRun captures metrics for background job executions.
func (m *Metrics) RecordJobRun(ctx context.Context, name, status string, duration time.Duration) {
	end := m.startSpan(ctx, "observability.metrics.job",
		attribute.String("job.name", name),
		attribute.String("job.status", status))
	defer end()

	name = sanitizeResource(name)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "unknown"
	}

	m.jobRuns.WithLabelValues(name, status).Inc()
	if duration < 0 {
		duration = 0
	}
	m.jobDuration.WithLabelValues(name).Observe(duration.Seconds())
}

// 1.- Registry exposes the underlying Prometheus registry for advanced integrations.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// 1.- startSpan starts a tracing span when instrumentation has been configured.
func (m *Metrics) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) func() {
	if m == nil || m.tracer == nil {
		return func() {}
	}
	ctx, span := m.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	_ = ctx
	return func() {
		span.End()
	}
}

// 1.- sanitizeRoute normalizes HTTP route labels for Prometheus usage.
func sanitizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "unknown"
	}
	return route
}

// 1.- sanitizeResource normalizes resource labels to avoid high cardinality.
func sanitizeResource(resource string) string {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return "unknown"
	}
	return resource
}
