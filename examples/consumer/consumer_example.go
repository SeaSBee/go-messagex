package main

import (
	"context"
	"fmt"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
	fmt.Println("=== RabbitMQ Consumer Example ===")

	// Create a logger (mandatory)
	logger, err := logx.NewLogger()
	if err != nil {
		logx.Fatal("Failed to create logger", logx.ErrorField(err))
	}

	// Create a new RabbitMQ client with default configuration
	client, err := messaging.NewClient(nil, logger)
	if err != nil {
		logx.Fatal("Failed to create client", logx.ErrorField(err))
	}
	defer client.Close()

	// Wait for connection to be established
	if err := client.WaitForConnection(30 * time.Second); err != nil {
		logx.Fatal("Failed to connect", logx.ErrorField(err))
	}

	fmt.Println("✓ Connected to RabbitMQ successfully!")

	// Get the consumer
	consumer := client.GetConsumer()
	if consumer == nil {
		logx.Fatal("Failed to get consumer")
	}

	ctx := context.Background()

	// Example 1: Simple message handler
	fmt.Println("\n📥 Starting simple message consumer...")
	simpleHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received simple message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Content Type: %s\n", delivery.Message.ContentType)
		fmt.Printf("   Timestamp: %s\n", delivery.Message.Timestamp.Format(time.RFC3339))
		fmt.Printf("   Priority: %d\n", delivery.Message.Priority)
		fmt.Printf("   Headers: %+v\n", delivery.Message.Headers)
		fmt.Printf("   Properties: %+v\n", delivery.Message.Properties)

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from test queue
	if err := client.Consume(ctx, "test.queue", simpleHandler); err != nil {
		logx.Error("❌ Failed to start consuming from test.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from test.queue")
	}

	// Example 2: JSON message handler
	fmt.Println("\n📥 Starting JSON message consumer...")
	jsonHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received JSON message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Content Type: %s\n", delivery.Message.ContentType)

		// Try to parse JSON if it's a JSON message
		if delivery.Message.ContentType == "application/json" {
			fmt.Printf("   Parsed JSON: %s\n", string(delivery.Message.Body))
		}

		// Check for specific headers
		if messageType, exists := delivery.Message.GetHeader("message_type"); exists {
			fmt.Printf("   Message Type: %v\n", messageType)
		}

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from user queue
	if err := client.Consume(ctx, "user.queue", jsonHandler); err != nil {
		logx.Error("❌ Failed to start consuming from user.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from user.queue")
	}

	// Example 3: Priority message handler
	fmt.Println("\n📥 Starting priority message consumer...")
	priorityHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received priority message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Priority: %d\n", delivery.Message.Priority)

		// Handle different priority levels
		switch delivery.Message.Priority {
		case messaging.PriorityCritical:
			fmt.Printf("   🚨 CRITICAL priority - processing immediately\n")
		case messaging.PriorityHigh:
			fmt.Printf("   ⚡ HIGH priority - processing with priority\n")
		case messaging.PriorityNormal:
			fmt.Printf("   📋 NORMAL priority - standard processing\n")
		case messaging.PriorityLow:
			fmt.Printf("   🐌 LOW priority - background processing\n")
		}

		// Simulate processing time based on priority
		processingTime := time.Duration(4-delivery.Message.Priority) * 100 * time.Millisecond
		time.Sleep(processingTime)

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from priority queue
	if err := client.Consume(ctx, "priority.queue", priorityHandler); err != nil {
		logx.Error("❌ Failed to start consuming from priority.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from priority.queue")
	}

	// Example 4: Batch message handler
	fmt.Println("\n📥 Starting batch message consumer...")
	batchHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received batch message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))

		// Check for batch-specific headers
		if batchID, exists := delivery.Message.GetHeader("batch_id"); exists {
			fmt.Printf("   Batch ID: %v\n", batchID)
		}
		if messageIndex, exists := delivery.Message.GetHeader("message_index"); exists {
			fmt.Printf("   Message Index: %v\n", messageIndex)
		}
		if totalMessages, exists := delivery.Message.GetHeader("total_messages"); exists {
			fmt.Printf("   Total Messages: %v\n", totalMessages)
		}

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from batch queue
	if err := client.Consume(ctx, "batch.queue", batchHandler); err != nil {
		logx.Error("❌ Failed to start consuming from batch.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from batch.queue")
	}

	// Example 5: RPC message handler
	fmt.Println("\n📥 Starting RPC message consumer...")
	rpcHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received RPC message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Correlation ID: %s\n", delivery.Message.Properties.CorrelationID)
		fmt.Printf("   Reply To: %s\n", delivery.Message.Properties.ReplyTo)

		// Check for RPC-specific headers
		if messageType, exists := delivery.Message.GetHeader("message_type"); exists {
			fmt.Printf("   Message Type: %v\n", messageType)
		}
		if requestID, exists := delivery.Message.GetHeader("request_id"); exists {
			fmt.Printf("   Request ID: %v\n", requestID)
		}

		// Simulate RPC processing
		time.Sleep(100 * time.Millisecond)

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from RPC request queue
	if err := client.Consume(ctx, "rpc.request", rpcHandler); err != nil {
		logx.Error("❌ Failed to start consuming from rpc.request", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from rpc.request")
	}

	// Start consuming from RPC response queue
	if err := client.Consume(ctx, "rpc.response", rpcHandler); err != nil {
		logx.Error("❌ Failed to start consuming from rpc.response", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from rpc.response")
	}

	// Example 6: Error handling and retry logic
	fmt.Println("\n📥 Starting error handling consumer...")
	errorHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received message for error handling:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Redelivered: %v\n", delivery.Redelivered)

		// Simulate occasional processing errors
		if delivery.Message.ID == "error-test" {
			fmt.Printf("   ❌ Simulating processing error for message: %s\n", delivery.Message.ID)
			// Nack with requeue to retry the message
			return delivery.Acknowledger.Nack(true)
		}

		// Simulate processing
		time.Sleep(50 * time.Millisecond)

		// Acknowledge successful processing
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from custom queue
	if err := client.Consume(ctx, "custom.queue", errorHandler); err != nil {
		logx.Error("❌ Failed to start consuming from custom.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from custom.queue")
	}

	// Example 7: TTL and expiration message handler
	fmt.Println("\n📥 Starting TTL/expiration message consumer...")
	ttlHandler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received TTL/expiration message:\n")
		fmt.Printf("   ID: %s\n", delivery.Message.ID)
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   TTL: %v\n", delivery.Message.Properties.TTL)
		fmt.Printf("   Expiration: %v\n", delivery.Message.Properties.Expiration)

		// Check for TTL-specific headers
		if messageType, exists := delivery.Message.GetHeader("message_type"); exists {
			fmt.Printf("   Message Type: %v\n", messageType)
		}

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming from TTL queue
	if err := client.Consume(ctx, "ttl.queue", ttlHandler); err != nil {
		logx.Error("❌ Failed to start consuming from ttl.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from ttl.queue")
	}

	// Start consuming from expiration queue
	if err := client.Consume(ctx, "expiration.queue", ttlHandler); err != nil {
		logx.Error("❌ Failed to start consuming from expiration.queue", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Started consuming from expiration.queue")
	}

	// Display consumer statistics
	fmt.Println("\n📊 Consumer Statistics:")
	stats := consumer.GetStats()
	fmt.Printf("   Messages Consumed: %d\n", stats["messages_consumed"])
	fmt.Printf("   Messages Acked: %d\n", stats["messages_acked"])
	fmt.Printf("   Messages Nacked: %d\n", stats["messages_nacked"])
	fmt.Printf("   Messages Rejected: %d\n", stats["messages_rejected"])
	fmt.Printf("   Consume Errors: %d\n", stats["consume_errors"])
	fmt.Printf("   Last Consume Time: %v\n", stats["last_consume_time"])
	fmt.Printf("   Last Error Time: %v\n", stats["last_error_time"])
	if stats["last_error"] != nil {
		fmt.Printf("   Last Error: %v\n", stats["last_error"])
	}
	fmt.Printf("   Active Consumers: %d\n", stats["active_consumers"])
	fmt.Printf("   Closed: %v\n", stats["closed"])

	// Display client statistics
	fmt.Println("\n📊 Client Statistics:")
	clientStats := client.GetStats()
	fmt.Printf("   Client: %+v\n", clientStats)

	// Display health status
	fmt.Println("\n🏥 Health Status:")
	healthChecker := client.GetHealthChecker()
	if healthChecker.IsHealthy() {
		fmt.Println("   ✓ RabbitMQ connection is healthy")
	} else {
		fmt.Println("   ❌ RabbitMQ connection is unhealthy")
	}

	healthStats := healthChecker.GetStatsMap()
	fmt.Printf("   Health Stats: %+v\n", healthStats)

	fmt.Println("\n✅ Consumer example started successfully!")
	fmt.Println("   The consumer is now running and waiting for messages...")
	fmt.Println("   Run the producer example to send messages to these queues.")
	fmt.Println("   Press Ctrl+C to stop the consumer.")

	// Keep the consumer running
	select {}
}
