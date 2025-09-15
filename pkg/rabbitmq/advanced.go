// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/rabbitmq/amqp091-go"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// AdvancedPublisher provides advanced publishing features for RabbitMQ.
type AdvancedPublisher struct {
	transport      *Transport
	config         *messaging.PublisherConfig
	persistence    *messaging.MessagePersistence
	transformation *messaging.MessageTransformation
	routing        *messaging.AdvancedRouting
	confirmManager *ChannelConfirmManager
	receiptManager *messaging.ReceiptManager
	mu             sync.RWMutex
	closed         bool
	observability  *messaging.ObservabilityContext
}

// NewAdvancedPublisher creates a new advanced publisher.
func NewAdvancedPublisher(transport *Transport, config *messaging.PublisherConfig, observability *messaging.ObservabilityContext) *AdvancedPublisher {
	// Validate required parameters
	if transport == nil {
		panic("transport cannot be nil")
	}
	if config == nil {
		panic("config cannot be nil")
	}
	if observability == nil {
		panic("observability cannot be nil")
	}

	receiptTimeout := config.PublishTimeout
	if receiptTimeout == 0 {
		receiptTimeout = 30 * time.Second
	}

	return &AdvancedPublisher{
		transport:      transport,
		config:         config,
		observability:  observability,
		confirmManager: NewChannelConfirmManager(),
		receiptManager: messaging.NewReceiptManager(receiptTimeout),
	}
}

// SetPersistence sets the message persistence for this publisher.
func (ap *AdvancedPublisher) SetPersistence(persistence *messaging.MessagePersistence) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.persistence = persistence
}

// SetTransformation sets the message transformation for this publisher.
func (ap *AdvancedPublisher) SetTransformation(transformation *messaging.MessageTransformation) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.transformation = transformation
}

// SetRouting sets the advanced routing for this publisher.
func (ap *AdvancedPublisher) SetRouting(routing *messaging.AdvancedRouting) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.routing = routing
}

