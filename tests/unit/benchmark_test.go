package unit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-validatorx"
	"github.com/stretchr/testify/require"
)

// BenchmarkMessage tests message creation and manipulation performance
func BenchmarkMessage(b *testing.B) {
	b.Run("NewMessage", func(b *testing.B) {
		body := make([]byte, 1024)
		for i := range body {
			body[i] = byte(i % 256)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = messaging.NewMessage(body, messaging.WithKey("benchmark.key"))
		}
	})

	b.Run("NewMessageWithOptions", func(b *testing.B) {
		body := make([]byte, 1024)
		for i := range body {
			body[i] = byte(i % 256)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = messaging.NewMessage(
				body,
				messaging.WithID("benchmark-msg"),
				messaging.WithContentType("application/json"),
				messaging.WithKey("benchmark.key"),
				messaging.WithPriority(5),
			)
		}
	})

	b.Run("MessageSerialization", func(b *testing.B) {
		msg := messaging.NewMessage([]byte("test message"), messaging.WithKey("test.key"))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(msg)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})
}

// BenchmarkMockPublisher tests mock publisher performance
func BenchmarkMockPublisher(b *testing.B) {
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)

	// Connect transport
	ctx := context.Background()
	err := transport.Connect(ctx)
	require.NoError(b, err)
	defer transport.Disconnect(ctx)

	publisher := transport.NewPublisher(&messaging.PublisherConfig{
		MaxInFlight: 1000,
		WorkerCount: 4,
	}, nil)

	msg := messaging.NewMessage([]byte("benchmark message"), messaging.WithKey("benchmark.key"))

	b.Run("PublishAsync", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			receipt, err := publisher.PublishAsync(ctx, "benchmark.exchange", *msg)
			if err != nil {
				b.Fatal(err)
			}
			// Wait for completion
			<-receipt.Done()
			_, err = receipt.Result()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PublishAsyncParallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				receipt, err := publisher.PublishAsync(ctx, "benchmark.exchange", *msg)
				if err != nil {
					b.Fatal(err)
				}
				// Wait for completion
				<-receipt.Done()
				_, err = receipt.Result()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkMockConsumer tests mock consumer performance
func BenchmarkMockConsumer(b *testing.B) {
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)

	// Connect transport
	ctx := context.Background()
	err := transport.Connect(ctx)
	require.NoError(b, err)
	defer transport.Disconnect(ctx)

	consumer := transport.NewConsumer(&messaging.ConsumerConfig{
		Queue:                 "benchmark.queue",
		Prefetch:              256,
		MaxConcurrentHandlers: 512,
	}, nil)

	var processedCount int

	b.Run("ConsumeMessages", func(b *testing.B) {
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			processedCount++
			return messaging.Ack, nil
		})

		err := consumer.Start(ctx, handler)
		require.NoError(b, err)
		defer consumer.Stop(ctx)

		// Simulate messages
		mockConsumer := consumer.(*messaging.MockConsumer)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := messaging.NewMessage([]byte("benchmark message"), messaging.WithKey("benchmark.key"))
			mockConsumer.SimulateMessage(*msg)
		}

		// Wait a bit for processing
		time.Sleep(100 * time.Millisecond)
	})
}

// BenchmarkLatencyHistogram tests latency histogram performance
func BenchmarkLatencyHistogram(b *testing.B) {
	histogram := messaging.NewLatencyHistogram()

	b.Run("Record", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			histogram.Record(int64(i * 1000)) // 1µs, 2µs, 3µs, etc.
		}
	})

	// Record some data first
	for i := 0; i < 10000; i++ {
		histogram.Record(int64(i * 1000))
	}

	b.Run("Percentile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = histogram.Percentile(95)
		}
	})
}

// BenchmarkPerformanceMonitor tests performance monitor functionality
func BenchmarkPerformanceMonitor(b *testing.B) {
	config := &messaging.PerformanceConfig{
		EnableObjectPooling:        true,
		ObjectPoolSize:             1000,
		EnableMemoryProfiling:      true,
		PerformanceMetricsInterval: 1 * time.Second,
	}

	// Create observability context
	telemetryConfig := &messaging.TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: false,
		ServiceName:    "benchmark-test",
	}

	provider, err := messaging.NewObservabilityProvider(telemetryConfig)
	require.NoError(b, err)

	ctx := context.Background()
	obsCtx := messaging.NewObservabilityContext(ctx, provider)

	monitor := messaging.NewPerformanceMonitor(config, obsCtx)
	defer monitor.Close(ctx)

	b.Run("RecordPublish", func(b *testing.B) {
		duration := 1 * time.Millisecond
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.RecordPublish(duration, true)
		}
	})

	b.Run("RecordConsume", func(b *testing.B) {
		duration := 1 * time.Millisecond
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.RecordConsume(duration, true)
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		// Record some data first
		for i := 0; i < 100; i++ {
			monitor.RecordPublish(time.Millisecond, true)
			monitor.RecordConsume(time.Millisecond, true)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = monitor.GetMetrics()
		}
	})
}

