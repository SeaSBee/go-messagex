// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"time"

	"github.com/seasbee/go-logx"
)

// Metrics defines the interface for collecting messaging metrics.
type Metrics interface {
	// PublishTotal increments the total number of published messages.
	PublishTotal(transport, exchange string)

	// PublishSuccess increments the number of successfully published messages.
	PublishSuccess(transport, exchange string)

	// PublishFailure increments the number of failed published messages.
	PublishFailure(transport, exchange, reason string)

	// PublishDuration records the duration of a publish operation.
	PublishDuration(transport, exchange string, duration time.Duration)

	// ConsumeTotal increments the total number of consumed messages.
	ConsumeTotal(transport, queue string)

	// ConsumeSuccess increments the number of successfully consumed messages.
	ConsumeSuccess(transport, queue string)

	// ConsumeFailure increments the number of failed consumed messages.
	ConsumeFailure(transport, queue, reason string)

	// ConsumeDuration records the duration of a consume operation.
	ConsumeDuration(transport, queue string, duration time.Duration)

	// ConnectionsActive records the number of active connections.
	ConnectionsActive(transport string, count int)

	// ChannelsActive records the number of active channels.
	ChannelsActive(transport string, count int)

	// MessagesInFlight records the number of in-flight messages.
	MessagesInFlight(transport string, count int)

	// BackpressureTotal increments the backpressure counter.
	BackpressureTotal(transport, mode string)

	// RetryTotal increments the retry counter.
	RetryTotal(transport, stage string)

	// DroppedTotal increments the dropped messages counter.
	DroppedTotal(transport, reason string)

	// PersistenceTotal increments the total number of persistence operations.
	PersistenceTotal(operation string)

	// PersistenceSuccess increments the number of successful persistence operations.
	PersistenceSuccess(operation string)

	// PersistenceFailure increments the number of failed persistence operations.
	PersistenceFailure(operation, reason string)

	// PersistenceDuration records the duration of a persistence operation.
	PersistenceDuration(operation string, duration time.Duration)

	// DLQTotal increments the total number of DLQ operations.
	DLQTotal(operation string)

	// DLQSuccess increments the number of successful DLQ operations.
	DLQSuccess(operation string)

	// DLQFailure increments the number of failed DLQ operations.
	DLQFailure(operation, reason string)

	// DLQDuration records the duration of a DLQ operation.
	DLQDuration(operation string, duration time.Duration)

	// TransformationTotal increments the total number of transformation operations.
	TransformationTotal(operation string)

	// TransformationSuccess increments the number of successful transformation operations.
	TransformationSuccess(operation string)

	// TransformationFailure increments the number of failed transformation operations.
	TransformationFailure(operation, reason string)

	// TransformationDuration records the duration of a transformation operation.
	TransformationDuration(operation string, duration time.Duration)

	// RoutingTotal increments the total number of routing operations.
	RoutingTotal(operation string)

	// RoutingSuccess increments the number of successful routing operations.
	RoutingSuccess(operation string)

	// RoutingFailure increments the number of failed routing operations.
	RoutingFailure(operation, reason string)

	// RoutingDuration records the duration of a routing operation.
	RoutingDuration(operation string, duration time.Duration)

	// PerformanceThroughput records performance throughput metrics.
	PerformanceThroughput(operation string, throughput float64)

	// PerformanceLatency records performance latency metrics.
	PerformanceLatency(operation string, latencyNs int64)

	// PerformanceMemory records performance memory metrics.
	PerformanceMemory(operation string, bytes uint64)

	// PerformanceGC records performance GC metrics.
	PerformanceGC(operation string, value int64)

	// PerformanceErrors records performance error metrics.
	PerformanceErrors(operation string, value float64)

	// PerformanceResources records performance resource metrics.
	PerformanceResources(operation string, value int64)

	// Publisher confirmation metrics
	// ConfirmTotal increments the total number of publisher confirmations.
	ConfirmTotal(transport string)

	// ConfirmSuccess increments the number of successful publisher confirmations.
	ConfirmSuccess(transport string)

	// ConfirmFailure increments the number of failed publisher confirmations.
	ConfirmFailure(transport, reason string)

	// ConfirmDuration records the duration from publish to confirmation.
	ConfirmDuration(transport string, duration time.Duration)

	// ReturnTotal increments the total number of returned messages.
	ReturnTotal(transport string)

	// ReturnDuration records the duration from publish to return.
	ReturnDuration(transport string, duration time.Duration)
}

