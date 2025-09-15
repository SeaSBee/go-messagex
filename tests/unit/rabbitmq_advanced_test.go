package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

// TestNewAdvancedPublisher tests the AdvancedPublisher constructor
func TestNewAdvancedPublisher(t *testing.T) {
	t.Run("ValidParameters", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{
			Confirms:       true,
			Mandatory:      true,
			MaxInFlight:    1000,
			PublishTimeout: 5 * time.Second,
		}

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
	})

	t.Run("NilTransport", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create config
		config := &messaging.PublisherConfig{}

		// Should panic with nil transport
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedPublisher(nil, config, obsCtx)
		})
	})

	t.Run("NilConfig", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Should panic with nil config
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedPublisher(transport, nil, obsCtx)
		})
	})

	t.Run("NilObservability", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, nil)

		// Create config
		config := &messaging.PublisherConfig{}

		// Should panic with nil observability
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedPublisher(transport, config, nil)
		})
	})
}

// TestAdvancedPublisherSetters tests the setter methods
func TestAdvancedPublisherSetters(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	require.NoError(t, err)
	obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

	// Create transport
	transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
		URIs: []string{"amqp://localhost:5672"},
	}, obsCtx)

	// Create config
	config := &messaging.PublisherConfig{}

	// Create advanced publisher
	publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)

	t.Run("SetPersistence", func(t *testing.T) {
		// Create persistence
		persistenceConfig := &messaging.MessagePersistenceConfig{
			Enabled:         true,
			StorageType:     "memory",
			CleanupInterval: time.Hour,
			MessageTTL:      24 * time.Hour,
		}
		persistence, err := messaging.NewMessagePersistence(persistenceConfig, obsCtx)
		require.NoError(t, err)

		// Set persistence
		publisher.SetPersistence(persistence)

		// Test with nil
		publisher.SetPersistence(nil)
	})

	t.Run("SetTransformation", func(t *testing.T) {
		// Create transformation
		transformationConfig := &messaging.MessageTransformationConfig{
			Enabled: true,
		}
		transformation, err := messaging.NewMessageTransformation(transformationConfig, obsCtx)
		require.NoError(t, err)

		// Set transformation
		publisher.SetTransformation(transformation)

		// Test with nil
		publisher.SetTransformation(nil)
	})

	t.Run("SetRouting", func(t *testing.T) {
		// Create routing
		routingConfig := &messaging.AdvancedRoutingConfig{
			Enabled: true,
		}
		routing, err := messaging.NewAdvancedRouting(routingConfig, obsCtx)
		require.NoError(t, err)

		// Set routing
		publisher.SetRouting(routing)

		// Test with nil
		publisher.SetRouting(nil)
	})
}

// TestAdvancedPublisherClose tests the Close method
func TestAdvancedPublisherClose(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	require.NoError(t, err)
	obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

	t.Run("NilContext", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{}

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)

		// Close with nil context - this should work (no validation)
		err := publisher.Close(nil)
		assert.NoError(t, err)
	})

	t.Run("SuccessfulClose", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{}

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)

		// Close publisher
		err := publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("CloseWithComponents", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{}

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)

		// Add components
		persistenceConfig := &messaging.MessagePersistenceConfig{
			Enabled:         true,
			StorageType:     "memory",
			CleanupInterval: time.Hour,
			MessageTTL:      24 * time.Hour,
		}
		persistence, err := messaging.NewMessagePersistence(persistenceConfig, obsCtx)
		require.NoError(t, err)
		publisher.SetPersistence(persistence)

		transformationConfig := &messaging.MessageTransformationConfig{
			Enabled: true,
		}
		transformation, err := messaging.NewMessageTransformation(transformationConfig, obsCtx)
		require.NoError(t, err)
		publisher.SetTransformation(transformation)

		routingConfig := &messaging.AdvancedRoutingConfig{
			Enabled: true,
		}
		routing, err := messaging.NewAdvancedRouting(routingConfig, obsCtx)
		require.NoError(t, err)
		publisher.SetRouting(routing)

		// Close publisher
		err = publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("DoubleClose", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{}

		// Create advanced publisher
		publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)

		// Close publisher twice
		err1 := publisher.Close(context.Background())
		err2 := publisher.Close(context.Background())

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should not error on second close
	})
}

// TestNewAdvancedConsumer tests the AdvancedConsumer constructor
func TestNewAdvancedConsumer(t *testing.T) {
	t.Run("ValidParameters", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.ConsumerConfig{
			Queue:      "test.queue",
			AutoAck:    false,
			Exclusive:  false,
			NoWait:     false,
			MaxRetries: 3,
		}

		// Create advanced consumer
		consumer := rabbitmq.NewAdvancedConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("NilTransport", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create config
		config := &messaging.ConsumerConfig{}

		// Should panic with nil transport
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedConsumer(nil, config, obsCtx)
		})
	})

	t.Run("NilConfig", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Should panic with nil config
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedConsumer(transport, nil, obsCtx)
		})
	})

	t.Run("NilObservability", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, nil)

		// Create config
		config := &messaging.ConsumerConfig{}

		// Should panic with nil observability
		assert.Panics(t, func() {
			rabbitmq.NewAdvancedConsumer(transport, config, nil)
		})
	})
}

