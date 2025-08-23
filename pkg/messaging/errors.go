// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"errors"
	"fmt"
)

// Common error variables
var (
	// ErrUnsupportedTransport is returned when an unsupported transport is requested.
	ErrUnsupportedTransport = errors.New("unsupported transport")

	// ErrTransportNotRegistered is returned when a transport is not registered.
	ErrTransportNotRegistered = errors.New("transport not registered")

	// ErrPublisherClosed is returned when attempting to use a closed publisher.
	ErrPublisherClosed = errors.New("publisher is closed")

	// ErrConsumerClosed is returned when attempting to use a closed consumer.
	ErrConsumerClosed = errors.New("consumer is closed")

	// ErrConsumerAlreadyStarted is returned when starting an already started consumer.
	ErrConsumerAlreadyStarted = errors.New("consumer already started")

	// ErrBackpressure is returned when the system is under backpressure.
	ErrBackpressure = errors.New("backpressure limit exceeded")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("operation timed out")

	// ErrInvalidMessage is returned when a message is invalid.
	ErrInvalidMessage = errors.New("invalid message")

	// ErrInvalidContentType is returned when content type is invalid for operation.
	ErrInvalidContentType = errors.New("invalid content type")

	// ErrMessageTooLarge is returned when a message exceeds size limits.
	ErrMessageTooLarge = errors.New("message too large")

	// ErrInvalidConfiguration is returned when configuration is invalid.
	ErrInvalidConfiguration = errors.New("invalid configuration")

	// ErrConnectionFailed is returned when connection establishment fails.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrChannelClosed is returned when a channel is unexpectedly closed.
	ErrChannelClosed = errors.New("channel closed")

	// ErrUnroutable is returned when a message cannot be routed.
	ErrUnroutable = errors.New("message unroutable")

	// ErrNotConfirmed is returned when a message publish is not confirmed.
	ErrNotConfirmed = errors.New("message not confirmed")

	// ErrHandlerPanic is returned when a message handler panics.
	ErrHandlerPanic = errors.New("handler panic")

	// ErrInvalidAckDecision is returned when an invalid ack decision is provided.
	ErrInvalidAckDecision = errors.New("invalid ack decision")
)

// ErrorCode represents an error code for categorizing errors.
type ErrorCode string

const (
	// ErrorCodeTransport indicates a transport-related error.
	ErrorCodeTransport ErrorCode = "TRANSPORT"

	// ErrorCodeConfiguration indicates a configuration error.
	ErrorCodeConfiguration ErrorCode = "CONFIGURATION"

	// ErrorCodeConnection indicates a connection error.
	ErrorCodeConnection ErrorCode = "CONNECTION"

	// ErrorCodeChannel indicates a channel error.
	ErrorCodeChannel ErrorCode = "CHANNEL"

	// ErrorCodePublish indicates a publish error.
	ErrorCodePublish ErrorCode = "PUBLISH"

	// ErrorCodeConsume indicates a consume error.
	ErrorCodeConsume ErrorCode = "CONSUME"

	// ErrorCodeSerialization indicates a serialization error.
	ErrorCodeSerialization ErrorCode = "SERIALIZATION"

	// ErrorCodeValidation indicates a validation error.
	ErrorCodeValidation ErrorCode = "VALIDATION"

	// ErrorCodeTimeout indicates a timeout error.
	ErrorCodeTimeout ErrorCode = "TIMEOUT"

	// ErrorCodeBackpressure indicates a backpressure error.
	ErrorCodeBackpressure ErrorCode = "BACKPRESSURE"

	// ErrorCodePersistence indicates a persistence error.
	ErrorCodePersistence ErrorCode = "PERSISTENCE"

	// ErrorCodeDLQ indicates a dead letter queue error.
	ErrorCodeDLQ ErrorCode = "DLQ"

	// ErrorCodeTransformation indicates a message transformation error.
	ErrorCodeTransformation ErrorCode = "TRANSFORMATION"

	// ErrorCodeRouting indicates a routing error.
	ErrorCodeRouting ErrorCode = "ROUTING"

	// ErrorCodeInternal indicates an internal error.
	ErrorCodeInternal ErrorCode = "INTERNAL"
)

// MessagingError represents a structured error with additional context.
type MessagingError struct {
	// Code categorizes the error.
	Code ErrorCode

	// Operation describes what operation was being performed.
	Operation string

	// Message provides a human-readable error description.
	Message string

	// Cause is the underlying error that caused this error.
	Cause error

	// Context provides additional context information.
	Context map[string]interface{}

	// Retryable indicates whether the operation can be retried.
	Retryable bool

	// Temporary indicates whether the error is temporary.
	Temporary bool
}

