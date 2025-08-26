// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// TestConfig provides test configuration utilities.
type TestConfig struct {
	// Transport is the transport name for testing
	Transport string

	// Observability provides test observability
	Observability *ObservabilityContext

	// FailureRate is the failure rate for error injection (0.0-1.0)
	FailureRate float64

	// Latency is the simulated latency for operations
	Latency time.Duration

	// EnableMetrics enables metrics collection during testing
	EnableMetrics bool

	// EnableTracing enables tracing during testing
	EnableTracing bool
}

// Validate validates the test configuration.
func (tc *TestConfig) Validate() error {
	if tc.FailureRate < 0 || tc.FailureRate > 1 || math.IsNaN(tc.FailureRate) || math.IsInf(tc.FailureRate, 0) {
		return NewError(ErrorCodeConfiguration, "validate", "failure rate must be between 0.0 and 1.0")
	}
	if tc.Latency < 0 {
		return NewError(ErrorCodeConfiguration, "validate", "latency cannot be negative")
	}
	return nil
}

// GetObservabilityContext safely returns the observability context, creating a default one if nil.
func (tc *TestConfig) GetObservabilityContext(ctx context.Context) *ObservabilityContext {
	if tc.Observability != nil {
		return tc.Observability
	}

	// Create a minimal observability context if none exists
	obsProvider, _ := NewObservabilityProvider(&TelemetryConfig{
		MetricsEnabled: tc.EnableMetrics,
		TracingEnabled: tc.EnableTracing,
	})

	if obsProvider != nil {
		return NewObservabilityContext(ctx, obsProvider)
	}

	// Return a minimal context if provider creation fails
	return NewObservabilityContext(ctx, nil)
}

// NewTestConfig creates a new test configuration.
func NewTestConfig() *TestConfig {
	obsProvider, err := NewObservabilityProvider(&TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: true,
	})

	// If observability provider creation fails, create a minimal config
	if err != nil || obsProvider == nil {
		config := &TestConfig{
			Transport:     "test",
			Observability: nil, // Will be handled gracefully in usage
			FailureRate:   0.0,
			Latency:       0,
			EnableMetrics: false,
			EnableTracing: false,
		}
		// Validate the config
		if validateErr := config.Validate(); validateErr != nil {
			// Return a safe default if validation fails
			return &TestConfig{
				Transport:     "test",
				Observability: nil,
				FailureRate:   0.0,
				Latency:       0,
				EnableMetrics: false,
				EnableTracing: false,
			}
		}
		return config
	}

	config := &TestConfig{
		Transport:     "test",
		Observability: NewObservabilityContext(context.Background(), obsProvider),
		FailureRate:   0.0,
		Latency:       0,
		EnableMetrics: true,
		EnableTracing: true,
	}

	// Validate the config
	if validateErr := config.Validate(); validateErr != nil {
		// Return a safe default if validation fails
		return &TestConfig{
			Transport:     "test",
			Observability: NewObservabilityContext(context.Background(), obsProvider),
			FailureRate:   0.0,
			Latency:       0,
			EnableMetrics: true,
			EnableTracing: true,
		}
	}

	return config
}

// TestMessageFactory provides utilities for creating test messages.
type TestMessageFactory struct {
	counter uint64
}

// NewTestMessageFactory creates a new test message factory.
func NewTestMessageFactory() *TestMessageFactory {
	return &TestMessageFactory{}
}

// CreateMessage creates a test message with the given body.
func (tmf *TestMessageFactory) CreateMessage(body []byte, options ...MessageOption) Message {
	id := atomic.AddUint64(&tmf.counter, 1)

	// Add default test options
	defaultOptions := []MessageOption{
		WithID(fmt.Sprintf("test-msg-%d", id)),
		WithContentType("application/json"),
		WithTimestamp(time.Now()),
		WithKey("test.key"), // Add default routing key for tests
	}

	// Combine with provided options
	allOptions := append(defaultOptions, options...)

	msg := NewMessage(body, allOptions...)
	return *msg
}

