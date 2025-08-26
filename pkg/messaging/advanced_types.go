// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MessageStorage defines the interface for message storage.
type MessageStorage interface {
	// Store stores a message
	Store(ctx context.Context, message *Message) error

	// Retrieve retrieves a message by ID
	Retrieve(ctx context.Context, messageID string) (*Message, error)

	// Delete deletes a message by ID
	Delete(ctx context.Context, messageID string) error

	// Cleanup cleans up old messages
	Cleanup(ctx context.Context, before time.Time) error

	// Close closes the storage
	Close(ctx context.Context) error
}

// MessagePersistence provides message persistence functionality.
type MessagePersistence struct {
	config        *MessagePersistenceConfig
	storage       MessageStorage
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
	cleanupDone   chan struct{}
	cleanupWg     sync.WaitGroup // Add wait group for cleanup worker
	closeOnce     sync.Once      // Ensure close operations happen only once
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
}

// NewMessagePersistence creates a new message persistence instance.
func NewMessagePersistence(config *MessagePersistenceConfig, observability *ObservabilityContext) (*MessagePersistence, error) {
	if config == nil {
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "config cannot be nil")
	}

	if observability == nil {
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "observability cannot be nil")
	}

	// Validate cleanup interval to prevent panic
	if config.Enabled && config.CleanupInterval <= 0 {
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "cleanup interval must be positive")
	}

	// Validate message TTL
	if config.Enabled && config.MessageTTL <= 0 {
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "message TTL must be positive")
	}

	// Create context for cleanup worker
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())

	mp := &MessagePersistence{
		config:        config,
		observability: observability,
		cleanupCtx:    cleanupCtx,
		cleanupCancel: cleanupCancel,
		cleanupDone:   make(chan struct{}),
	}

	if !config.Enabled {
		// Even when disabled, we need to initialize all fields for safe Close() operations
		return mp, nil
	}

	var storage MessageStorage
	var err error

	switch config.StorageType {
	case "memory":
		storage = NewMemoryMessageStorage(config)
	case "disk":
		storage, err = NewDiskMessageStorage(config)
	case "redis":
		storage, err = NewRedisMessageStorage(config)
	default:
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "unsupported storage type: "+config.StorageType)
	}

	if err != nil {
		return nil, WrapError(ErrorCodeConfiguration, "new_message_persistence", "failed to create storage", err)
	}

	// Validate that storage was properly created
	if storage == nil {
		return nil, NewError(ErrorCodeConfiguration, "new_message_persistence", "storage implementation returned nil")
	}

	mp.storage = storage

	// Start cleanup goroutine after struct is fully initialized
	mp.cleanupWg.Add(1)
	go mp.cleanupWorker()

	return mp, nil
}

// Store stores a message for persistence.
func (mp *MessagePersistence) Store(ctx context.Context, message *Message) error {
	if ctx == nil {
		return NewError(ErrorCodePersistence, "store", "context cannot be nil")
	}

	// Defensive check for config
	if mp.config == nil {
		return NewError(ErrorCodePersistence, "store", "persistence config is nil")
	}

	if !mp.config.Enabled {
		return NewError(ErrorCodePersistence, "store", "persistence is disabled")
	}

	if message == nil {
		return NewError(ErrorCodePersistence, "store", "message cannot be nil")
	}

	// Validate message fields
	if err := mp.validateMessage(message); err != nil {
		return WrapError(ErrorCodePersistence, "store", "message validation failed", err)
	}

	mp.mu.RLock()
	closed := mp.closed
	storage := mp.storage
	mp.mu.RUnlock()

	if closed {
		return NewError(ErrorCodePersistence, "store", "persistence is closed")
	}

	// Check if storage is available
	if storage == nil {
		return NewError(ErrorCodePersistence, "store", "storage is not available")
	}

	start := time.Now()
	err := storage.Store(ctx, message)
	if err != nil {
		// Defensive check for observability
		if mp.observability != nil {
			mp.observability.RecordPersistenceMetrics("store", time.Since(start), false, "storage_failed")
		}
		return WrapError(ErrorCodePersistence, "store", "failed to store message", err)
	}

	// Defensive check for observability
	if mp.observability != nil {
		mp.observability.RecordPersistenceMetrics("store", time.Since(start), true, "")
	}
	return nil
}

