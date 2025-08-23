// Package main demonstrates publisher confirms functionality in go-messagex.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

func main() {
	fmt.Println("🐰 Publisher Confirms Demo")
	fmt.Println("==========================")

	// Example 1: Publisher with confirms enabled
	demonstratePublisherWithConfirms()

	// Example 2: Publisher with confirms disabled
	demonstratePublisherWithoutConfirms()

	// Example 3: Advanced publisher with confirms
	demonstrateAdvancedPublisherConfirms()

	// Example 4: Receipt handling
	demonstrateReceiptHandling()

	fmt.Println("✅ Demo completed successfully!")
}

func demonstratePublisherWithConfirms() {
	fmt.Println("\n📝 Example 1: Publisher with Confirms Enabled")
	fmt.Println("----------------------------------------------")

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
		log.Printf("Failed to create observability provider: %v", err)
		return
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create transport
	transport := rabbitmq.NewTransport(config, obsCtx)
	fmt.Printf("✅ Created transport (connection pooling enabled)\n")

	// Create publisher
	publisher := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)
	fmt.Printf("✅ Created publisher (confirms enabled: %v)\n", config.Publisher.Confirms)

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

	fmt.Printf("📤 Publishing message: %s\n", message.ID)

	// Note: In a real environment with RabbitMQ running, this would:
	// 1. Enable confirm mode on the channel
	// 2. Set up NotifyPublish and NotifyReturn handlers
	// 3. Track the message for confirmation
	// 4. Return a receipt that completes when broker confirms
	receipt, err := publisher.PublishAsync(ctx, "demo.exchange", message)
	if err != nil {
		log.Printf("❌ Failed to publish message: %v", err)
	} else {
		fmt.Printf("🎫 Received receipt for message: %s\n", receipt.ID())

		// In a real environment, you would wait for confirmation:
		// select {
		// case <-receipt.Done():
		//     result, err := receipt.Result()
		//     if err != nil {
		//         fmt.Printf("❌ Message failed: %v\n", err)
		//     } else {
		//         fmt.Printf("✅ Message confirmed: delivery tag %d\n", result.DeliveryTag)
		//     }
		// case <-time.After(10 * time.Second):
		//     fmt.Printf("⏰ Confirmation timeout\n")
		// }
	}

	// Close publisher
	publisher.Close(ctx)
	fmt.Printf("🔒 Publisher closed\n")
}

func demonstratePublisherWithoutConfirms() {
	fmt.Println("\n📝 Example 2: Publisher with Confirms Disabled")
	fmt.Println("-----------------------------------------------")

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
		log.Printf("Failed to create observability provider: %v", err)
		return
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	transport := rabbitmq.NewTransport(config, obsCtx)
	publisher := rabbitmq.NewPublisher(transport, config.Publisher, obsCtx)

	fmt.Printf("✅ Created publisher (confirms disabled)\n")
	fmt.Printf("⚡ Receipts complete immediately after publish (fire-and-forget)\n")

	publisher.Close(context.Background())
	fmt.Printf("🔒 Publisher closed\n")
}

func demonstrateAdvancedPublisherConfirms() {
	fmt.Println("\n📝 Example 3: Advanced Publisher with Confirms")
	fmt.Println("----------------------------------------------")

	config := &messaging.PublisherConfig{
		Confirms:       true,
		Mandatory:      true,
		MaxInFlight:    5000,
		PublishTimeout: 3 * time.Second,
	}

	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		log.Printf("Failed to create observability provider: %v", err)
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
	fmt.Printf("✅ Created advanced publisher with confirms\n")
	fmt.Printf("🚀 Supports: persistence, transformation, routing + confirms\n")

	publisher.Close(context.Background())
	fmt.Printf("🔒 Advanced publisher closed\n")
}

func demonstrateReceiptHandling() {
	fmt.Println("\n📝 Example 4: Receipt Handling")
	fmt.Println("------------------------------")

	// Create a receipt manager (used internally by publishers)
	manager := messaging.NewReceiptManager(30 * time.Second)
	fmt.Printf("✅ Created receipt manager\n")

	// Create a receipt for tracking
	ctx := context.Background()
	receipt := manager.CreateReceipt(ctx, "test-message-123")
	fmt.Printf("🎫 Created receipt for message: %s\n", receipt.ID())

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
			fmt.Printf("✅ Receipt completed successfully\n")
		}
	}()

	// Wait for receipt completion
	select {
	case <-receipt.Done():
		result, err := receipt.Result()
		if err != nil {
			fmt.Printf("❌ Receipt completed with error: %v\n", err)
		} else {
			fmt.Printf("🎉 Message confirmed! Delivery tag: %d, Timestamp: %v\n",
				result.DeliveryTag, result.Timestamp.Format(time.RFC3339))
		}
	case <-time.After(5 * time.Second):
		fmt.Printf("⏰ Receipt timeout\n")
	}

	fmt.Printf("📊 Pending receipts: %d\n", manager.PendingCount())

	manager.Close()
	fmt.Printf("🔒 Receipt manager closed\n")
}
