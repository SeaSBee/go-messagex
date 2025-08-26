// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Publisher defines the interface for asynchronous message publishing.
type Publisher interface {
	// PublishAsync publishes a message asynchronously and returns a receipt.
	// The method should return immediately without blocking on network I/O.
	PublishAsync(ctx context.Context, topic string, msg Message) (Receipt, error)

	// Close gracefully shuts down the publisher.
	// It waits for in-flight messages to be confirmed or times out based on context.
	Close(ctx context.Context) error
}

// Consumer defines the interface for message consumption.
type Consumer interface {
	// Start begins consuming messages with the provided handler.
	// The method should not block; consumption happens in background goroutines.
	Start(ctx context.Context, handler Handler) error

	// Stop gracefully stops consuming messages.
	// It waits for in-flight handlers to complete or times out based on context.
	Stop(ctx context.Context) error
}

// Handler defines the interface for processing consumed messages.
type Handler interface {
	// Process handles a delivered message.
	// Implementations should be idempotent as messages may be redelivered.
	// The method should return quickly and not block for extended periods.
	Process(ctx context.Context, msg Delivery) (AckDecision, error)
}

// HandlerFunc is a function adapter for Handler interface.
type HandlerFunc func(ctx context.Context, msg Delivery) (AckDecision, error)

// Process implements the Handler interface.
func (f HandlerFunc) Process(ctx context.Context, msg Delivery) (AckDecision, error) {
	if f == nil {
		return NackRequeue, errors.New("handler function is nil")
	}
	return f(ctx, msg)
}

// TransportFactory defines the interface for creating transport-specific implementations.
type TransportFactory interface {
	// NewPublisher creates a new publisher instance for the given configuration.
	NewPublisher(ctx context.Context, cfg *Config) (Publisher, error)

	// NewConsumer creates a new consumer instance for the given configuration.
	NewConsumer(ctx context.Context, cfg *Config) (Consumer, error)

	// ValidateConfig validates that the configuration is compatible with this transport.
	ValidateConfig(cfg *Config) error

	// Name returns the name of the transport (e.g., "rabbitmq", "kafka").
	Name() string
}

// Receipt represents the result of an async publish operation.
type Receipt interface {
	// Done returns a channel that's closed when the publish operation completes.
	Done() <-chan struct{}

	// Result returns the publish result and any error.
	// This method should only be called after Done() channel is closed.
	Result() (PublishResult, error)

	// Context returns the context associated with this receipt.
	Context() context.Context

	// ID returns a unique identifier for this publish operation.
	ID() string
}

// PublishResult contains the result of a publish operation.
type PublishResult struct {
	// MessageID is the unique identifier assigned to the published message.
	MessageID string

	// DeliveryTag is the transport-specific delivery tag (if applicable).
	DeliveryTag uint64

	// Timestamp is when the message was confirmed by the broker.
	Timestamp time.Time

	// Success indicates whether the publish was successful.
	Success bool

	// Reason provides additional information about the result (e.g., error reason).
	Reason string
}

// AckDecision represents the decision for acknowledging a consumed message.
type AckDecision int

const (
	// Ack acknowledges the message as successfully processed.
	Ack AckDecision = iota

	// NackRequeue rejects the message and requests redelivery.
	NackRequeue

	// NackDLQ rejects the message and sends it to dead letter queue.
	NackDLQ
)

// String returns the string representation of AckDecision.
func (d AckDecision) String() string {
	switch d {
	case Ack:
		return "ack"
	case NackRequeue:
		return "nack_requeue"
	case NackDLQ:
		return "nack_dlq"
	default:
		return "unknown"
	}
}

// TransportRegistry manages registered transport factories.
type TransportRegistry struct {
	mu        sync.RWMutex
	factories map[string]TransportFactory
}

// NewTransportRegistry creates a new transport registry.
func NewTransportRegistry() *TransportRegistry {
	return &TransportRegistry{
		factories: make(map[string]TransportFactory),
	}
}

// Register registers a transport factory with the given name.
func (r *TransportRegistry) Register(name string, factory TransportFactory) {
	if name == "" {
		panic("transport name cannot be empty")
	}
	if factory == nil {
		panic("transport factory cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get returns the transport factory for the given name.
func (r *TransportRegistry) Get(name string) (TransportFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.factories[name]
	return factory, exists
}

// List returns all registered transport names.
func (r *TransportRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// Default transport registry instance.
var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *TransportRegistry
)

func getDefaultRegistry() *TransportRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewTransportRegistry()
	})
	return defaultRegistry
}

// RegisterTransport registers a transport factory in the default registry.
func RegisterTransport(name string, factory TransportFactory) {
	getDefaultRegistry().Register(name, factory)
}

// GetTransport returns a transport factory from the default registry.
func GetTransport(name string) (TransportFactory, bool) {
	if name == "" {
		return nil, false
	}
	return getDefaultRegistry().Get(name)
}

// NewPublisher creates a new publisher using the default registry.
func NewPublisher(ctx context.Context, cfg *Config) (Publisher, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	factory, exists := GetTransport(cfg.Transport)
	if !exists {
		return nil, ErrUnsupportedTransport
	}
	return factory.NewPublisher(ctx, cfg)
}

// NewConsumer creates a new consumer using the default registry.
func NewConsumer(ctx context.Context, cfg *Config) (Consumer, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	factory, exists := GetTransport(cfg.Transport)
	if !exists {
		return nil, ErrUnsupportedTransport
	}
	return factory.NewConsumer(ctx, cfg)
}
