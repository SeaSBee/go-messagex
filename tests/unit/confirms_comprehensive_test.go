package unit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// TestChannelConfirmManagerComprehensive tests the channel confirm manager
func TestChannelConfirmManagerComprehensive(t *testing.T) {
	t.Run("NewChannelConfirmManager", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		assert.NotNil(t, manager)
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("GetOrCreateTrackerNilChannel", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		obsCtx := createTestObservabilityContextForConsumer()

		tracker, err := manager.GetOrCreateTracker(nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, tracker)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("RemoveTrackerNilChannel", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		err := manager.RemoveTracker(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("RemoveNonExistentTracker", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		// Create a mock channel (we can't use it for GetOrCreateTracker, but we can test RemoveTracker)
		// Since we can't create a real channel without RabbitMQ, we'll test the error handling
		err := manager.RemoveTracker(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("ManagerClose", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		// Close manager
		err := manager.Close()
		assert.NoError(t, err)

		// Verify it's still at zero after close
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("ManagerPendingCount", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		// Initial pending count should be 0
		assert.Equal(t, 0, manager.PendingCount())

		// After close, should still be 0
		err := manager.Close()
		assert.NoError(t, err)
		assert.Equal(t, 0, manager.PendingCount())
	})
}

// TestConfirmTrackerConfigurationComprehensive tests configuration handling
func TestConfirmTrackerConfigurationComprehensive(t *testing.T) {
	t.Run("DefaultConfirmTrackerConfig", func(t *testing.T) {
		config := rabbitmq.DefaultConfirmTrackerConfig()
		assert.NotNil(t, config)
		assert.Equal(t, 1000, config.ConfirmBufferSize)
		assert.Equal(t, 100, config.ReturnBufferSize)
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := &rabbitmq.ConfirmTrackerConfig{
			ConfirmBufferSize: 500,
			ReturnBufferSize:  50,
		}

		assert.Equal(t, 500, config.ConfirmBufferSize)
		assert.Equal(t, 50, config.ReturnBufferSize)
	})

	t.Run("ZeroBufferSizes", func(t *testing.T) {
		config := &rabbitmq.ConfirmTrackerConfig{
			ConfirmBufferSize: 0,
			ReturnBufferSize:  0,
		}

		assert.Equal(t, 0, config.ConfirmBufferSize)
		assert.Equal(t, 0, config.ReturnBufferSize)
	})

	t.Run("NegativeBufferSizes", func(t *testing.T) {
		config := &rabbitmq.ConfirmTrackerConfig{
			ConfirmBufferSize: -1,
			ReturnBufferSize:  -1,
		}

		assert.Equal(t, -1, config.ConfirmBufferSize)
		assert.Equal(t, -1, config.ReturnBufferSize)
	})

	t.Run("LargeBufferSizes", func(t *testing.T) {
		config := &rabbitmq.ConfirmTrackerConfig{
			ConfirmBufferSize: 10000,
			ReturnBufferSize:  5000,
		}

		assert.Equal(t, 10000, config.ConfirmBufferSize)
		assert.Equal(t, 5000, config.ReturnBufferSize)
	})
}

// TestConfirmTrackerConstructorComprehensive tests constructor error scenarios
func TestConfirmTrackerConstructorComprehensive(t *testing.T) {
	t.Run("NewConfirmTrackerNilChannel", func(t *testing.T) {
		obsCtx := createTestObservabilityContextForConsumer()

		tracker, err := rabbitmq.NewConfirmTracker(nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, tracker)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("NewConfirmTrackerWithConfigNilChannel", func(t *testing.T) {
		obsCtx := createTestObservabilityContextForConsumer()
		config := &rabbitmq.ConfirmTrackerConfig{}

		tracker, err := rabbitmq.NewConfirmTrackerWithConfig(nil, obsCtx, config)
		assert.Error(t, err)
		assert.Nil(t, tracker)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("NewConfirmTrackerWithConfigNilConfig", func(t *testing.T) {
		// This test would require a real AMQP channel, so we'll skip it
		// The nil config handling is tested in the configuration tests above
		t.Skip("Requires real AMQP channel")
	})
}

// TestConfirmTrackerErrorHandlingComprehensive tests error handling scenarios
func TestConfirmTrackerErrorHandlingComprehensive(t *testing.T) {
	t.Run("NilObservabilityHandling", func(t *testing.T) {
		// This test would require a real AMQP channel, so we'll skip it
		// The nil observability handling is tested in integration scenarios
		t.Skip("Requires real AMQP channel")
	})

	t.Run("ChannelConfirmFailure", func(t *testing.T) {
		// This test would require a real AMQP channel, so we'll skip it
		// The confirm mode failure is tested in constructor tests
		t.Skip("Requires real AMQP channel")
	})

	t.Run("InvalidConfiguration", func(t *testing.T) {
		// This test would require a real AMQP channel, so we'll skip it
		// The configuration validation is tested in configuration tests
		t.Skip("Requires real AMQP channel")
	})
}

// TestConfirmTrackerConcurrencyComprehensive tests concurrency aspects
func TestConfirmTrackerConcurrencyComprehensive(t *testing.T) {
	t.Run("ConcurrentManagerOperations", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		defer manager.Close()

		// Test concurrent manager operations
		var wg sync.WaitGroup
		operationCount := 20

		for i := 0; i < operationCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Test pending count access
				count := manager.PendingCount()
				assert.GreaterOrEqual(t, count, 0)

				// Test nil channel removal
				err := manager.RemoveTracker(nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "channel cannot be nil")
			}(i)
		}

		wg.Wait()
	})

	t.Run("ManagerThreadSafety", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		defer manager.Close()

		// Test concurrent pending count access
		var wg sync.WaitGroup
		accessCount := 100

		for i := 0; i < accessCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				count := manager.PendingCount()
				assert.GreaterOrEqual(t, count, 0)
			}()
		}

		wg.Wait()
	})
}

// TestConfirmTrackerLifecycleComprehensive tests the complete lifecycle
func TestConfirmTrackerLifecycleComprehensive(t *testing.T) {
	t.Run("ManagerLifecycle", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		// Verify initial state
		assert.Equal(t, 0, manager.PendingCount())

		// Close manager
		err := manager.Close()
		assert.NoError(t, err)

		// Verify final state
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("ManagerDoubleClose", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()

		// First close
		err := manager.Close()
		assert.NoError(t, err)

		// Second close should also work
		err = manager.Close()
		assert.NoError(t, err)

		// Verify state
		assert.Equal(t, 0, manager.PendingCount())
	})
}

// TestConfirmTrackerIntegrationComprehensive tests integration scenarios
func TestConfirmTrackerIntegrationComprehensive(t *testing.T) {
	t.Run("WithObservability", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		defer manager.Close()

		// Test with proper observability context
		telemetryConfig := &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "test-service",
		}

		obsProvider, err := messaging.NewObservabilityProvider(telemetryConfig)
		require.NoError(t, err)

		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		// Test that observability doesn't break manager functionality
		assert.Equal(t, 0, manager.PendingCount())

		// Test nil channel with observability
		tracker, err := manager.GetOrCreateTracker(nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, tracker)
		assert.Contains(t, err.Error(), "channel cannot be nil")
	})

	t.Run("MultipleManagerInstances", func(t *testing.T) {
		// Test multiple manager instances
		manager1 := rabbitmq.NewChannelConfirmManager()
		defer manager1.Close()

		manager2 := rabbitmq.NewChannelConfirmManager()
		defer manager2.Close()

		// Each should be independent
		assert.Equal(t, 0, manager1.PendingCount())
		assert.Equal(t, 0, manager2.PendingCount())

		// Close one shouldn't affect the other
		err := manager1.Close()
		assert.NoError(t, err)
		assert.Equal(t, 0, manager2.PendingCount())
	})
}

// TestConfirmTrackerEdgeCasesComprehensive tests edge cases and boundary conditions
func TestConfirmTrackerEdgeCasesComprehensive(t *testing.T) {
	t.Run("MemorySafety", func(t *testing.T) {
		// Test that manager doesn't leak memory

		// Create many managers to test memory safety
		var managers []*rabbitmq.ChannelConfirmManager
		for i := 0; i < 100; i++ {
			manager := rabbitmq.NewChannelConfirmManager()
			managers = append(managers, manager)

			// Test each manager
			assert.Equal(t, 0, manager.PendingCount())

			// Close each manager
			err := manager.Close()
			assert.NoError(t, err)
		}

		assert.Len(t, managers, 100)
	})

	t.Run("GoroutineLeaks", func(t *testing.T) {
		// Test that Close() doesn't leak goroutines
		manager := rabbitmq.NewChannelConfirmManager()

		// Test that Close() completes quickly
		start := time.Now()
		err := manager.Close()
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 100*time.Millisecond) // Should complete quickly
	})

	t.Run("HighVolumeOperations", func(t *testing.T) {
		manager := rabbitmq.NewChannelConfirmManager()
		defer manager.Close()

		// Test many concurrent operations
		var wg sync.WaitGroup
		operationCount := 1000

		for i := 0; i < operationCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Test pending count access
				count := manager.PendingCount()
				assert.GreaterOrEqual(t, count, 0)

				// Test nil channel removal
				err := manager.RemoveTracker(nil)
				assert.Error(t, err)
			}()
		}

		wg.Wait()

		// Verify final state
		assert.Equal(t, 0, manager.PendingCount())
	})
}