// PublishAsync publishes a message with advanced features.
func (ap *AdvancedPublisher) PublishAsync(ctx context.Context, topic string, msg messaging.Message) (messaging.Receipt, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	if ap.closed {
		return nil, messaging.NewError(messaging.ErrorCodePublish, "publish_async", "publisher is closed")
	}

	// Check if transport is nil
	if ap.transport == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "publish_async", "transport is nil")
	}

	if !ap.transport.IsConnected() {
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "publish_async", "transport is not connected")
	}

	// Validate message ID
	if msg.ID == "" {
		msg.ID = GenerateMessageID()
	}

	// Apply message transformation if configured
	if ap.transformation != nil {
		transformed, err := ap.transformation.Transform(ctx, &msg)
		if err != nil {
			return nil, messaging.WrapError(messaging.ErrorCodeTransformation, "publish_async", "failed to transform message", err)
		}
		msg = *transformed
	}

	// Apply advanced routing if configured
	if ap.routing != nil {
		targetExchange, targetRoutingKey, err := ap.routing.Route(ctx, &msg)
		if err != nil {
			return nil, messaging.WrapError(messaging.ErrorCodeRouting, "publish_async", "failed to route message", err)
		}
		if targetExchange != "" {
			topic = targetExchange
		}
		if targetRoutingKey != "" {
			msg.Key = targetRoutingKey
		}
	}

	// Store message for persistence if configured
	if ap.persistence != nil {
		err := ap.persistence.Store(ctx, &msg)
		if err != nil {
			ap.observability.RecordPersistenceMetrics("store", 0, false, "persistence_failed")
		} else {
			ap.observability.RecordPersistenceMetrics("store", 0, true, "")
		}
	}

	// Convert to AMQP format
	amqpMsg := amqp091.Publishing{
		Headers:     amqp091.Table{"message_id": msg.ID},
		ContentType: msg.ContentType,
		Body:        msg.Body,
		Priority:    uint8(msg.Priority),
		Timestamp:   msg.Timestamp,
	}

	// Add custom headers
	if msg.Headers != nil {
		for k, v := range msg.Headers {
			amqpMsg.Headers[k] = v
		}
	}

	// Add idempotency key if present
	if msg.IdempotencyKey != "" {
		amqpMsg.Headers["x-idempotency-key"] = msg.IdempotencyKey
	}

	// Add correlation ID if present
	if msg.CorrelationID != "" {
		amqpMsg.Headers["x-correlation-id"] = msg.CorrelationID
	}

	// Add reply-to if present
	if msg.ReplyTo != "" {
		amqpMsg.ReplyTo = msg.ReplyTo
	}

	// Add expiration if present
	if msg.Expiration > 0 {
		amqpMsg.Expiration = fmt.Sprintf("%d", int64(msg.Expiration.Milliseconds()))
	}

	// Get channel from transport
	channel := ap.transport.GetChannel()
	if channel == nil {
		return nil, messaging.NewError(messaging.ErrorCodeChannel, "publish_async", "failed to get channel")
	}

	// Create receipt first
	receipt := ap.receiptManager.CreateReceipt(ctx, msg.ID)

	// Set up publisher confirms if enabled
	var confirmTracker *ConfirmTracker
	var deliveryTag uint64
	if ap.config.Confirms {
		var err error
		confirmTracker, err = ap.confirmManager.GetOrCreateTracker(channel, ap.observability)
		if err != nil {
			return nil, messaging.WrapError(messaging.ErrorCodeChannel, "publish_async", "failed to create confirm tracker", err)
		}
		deliveryTag = confirmTracker.TrackMessage(msg.ID, ap.receiptManager)
	}

	// Add additional AMQP properties
	amqpMsg.DeliveryMode = 2 // Persistent delivery mode
	amqpMsg.MessageId = msg.ID
	amqpMsg.CorrelationId = msg.CorrelationID

	start := time.Now()
	mandatory := ap.config.Mandatory
	immediate := ap.config.Immediate

	err := channel.Publish(topic, msg.Key, mandatory, immediate, amqpMsg)
	publishDuration := time.Since(start)

	if err != nil {
		ap.observability.RecordPublishMetrics("rabbitmq", topic, publishDuration, false, "publish_failed")

		// Complete receipt with error if confirms are disabled
		if !ap.config.Confirms {
			ap.receiptManager.CompleteReceipt(msg.ID, messaging.PublishResult{},
				messaging.WrapError(messaging.ErrorCodePublish, "publish_async", "failed to publish message", err))
		}

		return receipt, messaging.WrapError(messaging.ErrorCodePublish, "publish_async", "failed to publish message", err)
	}

	// If confirms are disabled, complete the receipt immediately
	if !ap.config.Confirms {
		result := messaging.PublishResult{
			MessageID:   msg.ID,
			DeliveryTag: deliveryTag,
			Timestamp:   time.Now(),
			Success:     true,
		}
		ap.receiptManager.CompleteReceipt(msg.ID, result, nil)
		ap.observability.RecordPublishMetrics("rabbitmq", topic, publishDuration, true, "")
	}

	return receipt, nil
}

// Close closes the advanced publisher.
func (ap *AdvancedPublisher) Close(ctx context.Context) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.closed = true

	// Close confirm manager
	if ap.confirmManager != nil {
		if err := ap.confirmManager.Close(); err != nil {
			ap.observability.Logger().Error("failed to close confirm manager", logx.ErrorField(err))
		}
	}

	// Close receipt manager
	if ap.receiptManager != nil {
		ap.receiptManager.Close()
	}

	// Close persistence if configured with timeout
	if ap.persistence != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ap.persistence.Close(closeCtx)
	}

	// Close transformation if configured with timeout
	if ap.transformation != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ap.transformation.Close(closeCtx)
	}

	// Close routing if configured with timeout
	if ap.routing != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ap.routing.Close(closeCtx)
	}

	return nil
}

// AdvancedConsumer provides advanced consumption features for RabbitMQ.
type AdvancedConsumer struct {
	transport      *Transport
	config         *messaging.ConsumerConfig
	dlq            *messaging.DeadLetterQueue
	transformation *messaging.MessageTransformation
	routing        *messaging.AdvancedRouting
	mu             sync.RWMutex
	closed         bool
	started        bool
	deliveries     <-chan amqp091.Delivery
	observability  *messaging.ObservabilityContext
}

