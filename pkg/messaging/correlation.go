// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CorrelationID represents a unique identifier for tracking message flow.
type CorrelationID string

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() CorrelationID {
	return CorrelationID(uuid.New().String())
}

// String returns the string representation of the correlation ID.
func (c CorrelationID) String() string {
	return string(c)
}

// IsValid checks if the correlation ID is valid.
func (c CorrelationID) IsValid() bool {
	return len(c) > 0
}

// CorrelationContext provides correlation ID management and propagation.
type CorrelationContext struct {
	correlationID CorrelationID
	parentID      CorrelationID
	spanID        string
	traceID       string
	metadata      map[string]string
	createdAt     time.Time
	mu            sync.RWMutex
}

// NewCorrelationContext creates a new correlation context.
func NewCorrelationContext() *CorrelationContext {
	return &CorrelationContext{
		correlationID: NewCorrelationID(),
		metadata:      make(map[string]string),
		createdAt:     time.Now(),
	}
}

// NewCorrelationContextWithID creates a new correlation context with a specific ID.
func NewCorrelationContextWithID(id CorrelationID) *CorrelationContext {
	return &CorrelationContext{
		correlationID: id,
		metadata:      make(map[string]string),
		createdAt:     time.Now(),
	}
}

// NewCorrelationContextFromParent creates a new correlation context from a parent.
func NewCorrelationContextFromParent(parent *CorrelationContext) *CorrelationContext {
	child := NewCorrelationContext()
	child.parentID = parent.correlationID
	child.traceID = parent.traceID
	child.spanID = parent.spanID

	// Copy metadata from parent
	parent.mu.RLock()
	for k, v := range parent.metadata {
		child.metadata[k] = v
	}
	parent.mu.RUnlock()

	return child
}

// ID returns the correlation ID.
func (c *CorrelationContext) ID() CorrelationID {
	return c.correlationID
}

// ParentID returns the parent correlation ID.
func (c *CorrelationContext) ParentID() CorrelationID {
	return c.parentID
}

// TraceID returns the trace ID.
func (c *CorrelationContext) TraceID() string {
	return c.traceID
}

// SetTraceID sets the trace ID.
func (c *CorrelationContext) SetTraceID(traceID string) {
	c.traceID = traceID
}

// SpanID returns the span ID.
func (c *CorrelationContext) SpanID() string {
	return c.spanID
}

// SetSpanID sets the span ID.
func (c *CorrelationContext) SetSpanID(spanID string) {
	c.spanID = spanID
}

// CreatedAt returns when the correlation context was created.
func (c *CorrelationContext) CreatedAt() time.Time {
	return c.createdAt
}

// SetMetadata sets a metadata key-value pair.
func (c *CorrelationContext) SetMetadata(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata returns a metadata value.
func (c *CorrelationContext) GetMetadata(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// Metadata returns a copy of all metadata.
func (c *CorrelationContext) Metadata() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range c.metadata {
		result[k] = v
	}
	return result
}

// ToContext converts the correlation context to a Go context.
func (c *CorrelationContext) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, correlationContextKey, c)
}

// FromContext extracts a correlation context from a Go context.
func FromContext(ctx context.Context) (*CorrelationContext, bool) {
	value := ctx.Value(correlationContextKey)
	if value == nil {
		return nil, false
	}

	corrCtx, ok := value.(*CorrelationContext)
	return corrCtx, ok
}

// GetOrCreateCorrelationContext gets an existing correlation context or creates a new one.
func GetOrCreateCorrelationContext(ctx context.Context) *CorrelationContext {
	if corrCtx, exists := FromContext(ctx); exists {
		return corrCtx
	}
	return NewCorrelationContext()
}

// context key for correlation context
type correlationContextKeyType struct{}

var correlationContextKey = correlationContextKeyType{}

// CorrelationManager manages correlation contexts across the application.
type CorrelationManager struct {
	activeCorrelations map[CorrelationID]*CorrelationContext
	mu                 sync.RWMutex
	maxActive          int
}

// NewCorrelationManager creates a new correlation manager.
func NewCorrelationManager(maxActive int) *CorrelationManager {
	return &CorrelationManager{
		activeCorrelations: make(map[CorrelationID]*CorrelationContext),
		maxActive:          maxActive,
	}
}

// Register registers a correlation context.
func (cm *CorrelationManager) Register(corrCtx *CorrelationContext) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.activeCorrelations) >= cm.maxActive {
		return fmt.Errorf("maximum number of active correlations reached: %d", cm.maxActive)
	}

	cm.activeCorrelations[corrCtx.ID()] = corrCtx
	return nil
}

// Unregister removes a correlation context.
func (cm *CorrelationManager) Unregister(id CorrelationID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.activeCorrelations, id)
}

// Get retrieves a correlation context.
func (cm *CorrelationManager) Get(id CorrelationID) (*CorrelationContext, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	corrCtx, exists := cm.activeCorrelations[id]
	return corrCtx, exists
}

// ActiveCount returns the number of active correlations.
func (cm *CorrelationManager) ActiveCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.activeCorrelations)
}

// Cleanup removes expired correlation contexts.
func (cm *CorrelationManager) Cleanup(maxAge time.Duration) int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, corrCtx := range cm.activeCorrelations {
		if now.Sub(corrCtx.createdAt) > maxAge {
			delete(cm.activeCorrelations, id)
			removed++
		}
	}

	return removed
}

