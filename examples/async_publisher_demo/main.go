package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

func main() {
	fmt.Println("🚀 Async Publisher Demo")
	fmt.Println("========================")

	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logx.Sync()

	// Create configuration with async publisher features
	config := &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Publisher: &messaging.PublisherConfig{
				Confirms:       true,
				Mandatory:      true,
				MaxInFlight:    1000,  // Bounded queue size
				DropOnOverflow: false, // Block on overflow
				PublishTimeout: 2 * time.Second,
				WorkerCount:    4, // Worker pool size
				Retry: &messaging.RetryConfig{
					MaxAttempts:       3,
					BaseBackoff:       100 * time.Millisecond,
					MaxBackoff:        1 * time.Second,
					BackoffMultiplier: 2.0,
					Jitter:            true,
				},
				Serialization: &messaging.SerializationConfig{
					DefaultContentType: "application/json",
					CompressionEnabled: false,
					CompressionLevel:   6,
				},
			},
			Topology: &messaging.TopologyConfig{
				Exchanges: []messaging.ExchangeConfig{
					{
						Name:    "async.demo",
						Type:    "direct",
						Durable: true,
					},
				},
				Queues: []messaging.QueueConfig{
					{
						Name:    "async.demo.queue",
						Durable: true,
					},
				},
				Bindings: []messaging.BindingConfig{
					{
						Exchange: "async.demo",
						Queue:    "async.demo.queue",
						Key:      "demo.key",
					},
				},
			},
		},
		Telemetry: &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "async-publisher-demo",
		},
	}

	// Create observability provider
	obsProvider, err := messaging.NewObservabilityProvider(config.Telemetry)
	if err != nil {
		log.Fatalf("Failed to create observability provider: %v", err)
	}

	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create publisher
	publisher, err := factory.NewPublisher(context.Background(), config)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		cancel()
	}()

	// Run demos
	demonstrateAsyncPublishing(ctx, publisher, obsCtx)
	demonstrateCodecSystem(ctx, obsCtx)
	demonstrateBackpressureHandling(ctx, publisher, obsCtx)

	// Wait for shutdown signal
	<-ctx.Done()

	// Close publisher
	if err := publisher.Close(context.Background()); err != nil {
		log.Printf("Error closing publisher: %v", err)
	}

	fmt.Println("✅ Demo completed!")
}

func demonstrateAsyncPublishing(ctx context.Context, publisher messaging.Publisher, obsCtx *messaging.ObservabilityContext) {
	fmt.Println("\n📤 Example 1: Async Publishing with Worker Pools")
	fmt.Println("------------------------------------------------")

	// Create multiple messages
	messages := []messaging.Message{
		messaging.NewMessage([]byte("message 1"), messaging.WithID("msg-1")),
		messaging.NewMessage([]byte("message 2"), messaging.WithID("msg-2")),
		messaging.NewMessage([]byte("message 3"), messaging.WithID("msg-3")),
		messaging.NewMessage([]byte("message 4"), messaging.WithID("msg-4")),
		messaging.NewMessage([]byte("message 5"), messaging.WithID("msg-5")),
	}

	// Publish messages asynchronously
	receipts := make([]messaging.Receipt, len(messages))
	var wg sync.WaitGroup

	for i, msg := range messages {
		wg.Add(1)
		go func(index int, message messaging.Message) {
			defer wg.Done()

			receipt, err := publisher.PublishAsync(ctx, "async.demo", message)
			if err != nil {
				fmt.Printf("❌ Failed to publish message %d: %v\n", index+1, err)
				return
			}

			receipts[index] = receipt
			fmt.Printf("📤 Queued message %d (ID: %s)\n", index+1, message.ID)
		}(i, msg)
	}

	wg.Wait()

	// Wait for all receipts to complete
	fmt.Println("\n⏳ Waiting for confirmations...")
	if err := messaging.AwaitAll(ctx, receipts...); err != nil {
		fmt.Printf("❌ Error waiting for confirmations: %v\n", err)
		return
	}

	// Check results
	results, errors := messaging.GetResults(receipts...)
	successCount := 0
	for i, result := range results {
		if errors[i] == nil && result.Success {
			successCount++
			fmt.Printf("✅ Message %d confirmed (Delivery Tag: %d)\n", i+1, result.DeliveryTag)
		} else {
			fmt.Printf("❌ Message %d failed: %v\n", i+1, errors[i])
		}
	}

	fmt.Printf("\n📊 Summary: %d/%d messages published successfully\n", successCount, len(messages))
}

