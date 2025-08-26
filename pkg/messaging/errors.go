// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"errors"
	"fmt"
	"sync"
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

	// ErrInvalidErrorCode is returned when an invalid error code is provided.
	ErrInvalidErrorCode = errors.New("invalid error code")
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

// ValidErrorCodes contains all valid error codes for validation.
var ValidErrorCodes = map[ErrorCode]bool{
	ErrorCodeTransport:      true,
	ErrorCodeConfiguration:  true,
	ErrorCodeConnection:     true,
	ErrorCodeChannel:        true,
	ErrorCodePublish:        true,
	ErrorCodeConsume:        true,
	ErrorCodeSerialization:  true,
	ErrorCodeValidation:     true,
	ErrorCodeTimeout:        true,
	ErrorCodeBackpressure:   true,
	ErrorCodePersistence:    true,
	ErrorCodeDLQ:            true,
	ErrorCodeTransformation: true,
	ErrorCodeRouting:        true,
	ErrorCodeInternal:       true,
}

// IsValidErrorCode checks if the given error code is valid.
func IsValidErrorCode(code ErrorCode) bool {
	return ValidErrorCodes[code]
}

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
	// Using sync.Map for thread-safe access.
	Context *sync.Map

	// Retryable indicates whether the operation can be retried.
	Retryable bool

	// Temporary indicates whether the error is temporary.
	Temporary bool
}

// Error implements the error interface.
func (e *MessagingError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *MessagingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is checks if the error matches the target error.
func (e *MessagingError) Is(target error) bool {
	if e == nil {
		return false
	}
	if e.Cause != nil {
		return errors.Is(e.Cause, target)
	}
	return false
}

// WithContext adds context information to the error.
func (e *MessagingError) WithContext(key string, value interface{}) *MessagingError {
	if e == nil {
		return nil
	}
	if e.Context == nil {
		e.Context = &sync.Map{}
	}
	e.Context.Store(key, value)
	return e
}

// WithContextSafe creates a new error with additional context information.
// This method is thread-safe as it doesn't modify the original error.
func (e *MessagingError) WithContextSafe(key string, value interface{}) *MessagingError {
	if e == nil {
		return nil
	}

	// Create a copy of the error
	newErr := &MessagingError{
		Code:      e.Code,
		Operation: e.Operation,
		Message:   e.Message,
		Cause:     e.Cause,
		Retryable: e.Retryable,
		Temporary: e.Temporary,
		Context:   &sync.Map{},
	}

	// Copy existing context
	if e.Context != nil {
		e.Context.Range(func(k, v interface{}) bool {
			newErr.Context.Store(k, v)
			return true
		})
	}

	// Add new context
	newErr.Context.Store(key, value)
	return newErr
}

// NewError creates a new MessagingError.
func NewError(code ErrorCode, operation, message string) *MessagingError {
	if !IsValidErrorCode(code) {
		code = ErrorCodeInternal
	}
	if operation == "" {
		operation = "unknown"
	}
	if message == "" {
		message = "no error message provided"
	}
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Context:   &sync.Map{},
	}
}

// NewErrorf creates a new MessagingError with formatted message.
func NewErrorf(code ErrorCode, operation, format string, args ...interface{}) *MessagingError {
	if format == "" {
		return NewError(code, operation, "no error message provided")
	}
	return NewError(code, operation, fmt.Sprintf(format, args...))
}

// WrapError wraps an existing error with MessagingError.
func WrapError(code ErrorCode, operation, message string, cause error) *MessagingError {
	if !IsValidErrorCode(code) {
		code = ErrorCodeInternal
	}
	if operation == "" {
		operation = "unknown"
	}
	if message == "" {
		message = "no error message provided"
	}
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Cause:     cause,
		Context:   &sync.Map{},
	}
}

// WrapErrorf wraps an existing error with MessagingError and formatted message.
func WrapErrorf(code ErrorCode, operation string, cause error, format string, args ...interface{}) *MessagingError {
	if format == "" {
		return WrapError(code, operation, "no error message provided", cause)
	}
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
	if errors.As(err, &msgErr) && msgErr != nil {
		if msgErr.Context == nil {
			return make(map[string]interface{})
		}
		contextMap := make(map[string]interface{})
		msgErr.Context.Range(func(k, v interface{}) bool {
			if key, ok := k.(string); ok {
				contextMap[key] = v
			}
			return true
		})
		return contextMap
	}
	return make(map[string]interface{})
}

// GetContextValue safely retrieves a specific context value from an error.
func GetContextValue(err error, key string) (interface{}, bool) {
	var msgErr *MessagingError
	if errors.As(err, &msgErr) && msgErr != nil && msgErr.Context != nil {
		return msgErr.Context.Load(key)
	}
	return nil, false
}

