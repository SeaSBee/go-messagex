// Package messaging provides custom error types for RabbitMQ messaging
package messaging

import (
	"fmt"
	"sync"
	"time"
)

// ErrorType represents the type of messaging error
type ErrorType string

const (
	ErrorTypeConnection     ErrorType = "connection"
	ErrorTypePublish        ErrorType = "publish"
	ErrorTypeConsume        ErrorType = "consume"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeCircuitBreaker ErrorType = "circuit_breaker"
	ErrorTypeRetry          ErrorType = "retry"
	ErrorTypeBatch          ErrorType = "batch"
	ErrorTypeHealth         ErrorType = "health"
	ErrorTypeConfig         ErrorType = "config"
)

// MessagingError represents a messaging-specific error
type MessagingError struct {
	Type      ErrorType
	Message   string
	Err       error
	Timestamp time.Time
	Retryable bool
	Context   map[string]interface{}
	mu        sync.RWMutex // Protects Context map
}

// Error implements the error interface
func (e *MessagingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *MessagingError) Unwrap() error {
	return e.Err
}

// NewMessagingError creates a new messaging error
func NewMessagingError(errorType ErrorType, message string, err error) *MessagingError {
	if message == "" {
		message = "messaging error occurred"
	}

	return &MessagingError{
		Type:      errorType,
		Message:   message,
		Err:       err,
		Timestamp: time.Now(),
		Retryable: isRetryableError(errorType),
		Context:   make(map[string]interface{}),
	}
}

// NewMessagingErrorf creates a new messaging error with formatted message
func NewMessagingErrorf(errorType ErrorType, format string, args ...interface{}) *MessagingError {
	return NewMessagingError(errorType, fmt.Sprintf(format, args...), nil)
}

// WithContext adds context to the error
func (e *MessagingError) WithContext(key string, value interface{}) *MessagingError {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// IsRetryable returns true if the error is retryable
func (e *MessagingError) IsRetryable() bool {
	if e == nil {
		return false
	}
	return e.Retryable
}

// GetContext safely retrieves a context value
func (e *MessagingError) GetContext(key string) (interface{}, bool) {
	if e == nil || e.Context == nil {
		return nil, false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	value, exists := e.Context[key]
	return value, exists
}

// createErrorWithDefaults creates an error with default message if empty
func createErrorWithDefaults(errorType ErrorType, message string, err error, defaultMsg string) *MessagingError {
	if message == "" {
		message = defaultMsg
	}
	return NewMessagingError(errorType, message, err)
}

// isRetryableError determines if an error type is retryable
func isRetryableError(errorType ErrorType) bool {
	switch errorType {
	case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeCircuitBreaker:
		return true
	case ErrorTypePublish, ErrorTypeConsume, ErrorTypeBatch:
		return false // Business logic errors are not retryable
	case ErrorTypeRetry:
		return true // Explicit retry errors are retryable
	case ErrorTypeValidation, ErrorTypeConfig:
		return false // Validation and config errors are not retryable
	case ErrorTypeHealth:
		return false // Health check failures are not retryable
	default:
		return false // Default to non-retryable for safety
	}
}

// ConnectionError represents a connection-related error
type ConnectionError struct {
	*MessagingError
	URL     string
	Attempt int
}

// Error returns the error message including connection details
func (e *ConnectionError) Error() string {
	if e == nil {
		return "connection error is nil"
	}
	baseMsg := e.MessagingError.Error()
	return fmt.Sprintf("%s (URL: %s, Attempt: %d)", baseMsg, e.URL, e.Attempt)
}

// NewConnectionError creates a new connection error
func NewConnectionError(message string, url string, attempt int, err error) *ConnectionError {
	return &ConnectionError{
		MessagingError: createErrorWithDefaults(ErrorTypeConnection, message, err, "connection error occurred"),
		URL:            url,
		Attempt:        attempt,
	}
}

// PublishError represents a message publishing error
type PublishError struct {
	*MessagingError
	Queue      string
	Exchange   string
	RoutingKey string
}

// NewPublishError creates a new publish error
func NewPublishError(message string, queue, exchange, routingKey string, err error) *PublishError {
	return &PublishError{
		MessagingError: createErrorWithDefaults(ErrorTypePublish, message, err, "publish error occurred"),
		Queue:          queue,
		Exchange:       exchange,
		RoutingKey:     routingKey,
	}
}

// ConsumeError represents a message consumption error
type ConsumeError struct {
	*MessagingError
	Queue string
	Tag   string
}

// NewConsumeError creates a new consume error
func NewConsumeError(message string, queue, tag string, err error) *ConsumeError {
	return &ConsumeError{
		MessagingError: createErrorWithDefaults(ErrorTypeConsume, message, err, "consume error occurred"),
		Queue:          queue,
		Tag:            tag,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	*MessagingError
	Field string
	Value interface{}
	Rule  string
}

// NewValidationError creates a new validation error
func NewValidationError(message string, field string, value interface{}, rule string, err error) *ValidationError {
	return &ValidationError{
		MessagingError: createErrorWithDefaults(ErrorTypeValidation, message, err, "validation error occurred"),
		Field:          field,
		Value:          value,
		Rule:           rule,
	}
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	*MessagingError
	Duration  time.Duration
	Operation string
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(message string, duration time.Duration, operation string, err error) *TimeoutError {
	return &TimeoutError{
		MessagingError: createErrorWithDefaults(ErrorTypeTimeout, message, err, "timeout error occurred"),
		Duration:       duration,
		Operation:      operation,
	}
}

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct {
	*MessagingError
	State    string
	Failures int
}

// NewCircuitBreakerError creates a new circuit breaker error
func NewCircuitBreakerError(message string, state string, failures int, err error) *CircuitBreakerError {
	return &CircuitBreakerError{
		MessagingError: createErrorWithDefaults(ErrorTypeCircuitBreaker, message, err, "circuit breaker error occurred"),
		State:          state,
		Failures:       failures,
	}
}

// BatchError represents a batch processing error
type BatchError struct {
	*MessagingError
	BatchSize    int
	SuccessCount int
	FailureCount int
}

// NewBatchError creates a new batch error
func NewBatchError(message string, batchSize, successCount, failureCount int, err error) *BatchError {
	return &BatchError{
		MessagingError: createErrorWithDefaults(ErrorTypeBatch, message, err, "batch error occurred"),
		BatchSize:      batchSize,
		SuccessCount:   successCount,
		FailureCount:   failureCount,
	}
}
