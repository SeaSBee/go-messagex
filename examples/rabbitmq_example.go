package main

import (
	"context"
	"fmt"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
	fmt.Println("=== RabbitMQ Client Example ===")

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

	// Create a message handler
	handler := func(delivery *messaging.Delivery) error {
		fmt.Printf("📨 Received message: %s\n", delivery.Message.String())
		fmt.Printf("   Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("   Headers: %+v\n", delivery.Message.Headers)

		// Acknowledge the message
		return delivery.Acknowledger.Ack()
	}

	// Start consuming messages from the test queue
	ctx := context.Background()
	queueName := "test.queue"

	if err := client.Consume(ctx, queueName, handler); err != nil {
		logx.Fatal("Failed to start consuming", logx.ErrorField(err))
	}

	fmt.Printf("✓ Started consuming from queue: %s\n", queueName)

	// Publish some test messages
	fmt.Println("\n📤 Publishing test messages...")

	// Simple text message
	msg1 := messaging.NewTextMessage("Hello, RabbitMQ!")
	msg1.SetRoutingKey(queueName)
	msg1.SetExchange("")
	msg1.SetPersistent(true)

	if err := client.Publish(ctx, msg1); err != nil {
		logx.Error("Failed to publish message 1", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Published text message")
	}

	// JSON message
	userData := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	msg2, err := messaging.NewJSONMessage(userData)
	if err != nil {
		logx.Error("Failed to create JSON message", logx.ErrorField(err))
	} else {
		msg2.SetRoutingKey(queueName)
		msg2.SetExchange("")
		msg2.SetPersistent(true)
		msg2.SetHeader("message_type", "user_data")

		if err := client.Publish(ctx, msg2); err != nil {
			logx.Error("Failed to publish JSON message", logx.ErrorField(err))
		} else {
			fmt.Println("✓ Published JSON message")
		}
	}

	// High priority message
	msg3 := messaging.NewTextMessage("High priority message")
	msg3.SetPriority(messaging.PriorityHigh)
	msg3.SetRoutingKey(queueName)
	msg3.SetExchange("")
	msg3.SetPersistent(true)
	msg3.SetHeader("priority", "high")

	if err := client.Publish(ctx, msg3); err != nil {
		logx.Error("Failed to publish high priority message", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Published high priority message")
	}

	// Batch message
	fmt.Println("\n📦 Publishing batch messages...")
	var batchMessages []*messaging.Message
	for i := 0; i < 5; i++ {
		msg := messaging.NewTextMessage(fmt.Sprintf("Batch message %d", i+1))
		msg.SetRoutingKey(queueName)
		msg.SetExchange("")
		msg.SetPersistent(true)
		msg.SetHeader("batch_id", "batch_001")
		msg.SetHeader("message_index", i+1)
		batchMessages = append(batchMessages, msg)
	}

	batch := messaging.NewBatchMessage(batchMessages)
	if err := client.PublishBatch(ctx, batch); err != nil {
		logx.Error("Failed to publish batch", logx.ErrorField(err))
	} else {
		fmt.Printf("✓ Published batch with %d messages\n", batch.Count())
	}

	// Display statistics
	fmt.Println("\n📊 Client Statistics:")
	stats := client.GetStats()
	fmt.Printf("   Client: %+v\n", stats)

	producer := client.GetProducer()
	producerStats := producer.GetStats()
	fmt.Printf("   Producer: %+v\n", producerStats)

	consumer := client.GetConsumer()
	consumerStats := consumer.GetStats()
	fmt.Printf("   Consumer: %+v\n", consumerStats)

	healthChecker := client.GetHealthChecker()
	healthStats := healthChecker.GetStatsMap()
	fmt.Printf("   Health: %+v\n", healthStats)

	fmt.Println("\n⏳ Waiting for messages... (Press Ctrl+C to stop)")
	fmt.Println("   The consumer will process the published messages above.")

	// Keep the application running
	select {}
}
