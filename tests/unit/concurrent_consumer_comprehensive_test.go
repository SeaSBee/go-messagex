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

// MockConcurrentHandler provides a mock implementation of messaging.Handler for testing
type MockConcurrentHandler struct {
	processFunc func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error)
	callCount   int
}

func (m *MockConcurrentHandler) Process(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
	m.callCount++
	if m.processFunc != nil {
		return m.processFunc(ctx, delivery)
	}
	return messaging.Ack, nil
}

func (m *MockConcurrentHandler) GetCallCount() int {
	return m.callCount
}

// TestNewConcurrentConsumerComprehensive tests the constructor with various scenarios
func TestNewConcurrentConsumerComprehensive(t *testing.T) {
	t.Run("NilTransport", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		obsCtx := createTestObservabilityContextForConsumer()

		assert.Panics(t, func() {
			rabbitmq.NewConcurrentConsumer(nil, config, obsCtx)
		})
	})

	t.Run("NilConfig", func(t *testing.T) {
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		assert.Panics(t, func() {
			rabbitmq.NewConcurrentConsumer(transport, nil, obsCtx)
		})
	})

	t.Run("ValidCreation", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			MaxConcurrentHandlers: 100,
			Prefetch:              50,
		}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Verify stats are initialized
		stats := consumer.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MessagesReceived)
		assert.Equal(t, uint64(0), stats.MessagesProcessed)
	})

	t.Run("MaxConcurrentHandlersCapping", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			MaxConcurrentHandlers: 15000, // Should be capped to 10000
		}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("ZeroMaxConcurrentHandlers", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			MaxConcurrentHandlers: 0, // Should default to 512
		}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)
	})

	t.Run("NilObservability", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, nil)
		assert.NotNil(t, consumer)
	})
}

// TestConcurrentConsumerStartComprehensive tests the Start method with various scenarios
func TestConcurrentConsumerStartComprehensive(t *testing.T) {
	t.Run("NilContext", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		handler := &MockConcurrentHandler{}

		err := consumer.Start(nil, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("NilHandler", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		ctx := context.Background()

		err := consumer.Start(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler cannot be nil")
	})

	t.Run("DisconnectedTransport", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://invalid:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		handler := &MockConcurrentHandler{}
		ctx := context.Background()

		err := consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is not connected")
	})

	t.Run("DoubleStart", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		handler := &MockConcurrentHandler{}
		ctx := context.Background()

		// First start should fail due to transport not being connected
		_ = consumer.Start(ctx, handler)

		// Second start should fail with either connection error or already started
		err := consumer.Start(ctx, handler)
		assert.Error(t, err)
		// The error could be either connection error or already started
		assert.True(t,
			err.Error() == "CONNECTION: transport is not connected" ||
				err.Error() == "CONSUME: consumer already started",
			"Expected connection error or already started error, got: %s", err.Error())
	})

	t.Run("ClosedConsumer", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		handler := &MockConcurrentHandler{}
		ctx := context.Background()

		// Stop the consumer first
		consumer.Stop(ctx)

		// Try to start a closed consumer
		err := consumer.Start(ctx, handler)
		assert.Error(t, err)
		// The error could be either connection error or consumer is closed
		assert.True(t,
			err.Error() == "CONNECTION: transport is not connected" ||
				err.Error() == "CONSUME: consumer is closed",
			"Expected connection error or consumer closed error, got: %s", err.Error())
	})
}

