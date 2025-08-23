// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
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

// NewTestConfig creates a new test configuration.
func NewTestConfig() *TestConfig {
	obsProvider, _ := NewObservabilityProvider(&TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: true,
	})

	return &TestConfig{
		Transport:     "test",
		Observability: NewObservabilityContext(context.Background(), obsProvider),
		FailureRate:   0.0,
		Latency:       0,
		EnableMetrics: true,
		EnableTracing: true,
	}
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
	}

	// Combine with provided options
	allOptions := append(defaultOptions, options...)

	return NewMessage(body, allOptions...)
}

// CreateJSONMessage creates a test message with JSON body.
func (tmf *TestMessageFactory) CreateJSONMessage(data interface{}, options ...MessageOption) Message {
	body := []byte(fmt.Sprintf(`{"test": %v, "timestamp": "%s"}`, data, time.Now().Format(time.RFC3339)))
	return tmf.CreateMessage(body, options...)
}

// CreateBulkMessages creates multiple test messages.
func (tmf *TestMessageFactory) CreateBulkMessages(count int, bodyTemplate string, options ...MessageOption) []Message {
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

	id := fmt.Sprintf("publisher-%d", len(mt.publishers))
	publisher := NewMockPublisher(id, mt.config, mt)
	mt.publishers[id] = publisher
	return publisher
}

// NewConsumer creates a new mock consumer.
func (mt *MockTransport) NewConsumer(config *ConsumerConfig, observability *ObservabilityContext) Consumer {
	mt.mu.Lock()
	defer mt.mu.Unlock()

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

	// Simulate connection latency
	if mt.config.Latency > 0 {
		time.Sleep(mt.config.Latency)
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

	// Simulate async processing
	go func() {
		// Simulate processing latency
		if mp.config.Latency > 0 {
			time.Sleep(mp.config.Latency)
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

	// Process message
	go func() {
		// Simulate processing latency
		if mc.config.Latency > 0 {
			time.Sleep(mc.config.Latency)
		}

		decision, err := handler.Process(context.Background(), delivery.Delivery)
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

			// Simulate receiving a message
			msg := factory.CreateJSONMessage("test-data")
			mc.SimulateMessage(msg)
		}
	}
}

// MockReceipt provides a mock receipt for testing.
type MockReceipt struct {
	id     string
	ctx    context.Context
	done   chan struct{}
	result PublishResult
	err    error
	config *TestConfig
	mu     sync.RWMutex
}

// NewMockReceipt creates a new mock receipt.
func NewMockReceipt(id string, config *TestConfig) *MockReceipt {
	return &MockReceipt{
		id:     id,
		ctx:    context.Background(),
		done:   make(chan struct{}),
		config: config,
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

	mr.result = result
	mr.err = err
	close(mr.done)
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
	if rate <= 0 {
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
	return &TestRunner{
		config:  config,
		factory: NewTestMessageFactory(),
	}
}

// RunPublishTest runs a publish test scenario.
func (tr *TestRunner) RunPublishTest(ctx context.Context, publisher Publisher, messageCount int) (*TestResult, error) {
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

	return &TestResult{
		Duration:     duration,
		MessageCount: uint64(messageCount),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		Throughput:   float64(successCount) / duration.Seconds(),
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
