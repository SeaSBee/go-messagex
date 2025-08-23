package rabbitmq

import (
	"context"
	"sync"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

type Consumer struct {
	concurrentConsumer *ConcurrentConsumer
	config             *messaging.ConsumerConfig
	observability      *messaging.ObservabilityContext
	mu                 sync.RWMutex
	closed             bool
}

func NewConsumer(transport *Transport, config *messaging.ConsumerConfig, observability *messaging.ObservabilityContext) *Consumer {
	concurrentConsumer := NewConcurrentConsumer(transport, config, observability)

	return &Consumer{
		concurrentConsumer: concurrentConsumer,
		config:             config,
		observability:      observability,
	}
}

func (c *Consumer) Start(ctx context.Context, handler messaging.Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer is closed")
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

	c.closed = true

	// Delegate to concurrent consumer
	return c.concurrentConsumer.Stop(ctx)
}

// SetDLQ sets the dead letter queue for the consumer
func (c *Consumer) SetDLQ(dlq *messaging.DeadLetterQueue) {
	c.concurrentConsumer.SetDLQ(dlq)
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() *ConsumerStats {
	return c.concurrentConsumer.GetStats()
}

// GetWorkerStats returns worker statistics
func (c *Consumer) GetWorkerStats() []*ConsumerWorkerStats {
	return c.concurrentConsumer.GetWorkerStats()
}
