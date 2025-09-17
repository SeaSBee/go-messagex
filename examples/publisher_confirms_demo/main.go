// Package main demonstrates publisher confirms functionality in go-messagex.
package main

import (
	"context"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

func main() {
	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logx.Sync()

	logx.Info("🐰 Publisher Confirms Demo")
	logx.Info("==========================")

	// Example 1: Publisher with confirms enabled
	demonstratePublisherWithConfirms()

	// Example 2: Publisher with confirms disabled
	demonstratePublisherWithoutConfirms()

	// Example 3: Advanced publisher with confirms
	demonstrateAdvancedPublisherConfirms()

	// Example 4: Receipt handling
	demonstrateReceiptHandling()

	logx.Info("✅ Demo completed successfully!")
}

func demonstratePublisherWithConfirms() {
	logx.Info("📝 Example 1: Publisher with Confirms Enabled")
	logx.Info("----------------------------------------------")

	// Configure RabbitMQ with publisher confirms enabled
	config := &messaging.RabbitMQConfig{
		URIs: []string{"amqp://localhost:5672"},
		ConnectionPool: &messaging.ConnectionPoolConfig{
			Min:                 2,
			Max:                 8,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		},
		Publisher: &messaging.PublisherConfig{
			Confirms:       true, // ✅ Enable publisher confirms
			Mandatory:      true, // ✅ Ensure messages are routable
			Immediate:      false,
			MaxInFlight:    1000,
			DropOnOverflow: false,
			PublishTimeout: 5 * time.Second, // Receipt timeout
			WorkerCount:    4,
		},
	}

	// Create observability context
	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		logx.Error("Failed to create observability provider", logx.ErrorField(err))
		return
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create transport
	transport := rabbitmq.NewTransport(config, obsCtx)
	logx.Info("✅ Created transport", logx.String("connection_pooling", "enabled"))

	// Create publisher
	publisher, err := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)
	if err != nil {
		logx.Error("Failed to create publisher", logx.ErrorField(err))
		return
	}
	logx.Info("✅ Created publisher", logx.Bool("confirms_enabled", config.Publisher.Confirms))

	// Demonstrate message publishing with confirms
	ctx := context.Background()
	message := messaging.Message{
		ID:          "demo-message-1",
		Key:         "demo.key",
		Body:        []byte("Hello, Publisher Confirms!"),
		ContentType: "text/plain",
		Priority:    5,
		Headers: map[string]string{
			"demo":    "true",
			"version": "1.0",
		},
	}

	logx.Info("📤 Publishing message", logx.String("message_id", message.ID))

	// Note: In a real environment with RabbitMQ running, this would:
	// 1. Enable confirm mode on the channel
	// 2. Set up NotifyPublish and NotifyReturn handlers
	// 3. Track the message for confirmation
	// 4. Return a receipt that completes when broker confirms
	receipt, err := publisher.PublishAsync(ctx, "demo.exchange", message)
	if err != nil {
		logx.Error("❌ Failed to publish message", logx.ErrorField(err), logx.String("message_id", message.ID))
	} else {
		logx.Info("🎫 Received receipt for message", logx.String("receipt_id", receipt.ID()))

		// In a real environment, you would wait for confirmation:
		// select {
		// case <-receipt.Done():
		//     result, err := receipt.Result()
		//     if err != nil {
		//         logx.Error("❌ Message failed", logx.ErrorField(err))
		//     } else {
		//         logx.Info("✅ Message confirmed", logx.Int("delivery_tag", result.DeliveryTag))
		//     }
		// case <-time.After(10 * time.Second):
		//     logx.Warn("⏰ Confirmation timeout")
		// }
	}

	// Close publisher
	publisher.Close(ctx)
	logx.Info("🔒 Publisher closed")
}

func demonstratePublisherWithoutConfirms() {
	logx.Info("📝 Example 2: Publisher with Confirms Disabled")
	logx.Info("-----------------------------------------------")

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
			Confirms:       false, // ❌ Disable publisher confirms
			Mandatory:      false,
			MaxInFlight:    1000,
			PublishTimeout: 2 * time.Second,
		},
	}

	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		logx.Error("Failed to create observability provider", logx.ErrorField(err))
		return
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	transport := rabbitmq.NewTransport(config, obsCtx)
	publisher, err := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)
	if err != nil {
		logx.Error("Failed to create publisher", logx.ErrorField(err))
		return
	}

	logx.Info("✅ Created publisher", logx.Bool("confirms_disabled", true))
	logx.Info("⚡ Receipts complete immediately after publish", logx.String("mode", "fire-and-forget"))

	publisher.Close(context.Background())
	logx.Info("🔒 Publisher closed")
}

func demonstrateAdvancedPublisherConfirms() {
	logx.Info("📝 Example 3: Advanced Publisher with Confirms")
	logx.Info("----------------------------------------------")

	config := &messaging.PublisherConfig{
		Confirms:       true,
		Mandatory:      true,
		MaxInFlight:    5000,
		PublishTimeout: 3 * time.Second,
	}

	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		logx.Error("Failed to create observability provider", logx.ErrorField(err))
		return
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	rabbitmqConfig := &messaging.RabbitMQConfig{
		URIs: []string{"amqp://localhost:5672"},
		ConnectionPool: &messaging.ConnectionPoolConfig{
			Min:                 2,
			Max:                 10,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		},
	}
	transport := rabbitmq.NewTransport(rabbitmqConfig, obsCtx)

	// Create advanced publisher
	publisher := rabbitmq.NewAdvancedPublisher(transport, config, obsCtx)
	logx.Info("✅ Created advanced publisher", logx.Bool("confirms_enabled", true))
	logx.Info("🚀 Advanced features",
		logx.Bool("persistence", true),
		logx.Bool("transformation", true),
		logx.Bool("routing", true),
		logx.Bool("confirms", true))

	publisher.Close(context.Background())
	logx.Info("🔒 Advanced publisher closed")
}

func demonstrateReceiptHandling() {
	logx.Info("📝 Example 4: Receipt Handling")
	logx.Info("------------------------------")

	// Create a receipt manager (used internally by publishers)
	manager := messaging.NewReceiptManager(30 * time.Second)
	logx.Info("✅ Created receipt manager")

	// Create a receipt for tracking
	ctx := context.Background()
	receipt := manager.CreateReceipt(ctx, "test-message-123")
	logx.Info("🎫 Created receipt for message", logx.String("message_id", receipt.ID()))

	// Simulate successful confirmation
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulate network delay

		result := messaging.PublishResult{
			MessageID:   "test-message-123",
			DeliveryTag: 456,
			Timestamp:   time.Now(),
			Success:     true,
			Reason:      "",
		}

		completed := manager.CompleteReceipt("test-message-123", result, nil)
		if completed {
			logx.Info("✅ Receipt completed successfully")
		}
	}()

	// Wait for receipt completion
	select {
	case <-receipt.Done():
		result, err := receipt.Result()
		if err != nil {
			logx.Error("❌ Receipt completed with error", logx.ErrorField(err))
		} else {
			logx.Info("🎉 Message confirmed",
				logx.Int64("delivery_tag", int64(result.DeliveryTag)),
				logx.String("timestamp", result.Timestamp.Format(time.RFC3339)))
		}
	case <-time.After(5 * time.Second):
		logx.Warn("⏰ Receipt timeout")
	}

	logx.Info("📊 Receipt statistics", logx.Int("pending_receipts", manager.PendingCount()))

	manager.Close()
	logx.Info("🔒 Receipt manager closed")
}