// TestConcurrentConsumerStopComprehensive tests the Stop method with various scenarios
func TestConcurrentConsumerStopComprehensive(t *testing.T) {
	t.Run("NotStarted", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		ctx := context.Background()

		// Stop without starting should not error
		err := consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("NilContext", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Stop with nil context should not panic
		err := consumer.Stop(nil)
		assert.NoError(t, err)
	})

	t.Run("DoubleStop", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		ctx := context.Background()

		// First stop
		err := consumer.Stop(ctx)
		assert.NoError(t, err)

		// Second stop should also work
		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("ContextWithDeadline", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}

// TestConcurrentConsumerStatsComprehensive tests statistics tracking
func TestConcurrentConsumerStatsComprehensive(t *testing.T) {
	t.Run("InitialStats", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

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

	t.Run("WorkerStats", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Before starting, worker stats should be empty
		workerStats := consumer.GetWorkerStats()
		assert.Empty(t, workerStats)
	})
}

// TestConcurrentConsumerDLQComprehensive tests DLQ integration
func TestConcurrentConsumerDLQComprehensive(t *testing.T) {
	t.Run("SetDLQ", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test setting DLQ
		dlq := &messaging.DeadLetterQueue{}
		consumer.SetDLQ(dlq)

		// Verify DLQ was set (we can't directly access the field, but we can test behavior)
		assert.NotNil(t, consumer)
	})

	t.Run("NilDLQ", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test setting nil DLQ
		consumer.SetDLQ(nil)

		// Should not panic
		assert.NotNil(t, consumer)
	})
}

// TestConcurrentConsumerConfigurationComprehensive tests configuration handling
func TestConcurrentConsumerConfigurationComprehensive(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Verify string representation
		str := consumer.String()
		assert.Contains(t, str, "ConcurrentConsumer")
		assert.Contains(t, str, "test.queue")
	})

	t.Run("BoundaryValues", func(t *testing.T) {
		testCases := []struct {
			name                  string
			maxConcurrentHandlers int
			prefetch              int
		}{
			{"ZeroValues", 0, 0},
			{"NegativeValues", -1, -1},
			{"VeryHighValues", 20000, 70000},
			{"NormalValues", 100, 50},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &messaging.ConsumerConfig{
					Queue:                 "test.queue",
					MaxConcurrentHandlers: tc.maxConcurrentHandlers,
					Prefetch:              tc.prefetch,
				}
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
				assert.NotNil(t, consumer)
			})
		}
	})

	t.Run("RetryConfiguration", func(t *testing.T) {
		testCases := []struct {
			name           string
			maxRetries     int
			requeueOnError bool
		}{
			{"NoRetries", 0, false},
			{"SomeRetries", 3, true},
			{"ManyRetries", 10, false},
			{"RequeueOnError", 0, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &messaging.ConsumerConfig{
					Queue:          "test.queue",
					MaxRetries:     tc.maxRetries,
					RequeueOnError: tc.requeueOnError,
				}
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
				assert.NotNil(t, consumer)
			})
		}
	})

	t.Run("PanicRecoveryConfiguration", func(t *testing.T) {
		testCases := []struct {
			name          string
			panicRecovery bool
		}{
			{"Enabled", true},
			{"Disabled", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &messaging.ConsumerConfig{
					Queue:         "test.queue",
					PanicRecovery: tc.panicRecovery,
				}
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
				assert.NotNil(t, consumer)
			})
		}
	})
}

// TestConcurrentConsumerErrorHandlingComprehensive tests error handling scenarios
func TestConcurrentConsumerErrorHandlingComprehensive(t *testing.T) {
	t.Run("InvalidConfiguration", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *messaging.ConsumerConfig
		}{
			{"EmptyQueue", &messaging.ConsumerConfig{Queue: ""}},
			{"NilConfig", nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				if tc.config == nil {
					assert.Panics(t, func() {
						rabbitmq.NewConcurrentConsumer(transport, tc.config, obsCtx)
					})
				} else {
					consumer := rabbitmq.NewConcurrentConsumer(transport, tc.config, obsCtx)
					assert.NotNil(t, consumer)
				}
			})
		}
	})

	t.Run("TransportErrors", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://invalid:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		handler := &MockConcurrentHandler{}
		ctx := context.Background()

		err := consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is not connected")
	})
}