// Tracer defines the interface for distributed tracing.
type Tracer interface {
	// StartSpan starts a new span with the given name and context.
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)

	// Extract extracts trace context from message headers.
	Extract(ctx context.Context, headers map[string]string) context.Context

	// Inject injects trace context into message headers.
	Inject(ctx context.Context, headers map[string]string)
}

// Span represents a distributed tracing span.
type Span interface {
	// SetAttribute sets an attribute on the span.
	SetAttribute(key string, value interface{})

	// SetStatus sets the status of the span.
	SetStatus(status SpanStatus, description string)

	// AddEvent adds an event to the span.
	AddEvent(name string, attributes map[string]interface{})

	// End finishes the span.
	End()

	// Context returns the context containing this span.
	Context() context.Context
}

// SpanOption configures a span.
type SpanOption interface {
	apply(*spanConfig)
}

// SpanStatus represents the status of a span.
type SpanStatus int

const (
	// SpanStatusUnset indicates the span status is unset.
	SpanStatusUnset SpanStatus = iota

	// SpanStatusOK indicates the span completed successfully.
	SpanStatusOK

	// SpanStatusError indicates the span completed with an error.
	SpanStatusError
)

// spanConfig holds configuration for a span.
type spanConfig struct {
	kind       SpanKind
	attributes map[string]interface{}
}

// SpanKind represents the kind of span.
type SpanKind int

const (
	// SpanKindUnspecified is an unspecified span kind.
	SpanKindUnspecified SpanKind = iota

	// SpanKindInternal is an internal span.
	SpanKindInternal

	// SpanKindServer is a server span.
	SpanKindServer

	// SpanKindClient is a client span.
	SpanKindClient

	// SpanKindProducer is a producer span.
	SpanKindProducer

	// SpanKindConsumer is a consumer span.
	SpanKindConsumer
)

// withSpanKind sets the span kind.
type withSpanKind struct {
	kind SpanKind
}

func (w withSpanKind) apply(cfg *spanConfig) {
	cfg.kind = w.kind
}

// WithSpanKind creates a SpanOption that sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return withSpanKind{kind: kind}
}

// withAttributes sets span attributes.
type withAttributes struct {
	attributes map[string]interface{}
}

func (w withAttributes) apply(cfg *spanConfig) {
	if cfg.attributes == nil {
		cfg.attributes = make(map[string]interface{})
	}
	for k, v := range w.attributes {
		cfg.attributes[k] = v
	}
}

// WithAttributes creates a SpanOption that sets span attributes.
func WithAttributes(attributes map[string]interface{}) SpanOption {
	return withAttributes{attributes: attributes}
}

// TelemetryProvider bundles all telemetry interfaces.
type TelemetryProvider struct {
	Metrics Metrics
	Tracer  Tracer
	Logger  *logx.Logger
}

// NoOpMetrics provides a no-op implementation of Metrics.
type NoOpMetrics struct{}

