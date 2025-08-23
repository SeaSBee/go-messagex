// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Message represents a message to be published.
type Message struct {
	// ID is a unique identifier for the message.
	ID string

	// Key is the routing/partition key for the message.
	Key string

	// Body is the message payload.
	Body []byte

	// Headers contains additional metadata for the message.
	Headers map[string]string

	// ContentType specifies the content type of the message body.
	ContentType string

	// Timestamp is when the message was created.
	Timestamp time.Time

	// Priority is the message priority (0-255, higher numbers = higher priority).
	Priority uint8

	// IdempotencyKey ensures message deduplication.
	IdempotencyKey string

	// CorrelationID helps track related messages.
	CorrelationID string

	// ReplyTo specifies where responses should be sent.
	ReplyTo string

	// Expiration specifies when the message expires.
	Expiration time.Duration
}

// Delivery represents a message delivered to a consumer.
type Delivery struct {
	// Message contains the message data.
	Message

	// DeliveryTag is the transport-specific delivery identifier.
	DeliveryTag uint64

	// Exchange is the exchange the message was published to (RabbitMQ specific).
	Exchange string

	// RoutingKey is the routing key used to deliver the message.
	RoutingKey string

	// Queue is the queue the message was consumed from.
	Queue string

	// Redelivered indicates if this message was previously delivered.
	Redelivered bool

	// DeliveryCount is the number of times this message has been delivered.
	DeliveryCount int

	// ConsumerTag identifies the consumer that received this message.
	ConsumerTag string
}

// MessageOption is a function that configures a Message.
type MessageOption func(*Message)

// WithID sets the message ID.
func WithID(id string) MessageOption {
	return func(m *Message) {
		m.ID = id
	}
}

// WithKey sets the message key.
func WithKey(key string) MessageOption {
	return func(m *Message) {
		m.Key = key
	}
}

// WithHeaders sets the message headers.
func WithHeaders(headers map[string]string) MessageOption {
	return func(m *Message) {
		m.Headers = headers
	}
}

// WithHeader adds a single header to the message.
func WithHeader(key, value string) MessageOption {
	return func(m *Message) {
		if m.Headers == nil {
			m.Headers = make(map[string]string)
		}
		m.Headers[key] = value
	}
}

// WithContentType sets the message content type.
func WithContentType(contentType string) MessageOption {
	return func(m *Message) {
		m.ContentType = contentType
	}
}

// WithTimestamp sets the message timestamp.
func WithTimestamp(timestamp time.Time) MessageOption {
	return func(m *Message) {
		m.Timestamp = timestamp
	}
}

// WithPriority sets the message priority.
func WithPriority(priority uint8) MessageOption {
	return func(m *Message) {
		m.Priority = priority
	}
}

// WithIdempotencyKey sets the idempotency key.
func WithIdempotencyKey(key string) MessageOption {
	return func(m *Message) {
		m.IdempotencyKey = key
	}
}

// WithCorrelationID sets the correlation ID.
func WithCorrelationID(id string) MessageOption {
	return func(m *Message) {
		m.CorrelationID = id
	}
}

// WithReplyTo sets the reply-to address.
func WithReplyTo(replyTo string) MessageOption {
	return func(m *Message) {
		m.ReplyTo = replyTo
	}
}

// WithExpiration sets the message expiration.
func WithExpiration(expiration time.Duration) MessageOption {
	return func(m *Message) {
		m.Expiration = expiration
	}
}

// NewMessage creates a new message with the given body and options.
func NewMessage(body []byte, options ...MessageOption) Message {
	msg := Message{
		ID:          generateMessageID(),
		Body:        body,
		Headers:     make(map[string]string),
		ContentType: "application/octet-stream",
		Timestamp:   time.Now(),
	}

	for _, option := range options {
		option(&msg)
	}

	return msg
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to time-based ID if random generation fails
		return fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	return "msg-" + hex.EncodeToString(bytes)
}

// NewTextMessage creates a new text message.
func NewTextMessage(text string, options ...MessageOption) Message {
	return NewMessage([]byte(text), append(options, WithContentType("text/plain"))...)
}

// NewJSONMessage creates a new JSON message from the given data.
func NewJSONMessage(data interface{}, options ...MessageOption) (Message, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return Message{}, err
	}

	return NewMessage(body, append(options, WithContentType("application/json"))...), nil
}

// MustNewJSONMessage creates a new JSON message and panics on error.
func MustNewJSONMessage(data interface{}, options ...MessageOption) Message {
	msg, err := NewJSONMessage(data, options...)
	if err != nil {
		panic(err)
	}
	return msg
}

// Clone creates a deep copy of the message.
func (m Message) Clone() Message {
	clone := m
	clone.Body = make([]byte, len(m.Body))
	copy(clone.Body, m.Body)

	clone.Headers = make(map[string]string)
	for k, v := range m.Headers {
		clone.Headers[k] = v
	}

	return clone
}

// String returns a string representation of the message (without body).
func (m Message) String() string {
	return "Message{ID:" + m.ID +
		", Key:" + m.Key +
		", ContentType:" + m.ContentType +
		", BodyLen:" + string(rune(len(m.Body))) +
		", Priority:" + string(rune(m.Priority)) + "}"
}

// UnmarshalTo unmarshals JSON data into the provided destination.
func (m Message) UnmarshalTo(dest interface{}) error {
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
	size := len(m.Body) + len(m.ID) + len(m.Key) + len(m.ContentType) +
		len(m.IdempotencyKey) + len(m.CorrelationID) + len(m.ReplyTo)

	for k, v := range m.Headers {
		size += len(k) + len(v)
	}

	return size
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
	_, exists := m.Headers[key]
	return exists
}

// GetHeader returns the value of a header.
func (m Message) GetHeader(key string) (string, bool) {
	value, exists := m.Headers[key]
	return value, exists
}

// String returns a string representation of the delivery.
func (d Delivery) String() string {
	return "Delivery{" +
		"Exchange:" + d.Exchange +
		", RoutingKey:" + d.RoutingKey +
		", Queue:" + d.Queue +
		", DeliveryTag:" + string(rune(d.DeliveryTag)) +
		", Redelivered:" + string(rune(boolToInt(d.Redelivered))) +
		", Message:" + d.Message.String() + "}"
}

// boolToInt converts bool to int for string representation.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