// Error implements the error interface.
func (e *MessagingError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *MessagingError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error.
func (e *MessagingError) Is(target error) bool {
	if e.Cause != nil {
		return errors.Is(e.Cause, target)
	}
	return false
}

// WithContext adds context information to the error.
func (e *MessagingError) WithContext(key string, value interface{}) *MessagingError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError creates a new MessagingError.
func NewError(code ErrorCode, operation, message string) *MessagingError {
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Context:   make(map[string]interface{}),
	}
}

// NewErrorf creates a new MessagingError with formatted message.
func NewErrorf(code ErrorCode, operation, format string, args ...interface{}) *MessagingError {
	return NewError(code, operation, fmt.Sprintf(format, args...))
}

// WrapError wraps an existing error with MessagingError.
func WrapError(code ErrorCode, operation, message string, cause error) *MessagingError {
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
	}
}

// WrapErrorf wraps an existing error with MessagingError and formatted message.
func WrapErrorf(code ErrorCode, operation string, cause error, format string, args ...interface{}) *MessagingError {
	return WrapError(code, operation, fmt.Sprintf(format, args...), cause)
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	var msgErr *MessagingError
	if errors.As(err, &msgErr) {
		return msgErr.Retryable
	}
	return false
}

// IsTemporary checks if an error is temporary.
func IsTemporary(err error) bool {
	var msgErr *MessagingError
	if errors.As(err, &msgErr) {
		return msgErr.Temporary
	}
	return false
}

// GetErrorCode extracts the error code from an error.
func GetErrorCode(err error) ErrorCode {
	var msgErr *MessagingError
	if errors.As(err, &msgErr) {
		return msgErr.Code
	}
	return ErrorCodeInternal
}

// GetErrorContext extracts context information from an error.
func GetErrorContext(err error) map[string]interface{} {
	var msgErr *MessagingError
	if errors.As(err, &msgErr) {
		return msgErr.Context
	}
	return nil
}

// ErrorCategory represents a category of errors.
type ErrorCategory string

const (
	// CategoryConnection includes connection-related errors.
	CategoryConnection ErrorCategory = "connection"

	// CategoryPublisher includes publisher-related errors.
	CategoryPublisher ErrorCategory = "publisher"

	// CategoryConsumer includes consumer-related errors.
	CategoryConsumer ErrorCategory = "consumer"

	// CategoryMessage includes message-related errors.
	CategoryMessage ErrorCategory = "message"

	// CategoryConfiguration includes configuration-related errors.
	CategoryConfiguration ErrorCategory = "configuration"

	// CategoryTransport includes transport-related errors.
	CategoryTransport ErrorCategory = "transport"
)

// CategorizeError returns the category for an error code.
func CategorizeError(code ErrorCode) ErrorCategory {
	switch code {
	case ErrorCodeConnection, ErrorCodeChannel:
		return CategoryConnection
	case ErrorCodePublish:
		return CategoryPublisher
	case ErrorCodeConsume:
		return CategoryConsumer
	case ErrorCodeSerialization, ErrorCodeValidation:
		return CategoryMessage
	case ErrorCodeConfiguration:
		return CategoryConfiguration
	case ErrorCodeTransport:
		return CategoryTransport
	default:
		return CategoryTransport
	}
}

// PublishError creates a publish-specific error.
func PublishError(message string, cause error) *MessagingError {
	return WrapError(ErrorCodePublish, "publish", message, cause)
}

// ConsumeError creates a consume-specific error.
func ConsumeError(message string, cause error) *MessagingError {
	return WrapError(ErrorCodeConsume, "consume", message, cause)
}

// ConnectionError creates a connection-specific error.
func ConnectionError(message string, cause error) *MessagingError {
	return WrapError(ErrorCodeConnection, "connection", message, cause)
}

// ConfigurationError creates a configuration-specific error.
func ConfigurationError(message string, cause error) *MessagingError {
	return WrapError(ErrorCodeConfiguration, "configuration", message, cause)
}

// TimeoutError creates a timeout-specific error.
func TimeoutError(operation string, cause error) *MessagingError {
	err := WrapError(ErrorCodeTimeout, operation, "operation timed out", cause)
	err.Temporary = true
	err.Retryable = true
	return err
}

// BackpressureError creates a backpressure-specific error.
func BackpressureError(operation string) *MessagingError {
	err := NewError(ErrorCodeBackpressure, operation, "backpressure limit exceeded")
	err.Temporary = true
	err.Retryable = true
	return err
}
