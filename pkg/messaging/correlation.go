// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	logx "github.com/SeaSBee/go-logx"
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
	if !id.IsValid() {
		id = NewCorrelationID()
	}
	return &CorrelationContext{
		correlationID: id,
		metadata:      make(map[string]string),
		createdAt:     time.Now(),
	}
}

// NewCorrelationContextFromParent creates a new correlation context from a parent.
func NewCorrelationContextFromParent(parent *CorrelationContext) *CorrelationContext {
	if parent == nil {
		return NewCorrelationContext()
	}

	child := NewCorrelationContext()

	// Lock parent to ensure consistent state when copying fields
	parent.mu.RLock()
	child.parentID = parent.correlationID
	child.traceID = parent.traceID
	child.spanID = parent.spanID

	// Copy metadata from parent
	for k, v := range parent.metadata {
		child.metadata[k] = v
	}
	parent.mu.RUnlock()

	return child
}

// ID returns the correlation ID.
func (c *CorrelationContext) ID() CorrelationID {
	if c == nil {
		return ""
	}
	return c.correlationID
}

// ParentID returns the parent correlation ID.
func (c *CorrelationContext) ParentID() CorrelationID {
	if c == nil {
		return ""
	}
	return c.parentID
}

// TraceID returns the trace ID.
func (c *CorrelationContext) TraceID() string {
	if c == nil {
		return ""
	}
	return c.traceID
}

// SetTraceID sets the trace ID.
func (c *CorrelationContext) SetTraceID(traceID string) {
	if c == nil {
		return
	}
	c.traceID = traceID
}

// SpanID returns the span ID.
func (c *CorrelationContext) SpanID() string {
	if c == nil {
		return ""
	}
	return c.spanID
}

// SetSpanID sets the span ID.
func (c *CorrelationContext) SetSpanID(spanID string) {
	if c == nil {
		return
	}
	c.spanID = spanID
}

// CreatedAt returns when the correlation context was created.
func (c *CorrelationContext) CreatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	return c.createdAt
}

// SetMetadata sets a metadata key-value pair.
func (c *CorrelationContext) SetMetadata(key, value string) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata returns a metadata value.
func (c *CorrelationContext) GetMetadata(key string) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// Metadata returns a copy of all metadata.
func (c *CorrelationContext) Metadata() map[string]string {
	if c == nil {
		return make(map[string]string)
	}
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
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, correlationContextKey, c)
}

// FromContext extracts a correlation context from a Go context.
func FromContext(ctx context.Context) (*CorrelationContext, bool) {
	if ctx == nil {
		return nil, false
	}
	value := ctx.Value(correlationContextKey)
	if value == nil {
		return nil, false
	}

	corrCtx, ok := value.(*CorrelationContext)
	return corrCtx, ok
}

// GetOrCreateCorrelationContext gets an existing correlation context or creates a new one.
func GetOrCreateCorrelationContext(ctx context.Context) *CorrelationContext {
	if ctx == nil {
		return NewCorrelationContext()
	}
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
	cleanupInterval    time.Duration
	cleanupMaxAge      time.Duration
	stopCleanup        chan struct{}
	cleanupRunning     bool
	cleanupMu          sync.Mutex
	cleanupCtx         context.Context
	cleanupCancel      context.CancelFunc
}