func (m NoOpMetrics) PublishTotal(transport, exchange string)                            {}
func (m NoOpMetrics) PublishSuccess(transport, exchange string)                          {}
func (m NoOpMetrics) PublishFailure(transport, exchange, reason string)                  {}
func (m NoOpMetrics) PublishDuration(transport, exchange string, duration time.Duration) {}
func (m NoOpMetrics) ConsumeTotal(transport, queue string)                               {}
func (m NoOpMetrics) ConsumeSuccess(transport, queue string)                             {}
func (m NoOpMetrics) ConsumeFailure(transport, queue, reason string)                     {}
func (m NoOpMetrics) ConsumeDuration(transport, queue string, duration time.Duration)    {}
func (m NoOpMetrics) ConnectionsActive(transport string, count int)                      {}
func (m NoOpMetrics) ChannelsActive(transport string, count int)                         {}
func (m NoOpMetrics) MessagesInFlight(transport string, count int)                       {}
func (m NoOpMetrics) BackpressureTotal(transport, mode string)                           {}
func (m NoOpMetrics) RetryTotal(transport, stage string)                                 {}
func (m NoOpMetrics) DroppedTotal(transport, reason string)                              {}
func (m NoOpMetrics) PersistenceTotal(operation string)                                  {}
func (m NoOpMetrics) PersistenceSuccess(operation string)                                {}
func (m NoOpMetrics) PersistenceFailure(operation, reason string)                        {}
func (m NoOpMetrics) PersistenceDuration(operation string, duration time.Duration)       {}
func (m NoOpMetrics) DLQTotal(operation string)                                          {}
func (m NoOpMetrics) DLQSuccess(operation string)                                        {}
func (m NoOpMetrics) DLQFailure(operation, reason string)                                {}
func (m NoOpMetrics) DLQDuration(operation string, duration time.Duration)               {}
func (m NoOpMetrics) TransformationTotal(operation string)                               {}
func (m NoOpMetrics) TransformationSuccess(operation string)                             {}
func (m NoOpMetrics) TransformationFailure(operation, reason string)                     {}
func (m NoOpMetrics) TransformationDuration(operation string, duration time.Duration)    {}
func (m NoOpMetrics) RoutingTotal(operation string)                                      {}
func (m NoOpMetrics) RoutingSuccess(operation string)                                    {}
func (m NoOpMetrics) RoutingFailure(operation, reason string)                            {}
func (m NoOpMetrics) RoutingDuration(operation string, duration time.Duration)           {}
func (m NoOpMetrics) PerformanceThroughput(operation string, throughput float64)         {}
func (m NoOpMetrics) PerformanceLatency(operation string, latencyNs int64)               {}
func (m NoOpMetrics) PerformanceMemory(operation string, bytes uint64)                   {}
func (m NoOpMetrics) PerformanceGC(operation string, value int64)                        {}
func (m NoOpMetrics) PerformanceErrors(operation string, value float64)                  {}
func (m NoOpMetrics) PerformanceResources(operation string, value int64)                 {}
func (m NoOpMetrics) ConfirmTotal(transport string)                                      {}
func (m NoOpMetrics) ConfirmSuccess(transport string)                                    {}
func (m NoOpMetrics) ConfirmFailure(transport, reason string)                            {}
func (m NoOpMetrics) ConfirmDuration(transport string, duration time.Duration)           {}
func (m NoOpMetrics) ReturnTotal(transport string)                                       {}
func (m NoOpMetrics) ReturnDuration(transport string, duration time.Duration)            {}

// NoOpTracer provides a no-op implementation of Tracer.
type NoOpTracer struct{}

func (t NoOpTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, &NoOpSpan{}
}

func (t NoOpTracer) Extract(ctx context.Context, headers map[string]string) context.Context {
	return ctx
}

func (t NoOpTracer) Inject(ctx context.Context, headers map[string]string) {}

// NoOpSpan provides a no-op implementation of Span.
type NoOpSpan struct{}

func (s *NoOpSpan) SetAttribute(key string, value interface{})              {}
func (s *NoOpSpan) SetStatus(status SpanStatus, description string)         {}
func (s *NoOpSpan) AddEvent(name string, attributes map[string]interface{}) {}
func (s *NoOpSpan) End()                                                    {}
func (s *NoOpSpan) Context() context.Context                                { return context.Background() }

// NoOpLogger provides a no-op implementation of Logger.
func NoOpLogger() *logx.Logger {
	logger, _ := logx.NewLogger()
	return logger
}

// DefaultTelemetryProvider returns a default telemetry provider with no-op implementations.
func DefaultTelemetryProvider() *TelemetryProvider {
	return &TelemetryProvider{
		Metrics: NoOpMetrics{},
		Tracer:  NoOpTracer{},
		Logger:  NoOpLogger(),
	}
}
