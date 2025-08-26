package unit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

func TestCorrelationID(t *testing.T) {
	// Test new correlation ID generation
	id1 := messaging.NewCorrelationID()
	id2 := messaging.NewCorrelationID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.True(t, id1.IsValid())
	assert.True(t, id2.IsValid())

	// Test string conversion
	assert.Equal(t, string(id1), id1.String())

	// Test empty correlation ID
	emptyID := messaging.CorrelationID("")
	assert.False(t, emptyID.IsValid())
}

func TestCorrelationContext(t *testing.T) {
	// Test new correlation context
	corrCtx := messaging.NewCorrelationContext()
	assert.NotNil(t, corrCtx)
	assert.True(t, corrCtx.ID().IsValid())
	assert.NotZero(t, corrCtx.CreatedAt())

	// Test correlation context with specific ID
	id := messaging.NewCorrelationID()
	corrCtxWithID := messaging.NewCorrelationContextWithID(id)
	assert.Equal(t, id, corrCtxWithID.ID())

	// Test parent-child relationship
	parent := messaging.NewCorrelationContext()
	child := messaging.NewCorrelationContextFromParent(parent)

	assert.NotEqual(t, parent.ID(), child.ID())
	assert.Equal(t, parent.ID(), child.ParentID())
	assert.Equal(t, parent.TraceID(), child.TraceID())
	assert.Equal(t, parent.SpanID(), child.SpanID())

	// Test metadata operations
	corrCtx.SetMetadata("key1", "value1")
	corrCtx.SetMetadata("key2", "value2")

	value, exists := corrCtx.GetMetadata("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	value, exists = corrCtx.GetMetadata("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, value)

	metadata := corrCtx.Metadata()
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, "value2", metadata["key2"])

	// Test trace and span ID operations
	corrCtx.SetTraceID("trace-123")
	corrCtx.SetSpanID("span-456")

	assert.Equal(t, "trace-123", corrCtx.TraceID())
	assert.Equal(t, "span-456", corrCtx.SpanID())
}

func TestCorrelationContextIntegration(t *testing.T) {
	// Test context integration
	ctx := context.Background()
	corrCtx := messaging.NewCorrelationContext()

	ctxWithCorr := corrCtx.ToContext(ctx)

	extractedCtx, exists := messaging.FromContext(ctxWithCorr)
	assert.True(t, exists)
	assert.Equal(t, corrCtx.ID(), extractedCtx.ID())

	// Test get or create
	existingCtx := messaging.GetOrCreateCorrelationContext(ctxWithCorr)
	assert.Equal(t, corrCtx.ID(), existingCtx.ID())

	newCtx := messaging.GetOrCreateCorrelationContext(context.Background())
	assert.NotNil(t, newCtx)
	assert.True(t, newCtx.ID().IsValid())
}