// NewCorrelationManager creates a new correlation manager.
func NewCorrelationManager(maxActive int) *CorrelationManager {
	if maxActive <= 0 {
		maxActive = 1000 // Default value
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	return &CorrelationManager{
		activeCorrelations: make(map[CorrelationID]*CorrelationContext),
		maxActive:          maxActive,
		cleanupInterval:    5 * time.Minute,  // Default cleanup interval
		cleanupMaxAge:      30 * time.Minute, // Default max age
		stopCleanup:        make(chan struct{}),
		cleanupCtx:         cleanupCtx,
		cleanupCancel:      cleanupCancel,
	}
}

// SetCleanupConfig configures automatic cleanup parameters.
func (cm *CorrelationManager) SetCleanupConfig(interval, maxAge time.Duration) {
	if cm == nil {
		return
	}
	cm.cleanupMu.Lock()
	defer cm.cleanupMu.Unlock()

	if interval > 0 {
		cm.cleanupInterval = interval
	}
	if maxAge > 0 {
		cm.cleanupMaxAge = maxAge
	}
}

// IsAutoCleanupRunning returns whether automatic cleanup is currently running.
func (cm *CorrelationManager) IsAutoCleanupRunning() bool {
	if cm == nil {
		return false
	}
	cm.cleanupMu.Lock()
	defer cm.cleanupMu.Unlock()
	return cm.cleanupRunning
}

// StartAutoCleanup starts automatic cleanup of expired correlation contexts.
func (cm *CorrelationManager) StartAutoCleanup() {
	if cm == nil {
		return
	}

	cm.cleanupMu.Lock()
	defer cm.cleanupMu.Unlock()

	if cm.cleanupRunning {
		return
	}

	cm.cleanupRunning = true
	go cm.autoCleanup()
}

// StopAutoCleanup stops automatic cleanup.
func (cm *CorrelationManager) StopAutoCleanup() {
	if cm == nil {
		return
	}

	cm.cleanupMu.Lock()
	defer cm.cleanupMu.Unlock()

	if !cm.cleanupRunning {
		return
	}

	close(cm.stopCleanup)
	cm.cleanupRunning = false
	// Recreate the channel for potential future restarts
	cm.stopCleanup = make(chan struct{})
}

// Close gracefully shuts down the correlation manager.
func (cm *CorrelationManager) Close() error {
	if cm == nil {
		return nil
	}

	// Stop auto-cleanup
	cm.StopAutoCleanup()

	// Cancel cleanup context
	if cm.cleanupCancel != nil {
		cm.cleanupCancel()
	}

	return nil
}

// autoCleanup runs the automatic cleanup loop.
func (cm *CorrelationManager) autoCleanup() {
	ticker := time.NewTicker(cm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			removed := cm.Cleanup(cm.cleanupMaxAge)
			if removed > 0 {
				logx.Info("Auto-cleanup: removed expired correlation contexts", logx.Int("removed_count", removed))
			}
		case <-cm.stopCleanup:
			return
		case <-cm.cleanupCtx.Done():
			return
		}
	}
}

// Register registers a correlation context.
func (cm *CorrelationManager) Register(corrCtx *CorrelationContext) error {
	if cm == nil {
		return fmt.Errorf("correlation manager is nil")
	}
	if corrCtx == nil {
		return fmt.Errorf("correlation context is nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.activeCorrelations) >= cm.maxActive {
		return fmt.Errorf("maximum number of active correlations reached: %d (current: %d)", cm.maxActive, len(cm.activeCorrelations))
	}

	cm.activeCorrelations[corrCtx.ID()] = corrCtx
	return nil
}

// Unregister removes a correlation context.
func (cm *CorrelationManager) Unregister(id CorrelationID) {
	if cm == nil || !id.IsValid() {
		return
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.activeCorrelations, id)
}

// Get retrieves a correlation context.
func (cm *CorrelationManager) Get(id CorrelationID) (*CorrelationContext, bool) {
	if cm == nil || !id.IsValid() {
		return nil, false
	}
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	corrCtx, exists := cm.activeCorrelations[id]
	return corrCtx, exists
}

// ActiveCount returns the number of active correlations.
func (cm *CorrelationManager) ActiveCount() int {
	if cm == nil {
		return 0
	}
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.activeCorrelations)
}

// Cleanup removes expired correlation contexts.
func (cm *CorrelationManager) Cleanup(maxAge time.Duration) int {
	if cm == nil || maxAge <= 0 {
		return 0
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, corrCtx := range cm.activeCorrelations {
		if corrCtx != nil && now.Sub(corrCtx.createdAt) > maxAge {
			delete(cm.activeCorrelations, id)
			removed++
		}
	}

	return removed
}

// List returns all active correlation contexts.
func (cm *CorrelationManager) List() []*CorrelationContext {
	if cm == nil {
		return nil
	}
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*CorrelationContext, 0, len(cm.activeCorrelations))
	for _, corrCtx := range cm.activeCorrelations {
		if corrCtx != nil {
			result = append(result, corrCtx)
		}
	}
	return result
}

