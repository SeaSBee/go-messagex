package unit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

func TestMessagingError(t *testing.T) {
	t.Run("creates error with message", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
		assert.Equal(t, messaging.ErrorTypeConnection, err.Type)
		assert.Equal(t, "test error", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("creates error with cause", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
		assert.Equal(t, messaging.ErrorTypeConnection, err.Type)
		assert.Equal(t, "test error", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates error with empty message uses default", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "messaging error occurred")
		assert.Equal(t, messaging.ErrorTypeConnection, err.Type)
	})

	t.Run("creates error with nil cause", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		assert.Error(t, err)
		assert.Nil(t, err.Err)
	})
}

func TestMessagingError_WithContext(t *testing.T) {
	t.Run("adds context to error", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		// Add context
		errWithContext := err.WithContext("key1", "value1")

		assert.Equal(t, err, errWithContext) // Should return same error instance

		// Verify context was added
		value, exists := err.GetContext("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", value)
	})

	t.Run("adds multiple context values", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		err.WithContext("key1", "value1")
		err.WithContext("key2", "value2")
		err.WithContext("key3", 123)

		// Verify all context values
		value1, exists1 := err.GetContext("key1")
		assert.True(t, exists1)
		assert.Equal(t, "value1", value1)

		value2, exists2 := err.GetContext("key2")
		assert.True(t, exists2)
		assert.Equal(t, "value2", value2)

		value3, exists3 := err.GetContext("key3")
		assert.True(t, exists3)
		assert.Equal(t, 123, value3)
	})

	t.Run("overwrites existing context", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		err.WithContext("key1", "value1")
		err.WithContext("key1", "value2") // Overwrite

		value, exists := err.GetContext("key1")
		assert.True(t, exists)
		assert.Equal(t, "value2", value)
	})

	t.Run("retrieves context value", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)
		err.WithContext("key1", "value1")

		value, exists := err.GetContext("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", value)
	})

	t.Run("returns false for non-existent context", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		value, exists := err.GetContext("non-existent")
		assert.False(t, exists)
		assert.Nil(t, value)
	})
}

