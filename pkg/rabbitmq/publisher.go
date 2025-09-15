// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"regexp"
	"sync"

	"github.com/SeaSBee/go-logx"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// Pre-compiled regex patterns for better performance
var (
	topicNameRegex      = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	messageIDRegex      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	routingKeyRegex     = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	correlationIDRegex  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	replyToRegex        = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	idempotencyKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

type Publisher struct {
	asyncPublisher *AsyncPublisher
	config         *messaging.PublisherConfig
	observability  *messaging.ObservabilityContext
	mu             sync.RWMutex
	closed         bool
	startOnce      sync.Once
}

func NewPublisher(transport *Transport, config *messaging.PublisherConfig, observability *messaging.ObservabilityContext) (*Publisher, error) {
	// Validate input parameters
	if transport == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_publisher", "transport cannot be nil")
	}
	if config == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_publisher", "config cannot be nil")
	}
	if observability == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "new_publisher", "observability cannot be nil")
	}

	asyncPublisher := NewAsyncPublisher(transport, config, observability)
	if asyncPublisher == nil {
		return nil, messaging.NewError(messaging.ErrorCodeInternal, "new_publisher", "failed to create async publisher")
	}

	return &Publisher{
		asyncPublisher: asyncPublisher,
		config:         config,
		observability:  observability,
	}, nil
}

func (p *Publisher) PublishAsync(ctx context.Context, topic string, msg messaging.Message) (messaging.Receipt, error) {
	// Defensive nil check
	if p == nil {
		return nil, messaging.NewError(messaging.ErrorCodeValidation, "publish_async", "publisher is nil")
	}

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, messaging.NewError(messaging.ErrorCodePublish, "publish_async", "publisher is closed")
	}
	p.mu.RUnlock()

	// Runtime validation
	if err := p.validatePublishInputs(ctx, topic, msg); err != nil {
		return nil, err
	}

	// Start the async publisher once using sync.Once to prevent race conditions
	var startErr error
	p.startOnce.Do(func() {
		if p.asyncPublisher != nil {
			startErr = p.asyncPublisher.Start(ctx)
		} else {
			startErr = messaging.NewError(messaging.ErrorCodeInternal, "publish_async", "async publisher is nil")
		}
	})

	if startErr != nil {
		return nil, messaging.WrapError(messaging.ErrorCodePublish, "publish_async", "failed to start async publisher", startErr)
	}

	// Delegate to async publisher
	if p.asyncPublisher == nil {
		return nil, messaging.NewError(messaging.ErrorCodeInternal, "publish_async", "async publisher is nil")
	}
	return p.asyncPublisher.PublishAsync(ctx, topic, msg)
}

// validatePublishInputs validates all input parameters for publishing
func (p *Publisher) validatePublishInputs(ctx context.Context, topic string, msg messaging.Message) error {
	// Validate context
	if ctx == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "publish_async", "context cannot be nil")
	}

	// Validate topic
	if topic == "" {
		return messaging.NewError(messaging.ErrorCodeValidation, "publish_async", "topic cannot be empty")
	}
	if len(topic) > messaging.MaxTopicLength {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "publish_async", "topic too long: %d > %d", len(topic), messaging.MaxTopicLength)
	}
	if !isValidTopicName(topic) {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "publish_async", "invalid topic name: %s", topic)
	}

	// Validate message
	if err := p.validateMessage(msg); err != nil {
		return messaging.WrapError(messaging.ErrorCodeValidation, "publish_async", "message validation failed", err)
	}

	return nil
}