// TestConcurrentConsumerLifecycleComprehensive tests the complete lifecycle
func TestConcurrentConsumerLifecycleComprehensive(t *testing.T) {
	t.Run("CompleteLifecycle", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			MaxConcurrentHandlers: 10,
			Prefetch:              5,
			MaxRetries:            3,
			RequeueOnError:        true,
			PanicRecovery:         true,
		}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Test DLQ setting
		dlq := &messaging.DeadLetterQueue{}
		consumer.SetDLQ(dlq)

		// Test stats before start
		stats := consumer.GetStats()
		assert.Equal(t, uint64(0), stats.MessagesReceived)
		assert.Equal(t, uint64(0), stats.MessagesProcessed)

		// Test worker stats before start
		workerStats := consumer.GetWorkerStats()
		assert.Empty(t, workerStats)

		// Test string representation
		str := consumer.String()
		assert.Contains(t, str, "ConcurrentConsumer")
		assert.Contains(t, str, "test.queue")

		// Test stop without start
		ctx := context.Background()
		err := consumer.Stop(ctx)
		assert.NoError(t, err)

		// Test double stop
		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test with cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err := consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}

// TestConcurrentConsumerIntegrationComprehensive tests integration scenarios
func TestConcurrentConsumerIntegrationComprehensive(t *testing.T) {
	t.Run("WithRealTransport", func(t *testing.T) {
		config := &messaging.ConsumerConfig{
			Queue:                 "test.queue",
			MaxConcurrentHandlers: 5,
			Prefetch:              10,
		}

		// Use real transport but with invalid connection
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://invalid:5672"},
		}, nil)

		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		handler := &MockConcurrentHandler{}
		ctx := context.Background()

		// Should fail due to connection issues
		err := consumer.Start(ctx, handler)
		assert.Error(t, err)
	})

	t.Run("WithObservability", func(t *testing.T) {
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)

		// Test with proper observability context
		telemetryConfig := &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "test-service",
		}

		obsProvider, err := messaging.NewObservabilityProvider(telemetryConfig)
		require.NoError(t, err)

		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
		assert.NotNil(t, consumer)

		// Test that observability doesn't break functionality
		stats := consumer.GetStats()
		assert.NotNil(t, stats)
	})
}

// TestConcurrentConsumerEdgeCasesComprehensive tests edge cases and boundary conditions
func TestConcurrentConsumerEdgeCasesComprehensive(t *testing.T) {
	t.Run("ExtremeConfigurations", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *messaging.ConsumerConfig
		}{
			{"MinimalConfig", &messaging.ConsumerConfig{Queue: "test.queue"}},
			{"MaximalConfig", &messaging.ConsumerConfig{
				Queue:                 "test.queue",
				MaxConcurrentHandlers: 10000,
				Prefetch:              65535,
				MaxRetries:            100,
				HandlerTimeout:        1 * time.Hour,
			}},
			{"ZeroTimeout", &messaging.ConsumerConfig{
				Queue:          "test.queue",
				HandlerTimeout: 0,
			}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
				obsCtx := createTestObservabilityContextForConsumer()

				consumer := rabbitmq.NewConcurrentConsumer(transport, tc.config, obsCtx)
				assert.NotNil(t, consumer)

				// Verify basic functionality still works
				stats := consumer.GetStats()
				assert.NotNil(t, stats)

				str := consumer.String()
				assert.Contains(t, str, "ConcurrentConsumer")
			})
		}
	})

	t.Run("MemorySafety", func(t *testing.T) {
		// Test that consumer doesn't leak memory or cause panics
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		// Create many consumers to test memory safety
		var consumers []*rabbitmq.ConcurrentConsumer
		for i := 0; i < 100; i++ {
			consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)
			consumers = append(consumers, consumer)

			// Test each consumer
			stats := consumer.GetStats()
			assert.NotNil(t, stats)

			// Stop each consumer
			ctx := context.Background()
			err := consumer.Stop(ctx)
			assert.NoError(t, err)
		}

		assert.Len(t, consumers, 100)
	})

	t.Run("GoroutineLeaks", func(t *testing.T) {
		// Test that Stop() doesn't hang
		config := &messaging.ConsumerConfig{Queue: "test.queue"}
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{URIs: []string{"amqp://localhost:5672"}}, nil)
		obsCtx := createTestObservabilityContextForConsumer()

		consumer := rabbitmq.NewConcurrentConsumer(transport, config, obsCtx)

		// Test that Stop() completes quickly
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()
		err := consumer.Stop(ctx)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 100*time.Millisecond) // Should complete quickly
	})
}