// NewAdvancedConsumer creates a new advanced consumer.
func NewAdvancedConsumer(transport *Transport, config *messaging.ConsumerConfig, observability *messaging.ObservabilityContext) *AdvancedConsumer {
	// Validate required parameters
	if transport == nil {
		panic("transport cannot be nil")
	}
	if config == nil {
		panic("config cannot be nil")
	}
	if observability == nil {
		panic("observability cannot be nil")
	}

	return &AdvancedConsumer{
		transport:     transport,
		config:        config,
		observability: observability,
	}
}

// SetDLQ sets the dead letter queue for this consumer.
func (ac *AdvancedConsumer) SetDLQ(dlq *messaging.DeadLetterQueue) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.dlq = dlq
}

// SetTransformation sets the message transformation for this consumer.
func (ac *AdvancedConsumer) SetTransformation(transformation *messaging.MessageTransformation) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.transformation = transformation
}

// SetRouting sets the advanced routing for this consumer.
func (ac *AdvancedConsumer) SetRouting(routing *messaging.AdvancedRouting) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.routing = routing
}

// Start starts consuming messages with advanced features.
func (ac *AdvancedConsumer) Start(ctx context.Context, handler messaging.Handler) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.closed {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer is closed")
	}

	if ac.started {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer is already started")
	}

	// Validate handler parameter
	if handler == nil {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "handler cannot be nil")
	}

	// Check if transport is nil
	if ac.transport == nil {
		return messaging.NewError(messaging.ErrorCodeConnection, "start", "transport is nil")
	}

	if !ac.transport.IsConnected() {
		return messaging.NewError(messaging.ErrorCodeConnection, "start", "transport is not connected")
	}

	// Get channel and validate
	channel := ac.transport.GetChannel()
	if channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "start", "failed to get channel")
	}

	// Start consuming
	deliveries, err := channel.Consume(
		ac.config.Queue,     // queue
		"",                  // consumer
		ac.config.AutoAck,   // auto-ack
		ac.config.Exclusive, // exclusive
		false,               // no-local
		ac.config.NoWait,    // no-wait
		ac.config.Arguments, // args
	)
	if err != nil {
		return messaging.WrapError(messaging.ErrorCodeConsume, "start", "failed to start consuming", err)
	}

	ac.deliveries = deliveries
	ac.started = true

	// Start message processing goroutine
	go ac.processMessages(ctx, handler)

	return nil
}

// Stop stops the advanced consumer.
func (ac *AdvancedConsumer) Stop(ctx context.Context) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if !ac.started {
		return nil
	}

	ac.started = false
	ac.closed = true

	// Close DLQ if configured with timeout
	if ac.dlq != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ac.dlq.Close(closeCtx)
	}

	// Close transformation if configured with timeout
	if ac.transformation != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ac.transformation.Close(closeCtx)
	}

	// Close routing if configured with timeout
	if ac.routing != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ac.routing.Close(closeCtx)
	}

	return nil
}

// processMessages processes messages with advanced features.
func (ac *AdvancedConsumer) processMessages(ctx context.Context, handler messaging.Handler) {
	// Validate handler parameter
	if handler == nil {
		ac.observability.Logger().Error("handler cannot be nil in processMessages")
		return
	}

	// Get a local copy of the deliveries channel to avoid race conditions
	ac.mu.RLock()
	deliveries := ac.deliveries
	ac.mu.RUnlock()

	if deliveries == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			ac.handleMessage(ctx, delivery, handler)
		}
	}
}