// List returns all active correlation contexts.
func (cm *CorrelationManager) List() []*CorrelationContext {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*CorrelationContext, 0, len(cm.activeCorrelations))
	for _, corrCtx := range cm.activeCorrelations {
		result = append(result, corrCtx)
	}
	return result
}

// CorrelationPropagator handles correlation ID propagation across message boundaries.
type CorrelationPropagator struct {
	manager *CorrelationManager
}

// NewCorrelationPropagator creates a new correlation propagator.
func NewCorrelationPropagator(manager *CorrelationManager) *CorrelationPropagator {
	return &CorrelationPropagator{
		manager: manager,
	}
}

// Inject injects correlation information into message headers.
func (cp *CorrelationPropagator) Inject(ctx context.Context, headers map[string]string) {
	corrCtx, exists := FromContext(ctx)
	if !exists {
		// Create a new correlation context if none exists
		corrCtx = NewCorrelationContext()
	}

	headers["X-Correlation-ID"] = corrCtx.ID().String()
	if corrCtx.ParentID().IsValid() {
		headers["X-Parent-Correlation-ID"] = corrCtx.ParentID().String()
	}
	if corrCtx.TraceID() != "" {
		headers["X-Trace-ID"] = corrCtx.TraceID()
	}
	if corrCtx.SpanID() != "" {
		headers["X-Span-ID"] = corrCtx.SpanID()
	}

	// Inject metadata
	metadata := corrCtx.Metadata()
	for k, v := range metadata {
		headers[fmt.Sprintf("X-Correlation-Metadata-%s", k)] = v
	}
}

// Extract extracts correlation information from message headers.
func (cp *CorrelationPropagator) Extract(ctx context.Context, headers map[string]string) context.Context {
	correlationIDStr, exists := headers["X-Correlation-ID"]
	if !exists {
		// Create a new correlation context if no correlation ID is present
		corrCtx := NewCorrelationContext()
		return corrCtx.ToContext(ctx)
	}

	correlationID := CorrelationID(correlationIDStr)
	corrCtx := NewCorrelationContextWithID(correlationID)

	// Extract parent correlation ID
	if parentIDStr, exists := headers["X-Parent-Correlation-ID"]; exists {
		corrCtx.parentID = CorrelationID(parentIDStr)
	}

	// Extract trace ID
	if traceID, exists := headers["X-Trace-ID"]; exists {
		corrCtx.traceID = traceID
	}

	// Extract span ID
	if spanID, exists := headers["X-Span-ID"]; exists {
		corrCtx.spanID = spanID
	}

	// Extract metadata
	for k, v := range headers {
		if len(k) > 23 && k[:23] == "X-Correlation-Metadata-" {
			metadataKey := k[23:]
			corrCtx.SetMetadata(metadataKey, v)
		}
	}

	// Register with manager
	if cp.manager != nil {
		cp.manager.Register(corrCtx)
	}

	return corrCtx.ToContext(ctx)
}

// CreateChild creates a child correlation context from the current context.
func (cp *CorrelationPropagator) CreateChild(ctx context.Context) context.Context {
	parentCtx, exists := FromContext(ctx)
	if !exists {
		return NewCorrelationContext().ToContext(ctx)
	}

	childCtx := NewCorrelationContextFromParent(parentCtx)
	if cp.manager != nil {
		cp.manager.Register(childCtx)
	}

	return childCtx.ToContext(ctx)
}

// CorrelationMiddleware provides middleware for automatic correlation ID management.
type CorrelationMiddleware struct {
	propagator *CorrelationPropagator
}

// NewCorrelationMiddleware creates a new correlation middleware.
func NewCorrelationMiddleware(propagator *CorrelationPropagator) *CorrelationMiddleware {
	return &CorrelationMiddleware{
		propagator: propagator,
	}
}

// PublisherMiddleware returns middleware for publishers.
func (cm *CorrelationMiddleware) PublisherMiddleware() func(context.Context, string, Message) (context.Context, Message) {
	return func(ctx context.Context, topic string, msg Message) (context.Context, Message) {
		// Ensure we have a correlation context
		corrCtx := GetOrCreateCorrelationContext(ctx)
		ctx = corrCtx.ToContext(ctx)

		// Inject correlation information into message headers
		if msg.Headers == nil {
			msg.Headers = make(map[string]string)
		}
		cm.propagator.Inject(ctx, msg.Headers)

		// Set correlation ID in message
		msg.CorrelationID = corrCtx.ID().String()

		return ctx, msg
	}
}

// ConsumerMiddleware returns middleware for consumers.
func (cm *CorrelationMiddleware) ConsumerMiddleware() func(context.Context, Delivery) context.Context {
	return func(ctx context.Context, delivery Delivery) context.Context {
		// Extract correlation information from message headers
		return cm.propagator.Extract(ctx, delivery.Headers)
	}
}

// HandlerMiddleware returns middleware for message handlers.
func (cm *CorrelationMiddleware) HandlerMiddleware() func(context.Context, Delivery) context.Context {
	return func(ctx context.Context, delivery Delivery) context.Context {
		// Extract correlation information and create child context
		ctx = cm.propagator.Extract(ctx, delivery.Headers)
		return cm.propagator.CreateChild(ctx)
	}
}
