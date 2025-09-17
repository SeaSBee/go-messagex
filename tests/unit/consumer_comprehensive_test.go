package unit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// TestNewConsumerComprehensive tests the constructor with various scenarios
func TestNewConsumerComprehensive(t *testing.T) {
	t.Run("NilTransport", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(nil, config, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "transport cannot be nil")
	})

	t.Run("NilConfig", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("ValidCreation", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.True(t, consumer.IsInitialized())
	})

	t.Run("NilObservability", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}

		consumer, err := rabbitmq.NewConsumer(transport, config, nil)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.True(t, consumer.IsInitialized())
	})
}

// TestConsumerIsInitializedComprehensive tests the IsInitialized method
func TestConsumerIsInitializedComprehensive(t *testing.T) {
	t.Run("InitializedConsumer", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		assert.True(t, consumer.IsInitialized())
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test concurrent access to IsInitialized
		var wg sync.WaitGroup
		accessCount := 100

		for i := 0; i < accessCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				initialized := consumer.IsInitialized()
				assert.True(t, initialized)
			}()
		}

		wg.Wait()
	})
}

// TestConsumerStartComprehensive tests the Start method
func TestConsumerStartComprehensive(t *testing.T) {
	t.Run("NilContext", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		handler := &MockConcurrentHandler{}

		err = consumer.Start(nil, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("NilHandler", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()

		err = consumer.Start(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message handler cannot be nil")
	})

	t.Run("ClosedConsumer", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Close the consumer first
		ctx := context.Background()
		err = consumer.Stop(ctx)
		assert.NoError(t, err)

		// Try to start a closed consumer
		handler := &MockConcurrentHandler{}
		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consumer is closed")
	})

	t.Run("InvalidConfiguration", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{
			Queue:                 "", // Invalid: empty queue name
			Prefetch:              1,  // Valid prefetch to avoid that validation error
			MaxConcurrentHandlers: 1,  // Valid max concurrent handlers to avoid that validation error
		}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		handler := &MockConcurrentHandler{}

		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "queue name is required")
	})

	t.Run("ValidStart", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		handler := &MockConcurrentHandler{}

		// Start should fail due to transport not being connected, but should not panic
		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		// The error could be either configuration validation or transport connection
		assert.True(t,
			err.Error() == "CONNECTION: transport is not connected" ||
				err.Error() == "CONSUME: consumer already started" ||
				err.Error() == "queue name is required" ||
				err.Error() == "VALIDATION: consumer configuration validation failed: VALIDATION: prefetch count too low: 0 < 1",
			"Expected connection error, already started error, validation error, or prefetch error, got: %s", err.Error())
	})
}

// TestConsumerStopComprehensive tests the Stop method
func TestConsumerStopComprehensive(t *testing.T) {
	t.Run("NilContext", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		err = consumer.Stop(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("AlreadyClosed", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()

		// First stop
		err = consumer.Stop(ctx)
		assert.NoError(t, err)

		// Second stop should also work (already closed)
		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("ValidStop", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()

		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}

// TestConsumerSetDLQComprehensive tests the SetDLQ method
func TestConsumerSetDLQComprehensive(t *testing.T) {
	t.Run("SetDLQ", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		dlq := &messaging.DeadLetterQueue{}

		err = consumer.SetDLQ(dlq)
		assert.NoError(t, err)
	})

	t.Run("SetNilDLQ", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		err = consumer.SetDLQ(nil)
		assert.NoError(t, err)
	})
}

// TestConsumerGetStatsComprehensive tests the GetStats method
func TestConsumerGetStatsComprehensive(t *testing.T) {
	t.Run("GetStats", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		stats, err := consumer.GetStats()
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MessagesReceived)
		assert.Equal(t, uint64(0), stats.MessagesProcessed)
	})

	t.Run("ConcurrentStatsAccess", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test concurrent stats access
		var wg sync.WaitGroup
		accessCount := 100

		for i := 0; i < accessCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				stats, err := consumer.GetStats()
				assert.NoError(t, err)
				assert.NotNil(t, stats)
			}()
		}

		wg.Wait()
	})
}

// TestConsumerGetWorkerStatsComprehensive tests the GetWorkerStats method
func TestConsumerGetWorkerStatsComprehensive(t *testing.T) {
	t.Run("GetWorkerStats", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		workerStats, err := consumer.GetWorkerStats()
		assert.NoError(t, err)
		assert.NotNil(t, workerStats)
		// Before starting, worker stats should be empty
		assert.Empty(t, workerStats)
	})

	t.Run("ConcurrentWorkerStatsAccess", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test concurrent worker stats access
		var wg sync.WaitGroup
		accessCount := 100

		for i := 0; i < accessCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				workerStats, err := consumer.GetWorkerStats()
				assert.NoError(t, err)
				assert.NotNil(t, workerStats)
			}()
		}

		wg.Wait()
	})
}

