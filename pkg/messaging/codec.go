// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"
)

// Codec defines the interface for message serialization/deserialization
type Codec interface {
	// Encode encodes data to bytes
	Encode(data interface{}) ([]byte, error)

	// Decode decodes bytes to data
	Decode(data []byte, dest interface{}) error

	// ContentType returns the MIME type for this codec
	ContentType() string

	// Name returns the name of this codec
	Name() string
}

// JSONCodec provides JSON serialization
type JSONCodec struct{}

// NewJSONCodec creates a new JSON codec
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode encodes data to JSON bytes
func (jc *JSONCodec) Encode(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// Decode decodes JSON bytes to data
func (jc *JSONCodec) Decode(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

// ContentType returns the JSON content type
func (jc *JSONCodec) ContentType() string {
	return "application/json"
}

// Name returns the codec name
func (jc *JSONCodec) Name() string {
	return "json"
}

// ProtobufCodec provides Protocol Buffers serialization
type ProtobufCodec struct{}

// NewProtobufCodec creates a new Protocol Buffers codec
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// Encode encodes data to Protocol Buffers bytes
func (pc *ProtobufCodec) Encode(data interface{}) ([]byte, error) {
	// Check if the data implements the proto.Message interface
	if protoMsg, ok := data.(interface {
		Marshal() ([]byte, error)
	}); ok {
		return protoMsg.Marshal()
	}

	// Fallback to JSON if not a proto message
	return json.Marshal(data)
}

// Decode decodes Protocol Buffers bytes to data
func (pc *ProtobufCodec) Decode(data []byte, dest interface{}) error {
	// Check if the destination implements the proto.Message interface
	if protoMsg, ok := dest.(interface {
		Unmarshal([]byte) error
	}); ok {
		return protoMsg.Unmarshal(data)
	}

	// Fallback to JSON if not a proto message
	return json.Unmarshal(data, dest)
}

// ContentType returns the Protocol Buffers content type
func (pc *ProtobufCodec) ContentType() string {
	return "application/x-protobuf"
}

// Name returns the codec name
func (pc *ProtobufCodec) Name() string {
	return "protobuf"
}

// AvroCodec provides Avro serialization
type AvroCodec struct {
	schema string
}

// NewAvroCodec creates a new Avro codec
func NewAvroCodec(schema string) *AvroCodec {
	return &AvroCodec{
		schema: schema,
	}
}

// Encode encodes data to Avro bytes
func (ac *AvroCodec) Encode(data interface{}) ([]byte, error) {
	// For now, fallback to JSON since we don't have a full Avro implementation
	// In a real implementation, you would use a library like github.com/linkedin/goavro
	return json.Marshal(data)
}

// Decode decodes Avro bytes to data
func (ac *AvroCodec) Decode(data []byte, dest interface{}) error {
	// For now, fallback to JSON since we don't have a full Avro implementation
	// In a real implementation, you would use a library like github.com/linkedin/goavro
	return json.Unmarshal(data, dest)
}

// ContentType returns the Avro content type
func (ac *AvroCodec) ContentType() string {
	return "application/avro"
}

// Name returns the codec name
func (ac *AvroCodec) Name() string {
	return "avro"
}

// CodecRegistry manages available codecs
type CodecRegistry struct {
	codecs map[string]Codec
	mu     sync.RWMutex
}

// NewCodecRegistry creates a new codec registry
func NewCodecRegistry() *CodecRegistry {
	registry := &CodecRegistry{
		codecs: make(map[string]Codec),
	}

	// Register default codecs
	registry.Register(NewJSONCodec())
	registry.Register(NewProtobufCodec())

	return registry
}

// Register registers a codec
func (cr *CodecRegistry) Register(codec Codec) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.codecs[codec.Name()] = codec
}

// Get retrieves a codec by name
func (cr *CodecRegistry) Get(name string) (Codec, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	codec, exists := cr.codecs[name]
	return codec, exists
}

// GetByContentType retrieves a codec by content type
func (cr *CodecRegistry) GetByContentType(contentType string) (Codec, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	for _, codec := range cr.codecs {
		if codec.ContentType() == contentType {
			return codec, true
		}
	}
	return nil, false
}

// List returns all registered codec names
func (cr *CodecRegistry) List() []string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	names := make([]string, 0, len(cr.codecs))
	for name := range cr.codecs {
		names = append(names, name)
	}
	return names
}

// MessageCodec provides message-specific serialization
type MessageCodec struct {
	registry     *CodecRegistry
	defaultCodec Codec
}

// NewMessageCodec creates a new message codec
func NewMessageCodec(registry *CodecRegistry) *MessageCodec {
	defaultCodec, _ := registry.Get("json")
	return &MessageCodec{
		registry:     registry,
		defaultCodec: defaultCodec,
	}
}

// SetDefaultCodec sets the default codec
func (mc *MessageCodec) SetDefaultCodec(codec Codec) {
	mc.defaultCodec = codec
}

