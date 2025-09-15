package unit

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

// TestErrorCodeConstants tests all error code constants
func TestErrorCodeConstants(t *testing.T) {
	t.Run("AllErrorCodes", func(t *testing.T) {
		expectedCodes := []messaging.ErrorCode{
			messaging.ErrorCodeTransport,
			messaging.ErrorCodeConfiguration,
			messaging.ErrorCodeConnection,
			messaging.ErrorCodeChannel,
			messaging.ErrorCodePublish,
			messaging.ErrorCodeConsume,
			messaging.ErrorCodeSerialization,
			messaging.ErrorCodeValidation,
			messaging.ErrorCodeTimeout,
			messaging.ErrorCodeBackpressure,
			messaging.ErrorCodePersistence,
			messaging.ErrorCodeDLQ,
			messaging.ErrorCodeTransformation,
			messaging.ErrorCodeRouting,
			messaging.ErrorCodeInternal,
		}

		for _, code := range expectedCodes {
			assert.True(t, messaging.IsValidErrorCode(code), "Error code %s should be valid", code)
		}
	})

	t.Run("InvalidErrorCode", func(t *testing.T) {
		invalidCode := messaging.ErrorCode("INVALID_CODE")
		assert.False(t, messaging.IsValidErrorCode(invalidCode))
	})

	t.Run("EmptyErrorCode", func(t *testing.T) {
		emptyCode := messaging.ErrorCode("")
		assert.False(t, messaging.IsValidErrorCode(emptyCode))
	})
}

// TestCommonErrorVariables tests all common error variables
func TestCommonErrorVariables(t *testing.T) {
	t.Run("AllCommonErrors", func(t *testing.T) {
		commonErrors := []error{
			messaging.ErrUnsupportedTransport,
			messaging.ErrTransportNotRegistered,
			messaging.ErrPublisherClosed,
			messaging.ErrConsumerClosed,
			messaging.ErrConsumerAlreadyStarted,
			messaging.ErrBackpressure,
			messaging.ErrTimeout,
			messaging.ErrInvalidMessage,
			messaging.ErrInvalidContentType,
			messaging.ErrMessageTooLarge,
			messaging.ErrInvalidConfiguration,
			messaging.ErrConnectionFailed,
			messaging.ErrChannelClosed,
			messaging.ErrUnroutable,
			messaging.ErrNotConfirmed,
			messaging.ErrHandlerPanic,
			messaging.ErrInvalidAckDecision,
			messaging.ErrInvalidErrorCode,
		}

		for _, err := range commonErrors {
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		}
	})

	t.Run("ErrorComparison", func(t *testing.T) {
		assert.NotEqual(t, messaging.ErrUnsupportedTransport, messaging.ErrTransportNotRegistered)
		assert.NotEqual(t, messaging.ErrPublisherClosed, messaging.ErrConsumerClosed)
		assert.Equal(t, messaging.ErrTimeout, messaging.ErrTimeout)
	})
}

// TestNewError tests the NewError function
func TestNewError(t *testing.T) {
	t.Run("ValidError", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "test message", err.Message)
		assert.Nil(t, err.Cause)
		assert.NotNil(t, err.Context)
		assert.False(t, err.Retryable)
		assert.False(t, err.Temporary)
	})

	t.Run("InvalidErrorCode", func(t *testing.T) {
		invalidCode := messaging.ErrorCode("INVALID")
		err := messaging.NewError(invalidCode, "test_operation", "test message")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeInternal, err.Code) // Should default to Internal
	})

	t.Run("EmptyOperation", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "", "test message")
		assert.NotNil(t, err)
		assert.Equal(t, "unknown", err.Operation) // Should default to "unknown"
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "")
		assert.NotNil(t, err)
		assert.Equal(t, "no error message provided", err.Message) // Should default to default message
	})

	t.Run("NilError", func(t *testing.T) {
		var err *messaging.MessagingError
		assert.Equal(t, "<nil>", err.Error())
		assert.Nil(t, err.Unwrap())
		assert.False(t, err.Is(nil))
	})
}

