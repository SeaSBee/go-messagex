// Package messaging provides unified message structures for RabbitMQ
package messaging

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MessagePriority represents message priority levels
type MessagePriority int

const (
	PriorityLow      MessagePriority = 0
	PriorityNormal   MessagePriority = 1
	PriorityHigh     MessagePriority = 2
	PriorityCritical MessagePriority = 3
)

// Message represents a unified message structure
type Message struct {
	ID          string                 `json:"id"`
	Body        []byte                 `json:"body"`
	Headers     map[string]interface{} `json:"headers"`
	Properties  MessageProperties      `json:"properties"`
	Timestamp   time.Time              `json:"timestamp"`
	Priority    MessagePriority        `json:"priority"`
	ContentType string                 `json:"content_type"`
	Encoding    string                 `json:"encoding"`
	Metadata    map[string]interface{} `json:"metadata"`
	mu          sync.RWMutex           `json:"-"` // Protects Headers and Metadata maps
	cachedSize  int                    `json:"-"` // Cached size for performance
	sizeValid   bool                   `json:"-"` // Flag to indicate if cached size is valid
}

// MessageProperties contains message-specific properties
type MessageProperties struct {
	Queue         string            `json:"queue,omitempty"`
	Exchange      string            `json:"exchange,omitempty"`
	RoutingKey    string            `json:"routing_key,omitempty"`
	Persistent    bool              `json:"persistent"`
	Mandatory     bool              `json:"mandatory"`
	Immediate     bool              `json:"immediate"`
	Expiration    time.Duration     `json:"expiration,omitempty"`
	TTL           time.Duration     `json:"ttl,omitempty"`
	ReplyTo       string            `json:"reply_to,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	MessageID     string            `json:"message_id,omitempty"`
	UserID        string            `json:"user_id,omitempty"`
	AppID         string            `json:"app_id,omitempty"`
	Custom        map[string]string `json:"custom,omitempty"`
}

// Delivery represents a message delivery with acknowledgment capabilities
type Delivery struct {
	Message      *Message
	Acknowledger Acknowledger
	Tag          string
	Redelivered  bool
	Exchange     string
	RoutingKey   string
}

// Acknowledger defines the interface for message acknowledgment
type Acknowledger interface {
	Ack() error
	Nack(requeue bool) error
	Reject(requeue bool) error
}

// NewMessage creates a new message with the given body
func NewMessage(body []byte) *Message {
	return &Message{
		ID:          uuid.New().String(),
		Body:        body,
		Headers:     make(map[string]interface{}),
		Properties:  MessageProperties{},
		Timestamp:   time.Now(),
		Priority:    PriorityNormal,
		ContentType: "application/octet-stream",
		Encoding:    "utf-8",
		Metadata:    make(map[string]interface{}),
	}
}

// NewMessageWithID creates a new message with a specific ID
func NewMessageWithID(id string, body []byte) *Message {
	msg := NewMessage(body)
	msg.ID = id
	return msg
}

// NewTextMessage creates a new text message
func NewTextMessage(text string) *Message {
	msg := NewMessage([]byte(text))
	msg.ContentType = "text/plain"
	return msg
}

// NewJSONMessage creates a new JSON message
func NewJSONMessage(v interface{}) (*Message, error) {
	if v == nil {
		return nil, NewValidationError("cannot create JSON message from nil value", "value", v, "not_nil", nil)
	}

	body, err := json.Marshal(v)
	if err != nil {
		return nil, NewValidationError("failed to marshal JSON", "value", v, "json_marshal", err)
	}

	msg := NewMessage(body)
	msg.ContentType = "application/json"
	return msg, nil
}

// SetHeader sets a header value
func (m *Message) SetHeader(key string, value interface{}) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers == nil {
		m.Headers = make(map[string]interface{})
	}
	m.Headers[key] = value
	m.sizeValid = false // Invalidate cached size
}

// GetHeader gets a header value
func (m *Message) GetHeader(key string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Headers == nil {
		return nil, false
	}
	value, exists := m.Headers[key]
	return value, exists
}

// SetMetadata sets a metadata value
func (m *Message) SetMetadata(key string, value interface{}) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
	m.sizeValid = false // Invalidate cached size
}

// GetMetadata gets a metadata value
func (m *Message) GetMetadata(key string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Metadata == nil {
		return nil, false
	}
	value, exists := m.Metadata[key]
	return value, exists
}

// GetAllHeaders returns a copy of all headers (thread-safe)
func (m *Message) GetAllHeaders() map[string]interface{} {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Headers == nil {
		return nil
	}

	// Create a copy to avoid race conditions
	headers := make(map[string]interface{})
	for k, v := range m.Headers {
		headers[k] = v
	}
	return headers
}

// GetAllMetadata returns a copy of all metadata (thread-safe)
func (m *Message) GetAllMetadata() map[string]interface{} {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Metadata == nil {
		return nil
	}

	// Create a copy to avoid race conditions
	metadata := make(map[string]interface{})
	for k, v := range m.Metadata {
		metadata[k] = v
	}
	return metadata
}

// SetPriority sets the message priority
func (m *Message) SetPriority(priority MessagePriority) error {
	if m == nil {
		return fmt.Errorf("cannot set priority on nil message")
	}
	if priority < PriorityLow || priority > PriorityCritical {
		return fmt.Errorf("invalid priority: %d, must be between %d and %d", priority, PriorityLow, PriorityCritical)
	}
	m.Priority = priority
	return nil
}

// SetTTL sets the message time-to-live
func (m *Message) SetTTL(ttl time.Duration) error {
	if m == nil {
		return fmt.Errorf("cannot set TTL on nil message")
	}
	if ttl < 0 {
		return fmt.Errorf("TTL cannot be negative: %v", ttl)
	}
	m.Properties.TTL = ttl
	return nil
}

// SetExpiration sets the message expiration time
func (m *Message) SetExpiration(expiration time.Duration) error {
	if m == nil {
		return fmt.Errorf("cannot set expiration on nil message")
	}
	if expiration < 0 {
		return fmt.Errorf("expiration cannot be negative: %v", expiration)
	}
	m.Properties.Expiration = expiration
	return nil
}

// SetPersistent sets the message persistence flag
func (m *Message) SetPersistent(persistent bool) {
	if m == nil {
		return
	}
	m.Properties.Persistent = persistent
}

// SetRoutingKey sets the routing key
func (m *Message) SetRoutingKey(routingKey string) {
	if m == nil {
		return
	}
	m.Properties.RoutingKey = routingKey
}

// SetExchange sets the exchange
func (m *Message) SetExchange(exchange string) {
	if m == nil {
		return
	}
	m.Properties.Exchange = exchange
}

// SetQueue sets the queue
func (m *Message) SetQueue(queue string) {
	if m == nil {
		return
	}
	m.Properties.Queue = queue
}

// SetCorrelationID sets the correlation ID
func (m *Message) SetCorrelationID(correlationID string) {
	if m == nil {
		return
	}
	m.Properties.CorrelationID = correlationID
}

// SetReplyTo sets the reply-to queue
func (m *Message) SetReplyTo(replyTo string) {
	if m == nil {
		return
	}
	m.Properties.ReplyTo = replyTo
}

// ToJSON converts the message to JSON
func (m *Message) ToJSON() ([]byte, error) {
	if m == nil {
		return nil, NewValidationError("cannot convert nil message to JSON", "message", m, "not_nil", nil)
	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, NewValidationError("failed to marshal message to JSON", "message", m, "json_marshal", err)
	}

	return data, nil
}

// FromJSON creates a message from JSON
func FromJSON(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, NewValidationError("cannot create message from empty JSON data", "data", data, "not_empty", nil)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, NewValidationError("failed to unmarshal message from JSON", "data", string(data), "json_unmarshal", err)
	}

	// Validate the unmarshaled message
	if err := msg.Validate(); err != nil {
		return nil, NewValidationError("invalid message after JSON unmarshaling", "message", &msg, "validation", err)
	}

	return &msg, nil
}

// Clone creates a deep copy of the message
func (m *Message) Clone() *Message {
	if m == nil {
		return nil
	}

	clone := &Message{
		ID:          m.ID,
		Body:        make([]byte, len(m.Body)),
		Headers:     make(map[string]interface{}),
		Properties:  MessageProperties{},
		Timestamp:   m.Timestamp,
		Priority:    m.Priority,
		ContentType: m.ContentType,
		Encoding:    m.Encoding,
		Metadata:    make(map[string]interface{}),
	}

	// Copy body
	copy(clone.Body, m.Body)

	// Copy headers and metadata (thread-safe)
	m.mu.RLock()
	for k, v := range m.Headers {
		clone.Headers[k] = v
	}

	for k, v := range m.Metadata {
		clone.Metadata[k] = v
	}
	m.mu.RUnlock()

	// Copy properties (deep copy)
	clone.Properties.Queue = m.Properties.Queue
	clone.Properties.Exchange = m.Properties.Exchange
	clone.Properties.RoutingKey = m.Properties.RoutingKey
	clone.Properties.Persistent = m.Properties.Persistent
	clone.Properties.Mandatory = m.Properties.Mandatory
	clone.Properties.Immediate = m.Properties.Immediate
	clone.Properties.Expiration = m.Properties.Expiration
	clone.Properties.TTL = m.Properties.TTL
	clone.Properties.ReplyTo = m.Properties.ReplyTo
	clone.Properties.CorrelationID = m.Properties.CorrelationID
	clone.Properties.MessageID = m.Properties.MessageID
	clone.Properties.UserID = m.Properties.UserID
	clone.Properties.AppID = m.Properties.AppID

	// Copy custom properties
	if m.Properties.Custom != nil {
		clone.Properties.Custom = make(map[string]string)
		for k, v := range m.Properties.Custom {
			clone.Properties.Custom[k] = v
		}
	}

	return clone
}

// String returns a string representation of the message
func (m *Message) String() string {
	if m == nil {
		return "Message{nil}"
	}
	return fmt.Sprintf("Message{ID: %s, Body: %d bytes, Priority: %d, Timestamp: %s}",
		m.ID, len(m.Body), m.Priority, m.Timestamp.Format(time.RFC3339))
}

// IsEmpty returns true if the message body is empty
func (m *Message) IsEmpty() bool {
	if m == nil {
		return true
	}
	return len(m.Body) == 0
}

// Validate validates the message properties and returns any errors
func (m *Message) Validate() error {
	if m == nil {
		return fmt.Errorf("message is nil")
	}

	// Validate ID
	if m.ID == "" {
		return fmt.Errorf("message ID cannot be empty")
	}

	// Validate priority
	if m.Priority < PriorityLow || m.Priority > PriorityCritical {
		return fmt.Errorf("invalid priority: %d, must be between %d and %d", m.Priority, PriorityLow, PriorityCritical)
	}

	// Validate TTL
	if m.Properties.TTL < 0 {
		return fmt.Errorf("TTL cannot be negative: %v", m.Properties.TTL)
	}

	// Validate expiration
	if m.Properties.Expiration < 0 {
		return fmt.Errorf("expiration cannot be negative: %v", m.Properties.Expiration)
	}

	// Validate content type
	if m.ContentType == "" {
		return fmt.Errorf("content type cannot be empty")
	}

	// Validate encoding
	if m.Encoding == "" {
		return fmt.Errorf("encoding cannot be empty")
	}

	return nil
}

// Size returns the size of the message in bytes
func (m *Message) Size() int {
	if m == nil {
		return 0
	}

	// Check if we have a valid cached size
	m.mu.RLock()
	if m.sizeValid {
		size := m.cachedSize
		m.mu.RUnlock()
		return size
	}
	m.mu.RUnlock()

	// Calculate size and cache it
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.sizeValid {
		return m.cachedSize
	}

	size := len(m.Body)

	// Add header sizes
	for k, v := range m.Headers {
		size += len(k)
		if str, ok := v.(string); ok {
			size += len(str)
		}
	}

	// Add metadata sizes
	for k, v := range m.Metadata {
		size += len(k)
		if str, ok := v.(string); ok {
			size += len(str)
		}
	}

	// Cache the result
	m.cachedSize = size
	m.sizeValid = true

	return size
}

// MessageHandler defines the interface for handling messages
type MessageHandler func(*Delivery) error

// MessageBuilder provides a fluent interface for building immutable messages
type MessageBuilder struct {
	msg *Message
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		msg: &Message{
			ID:          uuid.New().String(),
			Headers:     make(map[string]interface{}),
			Properties:  MessageProperties{},
			Timestamp:   time.Now(),
			Priority:    PriorityNormal,
			ContentType: "application/octet-stream",
			Encoding:    "utf-8",
			Metadata:    make(map[string]interface{}),
		},
	}
}

// WithBody sets the message body
func (b *MessageBuilder) WithBody(body []byte) *MessageBuilder {
	b.msg.Body = make([]byte, len(body))
	copy(b.msg.Body, body)
	return b
}

// WithTextBody sets the message body as text
func (b *MessageBuilder) WithTextBody(text string) *MessageBuilder {
	b.msg.Body = []byte(text)
	b.msg.ContentType = "text/plain"
	return b
}

// WithJSONBody sets the message body as JSON
func (b *MessageBuilder) WithJSONBody(v interface{}) *MessageBuilder {
	body, err := json.Marshal(v)
	if err != nil {
		// Store error in metadata for later handling
		b.msg.Metadata["json_error"] = err.Error()
		return b
	}
	b.msg.Body = body
	b.msg.ContentType = "application/json"
	return b
}

// WithHeader adds a header
func (b *MessageBuilder) WithHeader(key string, value interface{}) *MessageBuilder {
	b.msg.Headers[key] = value
	return b
}

// WithMetadata adds metadata
func (b *MessageBuilder) WithMetadata(key string, value interface{}) *MessageBuilder {
	b.msg.Metadata[key] = value
	return b
}

// WithPriority sets the priority
func (b *MessageBuilder) WithPriority(priority MessagePriority) *MessageBuilder {
	b.msg.Priority = priority
	return b
}

// WithTTL sets the TTL
func (b *MessageBuilder) WithTTL(ttl time.Duration) *MessageBuilder {
	b.msg.Properties.TTL = ttl
	return b
}

// WithExpiration sets the expiration
func (b *MessageBuilder) WithExpiration(expiration time.Duration) *MessageBuilder {
	b.msg.Properties.Expiration = expiration
	return b
}

// WithRoutingKey sets the routing key
func (b *MessageBuilder) WithRoutingKey(routingKey string) *MessageBuilder {
	b.msg.Properties.RoutingKey = routingKey
	return b
}

// WithExchange sets the exchange
func (b *MessageBuilder) WithExchange(exchange string) *MessageBuilder {
	b.msg.Properties.Exchange = exchange
	return b
}

// WithQueue sets the queue
func (b *MessageBuilder) WithQueue(queue string) *MessageBuilder {
	b.msg.Properties.Queue = queue
	return b
}

// WithCorrelationID sets the correlation ID
func (b *MessageBuilder) WithCorrelationID(correlationID string) *MessageBuilder {
	b.msg.Properties.CorrelationID = correlationID
	return b
}

// WithReplyTo sets the reply-to queue
func (b *MessageBuilder) WithReplyTo(replyTo string) *MessageBuilder {
	b.msg.Properties.ReplyTo = replyTo
	return b
}

// WithPersistent sets the persistence flag
func (b *MessageBuilder) WithPersistent(persistent bool) *MessageBuilder {
	b.msg.Properties.Persistent = persistent
	return b
}

// Build creates the immutable message
func (b *MessageBuilder) Build() (*Message, error) {
	if err := b.msg.Validate(); err != nil {
		return nil, err
	}

	// Mark size as valid since we're building from scratch
	b.msg.sizeValid = true
	b.msg.cachedSize = b.msg.Size()

	return b.msg, nil
}

// BatchMessage represents a batch of messages
type BatchMessage struct {
	Messages []*Message
	Size     int
	Created  time.Time
}

// NewBatchMessage creates a new batch message
func NewBatchMessage(messages []*Message) *BatchMessage {
	size := 0
	for _, msg := range messages {
		size += msg.Size()
	}

	return &BatchMessage{
		Messages: messages,
		Size:     size,
		Created:  time.Now(),
	}
}

// AddMessage adds a message to the batch
func (b *BatchMessage) AddMessage(msg *Message) {
	if b == nil || msg == nil {
		return
	}
	b.Messages = append(b.Messages, msg)
	b.Size += msg.Size()
}

// IsEmpty returns true if the batch is empty
func (b *BatchMessage) IsEmpty() bool {
	if b == nil {
		return true
	}
	return len(b.Messages) == 0
}

// Count returns the number of messages in the batch
func (b *BatchMessage) Count() int {
	if b == nil {
		return 0
	}
	return len(b.Messages)
}

// Clear clears all messages from the batch
func (b *BatchMessage) Clear() {
	if b == nil {
		return
	}
	b.Messages = nil
	b.Size = 0
}