// handleMessage handles a single message with advanced features.
func (ac *AdvancedConsumer) handleMessage(ctx context.Context, amqpDelivery amqp091.Delivery, handler messaging.Handler) {
	// Validate handler parameter
	if handler == nil {
		ac.observability.Logger().Error("handler cannot be nil in handleMessage")
		return
	}

	// Convert AMQP delivery to messaging delivery
	delivery := messaging.Delivery{
		Message: messaging.Message{
			ID:          amqpDelivery.MessageId,
			Key:         amqpDelivery.RoutingKey,
			Body:        amqpDelivery.Body,
			ContentType: amqpDelivery.ContentType,
			Timestamp:   amqpDelivery.Timestamp,
			Priority:    uint8(amqpDelivery.Priority),
		},
		DeliveryTag: amqpDelivery.DeliveryTag,
		Exchange:    amqpDelivery.Exchange,
		RoutingKey:  amqpDelivery.RoutingKey,
		Queue:       ac.config.Queue,
		Redelivered: amqpDelivery.Redelivered,
		ConsumerTag: amqpDelivery.ConsumerTag,
	}

	// Extract custom headers with proper type handling
	if amqpDelivery.Headers != nil {
		delivery.Message.Headers = make(map[string]string)
		for k, v := range amqpDelivery.Headers {
			switch val := v.(type) {
			case string:
				delivery.Message.Headers[k] = val
			case []byte:
				delivery.Message.Headers[k] = string(val)
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				delivery.Message.Headers[k] = fmt.Sprintf("%v", val)
			case float32, float64:
				delivery.Message.Headers[k] = fmt.Sprintf("%v", val)
			case bool:
				delivery.Message.Headers[k] = fmt.Sprintf("%t", val)
			default:
				// Convert other types to string representation
				delivery.Message.Headers[k] = fmt.Sprintf("%v", val)
			}
		}
	}

	// Apply message transformation if configured
	if ac.transformation != nil {
		transformed, err := ac.transformation.Transform(ctx, &delivery.Message)
		if err != nil {
			ac.observability.RecordTransformationMetrics("transform", 0, false, "transformation_failed")
			amqpDelivery.Nack(false, true) // Requeue
			return
		}
		delivery.Message = *transformed
	}

	// Apply message filtering if configured
	if ac.routing != nil {
		accepted, err := ac.routing.Filter(ctx, &delivery.Message)
		if err != nil {
			ac.observability.RecordRoutingMetrics("filter", 0, false, "filter_failed")
			amqpDelivery.Nack(false, true) // Requeue
			return
		}
		if !accepted {
			amqpDelivery.Ack(false) // Acknowledge filtered message
			return
		}
	}

	// Process message
	decision, err := handler.Process(ctx, delivery)

	// Handle acknowledgment
	if err != nil {
		if ac.dlq != nil && ac.config.MaxRetries > 0 {
			// Check retry count with safe header access
			retryCount := 0
			if delivery.Message.Headers != nil {
				if deliveryCount, ok := delivery.Message.Headers["x-retry-count"]; ok {
					if count, parseErr := fmt.Sscanf(deliveryCount, "%d", &retryCount); parseErr == nil && count == 1 {
						// Successfully parsed retry count
					} else {
						// Reset to 0 if parsing failed
						retryCount = 0
					}
				}
			}

			if retryCount >= ac.config.MaxRetries {
				// Send to DLQ
				dlqErr := ac.dlq.SendToDLQ(ctx, &delivery.Message, err.Error())
				if dlqErr != nil {
					ac.observability.RecordDLQMetrics("send", 0, false, "dlq_failed")
				} else {
					ac.observability.RecordDLQMetrics("send", 0, true, "")
				}
				amqpDelivery.Ack(false) // Acknowledge to remove from queue
			} else {
				// Ensure headers map exists before setting retry count
				if delivery.Message.Headers == nil {
					delivery.Message.Headers = make(map[string]string)
				}
				delivery.Message.Headers["x-retry-count"] = fmt.Sprintf("%d", retryCount+1)
				amqpDelivery.Nack(false, true) // Requeue
			}
		} else if ac.config.RequeueOnError {
			amqpDelivery.Nack(false, true) // Requeue
		} else {
			amqpDelivery.Nack(false, false) // Don't requeue
		}
	} else {
		switch decision {
		case messaging.Ack:
			amqpDelivery.Ack(false)
		case messaging.NackRequeue:
			amqpDelivery.Nack(false, true) // Requeue
		case messaging.NackDLQ:
			if ac.dlq != nil {
				dlqErr := ac.dlq.SendToDLQ(ctx, &delivery.Message, "explicit DLQ routing")
				if dlqErr != nil {
					ac.observability.RecordDLQMetrics("send", 0, false, "dlq_failed")
				} else {
					ac.observability.RecordDLQMetrics("send", 0, true, "")
				}
			}
			amqpDelivery.Nack(false, false) // Don't requeue
		}
	}
}

// GenerateMessageID generates a unique message ID.
func GenerateMessageID() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])[:16]
}

// GenerateIdempotencyKey generates an idempotency key for a message.
func GenerateIdempotencyKey(message *messaging.Message) string {
	if message == nil {
		// Generate a fallback idempotency key if message is nil
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(hash[:])
	}

	data := fmt.Sprintf("%s:%s:%s", message.ID, message.Key, string(message.Body))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