// validateMessage validates a message for publishing
func (p *Publisher) validateMessage(msg messaging.Message) error {
	// Validate message ID
	if msg.ID == "" {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_message", "message ID cannot be empty")
	}
	if len(msg.ID) > messaging.MaxMessageIDLength {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "message ID too long: %d > %d", len(msg.ID), messaging.MaxMessageIDLength)
	}
	if !isValidMessageID(msg.ID) {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "invalid message ID format: %s", msg.ID)
	}

	// Validate message body
	if len(msg.Body) == 0 {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_message", "message body cannot be empty")
	}
	if len(msg.Body) > messaging.MaxMessageSize {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "message too large: %d > %d", len(msg.Body), messaging.MaxMessageSize)
	}

	// Validate routing key
	if msg.Key == "" {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_message", "routing key cannot be empty")
	}
	if len(msg.Key) > messaging.MaxRoutingKeyLength {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "routing key too long: %d > %d", len(msg.Key), messaging.MaxRoutingKeyLength)
	}
	if !isValidRoutingKey(msg.Key) {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "invalid routing key format: %s", msg.Key)
	}

	// Validate content type
	if msg.ContentType == "" {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_message", "content type cannot be empty")
	}
	if !isValidContentType(msg.ContentType) {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "unsupported content type: %s", msg.ContentType)
	}

	// Validate headers
	if msg.Headers != nil {
		if len(msg.Headers) > 100 {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "too many headers: %d > 100", len(msg.Headers))
		}
		for key, value := range msg.Headers {
			if len(key) > 255 {
				return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "header key too long: %d > 255", len(key))
			}
			if len(value) > messaging.MaxHeaderSize {
				return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "header value too large: %d > %d", len(value), messaging.MaxHeaderSize)
			}
		}
	}

	// Validate priority
	if msg.Priority > messaging.MaxPriority {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "priority too high: %d > %d", msg.Priority, messaging.MaxPriority)
	}

	// Validate correlation ID
	if msg.CorrelationID != "" {
		if len(msg.CorrelationID) > messaging.MaxCorrelationIDLength {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "correlation ID too long: %d > %d", len(msg.CorrelationID), messaging.MaxCorrelationIDLength)
		}
		if !isValidCorrelationID(msg.CorrelationID) {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "invalid correlation ID format: %s", msg.CorrelationID)
		}
	}

	// Validate reply-to
	if msg.ReplyTo != "" {
		if len(msg.ReplyTo) > messaging.MaxReplyToLength {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "reply-to too long: %d > %d", len(msg.ReplyTo), messaging.MaxReplyToLength)
		}
		if !isValidReplyTo(msg.ReplyTo) {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "invalid reply-to format: %s", msg.ReplyTo)
		}
	}

	// Validate idempotency key
	if msg.IdempotencyKey != "" {
		if len(msg.IdempotencyKey) > messaging.MaxIdempotencyKeyLength {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "idempotency key too long: %d > %d", len(msg.IdempotencyKey), messaging.MaxIdempotencyKeyLength)
		}
		if !isValidIdempotencyKey(msg.IdempotencyKey) {
			return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "invalid idempotency key format: %s", msg.IdempotencyKey)
		}
	}

	// Validate expiration
	if msg.Expiration < 0 {
		return messaging.NewError(messaging.ErrorCodeValidation, "validate_message", "expiration cannot be negative")
	}
	if msg.Expiration > messaging.MaxTimeout {
		return messaging.NewErrorf(messaging.ErrorCodeValidation, "validate_message", "expiration too long: %v > %v", msg.Expiration, messaging.MaxTimeout)
	}

	return nil
}

// Helper functions for validation using pre-compiled regex patterns
func isValidTopicName(topic string) bool {
	return topicNameRegex.MatchString(topic)
}

func isValidMessageID(id string) bool {
	return messageIDRegex.MatchString(id)
}

func isValidRoutingKey(key string) bool {
	return routingKeyRegex.MatchString(key)
}

func isValidContentType(contentType string) bool {
	supportedTypes := messaging.SupportedContentTypes()
	for _, supported := range supportedTypes {
		if contentType == supported {
			return true
		}
	}
	return false
}

func isValidCorrelationID(id string) bool {
	return correlationIDRegex.MatchString(id)
}

func isValidReplyTo(replyTo string) bool {
	return replyToRegex.MatchString(replyTo)
}

func isValidIdempotencyKey(key string) bool {
	return idempotencyKeyRegex.MatchString(key)
}

func (p *Publisher) Close(ctx context.Context) error {
	// Defensive nil check
	if p == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "close", "publisher is nil")
	}

	// Validate context
	if ctx == nil {
		return messaging.NewError(messaging.ErrorCodeValidation, "close", "context cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close async publisher and return any errors
	if p.asyncPublisher != nil {
		if err := p.asyncPublisher.Close(ctx); err != nil {
			// Log the error but also return it to the caller
			if p.observability != nil && p.observability.Logger() != nil {
				p.observability.Logger().Error("failed to close async publisher", logx.ErrorField(err))
			}
			return messaging.WrapError(messaging.ErrorCodeInternal, "close", "failed to close async publisher", err)
		}
	}

	return nil
}
