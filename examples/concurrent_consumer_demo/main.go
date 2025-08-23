package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// DemoHandler demonstrates concurrent message processing
type DemoHandler struct {
	processedCount int64
	failureRate    float64 // Simulate failure rate (0.0 to 1.0)
}

func NewDemoHandler(failureRate float64) *DemoHandler {
	return &DemoHandler{
		failureRate: failureRate,
	}
}

func (dh *DemoHandler) Process(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
	// Simulate processing time
	processingTime := time.Duration(10+delivery.Message.ID[0]%50) * time.Millisecond
	time.Sleep(processingTime)

	// Increment counter
	count := atomic.AddInt64(&dh.processedCount, 1)

	// Simulate failures based on failure rate
	if dh.failureRate > 0 && float64(count%100) < dh.failureRate*100 {
		logx.Error("simulated failure for message",
			logx.String("message_id", delivery.Message.ID),
			logx.Int64("count", count),
			logx.String("processing_time", processingTime.String()))
		return messaging.NackRequeue, fmt.Errorf("simulated failure for message %s", delivery.Message.ID)
	}

	// Simulate panic occasionally
	if count%1000 == 0 && dh.failureRate > 0.1 {
		logx.Error("simulated panic for message",
			logx.String("message_id", delivery.Message.ID),
			logx.Int64("count", count))
		panic(fmt.Sprintf("simulated panic for message %s", delivery.Message.ID))
	}

	logx.Info("processed message",
		logx.String("message_id", delivery.Message.ID),
		logx.Int64("count", count),
		logx.String("processing_time", processingTime.String()))

	return messaging.Ack, nil
}

func (dh *DemoHandler) GetProcessedCount() int64 {
	return atomic.LoadInt64(&dh.processedCount)
}

func main() {
	logx.Info("🚀 starting concurrent consumer demo")

	// Register RabbitMQ transport
	rabbitmq.Register()

	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		logx.Fatal("failed to initialize logger", logx.String("error", err.Error()))
	}
	defer logx.Sync()

	// Create configuration with concurrent consumer features
	config := &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Consumer: &messaging.ConsumerConfig{
				Queue:                 "concurrent.demo.queue",
				Prefetch:              100,              // Prefetch 100 messages
				MaxConcurrentHandlers: 50,               // Process up to 50 messages concurrently
				RequeueOnError:        true,             // Requeue failed messages
				AckOnSuccess:          true,             // Acknowledge successful messages
				AutoAck:               false,            // Manual acknowledgment
				Exclusive:             false,            // Allow multiple consumers
				NoLocal:               false,            // Allow messages from same connection
				NoWait:                false,            // Wait for consumer confirmation
				HandlerTimeout:        60 * time.Second, // Handler timeout
				PanicRecovery:         true,             // Enable panic recovery
				MaxRetries:            3,                // Retry up to 3 times before DLQ
			},
			DLQ: &messaging.DeadLetterQueueConfig{
				Enabled:    true,
				Exchange:   "dlx.demo",
				Queue:      "dlq.demo",
				RoutingKey: "dlq",
				MaxRetries: 3,
				RetryDelay: 5 * time.Second,
				AutoCreate: true,
			},
			Topology: &messaging.TopologyConfig{
				Exchanges: []messaging.ExchangeConfig{
					{
						Name:    "concurrent.demo",
						Type:    "direct",
						Durable: true,
					},
					{
						Name:    "dlx.demo",
						Type:    "direct",
						Durable: true,
					},
				},
				Queues: []messaging.QueueConfig{
					{
						Name:    "concurrent.demo.queue",
						Durable: true,
						Arguments: map[string]interface{}{
							"x-dead-letter-exchange":    "dlx.demo",
							"x-dead-letter-routing-key": "dlq",
						},
					},
					{
						Name:    "dlq.demo",
						Durable: true,
					},
				},
				Bindings: []messaging.BindingConfig{
					{
						Exchange: "concurrent.demo",
						Queue:    "concurrent.demo.queue",
						Key:      "demo.key",
					},
					{
						Exchange: "dlx.demo",
						Queue:    "dlq.demo",
						Key:      "dlq",
					},
				},
			},
		},
		Telemetry: &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "concurrent-consumer-demo",
		},
	}

	// Create observability provider
	obsProvider, err := messaging.NewObservabilityProvider(config.Telemetry)
	if err != nil {
		logx.Fatal("failed to create observability provider", logx.String("error", err.Error()))
	}

	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create consumer using the messaging package
	consumer, err := messaging.NewConsumer(context.Background(), config)
	if err != nil {
		logx.Fatal("failed to create consumer", logx.String("error", err.Error()))
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logx.Info("🛑 shutting down...")
		cancel()
	}()

	// Run demos
	demonstrateConcurrentConsumption(ctx, consumer, obsCtx)
	demonstrateFailureHandling(ctx, consumer, obsCtx)
	demonstratePanicRecovery(ctx, consumer, obsCtx)

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop consumer
	if err := consumer.Stop(context.Background()); err != nil {
		logx.Error("error stopping consumer", logx.String("error", err.Error()))
	}

	logx.Info("✅ demo completed")
}

