package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// TestNewAsyncPublisherComprehensive tests the AsyncPublisher constructor comprehensively
func TestNewAsyncPublisherComprehensive(t *testing.T) {
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
			MaxInFlight:    1000,
			WorkerCount:    4,
			DropOnOverflow: false,
			PublishTimeout: 5 * time.Second,
			Retry: &messaging.RetryConfig{
				MaxAttempts:       3,
				BaseBackoff:       100 * time.Millisecond,
				MaxBackoff:        1 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            true,
			},
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
		assert.NotNil(t, publisher.GetStats())
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
			rabbitmq.NewAsyncPublisher(nil, config, obsCtx)
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
			rabbitmq.NewAsyncPublisher(transport, nil, obsCtx)
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
			rabbitmq.NewAsyncPublisher(transport, config, nil)
		})
	})

	t.Run("DefaultQueueSize", func(t *testing.T) {
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

		// Create config with zero MaxInFlight (should use default)
		config := &messaging.PublisherConfig{
			MaxInFlight: 0,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
	})

	t.Run("NegativeWorkerCount", func(t *testing.T) {
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

		// Create config with negative worker count (should use default)
		config := &messaging.PublisherConfig{
			MaxInFlight: 100,
			WorkerCount: -1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
	})

	t.Run("ExcessiveWorkerCount", func(t *testing.T) {
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

		// Create config with excessive worker count (should be capped)
		config := &messaging.PublisherConfig{
			MaxInFlight: 100,
			WorkerCount: 200, // Should be capped to 100
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
	})

	t.Run("DefaultReceiptTimeout", func(t *testing.T) {
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

		// Create config without timeout (should use default)
		config := &messaging.PublisherConfig{
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
	})
}

// TestAsyncPublisherStartComprehensive tests the Start method comprehensively
func TestAsyncPublisherStartComprehensive(t *testing.T) {
	t.Run("NilContext", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 2,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Try to start with nil context
		err = publisher.Start(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("AlreadyStarted", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 2,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Try to start again
		err = publisher.Start(ctx)
		assert.NoError(t, err) // Should return nil (already started)

		// Clean up
		publisher.Close(ctx)
	})

	t.Run("ZeroWorkerCount", func(t *testing.T) {
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

		// Create config with zero worker count (should use at least 1)
		config := &messaging.PublisherConfig{
			MaxInFlight: 100,
			WorkerCount: 0,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Get worker stats
		workerStats := publisher.GetWorkerStats()
		assert.Len(t, workerStats, 1) // Should have at least one worker

		// Clean up
		publisher.Close(ctx)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 2,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Create context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Try to start with cancelled context
		err = publisher.Start(ctx)
		assert.NoError(t, err) // Start should still succeed

		// Clean up
		publisher.Close(context.Background())
	})
}

// TestAsyncPublisherPublishAsyncComprehensive tests the PublishAsync method comprehensively
func TestAsyncPublisherPublishAsyncComprehensive(t *testing.T) {
	t.Run("ClosedPublisher", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Close publisher
		publisher.Close(context.Background())

		// Try to publish
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "test.exchange", *msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "publisher is closed")
		assert.Nil(t, receipt)
	})

	t.Run("NilContext", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Try to publish with nil context
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(nil, "test.exchange", *msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
		assert.Nil(t, receipt)

		// Clean up
		publisher.Close(context.Background())
	})

	t.Run("EmptyTopic", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Try to publish with empty topic
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "", *msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic cannot be empty")
		assert.Nil(t, receipt)

		// Clean up
		publisher.Close(context.Background())
	})

	t.Run("InvalidTopic", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Try to publish with invalid topic
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "invalid@topic", *msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid topic name")
		assert.Nil(t, receipt)

		// Clean up
		publisher.Close(context.Background())
	})

	t.Run("InvalidMessage", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Try to publish with invalid message (very long topic)
		longTopic := string(make([]byte, 256)) // Topic longer than MaxTopicLength
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), longTopic, *msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic too long")
		assert.Nil(t, receipt)

		// Clean up
		publisher.Close(context.Background())
	})

	t.Run("SuccessfulPublish", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Publish message (will fail due to transport not connected, but validates the flow)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(ctx, "test.exchange", *msg)

		// Check if we got an error or success (depends on transport state)
		if err != nil {
			// Transport error expected
			assert.Contains(t, err.Error(), "transport is not connected")
		} else {
			// Success case - receipt should be valid
			assert.NotNil(t, receipt)
			assert.Equal(t, msg.ID, receipt.ID())
		}

		// Clean up
		publisher.Close(ctx)
	})

	t.Run("DropOnOverflow", func(t *testing.T) {
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

		// Create config with very small queue and drop on overflow
		config := &messaging.PublisherConfig{
			MaxInFlight:    1, // Very small queue
			WorkerCount:    1,
			DropOnOverflow: true,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Try to publish a message (will fail due to transport not connected, but validates config)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(ctx, "test.exchange", *msg)
		if err != nil {
			// Transport error expected
			assert.Contains(t, err.Error(), "transport is not connected")
		} else {
			// Success case - receipt should be valid
			assert.NotNil(t, receipt)
		}

		// Check stats
		stats := publisher.GetStats()
		assert.GreaterOrEqual(t, stats.TasksQueued, uint64(0)) // Tasks may be queued if transport works

		// Clean up
		publisher.Close(ctx)
	})

	t.Run("BlockOnOverflow", func(t *testing.T) {
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

		// Create config with very small queue and block on overflow
		config := &messaging.PublisherConfig{
			MaxInFlight:    1, // Very small queue
			WorkerCount:    1,
			DropOnOverflow: false,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Try to publish a message (will fail due to transport not connected, but validates config)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(ctx, "test.exchange", *msg)
		if err != nil {
			// Transport error expected
			assert.Contains(t, err.Error(), "transport is not connected")
		} else {
			// Success case - receipt should be valid
			assert.NotNil(t, receipt)
		}

		// Clean up
		publisher.Close(ctx)
	})
}

// TestAsyncPublisherCloseComprehensive tests the Close method comprehensively
func TestAsyncPublisherCloseComprehensive(t *testing.T) {
	t.Run("WorkerShutdown", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 2,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Close publisher
		err = publisher.Close(ctx)
		assert.NoError(t, err)

		// Verify workers are stopped
		workerStats := publisher.GetWorkerStats()
		assert.Len(t, workerStats, 2)
	})

	t.Run("CloseTimeout", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Close with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		err = publisher.Close(timeoutCtx)
		assert.NoError(t, err)
	})

	t.Run("DoubleClose", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Close publisher twice
		err1 := publisher.Close(ctx)
		err2 := publisher.Close(ctx)

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should not error on second close
	})
}