// CreateJSONMessage creates a test message with JSON body.
func (tmf *TestMessageFactory) CreateJSONMessage(data interface{}, options ...MessageOption) Message {
	// Handle nil data gracefully
	if data == nil {
		data = "null"
	}

	body := []byte(fmt.Sprintf(`{"test": %v, "timestamp": "%s"}`, data, time.Now().Format(time.RFC3339)))
	return tmf.CreateMessage(body, options...)
}

// CreateBulkMessages creates multiple test messages.
func (tmf *TestMessageFactory) CreateBulkMessages(count int, bodyTemplate string, options ...MessageOption) []Message {
	if count <= 0 {
		return []Message{}
	}

	messages := make([]Message, count)
	for i := 0; i < count; i++ {
		body := []byte(fmt.Sprintf(bodyTemplate, i))
		messages[i] = tmf.CreateMessage(body, options...)
	}
	return messages
}

// MockTransport provides a mock transport for testing.
type MockTransport struct {
	config       *TestConfig
	publishers   map[string]*MockPublisher
	consumers    map[string]*MockConsumer
	topology     map[string]interface{}
	connected    bool
	mu           sync.RWMutex
	publishCount uint64
	consumeCount uint64
	errorCount   uint64
}

// NewMockTransport creates a new mock transport.
func NewMockTransport(config *TestConfig) *MockTransport {
	// Use default config if nil is provided
	if config == nil {
		config = NewTestConfig()
	}

	return &MockTransport{
		config:     config,
		publishers: make(map[string]*MockPublisher),
		consumers:  make(map[string]*MockConsumer),
		topology:   make(map[string]interface{}),
		connected:  true,
	}
}

// NewPublisher creates a new mock publisher.
func (mt *MockTransport) NewPublisher(config *PublisherConfig, observability *ObservabilityContext) Publisher {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Use safe observability context if provided one is nil
	if observability == nil {
		observability = mt.config.GetObservabilityContext(context.Background())
	}

	id := fmt.Sprintf("publisher-%d", len(mt.publishers))
	publisher := NewMockPublisher(id, mt.config, mt)
	mt.publishers[id] = publisher
	return publisher
}

// NewConsumer creates a new mock consumer.
func (mt *MockTransport) NewConsumer(config *ConsumerConfig, observability *ObservabilityContext) Consumer {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Use safe observability context if provided one is nil
	if observability == nil {
		observability = mt.config.GetObservabilityContext(context.Background())
	}

	id := fmt.Sprintf("consumer-%d", len(mt.consumers))
	consumer := NewMockConsumer(id, mt.config, mt)
	mt.consumers[id] = consumer
	return consumer
}

// Connect simulates connecting to the transport.
func (mt *MockTransport) Connect(ctx context.Context) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if mt.config.FailureRate > 0 && shouldFail(mt.config.FailureRate) {
		atomic.AddUint64(&mt.errorCount, 1)
		return NewError(ErrorCodeConnection, "connect", "simulated connection failure")
	}

	// Simulate connection latency with context awareness
	if mt.config.Latency > 0 {
		select {
		case <-time.After(mt.config.Latency):
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "connect", "connection timeout")
		}
	}

	mt.connected = true
	return nil
}

// Disconnect simulates disconnecting from the transport.
func (mt *MockTransport) Disconnect(ctx context.Context) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.connected = false
	return nil
}

// Cleanup cleans up all resources and closes all publishers and consumers.
func (mt *MockTransport) Cleanup(ctx context.Context) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Close all publishers
	for _, publisher := range mt.publishers {
		if publisher != nil {
			publisher.Close(ctx)
		}
	}

	// Close all consumers
	for _, consumer := range mt.consumers {
		if consumer != nil {
			consumer.Close(ctx)
		}
	}

	// Clear maps to prevent memory leaks
	mt.publishers = make(map[string]*MockPublisher)
	mt.consumers = make(map[string]*MockConsumer)
	mt.topology = make(map[string]interface{})
	mt.connected = false

	return nil
}

// IsConnected returns the connection status.
func (mt *MockTransport) IsConnected() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.connected
}