// TestAdvancedConsumerSetters tests the setter methods
func TestAdvancedConsumerSetters(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	require.NoError(t, err)
	obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

	// Create transport
	transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
		URIs: []string{"amqp://localhost:5672"},
	}, obsCtx)

	// Create config
	config := &messaging.ConsumerConfig{
		Queue: "test.queue",
	}

	// Create advanced consumer
	consumer := rabbitmq.NewAdvancedConsumer(transport, config, obsCtx)

	t.Run("SetDLQ", func(t *testing.T) {
		// Create DLQ with nil transport (will use no-op transport)
		dlqConfig := &messaging.DeadLetterQueueConfig{
			Enabled: true,
		}
		dlq, err := messaging.NewDeadLetterQueue(dlqConfig, nil, obsCtx)
		require.NoError(t, err)

		// Set DLQ
		consumer.SetDLQ(dlq)

		// Test with nil
		consumer.SetDLQ(nil)
	})

	t.Run("SetTransformation", func(t *testing.T) {
		// Create transformation
		transformationConfig := &messaging.MessageTransformationConfig{
			Enabled: true,
		}
		transformation, err := messaging.NewMessageTransformation(transformationConfig, obsCtx)
		require.NoError(t, err)

		// Set transformation
		consumer.SetTransformation(transformation)

		// Test with nil
		consumer.SetTransformation(nil)
	})

	t.Run("SetRouting", func(t *testing.T) {
		// Create routing
		routingConfig := &messaging.AdvancedRoutingConfig{
			Enabled: true,
		}
		routing, err := messaging.NewAdvancedRouting(routingConfig, obsCtx)
		require.NoError(t, err)

		// Set routing
		consumer.SetRouting(routing)

		// Test with nil
		consumer.SetRouting(nil)
	})
}

// TestAdvancedConsumerStop tests the Stop method
func TestAdvancedConsumerStop(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	require.NoError(t, err)
	obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

	t.Run("NilContext", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.ConsumerConfig{
			Queue: "test.queue",
		}

		// Create advanced consumer
		consumer := rabbitmq.NewAdvancedConsumer(transport, config, obsCtx)

		// Stop with nil context - this should work (no validation)
		err := consumer.Stop(nil)
		assert.NoError(t, err)
	})

	t.Run("StopNotStarted", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.ConsumerConfig{
			Queue: "test.queue",
		}

		// Create advanced consumer
		consumer := rabbitmq.NewAdvancedConsumer(transport, config, obsCtx)

		// Stop without starting
		err := consumer.Stop(context.Background())
		assert.NoError(t, err)
	})

	t.Run("StopWithComponents", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.ConsumerConfig{
			Queue: "test.queue",
		}

		// Create advanced consumer
		consumer := rabbitmq.NewAdvancedConsumer(transport, config, obsCtx)

		// Add components
		dlqConfig := &messaging.DeadLetterQueueConfig{
			Enabled: true,
		}
		dlq, err := messaging.NewDeadLetterQueue(dlqConfig, nil, obsCtx)
		require.NoError(t, err)
		consumer.SetDLQ(dlq)

		transformationConfig := &messaging.MessageTransformationConfig{
			Enabled: true,
		}
		transformation, err := messaging.NewMessageTransformation(transformationConfig, obsCtx)
		require.NoError(t, err)
		consumer.SetTransformation(transformation)

		routingConfig := &messaging.AdvancedRoutingConfig{
			Enabled: true,
		}
		routing, err := messaging.NewAdvancedRouting(routingConfig, obsCtx)
		require.NoError(t, err)
		consumer.SetRouting(routing)

		// Stop consumer
		err = consumer.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestRabbitMQUtilityFunctions tests the utility functions
func TestRabbitMQUtilityFunctions(t *testing.T) {
	t.Run("GenerateMessageID", func(t *testing.T) {
		// Test multiple ID generation
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := rabbitmq.GenerateMessageID()

			// Verify ID format (16 character hex string)
			assert.Regexp(t, `^[a-f0-9]{16}$`, id)
			assert.NotEmpty(t, id)

			// Verify uniqueness
			assert.False(t, ids[id], "Duplicate ID generated: %s", id)
			ids[id] = true
		}
	})

	t.Run("GenerateIdempotencyKey", func(t *testing.T) {
		// Test with valid message
		msg := messaging.NewMessage(
			[]byte("test body"),
			messaging.WithKey("test.key"),
			messaging.WithID("test-id"),
		)

		key := rabbitmq.GenerateIdempotencyKey(msg)
		assert.NotEmpty(t, key)
		assert.Regexp(t, `^[a-f0-9]{64}$`, key) // SHA256 hash is 64 characters

		// Test with same message should generate same key
		key2 := rabbitmq.GenerateIdempotencyKey(msg)
		assert.Equal(t, key, key2)

		// Test with different message should generate different key
		msg2 := messaging.NewMessage(
			[]byte("different body"),
			messaging.WithKey("test.key"),
			messaging.WithID("test-id"),
		)
		key3 := rabbitmq.GenerateIdempotencyKey(msg2)
		assert.NotEqual(t, key, key3)

		// Test with nil message
		key4 := rabbitmq.GenerateIdempotencyKey(nil)
		assert.NotEmpty(t, key4)
		assert.Regexp(t, `^[a-f0-9]{64}$`, key4)
	})

	t.Run("GenerateIdempotencyKeyUniqueness", func(t *testing.T) {
		// Test that different messages generate different keys
		keys := make(map[string]bool)
		for i := 0; i < 100; i++ {
			msg := messaging.NewMessage(
				[]byte(fmt.Sprintf("test body %d", i)),
				messaging.WithKey(fmt.Sprintf("test.key.%d", i)),
				messaging.WithID(fmt.Sprintf("test-id-%d", i)),
			)

			key := rabbitmq.GenerateIdempotencyKey(msg)
			assert.False(t, keys[key], "Duplicate key generated: %s", key)
			keys[key] = true
		}
	})
}