func demonstrateCodecSystem(ctx context.Context, obsCtx *messaging.ObservabilityContext) {
	fmt.Println("\n🔧 Example 2: Custom Codec System")
	fmt.Println("---------------------------------")

	// Get global codec registry
	registry := messaging.GetGlobalCodecRegistry()

	// List available codecs
	fmt.Println("Available codecs:")
	for _, codecName := range registry.List() {
		codec, _ := registry.Get(codecName)
		fmt.Printf("  - %s (%s)\n", codec.Name(), codec.ContentType())
	}

	// Test different codecs
	testData := map[string]interface{}{
		"user_id":   12345,
		"name":      "John Doe",
		"email":     "john@example.com",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Test JSON codec
	jsonCodec, _ := registry.Get("json")
	jsonData, err := jsonCodec.Encode(testData)
	if err != nil {
		fmt.Printf("❌ JSON encoding failed: %v\n", err)
	} else {
		fmt.Printf("✅ JSON encoded: %d bytes\n", len(jsonData))
	}

	// Test Protobuf codec (fallback to JSON)
	protobufCodec, _ := registry.Get("protobuf")
	protoData, err := protobufCodec.Encode(testData)
	if err != nil {
		fmt.Printf("❌ Protobuf encoding failed: %v\n", err)
	} else {
		fmt.Printf("✅ Protobuf encoded: %d bytes\n", len(protoData))
	}

	// Test message codec
	messageCodec := messaging.NewMessageCodec(registry)
	msg := messaging.NewMessage(
		[]byte("test message"),
		messaging.WithID("codec-test"),
		messaging.WithContentType("application/json"),
		messaging.WithKey("test.key"),
	)

	encodedMsg, err := messageCodec.EncodeMessage(&msg)
	if err != nil {
		fmt.Printf("❌ Message encoding failed: %v\n", err)
	} else {
		fmt.Printf("✅ Message encoded: %d bytes\n", len(encodedMsg))

		// Decode message
		decodedMsg, err := messageCodec.DecodeMessage(encodedMsg, "application/json")
		if err != nil {
			fmt.Printf("❌ Message decoding failed: %v\n", err)
		} else {
			fmt.Printf("✅ Message decoded: ID=%s, Key=%s\n", decodedMsg.ID, decodedMsg.Key)
		}
	}
}

func demonstrateBackpressureHandling(ctx context.Context, publisher messaging.Publisher, obsCtx *messaging.ObservabilityContext) {
	fmt.Println("\n⚡ Example 3: Backpressure Handling")
	fmt.Println("-----------------------------------")

	// Create a large number of messages to test backpressure
	const messageCount = 100
	messages := make([]messaging.Message, messageCount)
	for i := 0; i < messageCount; i++ {
		messages[i] = messaging.NewMessage(
			[]byte(fmt.Sprintf("backpressure test message %d", i+1)),
			messaging.WithID(fmt.Sprintf("bp-msg-%d", i+1)),
		)
	}

	// Publish messages rapidly to trigger backpressure
	receipts := make([]messaging.Receipt, messageCount)
	startTime := time.Now()

	fmt.Printf("📤 Publishing %d messages rapidly...\n", messageCount)

	for i, msg := range messages {
		receipt, err := publisher.PublishAsync(ctx, "async.demo", msg)
		if err != nil {
			fmt.Printf("❌ Failed to publish message %d: %v\n", i+1, err)
			continue
		}
		receipts[i] = receipt
	}

	publishDuration := time.Since(startTime)
	fmt.Printf("📊 Queued %d messages in %v\n", messageCount, publishDuration)

	// Wait for all confirmations
	fmt.Println("⏳ Waiting for all confirmations...")
	confirmStart := time.Now()

	if err := messaging.AwaitAll(ctx, receipts...); err != nil {
		fmt.Printf("❌ Error waiting for confirmations: %v\n", err)
		return
	}

	confirmDuration := time.Since(confirmStart)
	fmt.Printf("📊 All messages confirmed in %v\n", confirmDuration)

	// Check results
	results, _ := messaging.GetResults(receipts...)
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	fmt.Printf("📊 Final Summary: %d/%d messages published successfully\n", successCount, messageCount)
	fmt.Printf("📊 Total time: %v (%.2f msgs/sec)\n",
		publishDuration+confirmDuration,
		float64(successCount)/float64((publishDuration+confirmDuration).Seconds()))
}