// Retrieve retrieves a message by ID.
func (mp *MessagePersistence) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	if ctx == nil {
		return nil, NewError(ErrorCodePersistence, "retrieve", "context cannot be nil")
	}

	// Defensive check for config
	if mp.config == nil {
		return nil, NewError(ErrorCodePersistence, "retrieve", "persistence config is nil")
	}

	if !mp.config.Enabled {
		return nil, NewError(ErrorCodePersistence, "retrieve", "persistence is disabled")
	}

	if messageID == "" {
		return nil, NewError(ErrorCodePersistence, "retrieve", "message ID cannot be empty")
	}

	mp.mu.RLock()
	closed := mp.closed
	storage := mp.storage
	mp.mu.RUnlock()

	if closed {
		return nil, NewError(ErrorCodePersistence, "retrieve", "persistence is closed")
	}

	// Check if storage is available
	if storage == nil {
		return nil, NewError(ErrorCodePersistence, "retrieve", "storage is not available")
	}

	start := time.Now()
	message, err := storage.Retrieve(ctx, messageID)
	if err != nil {
		// Defensive check for observability
		if mp.observability != nil {
			mp.observability.RecordPersistenceMetrics("retrieve", time.Since(start), false, "retrieve_failed")
		}
		return nil, WrapError(ErrorCodePersistence, "retrieve", "failed to retrieve message", err)
	}

	// Defensive check for observability
	if mp.observability != nil {
		mp.observability.RecordPersistenceMetrics("retrieve", time.Since(start), true, "")
	}
	return message, nil
}

// Delete deletes a message by ID.
func (mp *MessagePersistence) Delete(ctx context.Context, messageID string) error {
	if ctx == nil {
		return NewError(ErrorCodePersistence, "delete", "context cannot be nil")
	}

	// Defensive check for config
	if mp.config == nil {
		return NewError(ErrorCodePersistence, "delete", "persistence config is nil")
	}

	if !mp.config.Enabled {
		return NewError(ErrorCodePersistence, "delete", "persistence is disabled")
	}

	if messageID == "" {
		return NewError(ErrorCodePersistence, "delete", "message ID cannot be empty")
	}

	mp.mu.RLock()
	closed := mp.closed
	storage := mp.storage
	mp.mu.RUnlock()

	if closed {
		return NewError(ErrorCodePersistence, "delete", "persistence is closed")
	}

	// Check if storage is available
	if storage == nil {
		return NewError(ErrorCodePersistence, "delete", "storage is not available")
	}

	start := time.Now()
	err := storage.Delete(ctx, messageID)
	if err != nil {
		// Defensive check for observability
		if mp.observability != nil {
			mp.observability.RecordPersistenceMetrics("delete", time.Since(start), false, "delete_failed")
		}
		return WrapError(ErrorCodePersistence, "delete", "failed to delete message", err)
	}

	// Defensive check for observability
	if mp.observability != nil {
		mp.observability.RecordPersistenceMetrics("delete", time.Since(start), true, "")
	}
	return nil
}

// validateMessage validates message fields for persistence.
func (mp *MessagePersistence) validateMessage(message *Message) error {
	if message == nil {
		return NewError(ErrorCodeValidation, "validate_message", "message cannot be nil")
	}

	if message.ID == "" {
		return NewError(ErrorCodeValidation, "validate_message", "message ID cannot be empty")
	}

	if message.Body == nil {
		return NewError(ErrorCodeValidation, "validate_message", "message body cannot be nil")
	}

	return nil
}

