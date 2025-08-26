// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Cached regex patterns for validation to avoid repeated compilation
var (
	messageIDRegex      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	routingKeyRegex     = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	correlationIDRegex  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	replyToRegex        = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	idempotencyKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Message pool for object reuse to reduce allocation overhead
var messagePool = sync.Pool{
	New: func() interface{} {
		return &Message{
			Headers: make(map[string]string),
		}
	},
}

// resetMessage resets a message for reuse in the pool
func resetMessage(msg *Message) {
	if msg == nil {
		return
	}

	// Reset all fields to zero values
	msg.ID = ""
	msg.Key = ""
	msg.Body = nil
	msg.ContentType = ""
	msg.Timestamp = time.Time{}
	msg.Priority = 0
	msg.IdempotencyKey = ""
	msg.CorrelationID = ""
	msg.ReplyTo = ""
	msg.Expiration = 0

	// Clear headers map but keep the allocated map
	for k := range msg.Headers {
		delete(msg.Headers, k)
	}
}

// Message represents a message to be published.
type Message struct {
	// ID is a unique identifier for the message.
	ID string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9_-]+$"`

	// Key is the routing/partition key for the message.
	Key string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9._-]+$"`

	// Body is the message payload.
	Body []byte `validate:"required,min=1,max=10485760"` // 10MB max

	// Headers contains additional metadata for the message.
	Headers map[string]string `validate:"max=100"` // Max 100 headers

	// ContentType specifies the content type of the message body.
	ContentType string `validate:"required,oneof=application/json application/xml text/plain text/html application/octet-stream application/protobuf application/avro"`

	// Timestamp is when the message was created.
	Timestamp time.Time

	// Priority is the message priority (0-255, higher numbers = higher priority).
	Priority uint8 `validate:"max=255"`

	// IdempotencyKey ensures message deduplication.
	IdempotencyKey string `validate:"max=255,regexp=^[a-zA-Z0-9_-]*$"`

	// CorrelationID helps track related messages.
	CorrelationID string `validate:"max=255,regexp=^[a-zA-Z0-9_-]*$"`

	// ReplyTo specifies where responses should be sent.
	ReplyTo string `validate:"max=255,regexp=^[a-zA-Z0-9._-]*$"`

	// Expiration specifies when the message expires.
	Expiration time.Duration `validate:"min=0,max=86400000000000"` // 0 to 24 hours in nanoseconds
}

// Delivery represents a message delivered to a consumer.
type Delivery struct {
	// Message contains the message data.
	Message

	// DeliveryTag is the transport-specific delivery identifier.
	DeliveryTag uint64 `validate:"required"`

	// Exchange is the exchange the message was published to (RabbitMQ specific).
	Exchange string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9._-]+$"`

	// RoutingKey is the routing key used to deliver the message.
	RoutingKey string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9._-]+$"`

	// Queue is the queue the message was consumed from.
	Queue string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9._-]+$"`

	// Redelivered indicates if this message was previously delivered.
	Redelivered bool

	// DeliveryCount is the number of times this message has been delivered.
	DeliveryCount int `validate:"min=1,max=100"`

	// ConsumerTag identifies the consumer that received this message.
	ConsumerTag string `validate:"required,min=1,max=255,regexp=^[a-zA-Z0-9._-]+$"`
}

// MessageOption is a function that configures a Message.
type MessageOption func(*Message)

// WithID sets the message ID.
func WithID(id string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.ID = id
		}
	}
}

// WithKey sets the message key.
func WithKey(key string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.Key = key
		}
	}
}

// WithHeaders sets the message headers.
func WithHeaders(headers map[string]string) MessageOption {
	return func(m *Message) {
		if m != nil {
			// Validate headers immediately
			if headers != nil {
				if len(headers) > 100 {
					panic(fmt.Sprintf("too many headers: %d > 100", len(headers)))
				}
				for key, value := range headers {
					if len(key) > 255 {
						panic(fmt.Sprintf("header key too long: %d > 255", len(key)))
					}
					if len(value) > MaxHeaderSize {
						panic(fmt.Sprintf("header value too large: %d > %d", len(value), MaxHeaderSize))
					}
				}
			}
			m.Headers = headers
		}
	}
}

