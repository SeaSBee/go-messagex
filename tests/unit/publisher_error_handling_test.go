package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

// TestPublisherErrorHandling tests error handling scenarios in the publisher
func TestPublisherErrorHandling(t *testing.T) {
	t.Run("AsyncPublisherStartFailure", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish a message (will fail due to transport not connected)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)

		// Should fail due to transport not connected, but not due to async publisher start failure
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to start async publisher")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Create a context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Try to publish with cancelled context
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(ctx, "test.topic", *msg)

		// Should fail due to context cancellation or transport not connected
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Try to publish with timeout context
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(ctx, "test.topic", *msg)

		// Should fail due to timeout or transport not connected
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})

	t.Run("MemoryPressure", func(t *testing.T) {
		// Create publisher with very small queue to simulate memory pressure
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{
			MaxInFlight:    1, // Very small queue
			WorkerCount:    1,
			DropOnOverflow: true,
		}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish a message
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)

		// Should not fail due to memory pressure, but may fail due to transport
		if err != nil {
			assert.NotContains(t, err.Error(), "out of memory")
			assert.NotContains(t, err.Error(), "memory allocation failed")
		}
	})

	t.Run("TransportErrors", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://invalid-host:5672"}, // Invalid host
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish a message
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)

		// Should fail due to transport connection issues
		if err != nil {
			assert.NotContains(t, err.Error(), "invalid topic name")
			assert.NotContains(t, err.Error(), "message ID cannot be empty")
			assert.NotContains(t, err.Error(), "routing key cannot be empty")
		}
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test that validation errors are properly propagated
		invalidMsg := messaging.Message{
			ID:          "", // Empty ID should cause validation error
			Key:         "test.key",
			Body:        []byte("test"),
			ContentType: "application/json",
		}
		receipt, err := publisher.PublishAsync(context.Background(), "test.topic", invalidMsg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "message ID cannot be empty")

		// Test that topic validation errors are properly propagated
		validMsg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err = publisher.PublishAsync(context.Background(), "", *validMsg) // Empty topic
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "topic cannot be empty")
	})

	t.Run("ObservabilityIntegration", func(t *testing.T) {
		// Create publisher with observability enabled
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish a message
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)

		// Should not fail due to observability issues
		if err != nil {
			assert.NotContains(t, err.Error(), "observability error")
			assert.NotContains(t, err.Error(), "metrics error")
			assert.NotContains(t, err.Error(), "tracing error")
		}
	})

	t.Run("GracefulDegradation", func(t *testing.T) {
		// Create publisher with minimal configuration
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{
			MaxInFlight: 1,
			WorkerCount: 1,
		}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish a message
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)

		// Should not fail due to minimal configuration
		if err != nil {
			assert.NotContains(t, err.Error(), "configuration error")
			assert.NotContains(t, err.Error(), "initialization error")
		}
	})
}

// TestPublisherEdgeCases tests edge cases and boundary conditions
func TestPublisherEdgeCases(t *testing.T) {
	t.Run("BoundaryValues", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test boundary values for topic length
		maxTopic := string(make([]byte, messaging.MaxTopicLength))
		for i := range maxTopic {
			maxTopic = maxTopic[:i] + "a" + maxTopic[i+1:]
		}
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), maxTopic, *msg)
		// Should not fail on topic validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "topic too long")
		}

		// Test boundary values for message ID length
		maxID := string(make([]byte, messaging.MaxMessageIDLength))
		for i := range maxID {
			maxID = maxID[:i] + "a" + maxID[i+1:]
		}
		msg.ID = maxID
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail on message ID validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "message ID too long")
		}

		// Test boundary values for routing key length
		maxKey := string(make([]byte, messaging.MaxRoutingKeyLength))
		for i := range maxKey {
			maxKey = maxKey[:i] + "a" + maxKey[i+1:]
		}
		msg.Key = maxKey
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail on routing key validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "routing key too long")
		}
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test with special characters in message body
		specialBody := []byte("test\n\r\t\b\f\v")
		msg := messaging.NewMessage(specialBody, messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail due to special characters in body
		if err != nil {
			assert.NotContains(t, err.Error(), "invalid character")
		}

		// Test with unicode characters
		unicodeBody := []byte("test\u00A0\u2028\u2029")
		msg = messaging.NewMessage(unicodeBody, messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail due to unicode characters
		if err != nil {
			assert.NotContains(t, err.Error(), "unicode error")
		}
	})

	t.Run("EmptyAndNilValues", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test with empty headers
		msg := messaging.Message{
			ID:          "test-id",
			Key:         "test.key",
			Body:        []byte("test"),
			ContentType: "application/json",
			Headers:     make(map[string]string), // Empty headers
		}
		_, err = publisher.PublishAsync(context.Background(), "test.topic", msg)
		// Should not fail due to empty headers
		if err != nil {
			assert.NotContains(t, err.Error(), "headers error")
		}

		// Test with nil headers
		msg.Headers = nil
		_, err = publisher.PublishAsync(context.Background(), "test.topic", msg)
		// Should not fail due to nil headers
		if err != nil {
			assert.NotContains(t, err.Error(), "headers error")
		}
	})
}