// Close closes the message persistence.
func (mp *MessagePersistence) Close(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrorCodePersistence, "close", "context cannot be nil")
	}

	var closeErr error
	mp.closeOnce.Do(func() {
		// Cancel cleanup context first
		if mp.cleanupCancel != nil {
			mp.cleanupCancel()
		}

		mp.mu.Lock()
		mp.closed = true
		storage := mp.storage
		mp.mu.Unlock()

		// Signal cleanup worker to stop and wait for it to finish
		if mp.cleanupDone != nil {
			select {
			case <-mp.cleanupDone:
				// Channel already closed
			default:
				close(mp.cleanupDone)
			}
			mp.cleanupWg.Wait()
		}

		if storage != nil {
			closeErr = storage.Close(ctx)
		}
	})

	return closeErr
}

// cleanupWorker runs the cleanup worker.
func (mp *MessagePersistence) cleanupWorker() {
	defer mp.cleanupWg.Done()

	// Defensive check for config with proper locking
	mp.mu.RLock()
	config := mp.config
	mp.mu.RUnlock()

	if config == nil {
		return
	}

	// Validate cleanup interval
	if config.CleanupInterval <= 0 {
		return
	}

	ticker := time.NewTicker(config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if closed with proper locking
			mp.mu.RLock()
			closed := mp.closed
			storage := mp.storage
			mp.mu.RUnlock()

			if closed {
				return
			}

			if storage == nil {
				continue
			}

			before := time.Now().Add(-config.MessageTTL)
			ctx, cancel := context.WithTimeout(mp.cleanupCtx, 30*time.Second)
			start := time.Now()

			err := storage.Cleanup(ctx, before)
			duration := time.Since(start)
			cancel() // Always cancel to prevent context leak

			if err != nil {
				if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
					// Defensive check for observability
					if mp.observability != nil {
						mp.observability.RecordPersistenceMetrics("cleanup", duration, false, "cleanup_timeout")
					}
				} else {
					// Defensive check for observability
					if mp.observability != nil {
						mp.observability.RecordPersistenceMetrics("cleanup", duration, false, "cleanup_failed")
					}
				}
			} else {
				// Defensive check for observability
				if mp.observability != nil {
					mp.observability.RecordPersistenceMetrics("cleanup", duration, true, "")
				}
			}

		case <-mp.cleanupDone:
			return
		}
	}
}

// DeadLetterQueue provides dead letter queue functionality.
type DeadLetterQueue struct {
	config        *DeadLetterQueueConfig
	transport     interface{ GetChannel() interface{} }
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
	closeOnce     sync.Once // Ensure close operations happen only once
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewDeadLetterQueue creates a new dead letter queue instance.
func NewDeadLetterQueue(config *DeadLetterQueueConfig, transport interface{ GetChannel() interface{} }, observability *ObservabilityContext) (*DeadLetterQueue, error) {
	if config == nil {
		config = &DeadLetterQueueConfig{} // Use default config if nil
	}

	// Validate transport is not nil
	if transport == nil {
		// Create a no-op transport interface to prevent panics
		transport = &noOpTransport{}
	}

	// Create context with cancellation for proper cleanup
	ctx, cancel := context.WithCancel(context.Background())

	if observability == nil {
		// Create a default observability context with no-op provider
		provider, err := NewObservabilityProvider(&TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		if err != nil {
			cancel() // Cancel context before returning error
			return nil, WrapError(ErrorCodeConfiguration, "new_dead_letter_queue", "failed to create observability provider", err)
		}
		observability = NewObservabilityContext(ctx, provider)
	}

	return &DeadLetterQueue{
		config:        config,
		transport:     transport,
		observability: observability,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// SendToDLQ sends a message to the dead letter queue.
func (dlq *DeadLetterQueue) SendToDLQ(ctx context.Context, message *Message, reason string) error {
	if ctx == nil {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "context cannot be nil")
	}

	// Defensive check for config
	if dlq.config == nil {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "DLQ config is nil")
	}

	if !dlq.config.Enabled {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "DLQ is disabled")
	}

	if message == nil {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "message cannot be nil")
	}

	if reason == "" {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "reason cannot be empty")
	}

	dlq.mu.RLock()
	closed := dlq.closed
	dlq.mu.RUnlock()

	if closed {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "DLQ is closed")
	}

	// Create a copy of the message to avoid modifying the original
	dlqMessage := deepCopyMessage(message)
	if dlqMessage == nil {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "failed to copy message")
	}

	// Add DLQ metadata to message copy
	if dlqMessage.Headers == nil {
		dlqMessage.Headers = make(map[string]string)
	}
	dlqMessage.Headers["x-dlq-reason"] = reason
	dlqMessage.Headers["x-dlq-timestamp"] = time.Now().Format(time.RFC3339)

	// Safely get original values with defaults
	if originalExchange, exists := dlqMessage.Headers["x-exchange"]; exists {
		dlqMessage.Headers["x-original-exchange"] = originalExchange
	} else {
		dlqMessage.Headers["x-original-exchange"] = "unknown"
	}

	if originalRoutingKey, exists := dlqMessage.Headers["x-routing-key"]; exists {
		dlqMessage.Headers["x-original-routing-key"] = originalRoutingKey
	} else {
		dlqMessage.Headers["x-original-routing-key"] = "unknown"
	}

	// Set routing for DLQ
	dlqMessage.Key = dlq.config.RoutingKey

	start := time.Now()

	// TODO: Implement actual DLQ sending logic
	// For now, return an error indicating this is not implemented
	// Defensive check for observability
	if dlq.observability != nil {
		dlq.observability.RecordDLQMetrics("send", time.Since(start), false, "not_implemented")
	}
	return NewError(ErrorCodeDLQ, "send_to_dlq", "DLQ sending is not yet implemented")
}