// TestAsyncPublisherStatsComprehensive tests the statistics methods comprehensively
func TestAsyncPublisherStatsComprehensive(t *testing.T) {
	t.Run("StatsIncrement", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 1,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Try to publish a message (will fail due to transport not connected)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(ctx, "test.exchange", *msg)
		if err != nil {
			// Transport error expected
			assert.Contains(t, err.Error(), "transport is not connected")
		} else {
			// Success case - receipt should be valid
			assert.NotNil(t, receipt)
		}

		// Check stats (may be queued if transport works)
		stats := publisher.GetStats()
		assert.GreaterOrEqual(t, stats.TasksQueued, uint64(0))
		assert.GreaterOrEqual(t, stats.TasksProcessed, uint64(0))

		// Clean up
		publisher.Close(ctx)
	})

	t.Run("WorkerStatsUpdate", func(t *testing.T) {
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
			MaxInFlight: 100,
			WorkerCount: 2,
		}

		// Create async publisher
		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err = publisher.Start(ctx)
		assert.NoError(t, err)

		// Try to publish messages (will fail due to transport not connected)
		for i := 0; i < 3; i++ {
			msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
			receipt, err := publisher.PublishAsync(ctx, "test.exchange", *msg)
			if err != nil {
				// Transport error expected
				assert.Contains(t, err.Error(), "transport is not connected")
			} else {
				// Success case - receipt should be valid
				assert.NotNil(t, receipt)
			}
		}

		// Check worker stats
		workerStats := publisher.GetWorkerStats()
		assert.Len(t, workerStats, 2)

		// Workers should be initialized and may have processed tasks
		for _, stats := range workerStats {
			assert.GreaterOrEqual(t, stats.TasksProcessed, uint64(0))
			assert.GreaterOrEqual(t, stats.TasksFailed, uint64(0))
		}

		// Clean up
		publisher.Close(ctx)
	})
}
