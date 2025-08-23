package main

import (
	"context"
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
	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		logx.Fatal("Failed to initialize logger", logx.String("error", err.Error()))
	}
	defer logx.Sync()

	logx.Info("🚀 Async Publisher Demo", logx.String("component", "main"))

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
		logx.Fatal("Failed to create observability provider", logx.String("error", err.Error()))
	}

	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create publisher
	publisher, err := factory.NewPublisher(context.Background(), config)
	if err != nil {
		logx.Fatal("Failed to create publisher", logx.String("error", err.Error()))
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logx.Info("🛑 Shutting down...", logx.String("component", "shutdown"))
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
		logx.Error("Error closing publisher", logx.String("error", err.Error()))
	}

	logx.Info("✅ Demo completed!", logx.String("component", "main"))
}

func demonstrateAsyncPublishing(ctx context.Context, publisher messaging.Publisher, obsCtx *messaging.ObservabilityContext) {
	logx.Info("📤 Example 1: Async Publishing with Worker Pools",
		logx.String("component", "async_publishing"),
		logx.String("exchange", "async.demo"))

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
				logx.Error("Failed to publish message",
					logx.Int("message_index", index+1),
					logx.String("message_id", message.ID),
					logx.String("error", err.Error()))
				return
			}

			receipts[index] = receipt
			logx.Info("📤 Queued message",
				logx.Int("message_index", index+1),
				logx.String("message_id", message.ID))
		}(i, msg)
	}

	wg.Wait()

	// Wait for all receipts to complete
	logx.Info("⏳ Waiting for confirmations...", logx.Int("message_count", len(messages)))
	if err := messaging.AwaitAll(ctx, receipts...); err != nil {
		logx.Error("Error waiting for confirmations", logx.String("error", err.Error()))
		return
	}

	// Check results
	results, errors := messaging.GetResults(receipts...)
	successCount := 0
	for i, result := range results {
		if errors[i] == nil && result.Success {
			successCount++
			logx.Info("✅ Message confirmed",
				logx.Int("message_index", i+1),
				logx.Int64("delivery_tag", int64(result.DeliveryTag)))
		} else {
			logx.Error("❌ Message failed",
				logx.Int("message_index", i+1),
				logx.String("error", errors[i].Error()))
		}
	}

	logx.Info("📊 Async publishing summary",
		logx.Int("successful", successCount),
		logx.Int("total", len(messages)),
		logx.Float64("success_rate", float64(successCount)/float64(len(messages))))
}

func demonstrateCodecSystem(ctx context.Context, obsCtx *messaging.ObservabilityContext) {
	logx.Info("🔧 Example 2: Custom Codec System", logx.String("component", "codec_system"))

	// Get global codec registry
	registry := messaging.GetGlobalCodecRegistry()

	// List available codecs
	logx.Info("Available codecs:")
	for _, codecName := range registry.List() {
		codec, _ := registry.Get(codecName)
		logx.Info("Codec available",
			logx.String("name", codec.Name()),
			logx.String("content_type", codec.ContentType()))
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
		logx.Error("JSON encoding failed", logx.String("error", err.Error()))
	} else {
		logx.Info("✅ JSON encoded", logx.Int("bytes", len(jsonData)))
	}

	// Test Protobuf codec (fallback to JSON)
	protobufCodec, _ := registry.Get("protobuf")
	protoData, err := protobufCodec.Encode(testData)
	if err != nil {
		logx.Error("Protobuf encoding failed", logx.String("error", err.Error()))
	} else {
		logx.Info("✅ Protobuf encoded", logx.Int("bytes", len(protoData)))
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
		logx.Error("Message encoding failed", logx.String("error", err.Error()))
	} else {
		logx.Info("✅ Message encoded",
			logx.Int("bytes", len(encodedMsg)),
			logx.String("message_id", msg.ID))

		// Decode message
		decodedMsg, err := messageCodec.DecodeMessage(encodedMsg, "application/json")
		if err != nil {
			logx.Error("Message decoding failed", logx.String("error", err.Error()))
		} else {
			logx.Info("✅ Message decoded",
				logx.String("id", decodedMsg.ID),
				logx.String("key", decodedMsg.Key))
		}
	}
}

func demonstrateBackpressureHandling(ctx context.Context, publisher messaging.Publisher, obsCtx *messaging.ObservabilityContext) {
	logx.Info("⚡ Example 3: Backpressure Handling",
		logx.String("component", "backpressure"),
		logx.String("exchange", "async.demo"))

	// Create a large number of messages to test backpressure
	const messageCount = 100
	messages := make([]messaging.Message, messageCount)
	for i := 0; i < messageCount; i++ {
		messages[i] = messaging.NewMessage(
			[]byte("backpressure test message"),
			messaging.WithID("bp-msg"),
		)
	}

	// Publish messages rapidly to trigger backpressure
	receipts := make([]messaging.Receipt, messageCount)
	startTime := time.Now()

	logx.Info("📤 Publishing messages rapidly", logx.Int("message_count", messageCount))

	for i, msg := range messages {
		receipt, err := publisher.PublishAsync(ctx, "async.demo", msg)
		if err != nil {
			logx.Error("Failed to publish message",
				logx.Int("message_index", i+1),
				logx.String("message_id", msg.ID),
				logx.String("error", err.Error()))
			continue
		}
		receipts[i] = receipt
	}

	publishDuration := time.Since(startTime)
	logx.Info("📊 Messages queued",
		logx.Int("message_count", messageCount),
		logx.String("duration", publishDuration.String()),
		logx.Float64("rate_msgs_per_sec", float64(messageCount)/publishDuration.Seconds()))

	// Wait for all confirmations
	logx.Info("⏳ Waiting for all confirmations...")
	confirmStart := time.Now()

	if err := messaging.AwaitAll(ctx, receipts...); err != nil {
		logx.Error("Error waiting for confirmations", logx.String("error", err.Error()))
		return
	}

	confirmDuration := time.Since(confirmStart)
	logx.Info("📊 All messages confirmed", logx.String("duration", confirmDuration.String()))

	// Check results
	results, _ := messaging.GetResults(receipts...)
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	totalDuration := publishDuration + confirmDuration
	logx.Info("📊 Backpressure handling summary",
		logx.Int("successful", successCount),
		logx.Int("total", messageCount),
		logx.String("total_time", totalDuration.String()),
		logx.Float64("throughput_msgs_per_sec", float64(successCount)/totalDuration.Seconds()),
		logx.Float64("success_rate", float64(successCount)/float64(messageCount)))
}