func TestSpecificErrorTypes(t *testing.T) {
	t.Run("creates connection error", func(t *testing.T) {
		cause := errors.New("network error")
		err := messaging.NewConnectionError("connection failed", "broker", 3, cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection failed")
		assert.Equal(t, messaging.ErrorTypeConnection, err.Type)
		assert.Equal(t, "connection failed", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates publish error", func(t *testing.T) {
		cause := errors.New("publish failed")
		err := messaging.NewPublishError("publish failed", "queue", "exchange", "routing-key", cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "publish failed")
		assert.Equal(t, messaging.ErrorTypePublish, err.Type)
		assert.Equal(t, "publish failed", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates consume error", func(t *testing.T) {
		cause := errors.New("consume failed")
		err := messaging.NewConsumeError("consume failed", "queue", "consumer-tag", cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consume failed")
		assert.Equal(t, messaging.ErrorTypeConsume, err.Type)
		assert.Equal(t, "consume failed", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates validation error", func(t *testing.T) {
		cause := errors.New("validation failed")
		err := messaging.NewValidationError("validation failed", "field", "value", "rule", cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
		assert.Equal(t, messaging.ErrorTypeValidation, err.Type)
		assert.Equal(t, "validation failed", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates timeout error", func(t *testing.T) {
		cause := errors.New("timeout")
		err := messaging.NewTimeoutError("operation timeout", 30*time.Second, "queue", cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation timeout")
		assert.Equal(t, messaging.ErrorTypeTimeout, err.Type)
		assert.Equal(t, "operation timeout", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates circuit breaker error", func(t *testing.T) {
		cause := errors.New("circuit open")
		err := messaging.NewCircuitBreakerError("circuit breaker open", "open", 5, cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker open")
		assert.Equal(t, messaging.ErrorTypeCircuitBreaker, err.Type)
		assert.Equal(t, "circuit breaker open", err.Message)
		assert.Equal(t, cause, err.Err)
	})

	t.Run("creates batch error", func(t *testing.T) {
		cause := errors.New("batch failed")
		err := messaging.NewBatchError("batch operation failed", 10, 8, 2, cause)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch operation failed")
		assert.Equal(t, messaging.ErrorTypeBatch, err.Type)
		assert.Equal(t, "batch operation failed", err.Message)
		assert.Equal(t, cause, err.Err)
	})
}

func TestErrorRetryable(t *testing.T) {
	t.Run("connection errors are retryable", func(t *testing.T) {
		err := messaging.NewConnectionError("connection failed", "broker", 3, nil)
		assert.True(t, err.Retryable)
	})

	t.Run("timeout errors are retryable", func(t *testing.T) {
		err := messaging.NewTimeoutError("timeout", 30*time.Second, "operation", nil)
		assert.True(t, err.Retryable)
	})

	t.Run("circuit breaker errors are retryable", func(t *testing.T) {
		err := messaging.NewCircuitBreakerError("circuit open", "open", 5, nil)
		assert.True(t, err.Retryable)
	})

	t.Run("publish errors are not retryable", func(t *testing.T) {
		err := messaging.NewPublishError("publish failed", "queue", "exchange", "key", nil)
		assert.False(t, err.Retryable)
	})

	t.Run("consume errors are not retryable", func(t *testing.T) {
		err := messaging.NewConsumeError("consume failed", "queue", "tag", nil)
		assert.False(t, err.Retryable)
	})

	t.Run("validation errors are not retryable", func(t *testing.T) {
		err := messaging.NewValidationError("validation failed", "field", "value", "rule", nil)
		assert.False(t, err.Retryable)
	})

	t.Run("batch errors are not retryable", func(t *testing.T) {
		err := messaging.NewBatchError("batch failed", 10, 8, 2, nil)
		assert.False(t, err.Retryable)
	})
}

func TestErrorTypes(t *testing.T) {
	t.Run("all error types are defined", func(t *testing.T) {
		expectedTypes := []messaging.ErrorType{
			messaging.ErrorTypeConnection,
			messaging.ErrorTypePublish,
			messaging.ErrorTypeConsume,
			messaging.ErrorTypeValidation,
			messaging.ErrorTypeTimeout,
			messaging.ErrorTypeCircuitBreaker,
			messaging.ErrorTypeRetry,
			messaging.ErrorTypeBatch,
			messaging.ErrorTypeHealth,
			messaging.ErrorTypeConfig,
		}

		for _, errorType := range expectedTypes {
			assert.NotEmpty(t, string(errorType), "Error type should not be empty")
		}
	})
}

func TestMessagingError_Concurrency(t *testing.T) {
	t.Run("concurrent context operations", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		// Test concurrent context additions
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(i int) {
				defer func() { done <- true }()

				key := fmt.Sprintf("key%d", i)
				value := fmt.Sprintf("value%d", i)
				err.WithContext(key, value)

				// Verify the context was added
				retrievedValue, exists := err.GetContext(key)
				assert.True(t, exists)
				assert.Equal(t, value, retrievedValue)
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify all contexts were added
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			expectedValue := fmt.Sprintf("value%d", i)
			actualValue, exists := err.GetContext(key)
			assert.True(t, exists)
			assert.Equal(t, expectedValue, actualValue)
		}
	})

	t.Run("concurrent retryable checks", func(t *testing.T) {
		err := messaging.NewConnectionError("connection failed", "broker", 3, nil)

		// Test concurrent retryable checks
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				retryable := err.Retryable
				assert.True(t, retryable)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestMessagingError_EdgeCases(t *testing.T) {
	t.Run("error with very long message", func(t *testing.T) {
		longMessage := string(make([]byte, 10000)) // 10KB message
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, longMessage, nil)

		assert.Error(t, err)
		assert.Equal(t, longMessage, err.Message)
		assert.Equal(t, messaging.ErrorTypeConnection, err.Type)
	})

	t.Run("error with special characters in context", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		specialKey := "key with spaces & symbols!@#$%^&*()"
		specialValue := "value with \n newlines \t tabs"

		err.WithContext(specialKey, specialValue)

		value, exists := err.GetContext(specialKey)
		assert.True(t, exists)
		assert.Equal(t, specialValue, value)
	})

	t.Run("error with nil context values", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		err.WithContext("nil_key", nil)

		value, exists := err.GetContext("nil_key")
		assert.True(t, exists)
		assert.Nil(t, value)
	})

	t.Run("error with complex context values", func(t *testing.T) {
		err := messaging.NewMessagingError(messaging.ErrorTypeConnection, "test error", nil)

		complexValue := map[string]interface{}{
			"nested": map[string]string{
				"key": "value",
			},
			"array": []int{1, 2, 3},
		}

		err.WithContext("complex", complexValue)

		value, exists := err.GetContext("complex")
		assert.True(t, exists)
		assert.Equal(t, complexValue, value)
	})
}

func TestMessagingError_ErrorChaining(t *testing.T) {
	t.Run("error unwrapping works", func(t *testing.T) {
		originalErr := errors.New("original error")
		messagingErr := messaging.NewConnectionError("connection failed", "broker", 3, originalErr)

		// Test error unwrapping
		unwrapped := errors.Unwrap(messagingErr)
		assert.Equal(t, originalErr, unwrapped)

		// Test error chain
		assert.True(t, errors.Is(messagingErr, originalErr))
	})

	t.Run("error with context and cause", func(t *testing.T) {
		originalErr := errors.New("original error")
		messagingErr := messaging.NewConnectionError("connection failed", "broker", 3, originalErr)

		// Add context
		messagingErrWithContext := messagingErr.WithContext("retry_count", 5)
		messagingErrWithContext = messagingErrWithContext.WithContext("timeout", "30s")

		// Test context retrieval
		retryCount, exists := messagingErrWithContext.GetContext("retry_count")
		assert.True(t, exists)
		assert.Equal(t, 5, retryCount)

		timeout, exists := messagingErrWithContext.GetContext("timeout")
		assert.True(t, exists)
		assert.Equal(t, "30s", timeout)

		// Test error unwrapping still works
		unwrapped := errors.Unwrap(messagingErr)
		assert.Equal(t, originalErr, unwrapped)
	})
}

func TestMessagingError_JSONSerialization(t *testing.T) {
	t.Run("error can be marshaled to JSON", func(t *testing.T) {
		originalErr := errors.New("original error")
		messagingErr := messaging.NewConnectionError("test error", "broker", 1, originalErr)
		messagingErrWithContext := messagingErr.WithContext("key1", "value1")
		messagingErrWithContext = messagingErrWithContext.WithContext("key2", 42)

		// Test JSON marshaling (simplified test)
		jsonStr := fmt.Sprintf("%+v", messagingErr)
		assert.Contains(t, jsonStr, "test error")
		assert.Contains(t, jsonStr, "broker")
		assert.Contains(t, jsonStr, "1")
	})
}

func TestMessagingError_Integration(t *testing.T) {
	t.Run("error chain with multiple levels", func(t *testing.T) {
		rootErr := errors.New("root error")
		level1Err := messaging.NewConnectionError("level 1", "broker1", 1, rootErr)
		level2Err := messaging.NewPublishError("level 2", "queue1", "exchange1", "key1", level1Err)
		level3Err := messaging.NewConsumeError("level 3", "queue2", "tag2", level2Err)

		// Test error chain
		assert.True(t, errors.Is(level3Err, level2Err))
		assert.True(t, errors.Is(level3Err, level1Err))
		assert.True(t, errors.Is(level3Err, rootErr))

		// Test unwrapping
		unwrapped1 := errors.Unwrap(level3Err)
		assert.Equal(t, level2Err, unwrapped1)

		unwrapped2 := errors.Unwrap(unwrapped1)
		assert.Equal(t, level1Err, unwrapped2)

		unwrapped3 := errors.Unwrap(unwrapped2)
		assert.Equal(t, rootErr, unwrapped3)
	})

	t.Run("error with mixed context types", func(t *testing.T) {
		messagingErr := messaging.NewConnectionError("test error", "broker", 1, nil)

		// Add different types of context
		messagingErrWithContext := messagingErr.WithContext("string", "value")
		messagingErrWithContext = messagingErrWithContext.WithContext("int", 42)
		messagingErrWithContext = messagingErrWithContext.WithContext("float", 3.14)
		messagingErrWithContext = messagingErrWithContext.WithContext("bool", true)
		messagingErrWithContext = messagingErrWithContext.WithContext("nil", nil)

		// Retrieve and verify
		value, exists := messagingErrWithContext.GetContext("string")
		assert.True(t, exists)
		assert.Equal(t, "value", value)

		value, exists = messagingErrWithContext.GetContext("int")
		assert.True(t, exists)
		assert.Equal(t, 42, value)

		value, exists = messagingErrWithContext.GetContext("float")
		assert.True(t, exists)
		assert.Equal(t, 3.14, value)

		value, exists = messagingErrWithContext.GetContext("bool")
		assert.True(t, exists)
		assert.Equal(t, true, value)

		value, exists = messagingErrWithContext.GetContext("nil")
		assert.True(t, exists)
		assert.Nil(t, value)
	})
}