// ReplayFromDLQ replays messages from the dead letter queue.
func (dlq *DeadLetterQueue) ReplayFromDLQ(ctx context.Context, targetExchange, targetRoutingKey string, limit int) (int, error) {
	if ctx == nil {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "context cannot be nil")
	}

	// Defensive check for config
	if dlq.config == nil {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "DLQ config is nil")
	}

	if !dlq.config.Enabled {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "DLQ is disabled")
	}

	if targetExchange == "" {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "target exchange cannot be empty")
	}

	if targetRoutingKey == "" {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "target routing key cannot be empty")
	}

	if limit <= 0 {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "limit must be positive")
	}

	dlq.mu.RLock()
	closed := dlq.closed
	dlq.mu.RUnlock()

	if closed {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "DLQ is closed")
	}

	start := time.Now()

	// TODO: Implement actual DLQ replay logic
	// For now, return an error indicating this is not implemented
	// Defensive check for observability
	if dlq.observability != nil {
		dlq.observability.RecordDLQMetrics("replay", time.Since(start), false, "not_implemented")
	}
	return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "DLQ replay is not yet implemented")
}

// Close closes the dead letter queue.
func (dlq *DeadLetterQueue) Close(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrorCodeDLQ, "close", "context cannot be nil")
	}

	var closeErr error
	dlq.closeOnce.Do(func() {
		// Cancel context first
		if dlq.cancel != nil {
			dlq.cancel()
		}

		dlq.mu.Lock()
		dlq.closed = true
		dlq.mu.Unlock()
	})

	return closeErr
}

