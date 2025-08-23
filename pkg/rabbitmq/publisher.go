// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"sync"

	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

type Publisher struct {
	asyncPublisher *AsyncPublisher
	config         *messaging.PublisherConfig
	observability  *messaging.ObservabilityContext
	mu             sync.RWMutex
	closed         bool
}

func NewPublisher(transport *Transport, config *messaging.PublisherConfig, observability *messaging.ObservabilityContext) *Publisher {
	asyncPublisher := NewAsyncPublisher(transport, config, observability)

	return &Publisher{
		asyncPublisher: asyncPublisher,
		config:         config,
		observability:  observability,
	}
}

func (p *Publisher) PublishAsync(ctx context.Context, topic string, msg messaging.Message) (messaging.Receipt, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, messaging.NewError(messaging.ErrorCodePublish, "publish_async", "publisher is closed")
	}

	// Start the async publisher if not already started
	if err := p.asyncPublisher.Start(ctx); err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodePublish, "publish_async", "failed to start async publisher", err)
	}

	// Delegate to async publisher
	return p.asyncPublisher.PublishAsync(ctx, topic, msg)
}

func (p *Publisher) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close async publisher
	if p.asyncPublisher != nil {
		if err := p.asyncPublisher.Close(ctx); err != nil {
			p.observability.Logger().Error("failed to close async publisher", logx.ErrorField(err))
		}
	}

	return nil
}