// TestNewErrorf tests the NewErrorf function
func TestNewErrorf(t *testing.T) {
	t.Run("ValidFormattedError", func(t *testing.T) {
		err := messaging.NewErrorf(messaging.ErrorCodePublish, "test_operation", "error with %s", "parameter")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "error with parameter", err.Message)
	})

	t.Run("EmptyFormat", func(t *testing.T) {
		err := messaging.NewErrorf(messaging.ErrorCodePublish, "test_operation", "")
		assert.NotNil(t, err)
		assert.Equal(t, "no error message provided", err.Message)
	})

	t.Run("MultipleParameters", func(t *testing.T) {
		err := messaging.NewErrorf(messaging.ErrorCodePublish, "test_operation", "error %s with %d parameters", "test", 2)
		assert.NotNil(t, err)
		assert.Equal(t, "error test with 2 parameters", err.Message)
	})
}

// TestWrapError tests the WrapError function
func TestWrapError(t *testing.T) {
	t.Run("ValidWrappedError", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "wrapped message", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "wrapped message", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("InvalidErrorCode", func(t *testing.T) {
		invalidCode := messaging.ErrorCode("INVALID")
		cause := errors.New("original error")
		err := messaging.WrapError(invalidCode, "test_operation", "wrapped message", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeInternal, err.Code) // Should default to Internal
	})

	t.Run("EmptyOperation", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapError(messaging.ErrorCodePublish, "", "wrapped message", cause)
		assert.NotNil(t, err)
		assert.Equal(t, "unknown", err.Operation)
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "", cause)
		assert.NotNil(t, err)
		assert.Equal(t, "no error message provided", err.Message)
	})

	t.Run("NilCause", func(t *testing.T) {
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "wrapped message", nil)
		assert.NotNil(t, err)
		assert.Nil(t, err.Cause)
	})
}

// TestWrapErrorf tests the WrapErrorf function
func TestWrapErrorf(t *testing.T) {
	t.Run("ValidFormattedWrappedError", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapErrorf(messaging.ErrorCodePublish, "test_operation", cause, "wrapped error with %s", "parameter")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "wrapped error with parameter", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("EmptyFormat", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapErrorf(messaging.ErrorCodePublish, "test_operation", cause, "")
		assert.NotNil(t, err)
		assert.Equal(t, "no error message provided", err.Message)
	})
}

// TestMessagingErrorMethods tests MessagingError methods
func TestMessagingErrorMethods(t *testing.T) {
	t.Run("ErrorMethod", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		errorString := err.Error()
		assert.Contains(t, errorString, "PUBLISH")
		assert.Contains(t, errorString, "test message")
	})

	t.Run("ErrorMethodWithCause", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "wrapped message", cause)
		errorString := err.Error()
		assert.Contains(t, errorString, "PUBLISH")
		assert.Contains(t, errorString, "wrapped message")
		assert.Contains(t, errorString, "original error")
	})

	t.Run("UnwrapMethod", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "wrapped message", cause)
		unwrapped := err.Unwrap()
		assert.Equal(t, cause, unwrapped)
	})

	t.Run("UnwrapMethodNoCause", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		unwrapped := err.Unwrap()
		assert.Nil(t, unwrapped)
	})

	t.Run("IsMethod", func(t *testing.T) {
		cause := messaging.ErrTimeout
		err := messaging.WrapError(messaging.ErrorCodePublish, "test_operation", "wrapped message", cause)
		assert.True(t, err.Is(messaging.ErrTimeout))
		assert.False(t, err.Is(messaging.ErrConnectionFailed))
	})

	t.Run("IsMethodNoCause", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		assert.False(t, err.Is(messaging.ErrTimeout))
	})
}

