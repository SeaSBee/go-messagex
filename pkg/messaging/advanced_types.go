// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
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
}

// NewMessagePersistence creates a new message persistence instance.
func NewMessagePersistence(config *MessagePersistenceConfig, observability *ObservabilityContext) (*MessagePersistence, error) {
	if !config.Enabled {
		return &MessagePersistence{config: config, observability: observability}, nil
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

	mp := &MessagePersistence{
		config:        config,
		storage:       storage,
		observability: observability,
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go mp.cleanupWorker()
	}

	return mp, nil
}

// Store stores a message for persistence.
func (mp *MessagePersistence) Store(ctx context.Context, message *Message) error {
	if !mp.config.Enabled || mp.closed {
		return nil
	}

	mp.mu.RLock()
	defer mp.mu.RUnlock()

	start := time.Now()
	err := mp.storage.Store(ctx, message)

	if err != nil {
		mp.observability.RecordPersistenceMetrics("store", time.Since(start), false, "storage_failed")
		return WrapError(ErrorCodePersistence, "store", "failed to store message", err)
	}

	mp.observability.RecordPersistenceMetrics("store", time.Since(start), true, "")
	return nil
}

// Retrieve retrieves a message by ID.
func (mp *MessagePersistence) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	if !mp.config.Enabled || mp.closed {
		return nil, NewError(ErrorCodePersistence, "retrieve", "persistence is disabled or closed")
	}

	mp.mu.RLock()
	defer mp.mu.RUnlock()

	start := time.Now()
	message, err := mp.storage.Retrieve(ctx, messageID)

	if err != nil {
		mp.observability.RecordPersistenceMetrics("retrieve", time.Since(start), false, "retrieve_failed")
		return nil, WrapError(ErrorCodePersistence, "retrieve", "failed to retrieve message", err)
	}

	mp.observability.RecordPersistenceMetrics("retrieve", time.Since(start), true, "")
	return message, nil
}

// Delete deletes a message by ID.
func (mp *MessagePersistence) Delete(ctx context.Context, messageID string) error {
	if !mp.config.Enabled || mp.closed {
		return nil
	}

	mp.mu.RLock()
	defer mp.mu.RUnlock()

	start := time.Now()
	err := mp.storage.Delete(ctx, messageID)

	if err != nil {
		mp.observability.RecordPersistenceMetrics("delete", time.Since(start), false, "delete_failed")
		return WrapError(ErrorCodePersistence, "delete", "failed to delete message", err)
	}

	mp.observability.RecordPersistenceMetrics("delete", time.Since(start), true, "")
	return nil
}

// Close closes the message persistence.
func (mp *MessagePersistence) Close(ctx context.Context) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.closed {
		return nil
	}

	mp.closed = true

	if mp.storage != nil {
		return mp.storage.Close(ctx)
	}

	return nil
}

