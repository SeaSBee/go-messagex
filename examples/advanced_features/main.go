package main

import (
	"context"
	"fmt"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

// transportWrapper wraps the RabbitMQ transport to match the DLQ interface
type transportWrapper struct {
	transport *rabbitmq.Transport
}

func (tw *transportWrapper) GetChannel() interface{} {
	return tw.transport.GetChannel()
}

func main() {
	ctx := context.Background()

	// Initialize go-logx
	if err := logx.InitDefault(); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logx.Sync()

	// Create configuration with advanced features
	config := &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Persistence: &messaging.MessagePersistenceConfig{
				Enabled:         true,
				StorageType:     "memory",
				MaxStorageSize:  1073741824, // 1GB
				CleanupInterval: 1 * time.Hour,
				MessageTTL:      24 * time.Hour,
			},
			DLQ: &messaging.DeadLetterQueueConfig{
				Enabled:    true,
				Exchange:   "dlx",
				Queue:      "dlq",
				RoutingKey: "dlq",
				MaxRetries: 3,
				RetryDelay: 5 * time.Second,
				AutoCreate: true,
			},
			Transformation: &messaging.MessageTransformationConfig{
				Enabled:             false, // Disabled for this example
				CompressionEnabled:  false,
				SerializationFormat: "json",
				SchemaValidation:    false,
			},
			Routing: &messaging.AdvancedRoutingConfig{
				Enabled:          false, // Disabled for this example
				DynamicRouting:   false,
				MessageFiltering: false,
			},
			Publisher: &messaging.PublisherConfig{
				Confirms:  true,
				Mandatory: true,
			},
			Consumer: &messaging.ConsumerConfig{
				Queue:                 "advanced.queue",
				Prefetch:              256,
				MaxConcurrentHandlers: 512,
				RequeueOnError:        true,
				AckOnSuccess:          true,
				AutoAck:               false,
				MaxRetries:            3,
				HandlerTimeout:        30 * time.Second,
				PanicRecovery:         true,
			},
		},
	}

	// Create observability provider
	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	if err != nil {
		logx.Fatal("Failed to create observability provider", logx.ErrorField(err))
	}
	obsCtx := messaging.NewObservabilityContext(ctx, obsProvider)

	// Create RabbitMQ transport
	transport := rabbitmq.NewTransport(config.RabbitMQ, obsCtx)

	// Connect to RabbitMQ
	err = transport.Connect(ctx)
	if err != nil {
		logx.Fatal("Failed to connect to RabbitMQ", logx.ErrorField(err))
	}
	defer transport.Disconnect(ctx)

	// Create advanced publisher
	advancedPublisher := rabbitmq.NewAdvancedPublisher(transport, config.RabbitMQ.Publisher, obsCtx)

	// Create message persistence
	persistence, err := messaging.NewMessagePersistence(config.RabbitMQ.Persistence, obsCtx)
	if err != nil {
		logx.Fatal("Failed to create message persistence", logx.ErrorField(err))
	}
	advancedPublisher.SetPersistence(persistence)

	// Create message transformation
	transformation, err := messaging.NewMessageTransformation(config.RabbitMQ.Transformation, obsCtx)
	if err != nil {
		logx.Fatal("Failed to create message transformation", logx.ErrorField(err))
	}
	advancedPublisher.SetTransformation(transformation)

	// Create advanced routing
	routing, err := messaging.NewAdvancedRouting(config.RabbitMQ.Routing, obsCtx)
	if err != nil {
		logx.Fatal("Failed to create advanced routing", logx.ErrorField(err))
	}
	advancedPublisher.SetRouting(routing)

	// Create advanced consumer
	advancedConsumer := rabbitmq.NewAdvancedConsumer(transport, config.RabbitMQ.Consumer, obsCtx)

	// Create dead letter queue (using a wrapper to match the interface)
	dlq, err := messaging.NewDeadLetterQueue(config.RabbitMQ.DLQ, &transportWrapper{transport: transport}, obsCtx)
	if err != nil {
		logx.Fatal("Failed to create dead letter queue", logx.ErrorField(err))
	}
	advancedConsumer.SetDLQ(dlq)

	// Set transformation and routing for consumer
	advancedConsumer.SetTransformation(transformation)
	advancedConsumer.SetRouting(routing)

	// Start consumer
	err = advancedConsumer.Start(ctx, messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
		fmt.Printf("Processing message: %s\n", delivery.Message.ID)
		fmt.Printf("  Body: %s\n", string(delivery.Message.Body))
		fmt.Printf("  Priority: %d\n", delivery.Message.Priority)
		fmt.Printf("  Headers: %v\n", delivery.Message.Headers)

		// Simulate processing
		time.Sleep(100 * time.Millisecond)

		// Simulate occasional errors for DLQ testing
		if delivery.Message.ID == "error-msg" {
			return messaging.NackDLQ, fmt.Errorf("simulated processing error")
		}

		return messaging.Ack, nil
	}))
	if err != nil {
		logx.Fatal("Failed to start consumer", logx.ErrorField(err))
	}
	defer advancedConsumer.Stop(ctx)

	// Publish messages with advanced features
	messages := []struct {
		id       string
		body     string
		priority uint8
		headers  map[string]string
	}{
		{
			id:       "msg-1",
			body:     `{"type": "info", "message": "Hello World"}`,
			priority: 1,
			headers:  map[string]string{"content-type": "application/json"},
		},
		{
			id:       "msg-2",
			body:     `{"type": "warning", "message": "High priority message"}`,
			priority: 8,
			headers:  map[string]string{"content-type": "application/json", "priority": "high"},
		},
		{
			id:       "error-msg",
			body:     `{"type": "error", "message": "This will cause an error"}`,
			priority: 5,
			headers:  map[string]string{"content-type": "application/json"},
		},
	}

	for _, msg := range messages {
		message := messaging.NewMessage(
			[]byte(msg.body),
			messaging.WithID(msg.id),
			messaging.WithContentType("application/json"),
			messaging.WithKey("advanced.routing.key"),
			messaging.WithPriority(msg.priority),
			messaging.WithHeaders(msg.headers),
			messaging.WithIdempotencyKey(fmt.Sprintf("idempotency-%s", msg.id)),
			messaging.WithCorrelationID(fmt.Sprintf("correlation-%s", msg.id)),
		)

		receipt, err := advancedPublisher.PublishAsync(ctx, "advanced.exchange", *message)
		if err != nil {
			logx.Error("Failed to publish message",
				logx.String("message_id", msg.id),
				logx.ErrorField(err))
			continue
		}

		// Wait for confirmation
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			if err != nil {
				logx.Error("Publish failed for message",
					logx.String("message_id", msg.id),
					logx.ErrorField(err))
			} else {
				logx.Info("Successfully published message",
					logx.String("message_id", msg.id))
			}
		case <-time.After(5 * time.Second):
			logx.Warn("Publish timeout for message",
				logx.String("message_id", msg.id))
		}
	}

	// Wait for messages to be processed
	time.Sleep(2 * time.Second)

	// Demonstrate message persistence
	if persistence != nil {
		// Retrieve a message from persistence
		retrieved, err := persistence.Retrieve(ctx, "msg-1")
		if err != nil {
			logx.Error("Failed to retrieve message from persistence", logx.ErrorField(err))
		} else {
			logx.Info("Retrieved message from persistence",
				logx.String("message_id", retrieved.ID))
		}
	}

	// Demonstrate DLQ replay (if there are messages in DLQ)
	if dlq != nil {
		count, err := dlq.ReplayFromDLQ(ctx, "advanced.exchange", "advanced.routing.key", 10)
		if err != nil {
			logx.Error("Failed to replay from DLQ", logx.ErrorField(err))
		} else {
			logx.Info("Replayed messages from DLQ",
				logx.Int("count", count))
		}
	}

	fmt.Println("Advanced features example completed")
}