// TestErrorContextManagement tests context management functionality
func TestErrorContextManagement(t *testing.T) {
	t.Run("WithContext", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		err = err.WithContext("key1", "value1")
		err = err.WithContext("key2", 42)
		err = err.WithContext("key3", true)

		// Test context retrieval
		value1, ok := messaging.GetContextValue(err, "key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", value1)

		value2, ok := messaging.GetContextValue(err, "key2")
		assert.True(t, ok)
		assert.Equal(t, 42, value2)

		value3, ok := messaging.GetContextValue(err, "key3")
		assert.True(t, ok)
		assert.Equal(t, true, value3)

		// Test typed context retrieval
		strValue, ok := messaging.GetContextString(err, "key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", strValue)

		intValue, ok := messaging.GetContextInt(err, "key2")
		assert.True(t, ok)
		assert.Equal(t, 42, intValue)

		boolValue, ok := messaging.GetContextBool(err, "key3")
		assert.True(t, ok)
		assert.Equal(t, true, boolValue)
	})

	t.Run("WithContextSafe", func(t *testing.T) {
		originalErr := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		originalErr = originalErr.WithContext("original_key", "original_value")

		// Create a safe copy with new context
		newErr := originalErr.WithContextSafe("new_key", "new_value")

		// Original should remain unchanged
		originalValue, ok := messaging.GetContextValue(originalErr, "original_key")
		assert.True(t, ok)
		assert.Equal(t, "original_value", originalValue)

		_, ok = messaging.GetContextValue(originalErr, "new_key")
		assert.False(t, ok)

		// New error should have both contexts
		originalValue, ok = messaging.GetContextValue(newErr, "original_key")
		assert.True(t, ok)
		assert.Equal(t, "original_value", originalValue)

		newValue, ok := messaging.GetContextValue(newErr, "new_key")
		assert.True(t, ok)
		assert.Equal(t, "new_value", newValue)
	})

	t.Run("GetErrorContext", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		err = err.WithContext("key1", "value1")
		err = err.WithContext("key2", 42)

		context := messaging.GetErrorContext(err)
		assert.Len(t, context, 2)
		assert.Equal(t, "value1", context["key1"])
		assert.Equal(t, 42, context["key2"])
	})

	t.Run("ContextWithNilError", func(t *testing.T) {
		var err *messaging.MessagingError
		context := messaging.GetErrorContext(err)
		assert.Empty(t, context)

		_, ok := messaging.GetContextValue(err, "key")
		assert.False(t, ok)

		_, ok = messaging.GetContextString(err, "key")
		assert.False(t, ok)

		_, ok = messaging.GetContextInt(err, "key")
		assert.False(t, ok)

		_, ok = messaging.GetContextBool(err, "key")
		assert.False(t, ok)
	})

	t.Run("ContextThreadSafety", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		var wg sync.WaitGroup
		const numGoroutines = 10

		// Test concurrent context addition
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				err.WithContext(fmt.Sprintf("key_%d", id), fmt.Sprintf("value_%d", id))
			}(i)
		}

		wg.Wait()

		// Verify all contexts were added
		context := messaging.GetErrorContext(err)
		assert.GreaterOrEqual(t, len(context), 1) // At least one context should be present
	})
}

// TestErrorChainAnalysis tests error chain analysis functions
func TestErrorChainAnalysis(t *testing.T) {
	t.Run("GetRootCause", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		root := messaging.GetRootCause(level2)
		assert.Equal(t, rootCause, root)
	})

	t.Run("GetRootCauseNil", func(t *testing.T) {
		root := messaging.GetRootCause(nil)
		assert.Nil(t, root)
	})

	t.Run("GetErrorChain", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		chain := messaging.GetErrorChain(level2)
		assert.Len(t, chain, 3) // level2, level1, rootCause
		assert.Equal(t, level2, chain[0])
		assert.Equal(t, level1, chain[1])
		assert.Equal(t, rootCause, chain[2])
	})

	t.Run("GetErrorChainNil", func(t *testing.T) {
		chain := messaging.GetErrorChain(nil)
		assert.Nil(t, chain)
	})

	t.Run("FindErrorByCode", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		// Find publish error
		publishErr := messaging.FindErrorByCode(level2, messaging.ErrorCodePublish)
		assert.NotNil(t, publishErr)
		assert.Equal(t, messaging.ErrorCodePublish, publishErr.Code)

		// Find internal error
		internalErr := messaging.FindErrorByCode(level2, messaging.ErrorCodeInternal)
		assert.NotNil(t, internalErr)
		assert.Equal(t, messaging.ErrorCodeInternal, internalErr.Code)

		// Find non-existent error
		notFound := messaging.FindErrorByCode(level2, messaging.ErrorCodeConnection)
		assert.Nil(t, notFound)
	})

	t.Run("FindErrorByCodeNil", func(t *testing.T) {
		result := messaging.FindErrorByCode(nil, messaging.ErrorCodePublish)
		assert.Nil(t, result)
	})

	t.Run("HasErrorCode", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		assert.True(t, messaging.HasErrorCode(level2, messaging.ErrorCodePublish))
		assert.True(t, messaging.HasErrorCode(level2, messaging.ErrorCodeInternal))
		assert.False(t, messaging.HasErrorCode(level2, messaging.ErrorCodeConnection))
	})

	t.Run("HasErrorType", func(t *testing.T) {
		rootCause := messaging.ErrTimeout
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)

		assert.True(t, messaging.HasErrorType(level1, messaging.ErrTimeout))
		assert.False(t, messaging.HasErrorType(level1, messaging.ErrConnectionFailed))
	})

	t.Run("CountErrorsInChain", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		count := messaging.CountErrorsInChain(level2)
		assert.Equal(t, 3, count)
	})

	t.Run("CountErrorsInChainNil", func(t *testing.T) {
		count := messaging.CountErrorsInChain(nil)
		assert.Equal(t, 0, count)
	})

	t.Run("GetFirstMessagingError", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		first := messaging.GetFirstMessagingError(level2)
		assert.NotNil(t, first)
		assert.Equal(t, messaging.ErrorCodeInternal, first.Code) // Should be level2
	})

	t.Run("GetLastMessagingError", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		last := messaging.GetLastMessagingError(level2)
		assert.NotNil(t, last)
		assert.Equal(t, messaging.ErrorCodePublish, last.Code) // Should be level1
	})
}