// cleanupWorker runs the cleanup worker.
func (mp *MessagePersistence) cleanupWorker() {
	ticker := time.NewTicker(mp.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		if mp.closed {
			return
		}

		before := time.Now().Add(-mp.config.MessageTTL)
		ctx := context.Background()

		err := mp.storage.Cleanup(ctx, before)
		if err != nil {
			mp.observability.RecordPersistenceMetrics("cleanup", 0, false, "cleanup_failed")
		} else {
			mp.observability.RecordPersistenceMetrics("cleanup", 0, true, "")
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
}

// NewDeadLetterQueue creates a new dead letter queue instance.
func NewDeadLetterQueue(config *DeadLetterQueueConfig, transport interface{ GetChannel() interface{} }, observability *ObservabilityContext) *DeadLetterQueue {
	return &DeadLetterQueue{
		config:        config,
		transport:     transport,
		observability: observability,
	}
}

// SendToDLQ sends a message to the dead letter queue.
func (dlq *DeadLetterQueue) SendToDLQ(ctx context.Context, message *Message, reason string) error {
	if !dlq.config.Enabled || dlq.closed {
		return NewError(ErrorCodeDLQ, "send_to_dlq", "DLQ is disabled or closed")
	}

	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	// Add DLQ metadata to message
	if message.Headers == nil {
		message.Headers = make(map[string]string)
	}
	message.Headers["x-dlq-reason"] = reason
	message.Headers["x-dlq-timestamp"] = time.Now().Format(time.RFC3339)
	message.Headers["x-original-exchange"] = message.Headers["x-exchange"]
	message.Headers["x-original-routing-key"] = message.Headers["x-routing-key"]

	// Set routing for DLQ
	message.Key = dlq.config.RoutingKey

	start := time.Now()

	// Note: This is a placeholder - actual implementation would depend on the transport
	// For now, we'll just record metrics
	dlq.observability.RecordDLQMetrics("send", time.Since(start), true, "")
	return nil
}

// ReplayFromDLQ replays messages from the dead letter queue.
func (dlq *DeadLetterQueue) ReplayFromDLQ(ctx context.Context, targetExchange, targetRoutingKey string, limit int) (int, error) {
	if !dlq.config.Enabled || dlq.closed {
		return 0, NewError(ErrorCodeDLQ, "replay_from_dlq", "DLQ is disabled or closed")
	}

	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	start := time.Now()

	// Note: This is a placeholder - actual implementation would depend on the transport
	// For now, we'll just record metrics
	dlq.observability.RecordDLQMetrics("replay", time.Since(start), true, "")
	return 0, nil
}

// Close closes the dead letter queue.
func (dlq *DeadLetterQueue) Close(ctx context.Context) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	dlq.closed = true
	return nil
}

// MessageTransformation provides message transformation functionality.
type MessageTransformation struct {
	config        *MessageTransformationConfig
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
}

// NewMessageTransformation creates a new message transformation instance.
func NewMessageTransformation(config *MessageTransformationConfig, observability *ObservabilityContext) *MessageTransformation {
	return &MessageTransformation{
		config:        config,
		observability: observability,
	}
}

// Transform transforms a message according to the configuration.
func (mt *MessageTransformation) Transform(ctx context.Context, message *Message) (*Message, error) {
	if !mt.config.Enabled || mt.closed {
		return message, nil
	}

	mt.mu.RLock()
	defer mt.mu.RUnlock()

	start := time.Now()
	transformed := *message

	// Apply compression if enabled
	if mt.config.CompressionEnabled {
		compressed, err := mt.compressMessage(&transformed)
		if err != nil {
			mt.observability.RecordTransformationMetrics("compress", time.Since(start), false, "compression_failed")
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to compress message", err)
		}
		transformed = *compressed
	}

	// Apply serialization if needed
	if mt.config.SerializationFormat != "json" {
		serialized, err := mt.serializeMessage(&transformed)
		if err != nil {
			mt.observability.RecordTransformationMetrics("serialize", time.Since(start), false, "serialization_failed")
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to serialize message", err)
		}
		transformed = *serialized
	}

	// Apply schema validation if enabled
	if mt.config.SchemaValidation {
		err := mt.validateMessage(&transformed)
		if err != nil {
			mt.observability.RecordTransformationMetrics("validate", time.Since(start), false, "validation_failed")
			return nil, WrapError(ErrorCodeTransformation, "transform", "failed to validate message", err)
		}
	}

	mt.observability.RecordTransformationMetrics("transform", time.Since(start), true, "")
	return &transformed, nil
}

// compressMessage compresses a message.
func (mt *MessageTransformation) compressMessage(message *Message) (*Message, error) {
	// TODO: Implement compression
	return message, nil
}

// serializeMessage serializes a message.
func (mt *MessageTransformation) serializeMessage(message *Message) (*Message, error) {
	// TODO: Implement serialization
	return message, nil
}

// validateMessage validates a message against schema.
func (mt *MessageTransformation) validateMessage(message *Message) error {
	// TODO: Implement schema validation
	return nil
}

// Close closes the message transformation.
func (mt *MessageTransformation) Close(ctx context.Context) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.closed = true
	return nil
}

// AdvancedRouting provides advanced routing functionality.
type AdvancedRouting struct {
	config        *AdvancedRoutingConfig
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext
}

// NewAdvancedRouting creates a new advanced routing instance.
func NewAdvancedRouting(config *AdvancedRoutingConfig, observability *ObservabilityContext) *AdvancedRouting {
	return &AdvancedRouting{
		config:        config,
		observability: observability,
	}
}

// Route routes a message according to routing rules.
func (ar *AdvancedRouting) Route(ctx context.Context, message *Message) (string, string, error) {
	if !ar.config.Enabled || ar.closed {
		return "", "", nil
	}

	ar.mu.RLock()
	defer ar.mu.RUnlock()

	start := time.Now()

	// Apply routing rules
	for _, rule := range ar.config.RoutingRules {
		if !rule.Enabled {
			continue
		}

		matches, err := ar.evaluateCondition(rule.Condition, message)
		if err != nil {
			ar.observability.RecordRoutingMetrics("evaluate", time.Since(start), false, "evaluation_failed")
			continue
		}

		if matches {
			ar.observability.RecordRoutingMetrics("route", time.Since(start), true, "")
			return rule.TargetExchange, rule.TargetRoutingKey, nil
		}
	}

	ar.observability.RecordRoutingMetrics("route", time.Since(start), true, "no_match")
	return "", "", nil
}

// Filter filters a message according to filter rules.
func (ar *AdvancedRouting) Filter(ctx context.Context, message *Message) (bool, error) {
	if !ar.config.Enabled || ar.closed {
		return true, nil
	}

	ar.mu.RLock()
	defer ar.mu.RUnlock()

	start := time.Now()

	// Apply filter rules
	for _, rule := range ar.config.FilterRules {
		if !rule.Enabled {
			continue
		}

		matches, err := ar.evaluateCondition(rule.Condition, message)
		if err != nil {
			ar.observability.RecordRoutingMetrics("filter", time.Since(start), false, "evaluation_failed")
			continue
		}

		if matches {
			switch rule.Action {
			case "accept":
				ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "accepted")
				return true, nil
			case "reject":
				ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "rejected")
				return false, nil
			case "modify":
				// TODO: Implement message modification
				ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "modified")
				return true, nil
			}
		}
	}

	ar.observability.RecordRoutingMetrics("filter", time.Since(start), true, "no_match")
	return true, nil
}

// evaluateCondition evaluates a routing/filter condition.
func (ar *AdvancedRouting) evaluateCondition(condition string, message *Message) (bool, error) {
	// TODO: Implement condition evaluation using JSONPath
	return true, nil
}

// Close closes the advanced routing.
func (ar *AdvancedRouting) Close(ctx context.Context) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.closed = true
	return nil
}
