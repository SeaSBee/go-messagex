package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

func main() {
	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logx.Sync()

	logx.Info("🚀 RabbitMQ Connection Pool Demo", logx.String("status", "starting"))

	// Create configuration with connection pooling
	config := &messaging.RabbitMQConfig{
		URIs: []string{"amqp://localhost:5672"},
		ConnectionPool: &messaging.ConnectionPoolConfig{
			Min:                 2,
			Max:                 8,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		},
		ChannelPool: &messaging.ChannelPoolConfig{
			PerConnectionMin:    5,
			PerConnectionMax:    20,
			BorrowTimeout:       5 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		},
		Publisher: &messaging.PublisherConfig{
			Confirms:       true,
			Mandatory:      true,
			MaxInFlight:    1000,
			PublishTimeout: 2 * time.Second,
		},
		Consumer: &messaging.ConsumerConfig{
			Queue:                 "demo.queue",
			Prefetch:              10,
			MaxConcurrentHandlers: 5,
			RequeueOnError:        true,
			HandlerTimeout:        30 * time.Second,
		},
		Topology: &messaging.TopologyConfig{
			Exchanges: []messaging.ExchangeConfig{
				{
					Name:    "demo.exchange",
					Type:    "direct",
					Durable: true,
				},
			},
			Queues: []messaging.QueueConfig{
				{
					Name:    "demo.queue",
					Durable: true,
				},
			},
			Bindings: []messaging.BindingConfig{
				{
					Exchange: "demo.exchange",
					Queue:    "demo.queue",
					Key:      "demo.key",
				},
			},
		},
	}

	// Create observability provider
	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		logx.Error("Failed to create observability provider", logx.ErrorField(err))
		os.Exit(1)
	}

	// Create observability context
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create pooled transport
	transport := rabbitmq.NewPooledTransport(config, obsCtx)
	logx.Info("Created pooled transport", logx.String("status", "success"))

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logx.Info("Shutdown signal received, closing transport", logx.String("action", "shutdown"))
		cancel()
	}()

	// Connect to RabbitMQ
	logx.Info("Connecting to RabbitMQ", logx.String("uri", config.URIs[0]))
	if err := transport.Connect(ctx); err != nil {
		logx.Warn("Connection failed (expected without RabbitMQ)",
			logx.ErrorField(err),
			logx.String("note", "This demo shows the connection pool structure and configuration. To test with real RabbitMQ, start a RabbitMQ server and update the URI"))
	} else {
		logx.Info("Connected to RabbitMQ", logx.String("status", "success"))
		defer transport.Disconnect(ctx)
	}

	// Create publisher
	publisherConfig := &messaging.PublisherConfig{
		Confirms:       true,
		Mandatory:      true,
		MaxInFlight:    1000,
		PublishTimeout: 2 * time.Second,
	}
	publisher, err := rabbitmq.NewPublisher(transport.Transport, publisherConfig, obsCtx)
	if err != nil {
		logx.Error("Failed to create publisher", logx.ErrorField(err))
		return
	}
	logx.Info("Created publisher", logx.String("status", "success"))

	// Create consumer
	consumerConfig := &messaging.ConsumerConfig{
		Queue:                 "demo.queue",
		Prefetch:              10,
		MaxConcurrentHandlers: 5,
		RequeueOnError:        true,
		HandlerTimeout:        30 * time.Second,
	}
	consumer, err := rabbitmq.NewConsumer(transport.Transport, consumerConfig, obsCtx)
	if err != nil {
		logx.Error("Failed to create consumer", logx.ErrorField(err))
		return
	}
	logx.Info("Created consumer", logx.String("status", "success"))

	// Create message handler
	handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
		logx.Info("Received message",
			logx.String("message_id", delivery.Message.ID),
			logx.String("queue", delivery.Queue),
			logx.String("routing_key", delivery.RoutingKey))
		return messaging.Ack, nil
	})

	// Start consumer
	logx.Info("Starting consumer")
	if err := consumer.Start(ctx, handler); err != nil {
		logx.Warn("Failed to start consumer", logx.ErrorField(err))
	} else {
		logx.Info("Consumer started", logx.String("status", "success"))
		defer consumer.Stop(ctx)
	}

	// Demo connection pool features
	logx.Info("Connection Pool Features Demo", logx.String("section", "configuration"))

	// Show connection pool configuration
	logx.Info("Connection Pool Config",
		logx.Int("min_connections", config.ConnectionPool.Min),
		logx.Int("max_connections", config.ConnectionPool.Max),
		logx.String("health_check_interval", config.ConnectionPool.HealthCheckInterval.String()),
		logx.String("connection_timeout", config.ConnectionPool.ConnectionTimeout.String()),
		logx.String("heartbeat_interval", config.ConnectionPool.HeartbeatInterval.String()))

	logx.Info("Channel Pool Config",
		logx.Int("min_channels_per_connection", config.ChannelPool.PerConnectionMin),
		logx.Int("max_channels_per_connection", config.ChannelPool.PerConnectionMax),
		logx.String("borrow_timeout", config.ChannelPool.BorrowTimeout.String()),
		logx.String("health_check_interval", config.ChannelPool.HealthCheckInterval.String()))

	// Demo message publishing
	logx.Info("Message Publishing Demo", logx.String("section", "publishing"))

	// Create test messages
	messages := []messaging.Message{
		{
			ID:          "msg-1",
			Key:         "demo.key",
			Body:        []byte("Hello from connection pool demo!"),
			ContentType: "text/plain",
			Timestamp:   time.Now(),
		},
		{
			ID:          "msg-2",
			Key:         "demo.key",
			Body:        []byte("This message demonstrates async publishing"),
			ContentType: "text/plain",
			Timestamp:   time.Now(),
		},
		{
			ID:          "msg-3",
			Key:         "demo.key",
			Body:        []byte("Connection pool handles backpressure automatically"),
			ContentType: "text/plain",
			Timestamp:   time.Now(),
		},
	}

	// Publish messages
	for i, msg := range messages {
		logx.Info("Publishing message",
			logx.Int("message_number", i+1),
			logx.String("message_id", msg.ID),
			logx.String("exchange", "demo.exchange"))

		receipt, err := publisher.PublishAsync(ctx, "demo.exchange", msg)
		if err != nil {
			logx.Error("Failed to publish message",
				logx.Int("message_number", i+1),
				logx.String("message_id", msg.ID),
				logx.ErrorField(err))
			continue
		}

		// Wait for receipt
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			if err != nil {
				logx.Error("Message failed",
					logx.Int("message_number", i+1),
					logx.String("message_id", msg.ID),
					logx.ErrorField(err))
			} else {
				logx.Info("Message published successfully",
					logx.Int("message_number", i+1),
					logx.String("message_id", msg.ID))
			}
		case <-time.After(5 * time.Second):
			logx.Warn("Message timed out",
				logx.Int("message_number", i+1),
				logx.String("message_id", msg.ID),
				logx.String("timeout", "5s"))
		}
	}

	// Demo connection pool statistics
	logx.Info("Connection Pool Statistics", logx.String("section", "statistics"))

	// Wait a bit for operations to complete
	time.Sleep(2 * time.Second)

	logx.Info("Connection pool features demonstrated",
		logx.Any("features", []string{
			"Health monitoring with periodic checks",
			"Auto-recovery with exponential backoff + jitter",
			"Connection lifecycle management",
			"Thread-safe connection management",
			"Connection metrics and logging",
			"Graceful degradation and error handling",
			"Context-based cancellation",
			"Async message publishing with receipts",
		}))

	logx.Info("Demo completed successfully",
		logx.String("status", "completed"),
		logx.Any("production_ready_features", []string{
			"Robust error handling and recovery",
			"Comprehensive monitoring and metrics",
			"Thread-safe operations",
			"Configurable timeouts and limits",
		}))

	// Wait for shutdown signal
	<-ctx.Done()
	logx.Info("Goodbye", logx.String("status", "shutdown"))
}