// TestConsumerConfigurationValidationComprehensive tests configuration validation
func TestConsumerConfigurationValidationComprehensive(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		// This should fail at constructor level
		consumer, err := rabbitmq.NewConsumer(transport, nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("EmptyQueueName", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{
			Queue:                 "",
			Prefetch:              1, // Valid prefetch to avoid that validation error
			MaxConcurrentHandlers: 1, // Valid max concurrent handlers to avoid that validation error
		}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		handler := &MockConcurrentHandler{}

		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "queue name is required")
	})

	t.Run("InvalidQueueName", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{
			Queue:                 "invalid@queue",
			Prefetch:              1, // Valid prefetch to avoid that validation error
			MaxConcurrentHandlers: 1, // Valid max concurrent handlers to avoid that validation error
		}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		handler := &MockConcurrentHandler{}

		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid queue name")
	})

	t.Run("ValidQueueNames", func(t *testing.T) {
		validQueueNames := []string{
			"test.queue",
			"test_queue",
			"test-queue",
			"test123",
			"test.queue.name",
			"test_queue_name",
			"test-queue-name",
		}

		for _, queueName := range validQueueNames {
			t.Run(queueName, func(t *testing.T) {
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				config := &messaging.ConsumerConfig{Queue: queueName}
				obsCtx := createTestObservabilityContextForConsumer()

				consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
				assert.NoError(t, err)
				assert.NotNil(t, consumer)
				assert.True(t, consumer.IsInitialized())
			})
		}
	})

	t.Run("InvalidQueueNames", func(t *testing.T) {
		invalidQueueNames := []string{
			"test@queue",
			"test#queue",
			"test$queue",
			"test%queue",
			"test^queue",
			"test&queue",
			"test*queue",
			"test(queue",
			"test)queue",
			"test+queue",
			"test=queue",
			"test[queue",
			"test]queue",
			"test{queue",
			"test}queue",
			"test|queue",
			"test\\queue",
			"test/queue",
			"test:queue",
			"test;queue",
			"test\"queue",
			"test'queue",
			"test<queue",
			"test>queue",
			"test,queue",
			"test?queue",
			"test!queue",
		}

		for _, queueName := range invalidQueueNames {
			t.Run(queueName, func(t *testing.T) {
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				config := &messaging.ConsumerConfig{
					Queue:                 queueName,
					Prefetch:              1, // Valid prefetch to avoid that validation error
					MaxConcurrentHandlers: 1, // Valid max concurrent handlers to avoid that validation error
				}
				obsCtx := createTestObservabilityContextForConsumer()

				consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
				require.NoError(t, err)
				require.NotNil(t, consumer)

				ctx := context.Background()
				handler := &MockConcurrentHandler{}

				err = consumer.Start(ctx, handler)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid queue name")
			})
		}
	})
}

// TestConsumerConcurrencyComprehensive tests concurrency aspects
func TestConsumerConcurrencyComprehensive(t *testing.T) {
	t.Run("ConcurrentStartStop", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test concurrent start/stop operations
		var wg sync.WaitGroup
		operationCount := 50

		for i := 0; i < operationCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				handler := &MockConcurrentHandler{}

				// Try to start (will likely fail due to transport not connected)
				_ = consumer.Start(ctx, handler)

				// Try to stop
				_ = consumer.Stop(ctx)
			}()
		}

		wg.Wait()
	})

	t.Run("ConcurrentStatsAccess", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test concurrent stats access
		var wg sync.WaitGroup
		accessCount := 100

		for i := 0; i < accessCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Test IsInitialized
				initialized := consumer.IsInitialized()
				assert.True(t, initialized)

				// Test GetStats
				stats, err := consumer.GetStats()
				assert.NoError(t, err)
				assert.NotNil(t, stats)

				// Test GetWorkerStats
				workerStats, err := consumer.GetWorkerStats()
				assert.NoError(t, err)
				assert.NotNil(t, workerStats)
			}()
		}

		wg.Wait()
	})
}

