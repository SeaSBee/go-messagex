package main

import (
	"context"
	"fmt"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
	fmt.Println("=== RabbitMQ Producer Example ===")

	// Create a new RabbitMQ client with default configuration
	client, err := messaging.NewClient(nil)
	if err != nil {
		logx.Fatal("Failed to create client", logx.ErrorField(err))
	}
	defer client.Close()

	// Wait for connection to be established
	if err := client.WaitForConnection(30 * time.Second); err != nil {
		logx.Fatal("Failed to connect", logx.ErrorField(err))
	}

	fmt.Println("✓ Connected to RabbitMQ successfully!")

	// Get the producer
	producer := client.GetProducer()
	if producer == nil {
		logx.Fatal("Failed to get producer")
	}

	ctx := context.Background()

	// Example 1: Publish a simple text message
	fmt.Println("\n📤 Publishing simple text message...")
	msg1 := messaging.NewTextMessage("Hello, RabbitMQ Producer!")
	msg1.SetRoutingKey("test.queue")
	msg1.SetExchange("")
	msg1.SetPersistent(true)
	msg1.SetHeader("message_type", "text")
	msg1.SetHeader("timestamp", time.Now().Unix())

	if err := client.Publish(ctx, msg1); err != nil {
		logx.Error("❌ Failed to publish text message", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Text message published successfully")
	}

	// Example 2: Publish a JSON message
	fmt.Println("\n📤 Publishing JSON message...")
	userData := map[string]interface{}{
		"id":      1,
		"name":    "John Doe",
		"email":   "john@example.com",
		"age":     30,
		"city":    "New York",
		"country": "USA",
		"created": time.Now().Format(time.RFC3339),
	}

	msg2, err := messaging.NewJSONMessage(userData)
	if err != nil {
		logx.Error("❌ Failed to create JSON message", logx.ErrorField(err))
	} else {
		msg2.SetRoutingKey("user.queue")
		msg2.SetExchange("")
		msg2.SetPersistent(true)
		msg2.SetHeader("message_type", "user_data")
		msg2.SetPriority(messaging.PriorityHigh)

		if err := client.Publish(ctx, msg2); err != nil {
			logx.Error("❌ Failed to publish JSON message", logx.ErrorField(err))
		} else {
			fmt.Println("✓ JSON message published successfully")
		}
	}

	// Example 3: Publish messages with different priorities
	fmt.Println("\n📤 Publishing messages with different priorities...")
	priorities := []struct {
		priority messaging.MessagePriority
		message  string
	}{
		{messaging.PriorityLow, "Low priority message"},
		{messaging.PriorityNormal, "Normal priority message"},
		{messaging.PriorityHigh, "High priority message"},
		{messaging.PriorityCritical, "Critical priority message"},
	}

	for i, p := range priorities {
		msg := messaging.NewTextMessage(p.message)
		msg.SetRoutingKey("priority.queue")
		msg.SetExchange("")
		msg.SetPersistent(true)
		msg.SetPriority(p.priority)
		msg.SetHeader("priority_level", fmt.Sprintf("%d", p.priority))
		msg.SetHeader("message_index", i+1)

		if err := client.Publish(ctx, msg); err != nil {
			logx.Error("❌ Failed to publish priority message", logx.String("message", p.message), logx.ErrorField(err))
		} else {
			fmt.Printf("✓ %s published successfully\n", p.message)
		}
	}

	// Example 4: Publish messages with TTL and expiration
	fmt.Println("\n📤 Publishing messages with TTL and expiration...")

	// Message with TTL
	ttlMsg := messaging.NewTextMessage("This message has a TTL of 30 seconds")
	ttlMsg.SetRoutingKey("ttl.queue")
	ttlMsg.SetExchange("")
	ttlMsg.SetPersistent(true)
	ttlMsg.SetTTL(30 * time.Second)
	ttlMsg.SetHeader("message_type", "ttl_message")

	if err := client.Publish(ctx, ttlMsg); err != nil {
		logx.Error("❌ Failed to publish TTL message", logx.ErrorField(err))
	} else {
		fmt.Println("✓ TTL message published successfully")
	}

	// Message with expiration
	expMsg := messaging.NewTextMessage("This message expires in 1 minute")
	expMsg.SetRoutingKey("expiration.queue")
	expMsg.SetExchange("")
	expMsg.SetPersistent(true)
	expMsg.SetExpiration(1 * time.Minute)
	expMsg.SetHeader("message_type", "expiration_message")

	if err := client.Publish(ctx, expMsg); err != nil {
		logx.Error("❌ Failed to publish expiration message", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Expiration message published successfully")
	}

	// Example 5: Publish a batch of messages
	fmt.Println("\n📦 Publishing batch of messages...")
	var batchMessages []*messaging.Message
	for i := 0; i < 10; i++ {
		msg := messaging.NewTextMessage(fmt.Sprintf("Batch message %d", i+1))
		msg.SetRoutingKey("batch.queue")
		msg.SetExchange("")
		msg.SetPersistent(true)
		msg.SetHeader("batch_id", "batch_001")
		msg.SetHeader("message_index", i+1)
		msg.SetHeader("total_messages", 10)
		batchMessages = append(batchMessages, msg)
	}

	batch := messaging.NewBatchMessage(batchMessages)
	if err := client.PublishBatch(ctx, batch); err != nil {
		logx.Error("❌ Failed to publish batch", logx.ErrorField(err))
	} else {
		fmt.Printf("✓ Batch of %d messages published successfully\n", batch.Count())
	}

	// Example 6: Publish messages with correlation ID and reply-to
	fmt.Println("\n📤 Publishing RPC-style messages...")

	// Request message
	requestMsg := messaging.NewTextMessage("What is the current time?")
	requestMsg.SetRoutingKey("rpc.request")
	requestMsg.SetExchange("")
	requestMsg.SetPersistent(true)
	requestMsg.SetCorrelationID("req-001")
	requestMsg.SetReplyTo("rpc.response")
	requestMsg.SetHeader("message_type", "rpc_request")
	requestMsg.SetHeader("request_id", "req-001")

	if err := client.Publish(ctx, requestMsg); err != nil {
		logx.Error("❌ Failed to publish RPC request", logx.ErrorField(err))
	} else {
		fmt.Println("✓ RPC request published successfully")
	}

	// Response message
	responseMsg := messaging.NewTextMessage(time.Now().Format(time.RFC3339))
	responseMsg.SetRoutingKey("rpc.response")
	responseMsg.SetExchange("")
	responseMsg.SetPersistent(true)
	responseMsg.SetCorrelationID("req-001")
	responseMsg.SetHeader("message_type", "rpc_response")
	responseMsg.SetHeader("request_id", "req-001")

	if err := client.Publish(ctx, responseMsg); err != nil {
		logx.Error("❌ Failed to publish RPC response", logx.ErrorField(err))
	} else {
		fmt.Println("✓ RPC response published successfully")
	}

	// Example 7: Publish messages with custom properties
	fmt.Println("\n📤 Publishing messages with custom properties...")

	customMsg := messaging.NewTextMessage("Message with custom properties")
	customMsg.SetRoutingKey("custom.queue")
	customMsg.SetExchange("")
	customMsg.SetPersistent(true)
	customMsg.SetHeader("custom_header_1", "value1")
	customMsg.SetHeader("custom_header_2", "value2")
	customMsg.SetHeader("user_id", "user123")
	customMsg.SetHeader("session_id", "session456")
	customMsg.SetMetadata("processing_time", "fast")
	customMsg.SetMetadata("retry_count", 0)

	if err := client.Publish(ctx, customMsg); err != nil {
		logx.Error("❌ Failed to publish custom message", logx.ErrorField(err))
	} else {
		fmt.Println("✓ Custom message published successfully")
	}

	// Display producer statistics
	fmt.Println("\n📊 Producer Statistics:")
	stats := producer.GetStats()
	fmt.Printf("   Messages Published: %d\n", stats["messages_published"])
	fmt.Printf("   Batches Published: %d\n", stats["batches_published"])
	fmt.Printf("   Publish Errors: %d\n", stats["publish_errors"])
	fmt.Printf("   Last Publish Time: %v\n", stats["last_publish_time"])
	fmt.Printf("   Last Error Time: %v\n", stats["last_error_time"])
	if stats["last_error"] != nil {
		fmt.Printf("   Last Error: %v\n", stats["last_error"])
	}
	fmt.Printf("   Batch Size: %d\n", stats["batch_size"])
	fmt.Printf("   Closed: %v\n", stats["closed"])

	// Display client statistics
	fmt.Println("\n📊 Client Statistics:")
	clientStats := client.GetStats()
	fmt.Printf("   Client: %+v\n", clientStats)

	fmt.Println("\n✅ Producer example completed successfully!")
	fmt.Println("   Check your RabbitMQ management interface to see the published messages.")
}