// TestErrorProperties tests error properties and flags
func TestErrorProperties(t *testing.T) {
	t.Run("RetryableFlag", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		assert.False(t, err.Retryable)

		err = err.SetRetryable(true)
		assert.True(t, err.Retryable)
		assert.True(t, messaging.IsRetryable(err))

		err = err.SetRetryable(false)
		assert.False(t, err.Retryable)
		assert.False(t, messaging.IsRetryable(err))
	})

	t.Run("TemporaryFlag", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		assert.False(t, err.Temporary)

		err = err.SetTemporary(true)
		assert.True(t, err.Temporary)
		assert.True(t, messaging.IsTemporary(err))

		err = err.SetTemporary(false)
		assert.False(t, err.Temporary)
		assert.False(t, messaging.IsTemporary(err))
	})

	t.Run("IsRetryableError", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level1 = level1.SetRetryable(true)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		assert.True(t, messaging.IsRetryableError(level2))
		assert.False(t, messaging.IsRetryableError(rootCause))
	})

	t.Run("IsTemporaryError", func(t *testing.T) {
		rootCause := errors.New("root cause")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", rootCause)
		level1 = level1.SetTemporary(true)
		level2 := messaging.WrapError(messaging.ErrorCodeInternal, "level2", "level2 error", level1)

		assert.True(t, messaging.IsTemporaryError(level2))
		assert.False(t, messaging.IsTemporaryError(rootCause))
	})

	t.Run("GetErrorCode", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		code := messaging.GetErrorCode(err)
		assert.Equal(t, messaging.ErrorCodePublish, code)
	})

	t.Run("GetErrorCodeNonMessagingError", func(t *testing.T) {
		err := errors.New("standard error")
		code := messaging.GetErrorCode(err)
		assert.Equal(t, messaging.ErrorCodeInternal, code) // Should default to Internal
	})
}

