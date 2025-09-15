// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"time"

	"github.com/SeaSBee/go-logx"
)

// ObservabilityProvider provides observability capabilities.
type ObservabilityProvider struct {
	metrics Metrics
	tracer  Tracer
	logger  *logx.Logger
}

// NewObservabilityProvider creates a new observability provider.
func NewObservabilityProvider(cfg *TelemetryConfig) (*ObservabilityProvider, error) {
	// For now, we'll use no-op implementations
	// TODO: Implement full OpenTelemetry integration in future steps
	metrics := NoOpMetrics{}
	tracer := NoOpTracer{}
	logger := NoOpLogger()

	return &ObservabilityProvider{
		metrics: metrics,
		tracer:  tracer,
		logger:  logger,
	}, nil
}

// Metrics returns the metrics interface.
func (p *ObservabilityProvider) Metrics() Metrics {
	return p.metrics
}

// Tracer returns the tracer interface.
func (p *ObservabilityProvider) Tracer() Tracer {
	return p.tracer
}

// Logger returns the logger interface.
func (p *ObservabilityProvider) Logger() *logx.Logger {
	return p.logger
}

// ObservabilityContext provides observability context for operations.
type ObservabilityContext struct {
	ctx     context.Context
	metrics Metrics
	tracer  Tracer
	logger  *logx.Logger
}

// NewObservabilityContext creates a new observability context.
func NewObservabilityContext(ctx context.Context, provider *ObservabilityProvider) *ObservabilityContext {
	return &ObservabilityContext{
		ctx:     ctx,
		metrics: provider.metrics,
		tracer:  provider.tracer,
		logger:  provider.logger,
	}
}

// Context returns the underlying context.
func (oc *ObservabilityContext) Context() context.Context {
	return oc.ctx
}

// Metrics returns the metrics interface.
func (oc *ObservabilityContext) Metrics() Metrics {
	return oc.metrics
}

// Tracer returns the tracer interface.
func (oc *ObservabilityContext) Tracer() Tracer {
	return oc.tracer
}

// Logger returns the logger interface.
func (oc *ObservabilityContext) Logger() *logx.Logger {
	return oc.logger
}

// WithSpan creates a new span and returns an updated observability context.
func (oc *ObservabilityContext) WithSpan(name string, opts ...SpanOption) (*ObservabilityContext, Span) {
	ctx, span := oc.tracer.StartSpan(oc.ctx, name, opts...)
	return &ObservabilityContext{
		ctx:     ctx,
		metrics: oc.metrics,
		tracer:  oc.tracer,
		logger:  oc.logger,
	}, span
}

// WithLogger returns a new observability context with additional logger fields.
func (oc *ObservabilityContext) WithLogger(fields ...logx.Field) *ObservabilityContext {
	return &ObservabilityContext{
		ctx:     oc.ctx,
		metrics: oc.metrics,
		tracer:  oc.tracer,
		logger:  oc.logger.With(fields...),
	}
}

// RecordPublishMetrics records publish-related metrics.
func (oc *ObservabilityContext) RecordPublishMetrics(transport, exchange string, duration time.Duration, success bool, reason string) {
	oc.metrics.PublishTotal(transport, exchange)
	oc.metrics.PublishDuration(transport, exchange, duration)

	if success {
		oc.metrics.PublishSuccess(transport, exchange)
	} else {
		oc.metrics.PublishFailure(transport, exchange, reason)
	}
}

// RecordConsumeMetrics records consume-related metrics.
func (oc *ObservabilityContext) RecordConsumeMetrics(transport, queue string, duration time.Duration, success bool, reason string) {
	oc.metrics.ConsumeTotal(transport, queue)
	oc.metrics.ConsumeDuration(transport, queue, duration)

	if success {
		oc.metrics.ConsumeSuccess(transport, queue)
	} else {
		oc.metrics.ConsumeFailure(transport, queue, reason)
	}
}

// RecordConnectionMetrics records connection-related metrics.
func (oc *ObservabilityContext) RecordConnectionMetrics(transport string, connections, channels, inFlight int) {
	oc.metrics.ConnectionsActive(transport, connections)
	oc.metrics.ChannelsActive(transport, channels)
	oc.metrics.MessagesInFlight(transport, inFlight)
}

// RecordSystemMetrics records system-related metrics.
func (oc *ObservabilityContext) RecordSystemMetrics(transport, metricType, reason string) {
	switch metricType {
	case "backpressure":
		oc.metrics.BackpressureTotal(transport, reason)
	case "retry":
		oc.metrics.RetryTotal(transport, reason)
	case "dropped":
		oc.metrics.DroppedTotal(transport, reason)
	}
}