// GetContextString safely retrieves a string context value from an error.
func GetContextString(err error, key string) (string, bool) {
	if value, ok := GetContextValue(err, key); ok {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetContextInt safely retrieves an int context value from an error.
func GetContextInt(err error, key string) (int, bool) {
	if value, ok := GetContextValue(err, key); ok {
		if i, ok := value.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// GetContextBool safely retrieves a bool context value from an error.
func GetContextBool(err error, key string) (bool, bool) {
	if value, ok := GetContextValue(err, key); ok {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// GetRootCause returns the deepest underlying error in the error chain.
func GetRootCause(err error) error {
	if err == nil {
		return nil
	}

	var root error = err
	for {
		if unwrapped := errors.Unwrap(root); unwrapped != nil {
			root = unwrapped
		} else {
			break
		}
	}
	return root
}

// GetErrorChain returns all errors in the chain as a slice.
func GetErrorChain(err error) []error {
	if err == nil {
		return nil
	}

	var chain []error
	current := err
	for current != nil {
		chain = append(chain, current)
		current = errors.Unwrap(current)
	}
	return chain
}

// FindErrorByCode searches the error chain for an error with the specified code.
func FindErrorByCode(err error, code ErrorCode) *MessagingError {
	if err == nil {
		return nil
	}

	var msgErr *MessagingError
	if errors.As(err, &msgErr) && msgErr != nil && msgErr.Code == code {
		return msgErr
	}

	// Check the cause
	if cause := errors.Unwrap(err); cause != nil {
		return FindErrorByCode(cause, code)
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

// NewImmutableError creates a new MessagingError that cannot be modified after creation.
// This is useful for creating error constants or when you want to ensure thread safety.
func NewImmutableError(code ErrorCode, operation, message string) *MessagingError {
	if !IsValidErrorCode(code) {
		code = ErrorCodeInternal
	}
	if operation == "" {
		operation = "unknown"
	}
	if message == "" {
		message = "no error message provided"
	}
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Context:   &sync.Map{},
		Retryable: false,
		Temporary: false,
	}
}

// WrapImmutableError creates an immutable error that wraps an existing error.
func WrapImmutableError(code ErrorCode, operation, message string, cause error) *MessagingError {
	if !IsValidErrorCode(code) {
		code = ErrorCodeInternal
	}
	if operation == "" {
		operation = "unknown"
	}
	if message == "" {
		message = "no error message provided"
	}
	return &MessagingError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Cause:     cause,
		Context:   &sync.Map{},
		Retryable: false,
		Temporary: false,
	}
}

// ValidateErrorCode validates an error code and returns an error if invalid.
func ValidateErrorCode(code ErrorCode) error {
	if !IsValidErrorCode(code) {
		return fmt.Errorf("%w: %s", ErrInvalidErrorCode, code)
	}
	return nil
}

// SetRetryable marks an error as retryable.
func (e *MessagingError) SetRetryable(retryable bool) *MessagingError {
	if e == nil {
		return nil
	}
	e.Retryable = retryable
	return e
}

// SetTemporary marks an error as temporary.
func (e *MessagingError) SetTemporary(temporary bool) *MessagingError {
	if e == nil {
		return nil
	}
	e.Temporary = temporary
	return e
}

// HasErrorCode checks if the error chain contains an error with the specified code.
func HasErrorCode(err error, code ErrorCode) bool {
	return FindErrorByCode(err, code) != nil
}

// HasErrorType checks if the error chain contains an error of the specified type.
func HasErrorType(err error, target error) bool {
	return errors.Is(err, target)
}

// CountErrorsInChain returns the number of errors in the error chain.
func CountErrorsInChain(err error) int {
	if err == nil {
		return 0
	}

	count := 0
	current := err
	for current != nil {
		count++
		current = errors.Unwrap(current)
	}
	return count
}

// GetFirstMessagingError returns the first MessagingError in the error chain.
func GetFirstMessagingError(err error) *MessagingError {
	if err == nil {
		return nil
	}

	var msgErr *MessagingError
	if errors.As(err, &msgErr) {
		return msgErr
	}

	// Check the cause
	if cause := errors.Unwrap(err); cause != nil {
		return GetFirstMessagingError(cause)
	}

	return nil
}

// GetLastMessagingError returns the last MessagingError in the error chain.
func GetLastMessagingError(err error) *MessagingError {
	if err == nil {
		return nil
	}

	var lastMsgErr *MessagingError
	current := err

	for current != nil {
		var msgErr *MessagingError
		if errors.As(current, &msgErr) {
			lastMsgErr = msgErr
		}
		current = errors.Unwrap(current)
	}

	return lastMsgErr
}

// IsRetryableError checks if any error in the chain is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var msgErr *MessagingError
	if errors.As(err, &msgErr) && msgErr != nil && msgErr.Retryable {
		return true
	}

	// Check the cause
	if cause := errors.Unwrap(err); cause != nil {
		return IsRetryableError(cause)
	}

	return false
}

// IsTemporaryError checks if any error in the chain is temporary.
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	var msgErr *MessagingError
	if errors.As(err, &msgErr) && msgErr != nil && msgErr.Temporary {
		return true
	}

	// Check the cause
	if cause := errors.Unwrap(err); cause != nil {
		return IsTemporaryError(cause)
	}

	return false
}