func TestCorrelationManager(t *testing.T) {
	manager := messaging.NewCorrelationManager(10)

	// Test registration
	corrCtx1 := messaging.NewCorrelationContext()
	err := manager.Register(corrCtx1)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.ActiveCount())

	// Test duplicate registration
	err = manager.Register(corrCtx1)
	assert.NoError(t, err) // Should allow duplicate registration
	assert.Equal(t, 1, manager.ActiveCount())

	// Test retrieval
	retrieved, exists := manager.Get(corrCtx1.ID())
	assert.True(t, exists)
	assert.Equal(t, corrCtx1.ID(), retrieved.ID())

	// Test unregistration
	manager.Unregister(corrCtx1.ID())
	assert.Equal(t, 0, manager.ActiveCount())

	retrieved, exists = manager.Get(corrCtx1.ID())
	assert.False(t, exists)
	assert.Nil(t, retrieved)

	// Test max active limit
	for i := 0; i < 11; i++ {
		corrCtx := messaging.NewCorrelationContext()
		err := manager.Register(corrCtx)
		if i < 10 {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestCorrelationManagerCleanup(t *testing.T) {
	manager := messaging.NewCorrelationManager(100)

	// Add some correlation contexts
	corrCtx1 := messaging.NewCorrelationContext()
	corrCtx2 := messaging.NewCorrelationContext()

	manager.Register(corrCtx1)
	manager.Register(corrCtx2)

	assert.Equal(t, 2, manager.ActiveCount())

	// Cleanup with very short max age
	removed := manager.Cleanup(1 * time.Nanosecond)
	assert.Equal(t, 2, removed)
	assert.Equal(t, 0, manager.ActiveCount())

	// Test list
	list := manager.List()
	assert.Len(t, list, 0)
}

func TestCorrelationPropagator(t *testing.T) {
	manager := messaging.NewCorrelationManager(100)
	propagator := messaging.NewCorrelationPropagator(manager)

	// Test injection
	ctx := context.Background()
	corrCtx := messaging.NewCorrelationContext()
	corrCtx.SetMetadata("user_id", "123")
	corrCtx.SetTraceID("trace-123")
	corrCtx.SetSpanID("span-456")

	ctx = corrCtx.ToContext(ctx)
	headers := make(map[string]string)

	propagator.Inject(ctx, headers)

	assert.Equal(t, corrCtx.ID().String(), headers["X-Correlation-ID"])
	assert.Equal(t, "trace-123", headers["X-Trace-ID"])
	assert.Equal(t, "span-456", headers["X-Span-ID"])
	assert.Equal(t, "123", headers["X-Correlation-Metadata-user_id"])

	// Test extraction
	extractedCtx := propagator.Extract(context.Background(), headers)
	extractedCorrCtx, exists := messaging.FromContext(extractedCtx)
	assert.True(t, exists)
	assert.Equal(t, corrCtx.ID(), extractedCorrCtx.ID())
	assert.Equal(t, "trace-123", extractedCorrCtx.TraceID())
	assert.Equal(t, "span-456", extractedCorrCtx.SpanID())

	value, exists := extractedCorrCtx.GetMetadata("user_id")
	assert.True(t, exists)
	assert.Equal(t, "123", value)

	// Test child creation
	childCtx := propagator.CreateChild(ctx)
	childCorrCtx, exists := messaging.FromContext(childCtx)
	assert.True(t, exists)
	assert.NotEqual(t, corrCtx.ID(), childCorrCtx.ID())
	assert.Equal(t, corrCtx.ID(), childCorrCtx.ParentID())
}

func TestCorrelationMiddleware(t *testing.T) {
	manager := messaging.NewCorrelationManager(100)
	propagator := messaging.NewCorrelationPropagator(manager)
	middleware := messaging.NewCorrelationMiddleware(propagator)

	// Test publisher middleware
	publisherMiddleware := middleware.PublisherMiddleware()
	ctx := context.Background()
	msg := messaging.NewMessage([]byte("test"), messaging.WithID("msg-123"), messaging.WithKey("test.key"))

	ctx, modifiedMsg := publisherMiddleware(ctx, "test.topic", *msg)

	assert.NotEmpty(t, modifiedMsg.CorrelationID)
	assert.NotEmpty(t, modifiedMsg.Headers["X-Correlation-ID"])

	corrCtx, exists := messaging.FromContext(ctx)
	assert.True(t, exists)
	assert.Equal(t, modifiedMsg.CorrelationID, corrCtx.ID().String())

	// Test consumer middleware
	consumerMiddleware := middleware.ConsumerMiddleware()
	delivery := messaging.Delivery{
		Message:     modifiedMsg,
		Queue:       "test.queue",
		Exchange:    "test.exchange",
		RoutingKey:  "test.key",
		DeliveryTag: 1,
	}

	consumerCtx := consumerMiddleware(ctx, delivery)
	consumerCorrCtx, exists := messaging.FromContext(consumerCtx)
	assert.True(t, exists)
	assert.Equal(t, modifiedMsg.CorrelationID, consumerCorrCtx.ID().String())

	// Test handler middleware
	handlerMiddleware := middleware.HandlerMiddleware()
	handlerCtx := handlerMiddleware(consumerCtx, delivery)
	handlerCorrCtx, exists := messaging.FromContext(handlerCtx)
	assert.True(t, exists)
	assert.NotEqual(t, consumerCorrCtx.ID(), handlerCorrCtx.ID())
	assert.Equal(t, consumerCorrCtx.ID(), handlerCorrCtx.ParentID())
}

func TestSamplingConfig(t *testing.T) {
	// Test sampling configuration
	config := messaging.SamplingConfig{
		Enabled: true,
		Rate:    0.5,
		Seed:    12345,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 0.5, config.Rate)
	assert.Equal(t, int64(12345), config.Seed)
}

func TestAdvancedTelemetryProvider(t *testing.T) {
	// Create basic telemetry components
	metrics := messaging.NoOpMetrics{}
	tracer := messaging.NoOpTracer{}
	logger := messaging.NoOpLogger()

	samplingConfig := messaging.SamplingConfig{
		Enabled: true,
		Rate:    1.0, // 100% sampling for testing
		Seed:    12345,
	}

	batchingConfig := messaging.BatchingConfig{
		Enabled:       false,
		BatchSize:     100,
		FlushInterval: 1 * time.Second,
		MaxWaitTime:   5 * time.Second,
	}

	provider := messaging.NewAdvancedTelemetryProvider(
		metrics,
		tracer,
		logger,
		samplingConfig,
		batchingConfig,
	)

	assert.NotNil(t, provider)
	assert.True(t, provider.ShouldSample()) // Should always sample with 100% rate
}

func TestSampledMetrics(t *testing.T) {
	// Create a mock metrics implementation for testing
	mockMetrics := &MockMetrics{
		publishTotal:   0,
		publishSuccess: 0,
		publishFailure: 0,
	}

	samplingConfig := messaging.SamplingConfig{
		Enabled: true,
		Rate:    1.0, // 100% sampling
		Seed:    12345,
	}

	provider := messaging.NewAdvancedTelemetryProvider(
		mockMetrics,
		messaging.NoOpTracer{},
		messaging.NoOpLogger(),
		samplingConfig,
		messaging.BatchingConfig{},
	)

	sampledMetrics := messaging.NewSampledMetrics(mockMetrics, provider)

	// Test sampled metrics
	sampledMetrics.PublishTotal("rabbitmq", "test.exchange")
	sampledMetrics.PublishSuccess("rabbitmq", "test.exchange")
	sampledMetrics.PublishFailure("rabbitmq", "test.exchange", "test_error")

	assert.Equal(t, 1, mockMetrics.publishTotal)
	assert.Equal(t, 1, mockMetrics.publishSuccess)
	assert.Equal(t, 1, mockMetrics.publishFailure)
}

func TestBatchingConfig(t *testing.T) {
	// Test batching configuration
	config := messaging.BatchingConfig{
		Enabled:       true,
		BatchSize:     100,
		FlushInterval: 1 * time.Second,
		MaxWaitTime:   5 * time.Second,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 1*time.Second, config.FlushInterval)
	assert.Equal(t, 5*time.Second, config.MaxWaitTime)
}

func TestBatchedMetrics(t *testing.T) {
	// Create a mock metrics implementation for testing
	mockMetrics := &MockMetrics{
		publishTotal:   0,
		publishSuccess: 0,
		publishFailure: 0,
	}

	batchingConfig := messaging.BatchingConfig{
		Enabled:       true,
		BatchSize:     5,
		FlushInterval: 100 * time.Millisecond,
		MaxWaitTime:   1 * time.Second,
	}

	batchedMetrics := messaging.NewBatchedMetrics(mockMetrics, batchingConfig)
	defer batchedMetrics.Close()

	// Test batched metrics
	for i := 0; i < 10; i++ {
		batchedMetrics.PublishTotal("rabbitmq", "test.exchange")
		batchedMetrics.PublishSuccess("rabbitmq", "test.exchange")
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Should have processed all metrics
	assert.Equal(t, 10, mockMetrics.publishTotal)
	assert.Equal(t, 10, mockMetrics.publishSuccess)
}

func TestTelemetryMiddleware(t *testing.T) {
	// Create basic telemetry components
	metrics := messaging.NoOpMetrics{}
	tracer := messaging.NoOpTracer{}
	logger := messaging.NoOpLogger()

	provider := messaging.NewAdvancedTelemetryProvider(
		metrics,
		tracer,
		logger,
		messaging.SamplingConfig{},
		messaging.BatchingConfig{},
	)

	middleware := messaging.NewTelemetryMiddleware(provider)

	// Test publisher telemetry middleware
	publisherMiddleware := middleware.PublisherTelemetryMiddleware()
	ctx := context.Background()
	msg := messaging.NewMessage([]byte("test"), messaging.WithID("msg-123"), messaging.WithKey("test.key"))

	ctx, modifiedMsg := publisherMiddleware(ctx, "test.topic", *msg)

	assert.NotEmpty(t, modifiedMsg.Headers["X-Telemetry-Start"])

	// Test consumer telemetry middleware
	consumerMiddleware := middleware.ConsumerTelemetryMiddleware()
	delivery := messaging.Delivery{
		Message:     modifiedMsg,
		Queue:       "test.queue",
		Exchange:    "test.exchange",
		RoutingKey:  "test.key",
		DeliveryTag: 1,
	}

	consumerCtx := consumerMiddleware(ctx, delivery)

	// The telemetry middleware uses custom context key types, not string keys
	// We can't directly access them from the test, but we can verify the context was modified
	assert.NotEqual(t, ctx, consumerCtx)

	// Test handler telemetry middleware
	handlerMiddleware := middleware.HandlerTelemetryMiddleware()
	handlerCtx, _ := handlerMiddleware(consumerCtx, delivery)

	// Verify the handler context was modified
	assert.NotEqual(t, consumerCtx, handlerCtx)
}

// MockMetrics is a simple mock implementation for testing
type MockMetrics struct {
	publishTotal   int
	publishSuccess int
	publishFailure int
	mu             sync.Mutex
}

func (m *MockMetrics) PublishTotal(transport, exchange string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishTotal++
}

func (m *MockMetrics) PublishSuccess(transport, exchange string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishSuccess++
}

func (m *MockMetrics) PublishFailure(transport, exchange, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishFailure++
}

func (m *MockMetrics) PublishDuration(transport, exchange string, duration time.Duration) {}
func (m *MockMetrics) ConsumeTotal(transport, queue string)                               {}
func (m *MockMetrics) ConsumeSuccess(transport, queue string)                             {}
func (m *MockMetrics) ConsumeFailure(transport, queue, reason string)                     {}
func (m *MockMetrics) ConsumeDuration(transport, queue string, duration time.Duration)    {}
func (m *MockMetrics) ConnectionsActive(transport string, count int)                      {}
func (m *MockMetrics) ChannelsActive(transport string, count int)                         {}
func (m *MockMetrics) MessagesInFlight(transport string, count int)                       {}
func (m *MockMetrics) BackpressureTotal(transport, mode string)                           {}
func (m *MockMetrics) RetryTotal(transport, stage string)                                 {}
func (m *MockMetrics) DroppedTotal(transport, reason string)                              {}

// Advanced feature metrics
func (m *MockMetrics) PersistenceTotal(operation string)                               {}
func (m *MockMetrics) PersistenceSuccess(operation string)                             {}
func (m *MockMetrics) PersistenceFailure(operation, reason string)                     {}
func (m *MockMetrics) PersistenceDuration(operation string, duration time.Duration)    {}
func (m *MockMetrics) DLQTotal(operation string)                                       {}
func (m *MockMetrics) DLQSuccess(operation string)                                     {}
func (m *MockMetrics) DLQFailure(operation, reason string)                             {}
func (m *MockMetrics) DLQDuration(operation string, duration time.Duration)            {}
func (m *MockMetrics) TransformationTotal(operation string)                            {}
func (m *MockMetrics) TransformationSuccess(operation string)                          {}
func (m *MockMetrics) TransformationFailure(operation, reason string)                  {}
func (m *MockMetrics) TransformationDuration(operation string, duration time.Duration) {}
func (m *MockMetrics) RoutingTotal(operation string)                                   {}
func (m *MockMetrics) RoutingSuccess(operation string)                                 {}
func (m *MockMetrics) RoutingFailure(operation, reason string)                         {}
func (m *MockMetrics) RoutingDuration(operation string, duration time.Duration)        {}

// Performance metrics
func (m *MockMetrics) PerformanceThroughput(operation string, throughput float64) {}
func (m *MockMetrics) PerformanceLatency(operation string, latencyNs int64)       {}
func (m *MockMetrics) PerformanceMemory(operation string, bytes uint64)           {}
func (m *MockMetrics) PerformanceGC(operation string, value int64)                {}
func (m *MockMetrics) PerformanceErrors(operation string, value float64)          {}
func (m *MockMetrics) PerformanceResources(operation string, value int64)         {}

// Publisher confirmation metrics
func (m *MockMetrics) ConfirmTotal(transport string)                            {}
func (m *MockMetrics) ConfirmSuccess(transport string)                          {}
func (m *MockMetrics) ConfirmFailure(transport, reason string)                  {}
func (m *MockMetrics) ConfirmDuration(transport string, duration time.Duration) {}

// Return metrics
func (m *MockMetrics) ReturnTotal(transport string)                            {}
func (m *MockMetrics) ReturnDuration(transport string, duration time.Duration) {}
