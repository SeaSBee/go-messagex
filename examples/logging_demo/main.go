package main

import (
	"context"
	"fmt"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

func loggingDemo() {
	fmt.Println("Go-LogX Integration Demo")
	fmt.Println("========================")

	// Initialize go-logx with default configuration
	if err := logx.InitDefault(); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logx.Sync()

	// Create a custom logger with fields
	_, err := logx.NewLogger()
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	// Add some sensitive keys that should be masked
	logx.AddSensitiveKey("password")
	logx.AddSensitiveKey("secret")

	// Demonstrate basic logging
	fmt.Println("\n1. Basic Logging:")
	logx.Info("Application started",
		logx.String("version", "1.0.0"),
		logx.String("environment", "development"),
	)

	// Demonstrate structured logging with fields
	fmt.Println("\n2. Structured Logging:")
	logx.Info("Message published",
		logx.String("transport", "rabbitmq"),
		logx.String("exchange", "orders.exchange"),
		logx.String("routing_key", "order.created"),
		logx.Int("message_size", 1024),
		logx.Float64("latency_ms", 15.5),
		logx.Bool("success", true),
	)

	// Demonstrate error logging
	fmt.Println("\n3. Error Logging:")
	logx.Error("Failed to publish message",
		logx.String("transport", "rabbitmq"),
		logx.String("exchange", "orders.exchange"),
		logx.String("error_type", "connection_timeout"),
		logx.Int("retry_attempt", 3),
		logx.Float64("timeout_seconds", 30.0),
	)

	// Demonstrate sensitive data masking
	fmt.Println("\n4. Sensitive Data Masking:")
	logx.Info("User authentication",
		logx.String("username", "john.doe"),
		logx.String("password", "secret123"), // This will be masked
		logx.String("secret", "api-key-123"), // This will be masked
		logx.String("email", "john@example.com"),
	)

	// Demonstrate messaging integration
	fmt.Println("\n5. Messaging Integration:")
	demoMessagingIntegration()

	// Demonstrate performance logging
	fmt.Println("\n6. Performance Logging:")
	demoPerformanceLogging()

	fmt.Println("\nLogging demo completed!")
}

func main() {
	loggingDemo()
}

func demoMessagingIntegration() {
	// Create observability provider with go-logx logger
	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: true,
	})
	if err != nil {
		logx.Error("Failed to create observability provider", logx.ErrorField(err))
		return
	}

	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Simulate message operations
	msg := messaging.NewMessage([]byte(`{"order_id": "12345", "amount": 99.99}`),
		messaging.WithID("msg-123"),
		messaging.WithContentType("application/json"),
		messaging.WithKey("order.created"),
	)

	// Log message creation
	obsCtx.Logger().Info("Message created",
		logx.String("message_id", msg.ID),
		logx.String("content_type", msg.ContentType),
		logx.String("routing_key", msg.Key),
		logx.Int("body_size", len(msg.Body)),
	)

	// Simulate publishing
	start := time.Now()
	time.Sleep(10 * time.Millisecond) // Simulate network delay
	duration := time.Since(start)

	// Log publish metrics
	obsCtx.RecordPublishMetrics("publish", "orders.exchange", duration, true, "")
	obsCtx.Logger().Info("Message published successfully",
		logx.String("message_id", msg.ID),
		logx.String("exchange", "orders.exchange"),
		logx.String("duration", duration.String()),
		logx.Bool("success", true),
	)
}

func demoPerformanceLogging() {
	// Simulate performance monitoring
	operations := []struct {
		name     string
		duration time.Duration
		success  bool
	}{
		{"publish", 15 * time.Millisecond, true},
		{"consume", 8 * time.Millisecond, true},
		{"publish", 45 * time.Millisecond, false},
		{"consume", 12 * time.Millisecond, true},
		{"publish", 22 * time.Millisecond, true},
	}

	for _, op := range operations {
		status := "success"
		if !op.success {
			status = "failed"
		}

		logx.Info("Operation completed",
			logx.String("operation", op.name),
			logx.String("status", status),
			logx.String("duration", op.duration.String()),
			logx.Float64("duration_ms", float64(op.duration.Microseconds())/1000.0),
		)

		// Log warnings for slow operations
		if op.duration > 30*time.Millisecond {
			logx.Warn("Slow operation detected",
				logx.String("operation", op.name),
				logx.String("duration", op.duration.String()),
				logx.String("threshold", (30*time.Millisecond).String()),
			)
		}

		// Log errors for failed operations
		if !op.success {
			logx.Error("Operation failed",
				logx.String("operation", op.name),
				logx.String("duration", op.duration.String()),
				logx.String("error_type", "timeout"),
			)
		}
	}

	// Log summary statistics
	logx.Info("Performance summary",
		logx.Int("total_operations", len(operations)),
		logx.Int("successful_operations", 4),
		logx.Int("failed_operations", 1),
		logx.Float64("success_rate", 80.0),
		logx.String("avg_duration", (20*time.Millisecond).String()),
	)
}