// GetStats returns transport statistics.
func (mt *MockTransport) GetStats() map[string]interface{} {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	return map[string]interface{}{
		"publishers":   len(mt.publishers),
		"consumers":    len(mt.consumers),
		"publishCount": atomic.LoadUint64(&mt.publishCount),
		"consumeCount": atomic.LoadUint64(&mt.consumeCount),
		"errorCount":   atomic.LoadUint64(&mt.errorCount),
		"connected":    mt.connected,
	}
}

// MockPublisher provides a mock publisher for testing.
type MockPublisher struct {
	id        string
	transport *MockTransport
	config    *TestConfig
	receipts  map[string]*MockReceipt
	closed    bool
	mu        sync.RWMutex
}

// NewMockPublisher creates a new mock publisher.
func NewMockPublisher(id string, config *TestConfig, transport *MockTransport) *MockPublisher {
	return &MockPublisher{
		id:        id,
		transport: transport,
		config:    config,
		receipts:  make(map[string]*MockReceipt),
	}
}

// PublishAsync publishes a message asynchronously.
func (mp *MockPublisher) PublishAsync(ctx context.Context, topic string, msg Message) (Receipt, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.closed {
		return nil, NewError(ErrorCodePublish, "publish_async", "publisher is closed")
	}

	if mp.config.FailureRate > 0 && shouldFail(mp.config.FailureRate) {
		atomic.AddUint64(&mp.transport.errorCount, 1)
		return nil, NewError(ErrorCodePublish, "publish_async", "simulated publish failure")
	}

	// Create receipt
	receipt := NewMockReceipt(msg.ID, mp.config)
	mp.receipts[msg.ID] = receipt

	// Simulate async processing with context cancellation
	go func() {
		// Create a context with timeout for processing
		processCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Simulate processing latency with context awareness
		if mp.config.Latency > 0 {
			select {
			case <-time.After(mp.config.Latency):
			case <-processCtx.Done():
				receipt.Complete(PublishResult{}, NewError(ErrorCodePublish, "publish_async", "processing timeout"))
				return
			}
		}

		// Check if the original context was cancelled
		select {
		case <-ctx.Done():
			receipt.Complete(PublishResult{}, NewError(ErrorCodePublish, "publish_async", "context cancelled"))
			return
		default:
		}

		// Complete the receipt
		receipt.Complete(PublishResult{
			MessageID:   msg.ID,
			DeliveryTag: uint64(time.Now().UnixNano()),
			Timestamp:   time.Now(),
			Success:     true,
			Reason:      "published successfully",
		}, nil)

		atomic.AddUint64(&mp.transport.publishCount, 1)
	}()

	return receipt, nil
}

// Close closes the publisher.
func (mp *MockPublisher) Close(ctx context.Context) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.closed = true
	return nil
}

// MockConsumer provides a mock consumer for testing.
type MockConsumer struct {
	id        string
	transport *MockTransport
	config    *TestConfig
	handler   Handler
	running   bool
	closed    bool
	mu        sync.RWMutex
}

// NewMockConsumer creates a new mock consumer.
func NewMockConsumer(id string, config *TestConfig, transport *MockTransport) *MockConsumer {
	return &MockConsumer{
		id:        id,
		transport: transport,
		config:    config,
	}
}

// Start starts consuming messages.
func (mc *MockConsumer) Start(ctx context.Context, handler Handler) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return NewError(ErrorCodeConsume, "start", "consumer is closed")
	}

	if mc.running {
		return NewError(ErrorCodeConsume, "start", "consumer is already running")
	}

	mc.handler = handler
	mc.running = true

	// Start consuming in background
	go mc.consumeLoop(ctx)

	return nil
}

// Stop stops consuming messages.
func (mc *MockConsumer) Stop(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.running = false
	return nil
}

// Close closes the consumer.
func (mc *MockConsumer) Close(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.running = false
	mc.closed = true
	return nil
}