// CorrelationPropagator handles correlation ID propagation across message boundaries.
type CorrelationPropagator struct {
	manager *CorrelationManager
	logger  *logx.Logger
}

// NewCorrelationPropagator creates a new correlation propagator.
func NewCorrelationPropagator(manager *CorrelationManager) *CorrelationPropagator {
	logger, _ := logx.NewLogger()
	return &CorrelationPropagator{
		manager: manager,
		logger:  logger,
	}
}

// SetLogger sets a custom logger for the propagator.
func (cp *CorrelationPropagator) SetLogger(logger *logx.Logger) {
	if cp != nil && logger != nil {
		cp.logger = logger
	}
}

// Inject injects correlation information into message headers.
func (cp *CorrelationPropagator) Inject(ctx context.Context, headers map[string]string) {
	if cp == nil || ctx == nil || headers == nil {
		return
	}

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
	if cp == nil || ctx == nil {
		return ctx
	}
	if headers == nil {
		// Create a new correlation context if no headers are present
		corrCtx := NewCorrelationContext()
		return corrCtx.ToContext(ctx)
	}

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

	// Extract metadata with improved validation
	for k, v := range headers {
		if len(k) > 23 && k[:23] == "X-Correlation-Metadata-" {
			metadataKey := k[23:]
			if metadataKey != "" { // Validate that the key is not empty
				corrCtx.SetMetadata(metadataKey, v)
			}
		}
	}

	// Register with manager
	if cp.manager != nil {
		if err := cp.manager.Register(corrCtx); err != nil {
			// Use proper logging instead of fmt.Printf
			if cp.logger != nil {
				cp.logger.Error("Failed to register correlation context", logx.ErrorField(err))
			}
		}
	}

	return corrCtx.ToContext(ctx)
}

// CreateChild creates a child correlation context from the current context.
func (cp *CorrelationPropagator) CreateChild(ctx context.Context) context.Context {
	if cp == nil || ctx == nil {
		return ctx
	}

	parentCtx, exists := FromContext(ctx)
	if !exists {
		return NewCorrelationContext().ToContext(ctx)
	}

	childCtx := NewCorrelationContextFromParent(parentCtx)
	if cp.manager != nil {
		if err := cp.manager.Register(childCtx); err != nil {
			// Use proper logging instead of fmt.Printf
			if cp.logger != nil {
				cp.logger.Error("Failed to register child correlation context", logx.ErrorField(err))
			}
		}
	}

	return childCtx.ToContext(ctx)
}

// CorrelationMiddleware provides middleware for automatic correlation ID management.
type CorrelationMiddleware struct {
	propagator *CorrelationPropagator
}

// NewCorrelationMiddleware creates a new correlation middleware.
func NewCorrelationMiddleware(propagator *CorrelationPropagator) *CorrelationMiddleware {
	if propagator == nil {
		propagator = NewCorrelationPropagator(nil)
	}
	return &CorrelationMiddleware{
		propagator: propagator,
	}
}

// PublisherMiddleware returns middleware for publishers.
func (cm *CorrelationMiddleware) PublisherMiddleware() func(context.Context, string, Message) (context.Context, Message) {
	return func(ctx context.Context, topic string, msg Message) (context.Context, Message) {
		if cm == nil || cm.propagator == nil {
			return ctx, msg
		}

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
		if cm == nil || cm.propagator == nil {
			return ctx
		}
		// Extract correlation information from message headers
		return cm.propagator.Extract(ctx, delivery.Headers)
	}
}

// HandlerMiddleware returns middleware for message handlers.
func (cm *CorrelationMiddleware) HandlerMiddleware() func(context.Context, Delivery) context.Context {
	return func(ctx context.Context, delivery Delivery) context.Context {
		if cm == nil || cm.propagator == nil {
			return ctx
		}
		// Extract correlation information and create child context
		ctx = cm.propagator.Extract(ctx, delivery.Headers)
		return cm.propagator.CreateChild(ctx)
	}
}