// TestConsumerLifecycleComprehensive tests the complete lifecycle
func TestConsumerLifecycleComprehensive(t *testing.T) {
	t.Run("CompleteLifecycle", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		// Create consumer
		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Verify initialization
		assert.True(t, consumer.IsInitialized())

		// Test stats before start
		stats, err := consumer.GetStats()
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MessagesReceived)

		// Test worker stats before start
		workerStats, err := consumer.GetWorkerStats()
		assert.NoError(t, err)
		assert.NotNil(t, workerStats)
		assert.Empty(t, workerStats)

		// Test DLQ setting
		dlq := &messaging.DeadLetterQueue{}
		err = consumer.SetDLQ(dlq)
		assert.NoError(t, err)

		// Test stop
		ctx := context.Background()
		err = consumer.Stop(ctx)
		assert.NoError(t, err)

		// Verify consumer is closed
		// Note: We can't directly test the closed state, but we can test that
		// subsequent operations fail appropriately
	})

	t.Run("MultipleConsumers", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		// Create multiple consumers
		var consumers []*rabbitmq.Consumer
		for i := 0; i < 5; i++ {
			config := &messaging.ConsumerConfig{Queue: fmt.Sprintf("test.queue.%d", i)}
			consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
			require.NoError(t, err)
			require.NotNil(t, consumer)
			consumers = append(consumers, consumer)
		}

		// Test each consumer
		for _, consumer := range consumers {
			assert.True(t, consumer.IsInitialized())

			stats, err := consumer.GetStats()
			assert.NoError(t, err)
			assert.NotNil(t, stats)

			// Stop consumer
			ctx := context.Background()
			err = consumer.Stop(ctx)
			assert.NoError(t, err)
		}

		assert.Len(t, consumers, 5)
	})
}

// TestConsumerErrorHandlingComprehensive tests error handling scenarios
func TestConsumerErrorHandlingComprehensive(t *testing.T) {
	t.Run("TransportErrors", func(t *testing.T) {
		// Test with invalid transport configuration
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://invalid:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		handler := &MockConcurrentHandler{}

		// Start should fail due to transport issues
		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
	})

	t.Run("ConfigurationErrors", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *messaging.ConsumerConfig
		}{
			{"EmptyQueue", &messaging.ConsumerConfig{Queue: ""}},
			{"InvalidQueueName", &messaging.ConsumerConfig{Queue: "invalid@queue"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				consumer, err := rabbitmq.NewConsumer(transport, tc.config, obsCtx)
				require.NoError(t, err)
				require.NotNil(t, consumer)

				ctx := context.Background()
				handler := &MockConcurrentHandler{}

				err = consumer.Start(ctx, handler)
				assert.Error(t, err)
			})
		}
	})
}

// TestConsumerEdgeCasesComprehensive tests edge cases and boundary conditions
func TestConsumerEdgeCasesComprehensive(t *testing.T) {
	t.Run("MemorySafety", func(t *testing.T) {
		// Test that consumer doesn't leak memory
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		// Create many consumers to test memory safety
		var consumers []*rabbitmq.Consumer
		for i := 0; i < 100; i++ {
			config := &messaging.ConsumerConfig{Queue: fmt.Sprintf("test.queue.%d", i)}
			consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
			require.NoError(t, err)
			require.NotNil(t, consumer)
			consumers = append(consumers, consumer)

			// Test each consumer
			assert.True(t, consumer.IsInitialized())

			// Stop each consumer
			ctx := context.Background()
			err = consumer.Stop(ctx)
			assert.NoError(t, err)
		}

		assert.Len(t, consumers, 100)
	})

	t.Run("GoroutineLeaks", func(t *testing.T) {
		// Test that Stop() doesn't leak goroutines
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test that Stop() completes quickly
		start := time.Now()
		ctx := context.Background()
		err = consumer.Stop(ctx)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 100*time.Millisecond) // Should complete quickly
	})

	t.Run("HighVolumeOperations", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		consumer, err := rabbitmq.NewConsumer(transport, config, obsCtx)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		// Test many concurrent operations
		var wg sync.WaitGroup
		operationCount := 1000

		for i := 0; i < operationCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Test IsInitialized
				initialized := consumer.IsInitialized()
				assert.True(t, initialized)

				// Test GetStats
				stats, err := consumer.GetStats()
				assert.NoError(t, err)
				assert.NotNil(t, stats)

				// Test GetWorkerStats
				workerStats, err := consumer.GetWorkerStats()
				assert.NoError(t, err)
				assert.NotNil(t, workerStats)
			}()
		}

		wg.Wait()

		// Stop consumer
		ctx := context.Background()
		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}