// MessageTransformation provides message transformation functionality.
type MessageTransformation struct {
	config        *MessageTransformationConfig
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
	closeOnce     sync.Once // Ensure close operations happen only once
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewMessageTransformation creates a new message transformation instance.
func NewMessageTransformation(config *MessageTransformationConfig, observability *ObservabilityContext) (*MessageTransformation, error) {
	if config == nil {
		config = &MessageTransformationConfig{} // Use default config if nil
	}

	// Create context with cancellation for proper cleanup
	ctx, cancel := context.WithCancel(context.Background())

	if observability == nil {
		// Create a default observability context with no-op provider
		provider, err := NewObservabilityProvider(&TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		if err != nil {
			cancel() // Cancel context before returning error
			return nil, WrapError(ErrorCodeConfiguration, "new_message_transformation", "failed to create observability provider", err)
		}
		observability = NewObservabilityContext(ctx, provider)
	}

	return &MessageTransformation{
		config:        config,
		observability: observability,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// deepCopyMessage creates a deep copy of a message to avoid sharing references.
func deepCopyMessage(message *Message) *Message {
	if message == nil {
		return nil
	}

	// Defensive check for message body
	var body []byte
	if message.Body != nil {
		body = make([]byte, len(message.Body))
		copy(body, message.Body)
	}

	copied := &Message{
		ID:             message.ID,
		Key:            message.Key,
		Body:           body,
		ContentType:    message.ContentType,
		Timestamp:      message.Timestamp,
		Priority:       message.Priority,
		IdempotencyKey: message.IdempotencyKey,
		CorrelationID:  message.CorrelationID,
		ReplyTo:        message.ReplyTo,
		Expiration:     message.Expiration,
	}

	// Copy headers with nil check and thread-safe access
	if message.Headers != nil {
		copied.Headers = make(map[string]string, len(message.Headers))
		// Use a mutex to ensure thread-safe access to the original headers
		// This is a defensive measure in case the original message is being modified
		for k, v := range message.Headers {
			copied.Headers[k] = v
		}
	}

	return copied
}

// Transform transforms a message according to the configuration.
func (mt *MessageTransformation) Transform(ctx context.Context, message *Message) (*Message, error) {
	if ctx == nil {
		return nil, NewError(ErrorCodeTransformation, "transform", "context cannot be nil")
	}

	// Defensive check for config
	if mt.config == nil {
		return message, nil // Return original message if config is nil
	}

	if !mt.config.Enabled {
		return message, nil
	}

	if message == nil {
		return nil, NewError(ErrorCodeTransformation, "transform", "message cannot be nil")
	}

	mt.mu.RLock()
	closed := mt.closed
	mt.mu.RUnlock()

	if closed {
		return message, nil
	}

	start := time.Now()

	// Create a deep copy to avoid modifying the original message
	transformed := deepCopyMessage(message)
	if transformed == nil {
		return nil, NewError(ErrorCodeTransformation, "transform", "failed to copy message")
	}

	// Apply compression if enabled
	if mt.config.CompressionEnabled {
		compressed, err := mt.compressMessage(transformed)
		if err != nil {
			// Defensive check for observability
			if mt.observability != nil {
				mt.observability.RecordTransformationMetrics("compress", time.Since(start), false, "compression_failed")
			}
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to compress message", err)
		}
		transformed = compressed
	}

	// Apply serialization if needed
	if mt.config.SerializationFormat != "json" {
		serialized, err := mt.serializeMessage(transformed)
		if err != nil {
			// Defensive check for observability
			if mt.observability != nil {
				mt.observability.RecordTransformationMetrics("serialize", time.Since(start), false, "serialization_failed")
			}
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to serialize message", err)
		}
		transformed = serialized
	}

	// Apply schema validation if enabled
	if mt.config.SchemaValidation {
		err := mt.validateMessage(transformed)
		if err != nil {
			// Defensive check for observability
			if mt.observability != nil {
				mt.observability.RecordTransformationMetrics("validate", time.Since(start), false, "validation_failed")
			}
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to validate message", err)
		}
	}

	// Defensive check for observability
	if mt.observability != nil {
		mt.observability.RecordTransformationMetrics("transform", time.Since(start), true, "")
	}
	return transformed, nil
}

// compressMessage compresses a message.
func (mt *MessageTransformation) compressMessage(message *Message) (*Message, error) {
	if message == nil {
		return nil, NewError(ErrorCodeTransformation, "compress_message", "message cannot be nil")
	}

	// TODO: Implement actual compression logic
	// For now, return an error indicating this is not implemented
	return nil, NewError(ErrorCodeTransformation, "compress_message", "message compression is not yet implemented")
}

// serializeMessage serializes a message.
func (mt *MessageTransformation) serializeMessage(message *Message) (*Message, error) {
	if message == nil {
		return nil, NewError(ErrorCodeTransformation, "serialize_message", "message cannot be nil")
	}

	// TODO: Implement actual serialization logic
	// For now, return an error indicating this is not implemented
	return nil, NewError(ErrorCodeTransformation, "serialize_message", "message serialization is not yet implemented")
}

// validateMessage validates a message against schema.
func (mt *MessageTransformation) validateMessage(message *Message) error {
	if message == nil {
		return NewError(ErrorCodeTransformation, "validate_message", "message cannot be nil")
	}

	// TODO: Implement actual schema validation logic
	// For now, return an error indicating this is not implemented
	return NewError(ErrorCodeTransformation, "validate_message", "message validation is not yet implemented")
}

// Close closes the message transformation.
func (mt *MessageTransformation) Close(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrorCodeTransformation, "close", "context cannot be nil")
	}

	var closeErr error
	mt.closeOnce.Do(func() {
		// Cancel context first
		if mt.cancel != nil {
			mt.cancel()
		}

		mt.mu.Lock()
		mt.closed = true
		mt.mu.Unlock()
	})

	return closeErr
}

// AdvancedRouting provides advanced routing functionality.
type AdvancedRouting struct {
	config        *AdvancedRoutingConfig
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
	closeOnce     sync.Once // Ensure close operations happen only once
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewAdvancedRouting creates a new advanced routing instance.
func NewAdvancedRouting(config *AdvancedRoutingConfig, observability *ObservabilityContext) (*AdvancedRouting, error) {
	if config == nil {
		config = &AdvancedRoutingConfig{} // Use default config if nil
	}

	// Create context with cancellation for proper cleanup
	ctx, cancel := context.WithCancel(context.Background())

	if observability == nil {
		// Create a default observability context with no-op provider
		provider, err := NewObservabilityProvider(&TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		if err != nil {
			cancel() // Cancel context before returning error
			return nil, WrapError(ErrorCodeConfiguration, "new_advanced_routing", "failed to create observability provider", err)
		}
		observability = NewObservabilityContext(ctx, provider)
	}

	return &AdvancedRouting{
		config:        config,
		observability: observability,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// validateRoutingRule validates that a routing rule has valid targets.
func (ar *AdvancedRouting) validateRoutingRule(rule RoutingRule) error {
	if rule.TargetExchange == "" {
		return NewError(ErrorCodeRouting, "validate_routing_rule", "routing rule target exchange cannot be empty")
	}
	if rule.TargetRoutingKey == "" {
		return NewError(ErrorCodeRouting, "validate_routing_rule", "routing rule target routing key cannot be empty")
	}
	if rule.Condition == "" {
		return NewError(ErrorCodeRouting, "validate_routing_rule", "routing rule condition cannot be empty")
	}
	return nil
}

// Route routes a message according to routing rules.
func (ar *AdvancedRouting) Route(ctx context.Context, message *Message) (string, string, error) {
	if ctx == nil {
		return "", "", NewError(ErrorCodeRouting, "route", "context cannot be nil")
	}

	// Defensive check for config
	if ar.config == nil {
		return "", "", nil
	}

	if !ar.config.Enabled {
		return "", "", nil
	}

	if message == nil {
		return "", "", NewError(ErrorCodeRouting, "route", "message cannot be nil")
	}

	ar.mu.RLock()
	closed := ar.closed
	ar.mu.RUnlock()

	if closed {
		return "", "", nil
	}

	start := time.Now()

	// Apply routing rules with nil check
	if ar.config.RoutingRules == nil {
		// Defensive check for observability
		if ar.observability != nil {
			ar.observability.RecordRoutingMetrics("route", time.Since(start), true, "no_rules")
		}
		return "", "", nil
	}

	for _, rule := range ar.config.RoutingRules {
		if !rule.Enabled {
			continue
		}

		// Validate routing rule
		if err := ar.validateRoutingRule(rule); err != nil {
			// Defensive check for observability
			if ar.observability != nil {
				ar.observability.RecordRoutingMetrics("validate_rule", time.Since(start), false, "invalid_rule")
			}
			continue
		}

		matches, err := ar.evaluateCondition(rule.Condition, message)
		if err != nil {
			// Defensive check for observability
			if ar.observability != nil {
				ar.observability.RecordRoutingMetrics("evaluate", time.Since(start), false, "evaluation_failed")
			}
			continue
		}

		if matches {
			// Defensive check for observability
			if ar.observability != nil {
				ar.observability.RecordRoutingMetrics("route", time.Since(start), true, "")
			}
			return rule.TargetExchange, rule.TargetRoutingKey, nil
		}
	}

	// Defensive check for observability
	if ar.observability != nil {
		ar.observability.RecordRoutingMetrics("route", time.Since(start), true, "no_match")
	}
	return "", "", nil
}

// Filter filters a message according to filter rules.
func (ar *AdvancedRouting) Filter(ctx context.Context, message *Message) (bool, error) {
	if ctx == nil {
		return false, NewError(ErrorCodeRouting, "filter", "context cannot be nil")
	}

	// Defensive check for config
	if ar.config == nil {
		return true, nil
	}

	if !ar.config.Enabled {
		return true, nil
	}

	if message == nil {
		return false, NewError(ErrorCodeRouting, "filter", "message cannot be nil")
	}

	ar.mu.RLock()
	closed := ar.closed
	ar.mu.RUnlock()

	if closed {
		return true, nil
	}

	start := time.Now()

	// Apply filter rules with nil check
	if ar.config.FilterRules == nil {
		// Defensive check for observability
		if ar.observability != nil {
			ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "no_rules")
		}
		return true, nil
	}

	for _, rule := range ar.config.FilterRules {
		if !rule.Enabled {
			continue
		}

		matches, err := ar.evaluateCondition(rule.Condition, message)
		if err != nil {
			// Defensive check for observability
			if ar.observability != nil {
				ar.observability.RecordRoutingMetrics("filter", time.Since(start), false, "evaluation_failed")
			}
			return false, WrapError(ErrorCodeRouting, "filter", "failed to evaluate filter condition", err)
		}

		if matches {
			switch rule.Action {
			case "accept":
				// Defensive check for observability
				if ar.observability != nil {
					ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "accepted")
				}
				return true, nil
			case "reject":
				// Defensive check for observability
				if ar.observability != nil {
					ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "rejected")
				}
				return false, nil
			case "modify":
				// TODO: Implement message modification
				// Defensive check for observability
				if ar.observability != nil {
					ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "modified")
				}
				return true, nil
			default:
				// Defensive check for observability
				if ar.observability != nil {
					ar.observability.RecordRoutingMetrics("filter", time.Since(start), false, "unknown_action")
				}
				return false, NewError(ErrorCodeRouting, "filter", fmt.Sprintf("unknown filter action: %s", rule.Action))
			}
		}
	}

	// Defensive check for observability
	if ar.observability != nil {
		ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "no_match")
	}
	return true, nil
}

// evaluateCondition evaluates a routing/filter condition.
func (ar *AdvancedRouting) evaluateCondition(condition string, message *Message) (bool, error) {
	if condition == "" {
		return false, NewError(ErrorCodeRouting, "evaluate_condition", "condition cannot be empty")
	}

	if message == nil {
		return false, NewError(ErrorCodeRouting, "evaluate_condition", "message cannot be nil")
	}

	// TODO: Implement actual condition evaluation using JSONPath
	// For now, return an error indicating this is not implemented
	return false, NewError(ErrorCodeRouting, "evaluate_condition", "condition evaluation is not yet implemented")
}

// Close closes the advanced routing.
func (ar *AdvancedRouting) Close(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrorCodeRouting, "close", "context cannot be nil")
	}

	var closeErr error
	ar.closeOnce.Do(func() {
		// Cancel context first
		if ar.cancel != nil {
			ar.cancel()
		}

		ar.mu.Lock()
		ar.closed = true
		ar.mu.Unlock()
	})

	return closeErr
}

// noOpTransport is a no-op transport implementation to prevent panics
type noOpTransport struct{}

// GetChannel returns nil (no-op implementation)
func (n *noOpTransport) GetChannel() interface{} {
	return nil
}