// WithHeader adds a single header to the message.
func WithHeader(key, value string) MessageOption {
	return func(m *Message) {
		if m != nil {
			if m.Headers == nil {
				m.Headers = make(map[string]string)
			}
			m.Headers[key] = value
		}
	}
}

// WithContentType sets the message content type.
func WithContentType(contentType string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.ContentType = contentType
		}
	}
}

// WithTimestamp sets the message timestamp.
func WithTimestamp(timestamp time.Time) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.Timestamp = timestamp
		}
	}
}

// WithPriority sets the message priority.
func WithPriority(priority uint8) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.Priority = priority
		}
	}
}

// WithIdempotencyKey sets the idempotency key.
func WithIdempotencyKey(key string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.IdempotencyKey = key
		}
	}
}

// WithCorrelationID sets the correlation ID.
func WithCorrelationID(id string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.CorrelationID = id
		}
	}
}

// WithReplyTo sets the reply-to address.
func WithReplyTo(replyTo string) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.ReplyTo = replyTo
		}
	}
}

// WithExpiration sets the message expiration.
func WithExpiration(expiration time.Duration) MessageOption {
	return func(m *Message) {
		if m != nil {
			m.Expiration = expiration
		}
	}
}

// NewMessage creates a new message with the given body and options.
func NewMessage(body []byte, opts ...MessageOption) *Message {
	// Validate body
	if body == nil {
		panic("message body cannot be nil")
	}
	if len(body) == 0 {
		panic("message body cannot be empty")
	}
	if len(body) > MaxMessageSize {
		panic(fmt.Sprintf("message body too large: %d > %d", len(body), MaxMessageSize))
	}

	// Get message from pool
	msg := messagePool.Get().(*Message)

	// Initialize message with provided body and defaults
	msg.ID = generateMessageID()
	msg.Body = body
	msg.ContentType = "application/json" // Default content type
	msg.Timestamp = time.Now()

	// Apply options
	for _, opt := range opts {
		if opt != nil {
			opt(msg)
		}
	}

	// Validate final message
	if err := validateMessage(msg); err != nil {
		// Return message to pool before panicking
		resetMessage(msg)
		messagePool.Put(msg)
		panic(fmt.Sprintf("invalid message: %v", err))
	}

	return msg
}