// SimulateMessage simulates receiving a message.
func (mc *MockConsumer) SimulateMessage(msg Message) error {
	mc.mu.RLock()
	handler := mc.handler
	running := mc.running
	mc.mu.RUnlock()

	if !running || handler == nil {
		return NewError(ErrorCodeConsume, "simulate_message", "consumer not running")
	}

	// Create delivery
	delivery := NewMockDelivery(msg, mc.config)

	// Process message with context cancellation support
	go func() {
		// Create a context with timeout for processing
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Simulate processing latency with context awareness
		if mc.config.Latency > 0 {
			select {
			case <-time.After(mc.config.Latency):
			case <-ctx.Done():
				return
			}
		}

		decision, err := handler.Process(ctx, delivery.Delivery)
		if err != nil {
			atomic.AddUint64(&mc.transport.errorCount, 1)
		} else {
			atomic.AddUint64(&mc.transport.consumeCount, 1)
		}

		// Handle ack decision
		switch decision {
		case Ack:
			delivery.Ack()
		case NackRequeue:
			delivery.Nack(true)
		case NackDLQ:
			delivery.Nack(false)
		}
	}()

	return nil
}

// consumeLoop simulates the consume loop.
func (mc *MockConsumer) consumeLoop(ctx context.Context) {
	factory := NewTestMessageFactory()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.mu.RLock()
			running := mc.running
			mc.mu.RUnlock()

			if !running {
				return
			}

			// Simulate receiving a message with context awareness
			select {
			case <-ctx.Done():
				return
			default:
				msg := factory.CreateJSONMessage("test-data")
				// Use a separate goroutine to avoid blocking the consume loop
				go func() {
					if err := mc.SimulateMessage(msg); err != nil {
						// Log error but don't stop the loop
						atomic.AddUint64(&mc.transport.errorCount, 1)
					}
				}()
			}
		}
	}
}

// MockReceipt provides a mock receipt for testing.
type MockReceipt struct {
	id      string
	ctx     context.Context
	done    chan struct{}
	result  PublishResult
	err     error
	config  *TestConfig
	mu      sync.RWMutex
	_cancel context.CancelFunc
}

// NewMockReceipt creates a new mock receipt.
func NewMockReceipt(id string, config *TestConfig) *MockReceipt {
	// Create a cancellable context with timeout to prevent goroutine leaks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	return &MockReceipt{
		id:     id,
		ctx:    ctx,
		done:   make(chan struct{}),
		config: config,
		// Store cancel function for cleanup (though not exposed in interface)
		_cancel: cancel,
	}
}

// Done returns the done channel.
func (mr *MockReceipt) Done() <-chan struct{} {
	return mr.done
}

// Result returns the publish result.
func (mr *MockReceipt) Result() (PublishResult, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return mr.result, mr.err
}

// Context returns the context.
func (mr *MockReceipt) Context() context.Context {
	return mr.ctx
}

// ID returns the receipt ID.
func (mr *MockReceipt) ID() string {
	return mr.id
}

// Complete completes the receipt.
func (mr *MockReceipt) Complete(result PublishResult, err error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Prevent double completion
	if mr.result.MessageID != "" || mr.err != nil {
		return
	}

	mr.result = result
	mr.err = err
	close(mr.done)

	// Cancel the context to prevent goroutine leaks
	if mr._cancel != nil {
		mr._cancel()
	}
}

// IsCompleted returns whether the receipt has been completed.
func (mr *MockReceipt) IsCompleted() bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return mr.result.MessageID != "" || mr.err != nil
}

// MockDelivery provides a mock delivery for testing.
type MockDelivery struct {
	Delivery
	config   *TestConfig
	acked    bool
	nacked   bool
	rejected bool
	mu       sync.RWMutex
}

// NewMockDelivery creates a new mock delivery.
func NewMockDelivery(msg Message, config *TestConfig) *MockDelivery {
	return &MockDelivery{
		Delivery: Delivery{
			Message:     msg,
			DeliveryTag: uint64(time.Now().UnixNano()),
			Exchange:    "test.exchange",
			RoutingKey:  "test.key",
			Queue:       "test.queue",
		},
		config: config,
	}
}