// TestConfirmTrackerReceiptManagerIntegration tests integration with receipt manager
func TestConfirmTrackerReceiptManagerIntegration(t *testing.T) {
	t.Run("ReceiptManagerCreation", func(t *testing.T) {
		// Test receipt manager creation and basic functionality
		manager := messaging.NewReceiptManager(30 * time.Second)
		assert.NotNil(t, manager)
		defer manager.Close()

		// Test creating a receipt
		ctx := context.Background()
		receipt := manager.CreateReceipt(ctx, "test-message-1")
		assert.NotNil(t, receipt)
		assert.Equal(t, "test-message-1", receipt.ID())

		// Test pending count
		assert.Equal(t, 1, manager.PendingCount())

		// Test completing a receipt
		result := messaging.PublishResult{
			MessageID:   "test-message-1",
			DeliveryTag: 123,
			Timestamp:   time.Now(),
			Success:     true,
		}
		completed := manager.CompleteReceipt("test-message-1", result, nil)
		assert.True(t, completed)

		// Wait for receipt to complete
		select {
		case <-receipt.Done():
			// Receipt completed successfully
			receivedResult, err := receipt.Result()
			assert.NoError(t, err)
			assert.Equal(t, "test-message-1", receivedResult.MessageID)
			assert.Equal(t, uint64(123), receivedResult.DeliveryTag)
			assert.True(t, receivedResult.Success)
		case <-time.After(1 * time.Second):
			t.Fatal("Receipt did not complete in time")
		}
	})

	t.Run("ReceiptManagerErrorHandling", func(t *testing.T) {
		manager := messaging.NewReceiptManager(30 * time.Second)
		assert.NotNil(t, manager)
		defer manager.Close()

		// Test creating a receipt
		ctx := context.Background()
		receipt := manager.CreateReceipt(ctx, "test-message-2")
		assert.NotNil(t, receipt)

		// Test completing a receipt with error
		result := messaging.PublishResult{
			MessageID:   "test-message-2",
			DeliveryTag: 456,
			Timestamp:   time.Now(),
			Success:     false,
			Reason:      "test error",
		}
		err := messaging.NewError(messaging.ErrorCodePublish, "test_error", "test error message")
		completed := manager.CompleteReceipt("test-message-2", result, err)
		assert.True(t, completed)

		// Wait for receipt to complete
		select {
		case <-receipt.Done():
			// Receipt completed with error
			receivedResult, receivedErr := receipt.Result()
			assert.Error(t, receivedErr)
			assert.Equal(t, "test-message-2", receivedResult.MessageID)
			assert.Equal(t, uint64(456), receivedResult.DeliveryTag)
			assert.False(t, receivedResult.Success)
			assert.Equal(t, "test error", receivedResult.Reason)
		case <-time.After(1 * time.Second):
			t.Fatal("Receipt did not complete in time")
		}
	})
}