// validateMessage validates a message for correctness
func validateMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// Validate message ID
	if msg.ID == "" {
		return fmt.Errorf("message ID cannot be empty")
	}
	if len(msg.ID) > MaxMessageIDLength {
		return fmt.Errorf("message ID too long: %d > %d", len(msg.ID), MaxMessageIDLength)
	}
	if !isValidMessageID(msg.ID) {
		return fmt.Errorf("invalid message ID format: %s", msg.ID)
	}

	// Validate message body
	if msg.Body == nil {
		return fmt.Errorf("message body cannot be nil")
	}
	if len(msg.Body) == 0 {
		return fmt.Errorf("message body cannot be empty")
	}
	if len(msg.Body) > MaxMessageSize {
		return fmt.Errorf("message too large: %d > %d", len(msg.Body), MaxMessageSize)
	}

	// Validate routing key
	if msg.Key == "" {
		return fmt.Errorf("routing key cannot be empty")
	}
	if len(msg.Key) > MaxRoutingKeyLength {
		return fmt.Errorf("routing key too long: %d > %d", len(msg.Key), MaxRoutingKeyLength)
	}
	if !isValidRoutingKey(msg.Key) {
		return fmt.Errorf("invalid routing key format: %s", msg.Key)
	}

	// Validate content type
	if msg.ContentType == "" {
		return fmt.Errorf("content type cannot be empty")
	}
	if !isValidContentType(msg.ContentType) {
		return fmt.Errorf("unsupported content type: %s", msg.ContentType)
	}

	// Validate headers
	if msg.Headers != nil {
		if len(msg.Headers) > 100 {
			return fmt.Errorf("too many headers: %d > 100", len(msg.Headers))
		}
		for key, value := range msg.Headers {
			if len(key) > 255 {
				return fmt.Errorf("header key too long: %d > 255", len(key))
			}
			if len(value) > MaxHeaderSize {
				return fmt.Errorf("header value too large: %d > %d", len(value), MaxHeaderSize)
			}
		}
	}

	// Validate priority
	if msg.Priority > MaxPriority {
		return fmt.Errorf("priority too high: %d > %d", msg.Priority, MaxPriority)
	}

	// Validate correlation ID
	if msg.CorrelationID != "" {
		if len(msg.CorrelationID) > MaxCorrelationIDLength {
			return fmt.Errorf("correlation ID too long: %d > %d", len(msg.CorrelationID), MaxCorrelationIDLength)
		}
		if !isValidCorrelationID(msg.CorrelationID) {
			return fmt.Errorf("invalid correlation ID format: %s", msg.CorrelationID)
		}
	}

	// Validate reply-to
	if msg.ReplyTo != "" {
		if len(msg.ReplyTo) > MaxReplyToLength {
			return fmt.Errorf("reply-to too long: %d > %d", len(msg.ReplyTo), MaxReplyToLength)
		}
		if !isValidReplyTo(msg.ReplyTo) {
			return fmt.Errorf("invalid reply-to format: %s", msg.ReplyTo)
		}
	}

	// Validate idempotency key
	if msg.IdempotencyKey != "" {
		if len(msg.IdempotencyKey) > MaxIdempotencyKeyLength {
			return fmt.Errorf("idempotency key too long: %d > %d", len(msg.IdempotencyKey), MaxIdempotencyKeyLength)
		}
		if !isValidIdempotencyKey(msg.IdempotencyKey) {
			return fmt.Errorf("invalid idempotency key format: %s", msg.IdempotencyKey)
		}
	}

	// Validate expiration
	if msg.Expiration < 0 {
		return fmt.Errorf("expiration cannot be negative")
	}
	if msg.Expiration > MaxTimeout {
		return fmt.Errorf("expiration too long: %v > %v", msg.Expiration, MaxTimeout)
	}

	return nil
}

// Helper functions for validation
func isValidMessageID(id string) bool {
	// Message ID validation regex: alphanumeric, underscores, hyphens
	return messageIDRegex.MatchString(id)
}

func isValidRoutingKey(key string) bool {
	// Routing key validation regex: alphanumeric, dots, underscores, hyphens
	return routingKeyRegex.MatchString(key)
}

func isValidContentType(contentType string) bool {
	for _, supported := range supportedContentTypes {
		if contentType == supported {
			return true
		}
	}
	return false
}

func isValidCorrelationID(id string) bool {
	// Correlation ID validation regex: alphanumeric, underscores, hyphens
	return correlationIDRegex.MatchString(id)
}

func isValidReplyTo(replyTo string) bool {
	// Reply-to validation regex: alphanumeric, dots, underscores, hyphens
	return replyToRegex.MatchString(replyTo)
}

func isValidIdempotencyKey(key string) bool {
	// Idempotency key validation regex: alphanumeric, underscores, hyphens
	return idempotencyKeyRegex.MatchString(key)
}

// Atomic counter for generating unique IDs
var messageIDCounter uint64

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to time-based ID with atomic counter if random generation fails
		counter := atomic.AddUint64(&messageIDCounter, 1)
		return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), counter)
	}
	return "msg-" + hex.EncodeToString(bytes)
}

// NewTextMessage creates a new text message.
func NewTextMessage(text string, options ...MessageOption) *Message {
	if text == "" {
		panic("text message cannot be empty")
	}
	return NewMessage([]byte(text), append(options, WithContentType("text/plain"))...)
}