// Ack acknowledges the message.
func (md *MockDelivery) Ack() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.acked || md.nacked || md.rejected {
		return NewError(ErrorCodeConsume, "ack", "message already acknowledged")
	}

	md.acked = true
	return nil
}

// Nack negatively acknowledges the message.
func (md *MockDelivery) Nack(requeue bool) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.acked || md.nacked || md.rejected {
		return NewError(ErrorCodeConsume, "nack", "message already acknowledged")
	}

	if requeue {
		md.nacked = true
	} else {
		md.rejected = true
	}
	return nil
}

// IsAcked returns whether the message was acknowledged.
func (md *MockDelivery) IsAcked() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.acked
}

// IsNacked returns whether the message was nacked.
func (md *MockDelivery) IsNacked() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.nacked
}

// IsRejected returns whether the message was rejected.
func (md *MockDelivery) IsRejected() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.rejected
}

// shouldFail determines if an operation should fail based on failure rate.
func shouldFail(rate float64) bool {
	// Handle edge cases
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return false
	}
	if rate >= 1 {
		return true
	}
	// Simple pseudo-random based on current time
	return float64(time.Now().UnixNano()%1000)/1000.0 < rate
}

// TestRunner provides utilities for running tests.
type TestRunner struct {
	config  *TestConfig
	factory *TestMessageFactory
}

// NewTestRunner creates a new test runner.
func NewTestRunner(config *TestConfig) *TestRunner {
	// Use default config if nil is provided
	if config == nil {
		config = NewTestConfig()
	}

	return &TestRunner{
		config:  config,
		factory: NewTestMessageFactory(),
	}
}

// RunPublishTest runs a publish test scenario.
func (tr *TestRunner) RunPublishTest(ctx context.Context, publisher Publisher, messageCount int) (*TestResult, error) {
	if publisher == nil {
		return nil, NewError(ErrorCodePublish, "run_publish_test", "publisher is nil")
	}

	if messageCount <= 0 {
		return &TestResult{
			Duration:     0,
			MessageCount: 0,
			SuccessCount: 0,
			ErrorCount:   0,
			Throughput:   0,
		}, nil
	}

	start := time.Now()
	var successCount, errorCount uint64

	// Create messages
	messages := tr.factory.CreateBulkMessages(messageCount, `{"test": "message-%d"}`)

	// Publish messages
	receipts := make([]Receipt, len(messages))
	for i, msg := range messages {
		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		if err != nil {
			atomic.AddUint64(&errorCount, 1)
			continue
		}
		if receipt == nil {
			atomic.AddUint64(&errorCount, 1)
			continue
		}
		receipts[i] = receipt
	}

	// Wait for completions
	for _, receipt := range receipts {
		if receipt == nil {
			continue
		}

		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			if err != nil {
				atomic.AddUint64(&errorCount, 1)
			} else {
				atomic.AddUint64(&successCount, 1)
			}
		case <-ctx.Done():
			atomic.AddUint64(&errorCount, 1)
		}
	}

	duration := time.Since(start)

	// Calculate throughput safely to avoid division by zero
	var throughput float64
	if duration > 0 {
		throughput = float64(successCount) / duration.Seconds()
	}

	return &TestResult{
		Duration:     duration,
		MessageCount: uint64(messageCount),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		Throughput:   throughput,
	}, nil
}

// TestResult contains test execution results.
type TestResult struct {
	Duration     time.Duration
	MessageCount uint64
	SuccessCount uint64
	ErrorCount   uint64
	Throughput   float64
}

// SuccessRate returns the success rate as a percentage.
func (tr *TestResult) SuccessRate() float64 {
	if tr.MessageCount == 0 {
		return 0
	}
	return float64(tr.SuccessCount) / float64(tr.MessageCount) * 100
}

// ErrorRate returns the error rate as a percentage.
func (tr *TestResult) ErrorRate() float64 {
	if tr.MessageCount == 0 {
		return 0
	}
	return float64(tr.ErrorCount) / float64(tr.MessageCount) * 100
}