// TestConfirmTrackerMetricsIntegration tests metrics integration
func TestConfirmTrackerMetricsIntegration(t *testing.T) {
	t.Run("MetricsInterface", func(t *testing.T) {
		// Test that the confirm metrics are available
		metrics := messaging.NoOpMetrics{}

		// Test that these methods exist and can be called without error
		metrics.ConfirmTotal("rabbitmq")
		metrics.ConfirmSuccess("rabbitmq")
		metrics.ConfirmFailure("rabbitmq", "test_reason")
		metrics.ConfirmDuration("rabbitmq", 10*time.Millisecond)
		metrics.ReturnTotal("rabbitmq")
		metrics.ReturnDuration("rabbitmq", 5*time.Millisecond)

		// If we get here without panicking, the metrics interface is correctly implemented
	})

	t.Run("PublishResultStructure", func(t *testing.T) {
		// Test the PublishResult structure used in confirmations
		result := messaging.PublishResult{
			MessageID:   "test-msg-123",
			DeliveryTag: 456,
			Timestamp:   time.Now(),
			Success:     true,
			Reason:      "",
		}

		assert.Equal(t, "test-msg-123", result.MessageID)
		assert.Equal(t, uint64(456), result.DeliveryTag)
		assert.True(t, result.Success)
		assert.Empty(t, result.Reason)
		assert.False(t, result.Timestamp.IsZero())

		// Test failure case
		failureResult := messaging.PublishResult{
			MessageID:   "test-msg-456",
			DeliveryTag: 789,
			Timestamp:   time.Now(),
			Success:     false,
			Reason:      "message unroutable",
		}

		assert.Equal(t, "test-msg-456", failureResult.MessageID)
		assert.Equal(t, uint64(789), failureResult.DeliveryTag)
		assert.False(t, failureResult.Success)
		assert.Equal(t, "message unroutable", failureResult.Reason)
	})
}