// NewJSONMessage creates a new JSON message from the given data.
func NewJSONMessage(data interface{}, options ...MessageOption) (*Message, error) {
	if data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return NewMessage(body, append(options, WithContentType("application/json"))...), nil
}

// MustNewJSONMessage creates a new JSON message and panics on error.
func MustNewJSONMessage(data interface{}, options ...MessageOption) *Message {
	msg, err := NewJSONMessage(data, options...)
	if err != nil {
		panic(err)
	}
	return msg
}

// NewPooledMessage creates a new message using the object pool.
// The message should be returned to the pool using ReturnToPool() when no longer needed.
func NewPooledMessage(body []byte, opts ...MessageOption) *Message {
	return NewMessage(body, opts...)
}

// Clone creates a deep copy of the message.
func (m Message) Clone() Message {
	clone := m
	clone.Body = make([]byte, len(m.Body))
	copy(clone.Body, m.Body)

	if m.Headers != nil {
		clone.Headers = make(map[string]string)
		for k, v := range m.Headers {
			clone.Headers[k] = v
		}
	} else {
		clone.Headers = nil
	}

	return clone
}

// String returns a string representation of the message (without body).
func (m Message) String() string {
	return "Message{ID:" + m.ID +
		", Key:" + m.Key +
		", ContentType:" + m.ContentType +
		", BodyLen:" + strconv.Itoa(len(m.Body)) +
		", Priority:" + strconv.Itoa(int(m.Priority)) + "}"
}

// ReturnToPool returns the message to the object pool for reuse.
// This should be called when the message is no longer needed.
func (m *Message) ReturnToPool() {
	if m == nil {
		return
	}
	resetMessage(m)
	messagePool.Put(m)
}

// UnmarshalTo unmarshals JSON data into the provided destination.
func (m Message) UnmarshalTo(dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}
	if m.ContentType != "application/json" {
		return ErrInvalidContentType
	}
	return json.Unmarshal(m.Body, dest)
}

// Text returns the message body as text.
func (m Message) Text() string {
	return string(m.Body)
}

// Size returns the size of the message in bytes.
func (m Message) Size() int {
	// Use int64 to prevent overflow during calculation
	var size int64

	size += int64(len(m.Body))
	size += int64(len(m.ID))
	size += int64(len(m.Key))
	size += int64(len(m.ContentType))
	size += int64(len(m.IdempotencyKey))
	size += int64(len(m.CorrelationID))
	size += int64(len(m.ReplyTo))

	if m.Headers != nil {
		for k, v := range m.Headers {
			size += int64(len(k) + len(v))
		}
	}

	// Check for overflow
	if size > int64(^int(0)>>1) {
		return ^int(0) >> 1 // Return max int value on overflow
	}

	return int(size)
}

// IsExpired checks if the message has expired.
func (m Message) IsExpired() bool {
	if m.Expiration <= 0 {
		return false
	}
	return time.Since(m.Timestamp) > m.Expiration
}

// HasHeader checks if the message has a specific header.
func (m Message) HasHeader(key string) bool {
	if m.Headers == nil {
		return false
	}
	_, exists := m.Headers[key]
	return exists
}

// GetHeader returns the value of a header.
func (m Message) GetHeader(key string) (string, bool) {
	if m.Headers == nil {
		return "", false
	}
	value, exists := m.Headers[key]
	return value, exists
}

// String returns a string representation of the delivery.
func (d Delivery) String() string {
	return "Delivery{" +
		"Exchange:" + d.Exchange +
		", RoutingKey:" + d.RoutingKey +
		", Queue:" + d.Queue +
		", DeliveryTag:" + strconv.FormatUint(d.DeliveryTag, 10) +
		", Redelivered:" + strconv.FormatBool(d.Redelivered) +
		", Message:" + d.Message.String() + "}"
}

// boolToInt converts bool to int for string representation.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