// BenchmarkValidator tests validation performance
func BenchmarkValidator(b *testing.B) {
	validator := validatorx.NewValidator()

	// Test struct for validation
	type TestStruct struct {
		Name     string `validate:"required"`
		Email    string `validate:"email"`
		Age      int    `validate:"min:18"`
		URL      string `validate:"url"`
		Category string `validate:"oneof:admin user guest"`
	}

	validStruct := TestStruct{
		Name:     "John Doe",
		Email:    "john@example.com",
		Age:      25,
		URL:      "https://example.com",
		Category: "user",
	}

	b.Run("ValidateStruct", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := validator.ValidateStruct(validStruct)
			if !result.Valid {
				b.Fatal("validation should pass")
			}
		}
	})

	b.Run("ValidateStructParallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result := validator.ValidateStruct(validStruct)
				if !result.Valid {
					b.Fatal("validation should pass")
				}
			}
		})
	})
}

// BenchmarkCodec tests codec performance
func BenchmarkCodec(b *testing.B) {
	testData := map[string]interface{}{
		"name":   "John Doe",
		"age":    30,
		"active": true,
		"scores": []int{95, 87, 92},
		"metadata": map[string]string{
			"region": "us-west",
			"tier":   "premium",
		},
	}

	b.Run("JSONCodec", func(b *testing.B) {
		codec, ok := messaging.GetCodec("json")
		require.True(b, ok)

		b.Run("Encode", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := codec.Encode(testData)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		// Encode data for decode benchmark
		encoded, err := codec.Encode(testData)
		require.NoError(b, err)

		b.Run("Decode", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var result map[string]interface{}
				err := codec.Decode(encoded, &result)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkObservability tests observability performance
func BenchmarkObservability(b *testing.B) {
	config := &messaging.TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: true,
		ServiceName:    "benchmark-test",
	}

	provider, err := messaging.NewObservabilityProvider(config)
	require.NoError(b, err)
	// Note: ObservabilityProvider doesn't have Close method in current implementation

	ctx := context.Background()
	obsCtx := messaging.NewObservabilityContext(ctx, provider)

	b.Run("RecordPublishMetrics", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			obsCtx.RecordPublishMetrics("benchmark_operation", "test.exchange", 1*time.Millisecond, true, "")
		}
	})

	b.Run("RecordConsumeMetrics", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			obsCtx.RecordConsumeMetrics("benchmark_operation", "test.queue", 1*time.Millisecond, true, "")
		}
	})
}

// BenchmarkErrorHandling tests error handling performance
func BenchmarkErrorHandling(b *testing.B) {
	b.Run("NewError", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = messaging.NewError(messaging.ErrorCodeInternal, "benchmark", "test error")
		}
	})

	b.Run("WrapError", func(b *testing.B) {
		originalErr := messaging.NewError(messaging.ErrorCodeInternal, "original", "original error")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = messaging.WrapError(messaging.ErrorCodeInternal, "wrap", "wrapped error", originalErr)
		}
	})
}

// BenchmarkHealthManager tests health manager performance
func BenchmarkHealthManager(b *testing.B) {
	healthManager := messaging.NewHealthManager(5 * time.Second)

	// Add some health checks
	healthManager.RegisterFunc("test1", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Status:  messaging.HealthStatusHealthy,
			Message: "test1 is healthy",
		}
	})
	healthManager.RegisterFunc("test2", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Status:  messaging.HealthStatusHealthy,
			Message: "test2 is healthy",
		}
	})
	healthManager.RegisterFunc("test3", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Status:  messaging.HealthStatusHealthy,
			Message: "test3 is healthy",
		}
	})

	ctx := context.Background()

	b.Run("GetHealthReport", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = healthManager.GenerateReport(ctx)
		}
	})

	b.Run("GetHealthReportParallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = healthManager.GenerateReport(ctx)
			}
		})
	})
}

// BenchmarkTestMessageFactory tests test message factory performance
func BenchmarkTestMessageFactory(b *testing.B) {
	factory := messaging.NewTestMessageFactory()

	b.Run("CreateMessage", func(b *testing.B) {
		body := []byte("benchmark test message")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = factory.CreateMessage(body)
		}
	})

	b.Run("CreateJSONMessage", func(b *testing.B) {
		data := "benchmark json data"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = factory.CreateJSONMessage(data)
		}
	})

	b.Run("CreateBulkMessages", func(b *testing.B) {
		template := `{"id": %d, "data": "benchmark message %d"}`
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = factory.CreateBulkMessages(10, template)
		}
	})
}