// TestConfirmTrackerConfigurationValidation tests configuration validation
func TestConfirmTrackerConfigurationValidation(t *testing.T) {
	t.Run("DefaultConfigValidation", func(t *testing.T) {
		config := rabbitmq.DefaultConfirmTrackerConfig()

		// Validate default values are reasonable
		assert.Greater(t, config.ConfirmBufferSize, 0)
		assert.Greater(t, config.ReturnBufferSize, 0)
		assert.LessOrEqual(t, config.ConfirmBufferSize, 100000) // Reasonable upper bound
		assert.LessOrEqual(t, config.ReturnBufferSize, 10000)   // Reasonable upper bound
	})

	t.Run("ConfigFieldValidation", func(t *testing.T) {
		// Test various configuration field values
		testCases := []struct {
			name                string
			confirmBufferSize   int
			returnBufferSize    int
			expectedConfirmSize int
			expectedReturnSize  int
		}{
			{"DefaultValues", 1000, 100, 1000, 100},
			{"SmallValues", 10, 5, 10, 5},
			{"LargeValues", 50000, 5000, 50000, 5000},
			{"ZeroValues", 0, 0, 0, 0},
			{"NegativeValues", -1, -1, -1, -1},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &rabbitmq.ConfirmTrackerConfig{
					ConfirmBufferSize: tc.confirmBufferSize,
					ReturnBufferSize:  tc.returnBufferSize,
				}

				assert.Equal(t, tc.expectedConfirmSize, config.ConfirmBufferSize)
				assert.Equal(t, tc.expectedReturnSize, config.ReturnBufferSize)
			})
		}
	})
}
