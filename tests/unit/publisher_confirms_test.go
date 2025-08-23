package unit

import (
	"context"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublisherConfirms(t *testing.T) {
	t.Run("ConfirmTracker Creation", func(t *testing.T) {
		// This test verifies the confirm tracker can be created
		// In a real test environment, we would need a mock AMQP channel

		// Test that the channel confirm manager can be created
		manager := rabbitmq.NewChannelConfirmManager()
		assert.NotNil(t, manager)

		// Test pending count starts at zero
		assert.Equal(t, 0, manager.PendingCount())

		// Test close functionality
		err := manager.Close()
		assert.NoError(t, err)

		// Verify it's still at zero after close
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("PublisherWithConfirmsEnabled", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			Publisher: &messaging.PublisherConfig{
				Confirms:       true, // Enable confirms
				Mandatory:      true,
				MaxInFlight:    1000,
				PublishTimeout: 5 * time.Second,
			},
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		// Create transport (this will not actually connect without RabbitMQ)
		transport := rabbitmq.NewTransport(config, obsCtx)
		assert.NotNil(t, transport)

		// Create publisher with confirms enabled
		publisher := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)
		assert.NotNil(t, publisher)

		// Verify publisher was created successfully
		// In a real environment, we would test actual publishing with confirms

		// Test close functionality
		err = publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("PublisherWithConfirmsDisabled", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			Publisher: &messaging.PublisherConfig{
				Confirms:       false, // Disable confirms
				Mandatory:      false,
				MaxInFlight:    1000,
				PublishTimeout: 5 * time.Second,
			},
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		// Create transport (this will not actually connect without RabbitMQ)
		transport := rabbitmq.NewTransport(config, obsCtx)
		assert.NotNil(t, transport)

		// Create publisher with confirms disabled
		publisher := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)
		assert.NotNil(t, publisher)

		// Test close functionality
		err = publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("AdvancedPublisherWithConfirms", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			Confirms:       true,
			Mandatory:      true,
			MaxInFlight:    1000,
			PublishTimeout: 5 * time.Second,
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		// Create transport
		rabbitmqConfig := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
		}
		transport := rabbitmq.NewTransport(rabbitmqConfig, obsCtx)

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)

		// Test close functionality
		err = publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("ReceiptManagerFunctionality", func(t *testing.T) {
		// Test the receipt manager that's used for tracking confirmations
		manager := messaging.NewReceiptManager(30 * time.Second)
		assert.NotNil(t, manager)

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

		// Test closing the manager
		manager.Close()
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("MetricsIntegration", func(t *testing.T) {
		// Test that the new confirm metrics are available
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

func TestPublisherConfirmsConfiguration(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		// Test default publisher configuration values
		config := &messaging.PublisherConfig{}

		// In the actual config loader, these would be set to defaults
		// Here we're just verifying the structure exists
		assert.NotNil(t, config)
	})

	t.Run("ConfigurationWithConfirms", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			Confirms:       true,
			Mandatory:      true,
			Immediate:      false,
			MaxInFlight:    10000,
			DropOnOverflow: false,
			PublishTimeout: 2 * time.Second,
			WorkerCount:    4,
		}

		assert.True(t, config.Confirms)
		assert.True(t, config.Mandatory)
		assert.False(t, config.Immediate)
		assert.Equal(t, 10000, config.MaxInFlight)
		assert.False(t, config.DropOnOverflow)
		assert.Equal(t, 2*time.Second, config.PublishTimeout)
		assert.Equal(t, 4, config.WorkerCount)
	})
}
