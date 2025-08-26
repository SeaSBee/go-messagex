package rabbitmq

import (
	"context"
	"regexp"
	"sync"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

// Cached regex pattern for queue name validation
var queueNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type Consumer struct {
	concurrentConsumer *ConcurrentConsumer
	config             *messaging.ConsumerConfig
	mu                 sync.RWMutex
	closed             bool
}

func NewConsumer(transport *Transport, config *messaging.ConsumerConfig, observability *messaging.ObservabilityContext) (*Consumer, error) {
	// Validate input parameters
	if transport == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_consumer", "transport cannot be nil")
	}
	if config == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_consumer", "config cannot be nil")
	}

	concurrentConsumer := NewConcurrentConsumer(transport, config, observability)
	if concurrentConsumer == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_consumer", "failed to create concurrent consumer")
	}

	return &Consumer{
		concurrentConsumer: concurrentConsumer,
		config:             config,
	}, nil
}

// IsInitialized returns true if the consumer is properly initialized
func (c *Consumer) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.concurrentConsumer != nil
}

func (c *Consumer) Start(ctx context.Context, handler messaging.Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer is closed")
	}

	// Check if concurrent consumer is available
	if c.concurrentConsumer == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "start", "concurrent consumer not initialized")
	}

	// Runtime validation
	if err := c.validateStartInputs(ctx, handler); err != nil {
		return err
	}

	// Delegate to concurrent consumer
	return c.concurrentConsumer.Start(ctx, handler)
}

func (c *Consumer) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// Validate context
	if ctx == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "stop", "context cannot be nil")
	}

	// Check if concurrent consumer is available
	if c.concurrentConsumer == nil {
		c.closed = true
		return messaging.NewError(messaging.ErrorCodeValidation, "stop", "concurrent consumer not initialized")
	}

	// Delegate to concurrent consumer first
	err := c.concurrentConsumer.Stop(ctx)

	// Mark as closed regardless of the result to prevent further operations
	c.closed = true

	return err
}

// SetDLQ sets the dead letter queue for the consumer
// If dlq is nil, the DLQ will be cleared
func (c *Consumer) SetDLQ(dlq *messaging.DeadLetterQueue) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.concurrentConsumer == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "set_dlq", "concurrent consumer not initialized")
	}

	c.concurrentConsumer.SetDLQ(dlq)
	return nil
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() (*ConsumerStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.concurrentConsumer == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "get_stats", "concurrent consumer not initialized")
	}

	return c.concurrentConsumer.GetStats(), nil
}

// GetWorkerStats returns worker statistics
func (c *Consumer) GetWorkerStats() ([]*ConsumerWorkerStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.concurrentConsumer == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "get_worker_stats", "concurrent consumer not initialized")
	}

	return c.concurrentConsumer.GetWorkerStats(), nil
}

// validateStartInputs validates all input parameters for starting the consumer
func (c *Consumer) validateStartInputs(ctx context.Context, handler messaging.Handler) error {
	// Validate context
	if ctx == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "start", "context cannot be nil")
	}

	// Validate handler
	if handler == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "start", "message handler cannot be nil")
	}

	// Validate consumer configuration
	if err := c.validateConsumerConfig(); err != nil {
		return messaging.WrapError(messaging.ErrorCodeValidation, "start", "consumer configuration validation failed", err)
	}

	return nil
}

// validateConsumerConfig validates the consumer configuration
func (c *Consumer) validateConsumerConfig() error {
	// Validate config is not nil
	if c.config == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_consumer_config", "consumer configuration cannot be nil")
	}

	// Validate prefetch count
	if c.config.Prefetch < messaging.MinPrefetchCount {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "prefetch count too low: %d < %d", c.config.Prefetch, messaging.MinPrefetchCount)
	}
	if c.config.Prefetch > messaging.MaxPrefetchCount {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "prefetch count too high: %d > %d", c.config.Prefetch, messaging.MaxPrefetchCount)
	}

	// Validate max concurrent handlers
	if c.config.MaxConcurrentHandlers < messaging.MinConsumerHandlers {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "max concurrent handlers too low: %d < %d", c.config.MaxConcurrentHandlers, messaging.MinConsumerHandlers)
	}
	if c.config.MaxConcurrentHandlers > messaging.MaxConsumerHandlers {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "max concurrent handlers too high: %d > %d", c.config.MaxConcurrentHandlers, messaging.MaxConsumerHandlers)
	}

	// Validate queue name (required field according to ConsumerConfig struct)
	if c.config.Queue == "" {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_consumer_config", "queue name is required")
	}
	if len(c.config.Queue) > messaging.MaxTopicLength {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "queue name too long: %d > %d", len(c.config.Queue), messaging.MaxTopicLength)
	}
	if !isValidQueueName(c.config.Queue) {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_consumer_config", "invalid queue name: %s", c.config.Queue)
	}

	return nil
}

// Helper functions for validation
func isValidQueueName(queue string) bool {
	// Queue name validation regex: alphanumeric, dots, underscores, hyphens
	return queueNameRegex.MatchString(queue)
}