func demonstrateConcurrentConsumption(ctx context.Context, consumer messaging.Consumer, obsCtx *messaging.ObservabilityContext) {
	logx.Info("📥 starting example 1: concurrent message consumption")

	// Create a handler with no failures for this demo
	handler := NewDemoHandler(0.0)

	// Start consumer
	consumerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	go func() {
		err := consumer.Start(consumerCtx, handler)
		if err != nil {
			logx.Error("consumer error", logx.String("error", err.Error()))
		}
	}()

	// Let it run for a bit
	time.Sleep(5 * time.Second)

	// Check statistics if available
	if statsConsumer, ok := consumer.(interface{ GetStats() interface{} }); ok {
		stats := statsConsumer.GetStats()
		logx.Info("consumer stats", logx.Any("stats", stats))
	}

	logx.Info("messages processed", logx.Int64("count", handler.GetProcessedCount()))
}

func demonstrateFailureHandling(ctx context.Context, consumer messaging.Consumer, obsCtx *messaging.ObservabilityContext) {
	logx.Info("⚠️ starting example 2: failure handling and retries")

	// Create a handler with 20% failure rate
	handler := NewDemoHandler(0.2)

	// Start consumer
	consumerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	go func() {
		err := consumer.Start(consumerCtx, handler)
		if err != nil {
			logx.Error("consumer error", logx.String("error", err.Error()))
		}
	}()

	// Let it run for a bit
	time.Sleep(5 * time.Second)

	// Check statistics
	if statsConsumer, ok := consumer.(interface{ GetStats() interface{} }); ok {
		stats := statsConsumer.GetStats()
		logx.Info("consumer stats with failures", logx.Any("stats", stats))
	}

	logx.Info("messages processed", logx.Int64("count", handler.GetProcessedCount()))
}

func demonstratePanicRecovery(ctx context.Context, consumer messaging.Consumer, obsCtx *messaging.ObservabilityContext) {
	logx.Info("💥 starting example 3: panic recovery")

	// Create a handler with higher failure rate to trigger panics
	handler := NewDemoHandler(0.3)

	// Start consumer
	consumerCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	go func() {
		err := consumer.Start(consumerCtx, handler)
		if err != nil {
			logx.Error("consumer error", logx.String("error", err.Error()))
		}
	}()

	// Let it run for a bit
	time.Sleep(5 * time.Second)

	// Check final statistics
	if statsConsumer, ok := consumer.(interface{ GetStats() interface{} }); ok {
		stats := statsConsumer.GetStats()
		logx.Info("final consumer stats", logx.Any("stats", stats))
	}

	logx.Info("total messages processed", logx.Int64("count", handler.GetProcessedCount()))
	logx.Info("📋 demo summary",
		logx.String("feature1", "concurrent message processing with worker pools"),
		logx.String("feature2", "automatic retry handling with exponential backoff"),
		logx.String("feature3", "panic recovery with DLQ routing"),
		logx.String("feature4", "QoS control with prefetch limits"),
		logx.String("feature5", "comprehensive statistics and monitoring"))
}