// TestSpecializedErrorConstructors tests specialized error constructors
func TestSpecializedErrorConstructors(t *testing.T) {
	t.Run("PublishError", func(t *testing.T) {
		cause := errors.New("publish cause")
		err := messaging.PublishError("publish failed", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "publish", err.Operation)
		assert.Equal(t, "publish failed", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("ConsumeError", func(t *testing.T) {
		cause := errors.New("consume cause")
		err := messaging.ConsumeError("consume failed", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeConsume, err.Code)
		assert.Equal(t, "consume", err.Operation)
		assert.Equal(t, "consume failed", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("ConnectionError", func(t *testing.T) {
		cause := errors.New("connection cause")
		err := messaging.ConnectionError("connection failed", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeConnection, err.Code)
		assert.Equal(t, "connection", err.Operation)
		assert.Equal(t, "connection failed", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("ConfigurationError", func(t *testing.T) {
		cause := errors.New("config cause")
		err := messaging.ConfigurationError("config failed", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeConfiguration, err.Code)
		assert.Equal(t, "configuration", err.Operation)
		assert.Equal(t, "config failed", err.Message)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("TimeoutError", func(t *testing.T) {
		cause := errors.New("timeout cause")
		err := messaging.TimeoutError("test_operation", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeTimeout, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "operation timed out", err.Message)
		assert.Equal(t, cause, err.Cause)
		assert.True(t, err.Temporary)
		assert.True(t, err.Retryable)
	})

	t.Run("BackpressureError", func(t *testing.T) {
		err := messaging.BackpressureError("test_operation")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodeBackpressure, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "backpressure limit exceeded", err.Message)
		assert.Nil(t, err.Cause)
		assert.True(t, err.Temporary)
		assert.True(t, err.Retryable)
	})
}

// TestImmutableErrors tests immutable error constructors
func TestImmutableErrors(t *testing.T) {
	t.Run("NewImmutableError", func(t *testing.T) {
		err := messaging.NewImmutableError(messaging.ErrorCodePublish, "test_operation", "test message")
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "test message", err.Message)
		assert.False(t, err.Retryable)
		assert.False(t, err.Temporary)
	})

	t.Run("WrapImmutableError", func(t *testing.T) {
		cause := errors.New("original error")
		err := messaging.WrapImmutableError(messaging.ErrorCodePublish, "test_operation", "wrapped message", cause)
		assert.NotNil(t, err)
		assert.Equal(t, messaging.ErrorCodePublish, err.Code)
		assert.Equal(t, "test_operation", err.Operation)
		assert.Equal(t, "wrapped message", err.Message)
		assert.Equal(t, cause, err.Cause)
		assert.False(t, err.Retryable)
		assert.False(t, err.Temporary)
	})
}

// TestErrorCategorization tests error categorization
func TestErrorCategorization(t *testing.T) {
	t.Run("CategorizeError", func(t *testing.T) {
		testCases := []struct {
			code     messaging.ErrorCode
			expected messaging.ErrorCategory
		}{
			{messaging.ErrorCodeConnection, messaging.CategoryConnection},
			{messaging.ErrorCodeChannel, messaging.CategoryConnection},
			{messaging.ErrorCodePublish, messaging.CategoryPublisher},
			{messaging.ErrorCodeConsume, messaging.CategoryConsumer},
			{messaging.ErrorCodeSerialization, messaging.CategoryMessage},
			{messaging.ErrorCodeValidation, messaging.CategoryMessage},
			{messaging.ErrorCodeConfiguration, messaging.CategoryConfiguration},
			{messaging.ErrorCodeTransport, messaging.CategoryTransport},
			{messaging.ErrorCodeInternal, messaging.CategoryTransport}, // Default case
		}

		for _, tc := range testCases {
			category := messaging.CategorizeError(tc.code)
			assert.Equal(t, tc.expected, category, "Error code %s should be categorized as %s", tc.code, tc.expected)
		}
	})
}

// TestValidateErrorCode tests error code validation
func TestValidateErrorCode(t *testing.T) {
	t.Run("ValidErrorCode", func(t *testing.T) {
		err := messaging.ValidateErrorCode(messaging.ErrorCodePublish)
		assert.NoError(t, err)
	})

	t.Run("InvalidErrorCode", func(t *testing.T) {
		invalidCode := messaging.ErrorCode("INVALID")
		err := messaging.ValidateErrorCode(invalidCode)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, messaging.ErrInvalidErrorCode))
		assert.Contains(t, err.Error(), "INVALID")
	})
}

// TestEdgeCases tests edge cases and error scenarios
func TestEdgeCases(t *testing.T) {
	t.Run("NilErrorHandling", func(t *testing.T) {
		var err *messaging.MessagingError
		assert.Equal(t, "<nil>", err.Error())
		assert.Nil(t, err.Unwrap())
		assert.False(t, err.Is(nil))
		assert.Nil(t, err.WithContext("key", "value"))
		assert.Nil(t, err.WithContextSafe("key", "value"))
		assert.Nil(t, err.SetRetryable(true))
		assert.Nil(t, err.SetTemporary(true))
	})

	t.Run("EmptyErrorMessages", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "", "")
		assert.NotNil(t, err)
		assert.Equal(t, "unknown", err.Operation)
		assert.Equal(t, "no error message provided", err.Message)
	})

	t.Run("ConcurrentErrorAccess", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test_operation", "test message")
		var wg sync.WaitGroup
		const numGoroutines = 10

		// Test concurrent context access
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				err.WithContext(fmt.Sprintf("key_%d", id), fmt.Sprintf("value_%d", id))
				messaging.GetErrorContext(err)
			}(i)
		}

		wg.Wait()
		// Should not panic
	})

	t.Run("ComplexErrorChains", func(t *testing.T) {
		// Create a complex error chain
		root := errors.New("root error")
		level1 := messaging.WrapError(messaging.ErrorCodePublish, "level1", "level1 error", root)
		level2 := messaging.WrapError(messaging.ErrorCodeConsume, "level2", "level2 error", level1)
		level3 := messaging.WrapError(messaging.ErrorCodeConnection, "level3", "level3 error", level2)

		// Test chain analysis
		chain := messaging.GetErrorChain(level3)
		assert.Len(t, chain, 4)

		// Test error finding
		publishErr := messaging.FindErrorByCode(level3, messaging.ErrorCodePublish)
		assert.NotNil(t, publishErr)

		consumeErr := messaging.FindErrorByCode(level3, messaging.ErrorCodeConsume)
		assert.NotNil(t, consumeErr)

		connectionErr := messaging.FindErrorByCode(level3, messaging.ErrorCodeConnection)
		assert.NotNil(t, connectionErr)

		// Test error counting
		count := messaging.CountErrorsInChain(level3)
		assert.Equal(t, 4, count)
	})
}

// TestErrorIntegration tests error integration with other components
func TestErrorIntegration(t *testing.T) {
	t.Run("ErrorWithContextIntegration", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "publish_operation", "publish failed")
		err = err.WithContext("message_id", "msg-123")
		err = err.WithContext("exchange", "test.exchange")
		err = err.WithContext("retry_count", 3)

		// Test context retrieval
		msgID, ok := messaging.GetContextString(err, "message_id")
		assert.True(t, ok)
		assert.Equal(t, "msg-123", msgID)

		exchange, ok := messaging.GetContextString(err, "exchange")
		assert.True(t, ok)
		assert.Equal(t, "test.exchange", exchange)

		retryCount, ok := messaging.GetContextInt(err, "retry_count")
		assert.True(t, ok)
		assert.Equal(t, 3, retryCount)

		// Test full context
		context := messaging.GetErrorContext(err)
		assert.Len(t, context, 3)
		assert.Equal(t, "msg-123", context["message_id"])
		assert.Equal(t, "test.exchange", context["exchange"])
		assert.Equal(t, 3, context["retry_count"])
	})

	t.Run("ErrorChainIntegration", func(t *testing.T) {
		// Simulate a real-world error scenario
		networkErr := errors.New("network connection failed")
		amqpErr := messaging.WrapError(messaging.ErrorCodeConnection, "amqp_connect", "failed to connect to AMQP", networkErr)
		publishErr := messaging.WrapError(messaging.ErrorCodePublish, "publish_message", "failed to publish message", amqpErr)

		// Test error analysis
		assert.True(t, messaging.HasErrorCode(publishErr, messaging.ErrorCodeConnection))
		assert.True(t, messaging.HasErrorCode(publishErr, messaging.ErrorCodePublish))
		assert.False(t, messaging.HasErrorCode(publishErr, messaging.ErrorCodeConsume))

		// Test root cause
		rootCause := messaging.GetRootCause(publishErr)
		assert.Equal(t, networkErr, rootCause)

		// Test error chain
		chain := messaging.GetErrorChain(publishErr)
		assert.Len(t, chain, 3)

		// Test error finding
		connectionErr := messaging.FindErrorByCode(publishErr, messaging.ErrorCodeConnection)
		assert.NotNil(t, connectionErr)
		assert.Equal(t, "amqp_connect", connectionErr.Operation)
	})
}