// EncodeMessage encodes a message using the appropriate codec
func (mc *MessageCodec) EncodeMessage(msg *Message) ([]byte, error) {
	// Determine which codec to use
	var codec Codec

	if msg.ContentType != "" {
		if contentTypeCodec, exists := mc.registry.GetByContentType(msg.ContentType); exists {
			codec = contentTypeCodec
		}
	}

	if codec == nil {
		codec = mc.defaultCodec
	}

	if codec == nil {
		return nil, NewError(ErrorCodeSerialization, "encode_message", "no suitable codec found")
	}

	// Create a data structure for encoding
	data := map[string]interface{}{
		"id":             msg.ID,
		"key":            msg.Key,
		"body":           msg.Body,
		"headers":        msg.Headers,
		"contentType":    msg.ContentType,
		"timestamp":      msg.Timestamp,
		"priority":       msg.Priority,
		"idempotencyKey": msg.IdempotencyKey,
		"correlationID":  msg.CorrelationID,
		"replyTo":        msg.ReplyTo,
		"expiration":     msg.Expiration,
	}

	return codec.Encode(data)
}

// DecodeMessage decodes bytes to a message using the appropriate codec
func (mc *MessageCodec) DecodeMessage(data []byte, contentType string) (*Message, error) {
	// Determine which codec to use
	var codec Codec

	if contentType != "" {
		if contentTypeCodec, exists := mc.registry.GetByContentType(contentType); exists {
			codec = contentTypeCodec
		}
	}

	if codec == nil {
		codec = mc.defaultCodec
	}

	if codec == nil {
		return nil, NewError(ErrorCodeSerialization, "decode_message", "no suitable codec found")
	}

	// Decode to a map first
	var decoded map[string]interface{}
	if err := codec.Decode(data, &decoded); err != nil {
		return nil, WrapError(ErrorCodeSerialization, "decode_message", "failed to decode message", err)
	}

	// Convert to Message struct
	msg := &Message{}

	if id, ok := decoded["id"].(string); ok {
		msg.ID = id
	}
	if key, ok := decoded["key"].(string); ok {
		msg.Key = key
	}
	if body, ok := decoded["body"].(string); ok {
		// Body is encoded as base64 string in JSON
		if decoded, err := base64.StdEncoding.DecodeString(body); err == nil {
			msg.Body = decoded
		} else {
			// If base64 decoding fails, treat as plain string
			msg.Body = []byte(body)
		}
	} else if body, ok := decoded["body"].([]byte); ok {
		msg.Body = body
	}
	if headers, ok := decoded["headers"].(map[string]interface{}); ok {
		msg.Headers = make(map[string]string)
		for k, v := range headers {
			if str, ok := v.(string); ok {
				msg.Headers[k] = str
			}
		}
	}
	if ct, ok := decoded["contentType"].(string); ok {
		msg.ContentType = ct
	}
	if ts, ok := decoded["timestamp"].(string); ok {
		if timestamp, err := time.Parse(time.RFC3339, ts); err == nil {
			msg.Timestamp = timestamp
		}
	}
	if priority, ok := decoded["priority"].(float64); ok {
		msg.Priority = uint8(priority)
	}
	if idempotencyKey, ok := decoded["idempotencyKey"].(string); ok {
		msg.IdempotencyKey = idempotencyKey
	}
	if correlationID, ok := decoded["correlationID"].(string); ok {
		msg.CorrelationID = correlationID
	}
	if replyTo, ok := decoded["replyTo"].(string); ok {
		msg.ReplyTo = replyTo
	}
	if expiration, ok := decoded["expiration"].(string); ok {
		if duration, err := time.ParseDuration(expiration); err == nil {
			msg.Expiration = duration
		}
	}

	return msg, nil
}

// EncodeData encodes arbitrary data using the specified codec
func (mc *MessageCodec) EncodeData(data interface{}, codecName string) ([]byte, error) {
	codec, exists := mc.registry.Get(codecName)
	if !exists {
		return nil, NewError(ErrorCodeSerialization, "encode_data", "codec not found: "+codecName)
	}

	return codec.Encode(data)
}

// DecodeData decodes bytes to arbitrary data using the specified codec
func (mc *MessageCodec) DecodeData(data []byte, dest interface{}, codecName string) error {
	codec, exists := mc.registry.Get(codecName)
	if !exists {
		return NewError(ErrorCodeSerialization, "decode_data", "codec not found: "+codecName)
	}

	return codec.Decode(data, dest)
}

// Global codec registry instance
var globalCodecRegistry = NewCodecRegistry()

// GetGlobalCodecRegistry returns the global codec registry
func GetGlobalCodecRegistry() *CodecRegistry {
	return globalCodecRegistry
}

// RegisterCodec registers a codec in the global registry
func RegisterCodec(codec Codec) {
	globalCodecRegistry.Register(codec)
}

// GetCodec retrieves a codec from the global registry
func GetCodec(name string) (Codec, bool) {
	return globalCodecRegistry.Get(name)
}

// GetCodecByContentType retrieves a codec by content type from the global registry
func GetCodecByContentType(contentType string) (Codec, bool) {
	return globalCodecRegistry.GetByContentType(contentType)
}