// RecordPersistenceMetrics records persistence-related metrics.
func (oc *ObservabilityContext) RecordPersistenceMetrics(operation string, duration time.Duration, success bool, errorType string) {
	oc.metrics.PersistenceTotal(operation)
	oc.metrics.PersistenceDuration(operation, duration)

	if success {
		oc.metrics.PersistenceSuccess(operation)
	} else {
		oc.metrics.PersistenceFailure(operation, errorType)
	}
}

// RecordDLQMetrics records dead letter queue metrics.
func (oc *ObservabilityContext) RecordDLQMetrics(operation string, duration time.Duration, success bool, errorType string) {
	oc.metrics.DLQTotal(operation)
	oc.metrics.DLQDuration(operation, duration)

	if success {
		oc.metrics.DLQSuccess(operation)
	} else {
		oc.metrics.DLQFailure(operation, errorType)
	}
}

// RecordTransformationMetrics records transformation metrics.
func (oc *ObservabilityContext) RecordTransformationMetrics(operation string, duration time.Duration, success bool, errorType string) {
	oc.metrics.TransformationTotal(operation)
	oc.metrics.TransformationDuration(operation, duration)

	if success {
		oc.metrics.TransformationSuccess(operation)
	} else {
		oc.metrics.TransformationFailure(operation, errorType)
	}
}

// RecordRoutingMetrics records routing metrics.
func (oc *ObservabilityContext) RecordRoutingMetrics(operation string, duration time.Duration, success bool, errorType string) {
	oc.metrics.RoutingTotal(operation)
	oc.metrics.RoutingDuration(operation, duration)

	if success {
		oc.metrics.RoutingSuccess(operation)
	} else {
		oc.metrics.RoutingFailure(operation, errorType)
	}
}

// RecordPerformanceMetrics records performance metrics.
func (oc *ObservabilityContext) RecordPerformanceMetrics(metrics *PerformanceMetrics) {
	// Record throughput metrics
	oc.metrics.PerformanceThroughput("publish", metrics.PublishThroughput)
	oc.metrics.PerformanceThroughput("consume", metrics.ConsumeThroughput)
	oc.metrics.PerformanceThroughput("total", metrics.TotalThroughput)

	// Record latency metrics
	oc.metrics.PerformanceLatency("publish_p50", metrics.PublishLatencyP50)
	oc.metrics.PerformanceLatency("publish_p95", metrics.PublishLatencyP95)
	oc.metrics.PerformanceLatency("publish_p99", metrics.PublishLatencyP99)
	oc.metrics.PerformanceLatency("consume_p50", metrics.ConsumeLatencyP50)
	oc.metrics.PerformanceLatency("consume_p95", metrics.ConsumeLatencyP95)
	oc.metrics.PerformanceLatency("consume_p99", metrics.ConsumeLatencyP99)

	// Record memory metrics
	oc.metrics.PerformanceMemory("heap_alloc", metrics.HeapAlloc)
	oc.metrics.PerformanceMemory("heap_sys", metrics.HeapSys)
	oc.metrics.PerformanceMemory("heap_objects", metrics.HeapObjects)
	oc.metrics.PerformanceMemory("utilization", uint64(metrics.MemoryUtilization))

	// Record GC metrics
	oc.metrics.PerformanceGC("pause_total_ns", int64(metrics.GCPauseTotalNs))
	oc.metrics.PerformanceGC("num_gc", int64(metrics.NumGC))
	oc.metrics.PerformanceGC("num_goroutines", int64(metrics.NumGoroutines))

	// Record error metrics
	oc.metrics.PerformanceErrors("error_rate", metrics.ErrorRate)
	oc.metrics.PerformanceErrors("total_errors", float64(metrics.TotalErrors))

	// Record resource utilization
	oc.metrics.PerformanceResources("active_connections", int64(metrics.ActiveConnections))
	oc.metrics.PerformanceResources("active_channels", int64(metrics.ActiveChannels))
	oc.metrics.PerformanceResources("pool_utilization", int64(metrics.PoolUtilization))
}

// TODO: Implement full OpenTelemetry integration in future steps
// This will include:
// - OpenTelemetry metrics with Prometheus exporter
// - Distributed tracing with OTLP exporter
// - Structured logging with proper log levels
// - Correlation IDs and trace propagation
// - Custom metrics for business logic
// - Health checks and readiness probes
