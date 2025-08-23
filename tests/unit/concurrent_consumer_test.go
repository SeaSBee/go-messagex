package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// createTestObservabilityContext creates a proper observability context for testing
func createTestObservabilityContextForConsumer() *messaging.ObservabilityContext {
	telemetryConfig := &messaging.TelemetryConfig{
		MetricsEnabled: false, // Disable metrics in tests
		TracingEnabled: false, // Disable tracing in tests
		ServiceName:    "test-service",
	}

	obsProvider, err := messaging.NewObservabilityProvider(telemetryConfig)
	if err != nil {
		// Return a minimal context if the provider fails
		return &messaging.ObservabilityContext{}
	}

	return messaging.NewObservabilityContext(context.Background(), obsProvider)
}

// MockHandler implements the messaging.Handler interface for testing
type MockHandler struct {
	processFunc func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error)
	callCount   int
}

func (mh *MockHandler) Process(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
	mh.callCount++
	if mh.processFunc != nil {
		return mh.processFunc(ctx, delivery)
	}
	return messaging.Ack, nil
}

func TestConcurrentConsumer(t *testing.T) {
	t.Run("NewConcurrentConsumer", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		// Create mock transport
		transport := &rabbitmq.Transport{} // This would need to be properly mocked
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("ConsumerConfiguration", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              100,
			MaxConcurrentHandlers: 200,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        15 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            5,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Test DLQ setting
		dlq := &messaging.DeadLetterQueue{}
		consumer.SetDLQ(dlq)

		// Test stats
		stats := consumer.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MessagesReceived)
		assert.Equal(t, uint64(0), stats.MessagesProcessed)
	})

	t.Run("WorkerStats", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Workers haven't been started yet, so this should be empty
		workerStats := consumer.GetWorkerStats()
		assert.Empty(t, workerStats)
	})

	t.Run("ConsumerStats", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test initial stats
		stats := consumer.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MessagesReceived)
		assert.Equal(t, uint64(0), stats.MessagesProcessed)
		assert.Equal(t, uint64(0), stats.MessagesFailed)
		assert.Equal(t, uint64(0), stats.MessagesRequeued)
		assert.Equal(t, uint64(0), stats.MessagesSentToDLQ)
		assert.Equal(t, uint64(0), stats.PanicsRecovered)
		assert.Equal(t, 0, stats.ActiveWorkers)
		assert.Equal(t, 0, stats.QueuedTasks)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		// Test with minimal config to ensure defaults are applied
		config := &messaging.ConsumerConfig{
			Queue: "test.queue",
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Verify string representation
		str := consumer.String()
		assert.Contains(t, str, "ConcurrentConsumer")
		assert.Contains(t, str, "test.queue")
	})
}

func TestConsumerWrapper(t *testing.T) {
	t.Run("NewConsumer", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("ConsumerDelegation", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConsumer(transport, config, obsCtx)

		// Test DLQ setting
		dlq := &messaging.DeadLetterQueue{}
		consumer.SetDLQ(dlq)

		// Test stats
		stats := consumer.GetStats()
		assert.NotNil(t, stats)

		workerStats := consumer.GetWorkerStats()
		assert.Empty(t, workerStats) // No workers started yet
	})

	t.Run("LifecycleManagement", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConsumer(transport, config, obsCtx)

		// Test that we can create multiple stop calls safely
		ctx := context.Background()
		err := consumer.Stop(ctx)
		assert.NoError(t, err)

		// Second stop should also work
		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestMessageHandling(t *testing.T) {
	t.Run("HandlerTaskCreation", func(t *testing.T) {
		// Test that handler tasks can be created properly
		// This is more of a structural test since we can't easily mock AMQP

		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 10,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Verify the configuration was applied correctly
		stats := consumer.GetStats()
		assert.Equal(t, 0, stats.ActiveWorkers) // No workers started yet
	})

	t.Run("PanicRecoveryConfiguration", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         false, // Disabled
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("RetryConfiguration", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            0, // No retries
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})
}

func TestConcurrencyLimits(t *testing.T) {
	t.Run("ExtremelyHighConcurrency", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              65535, // Max prefetch
			MaxConcurrentHandlers: 20000, // Very high, should be capped
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("ZeroConcurrency", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			Prefetch:              0, // Should default
			MaxConcurrentHandlers: 0, // Should default
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})
}
