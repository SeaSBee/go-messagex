# RabbitMQ Messaging Client

A high-performance, feature-rich RabbitMQ client built with Go using the `github.com/wagslane/go-rabbitmq` library.

## Features

- **Easy Configuration**: Simple configuration with validation using `go-validatorx`
- **Message Batching**: Efficient batch processing for high-throughput scenarios
- **Health Monitoring**: Built-in health checks and monitoring
- **Error Handling**: Comprehensive error handling with custom error types
- **Message Types**: Support for text, JSON, and custom message formats
- **Priority Support**: Message priority handling
- **Persistent Messages**: Configurable message persistence
- **Connection Pooling**: Efficient connection and channel management
- **Statistics**: Detailed statistics for monitoring and debugging

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    
    "github.com/seasbee/go-logx"
    "github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
    // Create a new RabbitMQ client
    client, err := messaging.NewClient(nil) // Uses default config
    if err != nil {
        logx.Fatal("Failed to create client", logx.ErrorField(err))
    }
    defer client.Close()

    // Wait for connection
    if err := client.WaitForConnection(30 * time.Second); err != nil {
        logx.Fatal("Failed to connect", logx.ErrorField(err))
    }

    // Publish a message
    msg := messaging.NewTextMessage("Hello, RabbitMQ!")
    msg.SetRoutingKey("test.queue")
    
    if err := client.Publish(context.Background(), msg); err != nil {
        logx.Fatal("Failed to publish message", logx.ErrorField(err))
    }

    // Consume messages
    handler := func(delivery *messaging.Delivery) error {
        logx.Info("Received message", logx.String("body", string(delivery.Message.Body)))
        return delivery.Acknowledger.Ack()
    }

    if err := client.Consume(context.Background(), "test.queue", handler); err != nil {
        logx.Fatal("Failed to start consuming", logx.ErrorField(err))
    }
}
```

### Custom Configuration

```go
config := &messaging.Config{
    URL: "amqp://user:pass@localhost:5672/",
    MaxRetries: 5,
    RetryDelay: 2 * time.Second,
    ProducerConfig: messaging.ProducerConfig{
        BatchSize: 100,
        BatchTimeout: 1 * time.Second,
        ConfirmMode: true,
    },
    ConsumerConfig: messaging.ConsumerConfig{
        AutoAck: false,
        PrefetchCount: 10,
        QueueDurable: true,
    },
}

client, err := messaging.NewClient(config)
```

### Batch Processing

```go
// Create a batch of messages
var messages []*messaging.Message
for i := 0; i < 100; i++ {
    msg := messaging.NewTextMessage(fmt.Sprintf("Message %d", i))
    msg.SetRoutingKey("batch.queue")
    messages = append(messages, msg)
}

batch := messaging.NewBatchMessage(messages)
if err := client.PublishBatch(context.Background(), batch); err != nil {
    logx.Fatal("Failed to publish batch", logx.ErrorField(err))
}
```

### JSON Messages

```go
userData := map[string]interface{}{
    "name": "John Doe",
    "email": "john@example.com",
    "age": 30,
}

msg, err := messaging.NewJSONMessage(userData)
if err != nil {
    logx.Fatal("Failed to create JSON message", logx.ErrorField(err))
}

msg.SetRoutingKey("user.queue")
client.Publish(context.Background(), msg)
```

### Health Monitoring

```go
healthChecker := client.GetHealthChecker()

// Check health status
if healthChecker.IsHealthy() {
    logx.Info("RabbitMQ is healthy")
}

// Get statistics
stats := healthChecker.GetStatsMap()
logx.Info("Health stats", logx.Any("stats", stats))

// Set health callback
healthChecker.SetHealthCallback(func(status messaging.HealthStatus, err error) {
    logx.Info("Health status changed", logx.String("status", string(status)))
    if err != nil {
        logx.Error("Health check error", logx.ErrorField(err))
    }
})
```

## Configuration Options

### Client Configuration

- `URL`: RabbitMQ connection URL
- `MaxRetries`: Maximum number of connection retries
- `RetryDelay`: Delay between retry attempts
- `ConnectionTimeout`: Connection timeout duration
- `MaxConnections`: Maximum number of connections
- `MaxChannels`: Maximum number of channels
- `MetricsEnabled`: Enable metrics collection
- `HealthCheckInterval`: Health check interval

### Producer Configuration

- `BatchSize`: Number of messages to batch together
- `BatchTimeout`: Maximum time to wait before flushing batch
- `PublishTimeout`: Timeout for publish operations
- `Mandatory`: Make publishing mandatory
- `Immediate`: Make publishing immediate
- `ConfirmMode`: Enable publisher confirmations
- `DefaultExchange`: Default exchange name
- `DefaultRoutingKey`: Default routing key

### Consumer Configuration

- `AutoCommit`: Enable auto-commit
- `CommitInterval`: Commit interval for auto-commit
- `AutoAck`: Enable auto-acknowledgment
- `Exclusive`: Exclusive consumer
- `NoLocal`: No local delivery
- `NoWait`: No wait for operations
- `PrefetchCount`: Prefetch count for QoS
- `PrefetchSize`: Prefetch size for QoS
- `ConsumerTag`: Consumer tag
- `QueueDurable`: Make queue durable
- `QueueAutoDelete`: Auto-delete queue
- `QueueExclusive`: Exclusive queue
- `ExchangeDurable`: Make exchange durable
- `ExchangeAutoDelete`: Auto-delete exchange
- `ExchangeType`: Exchange type (direct, fanout, topic, headers)

## Error Handling

The client provides comprehensive error handling with custom error types:

- `ConnectionError`: Connection-related errors
- `PublishError`: Message publishing errors
- `ConsumeError`: Message consumption errors
- `ValidationError`: Configuration validation errors
- `TimeoutError`: Timeout errors
- `BatchError`: Batch processing errors

## Examples

See the `examples/` directory for complete working examples:

- `rabbitmq_example.go`: Comprehensive example showing all features

## Dependencies

- `github.com/wagslane/go-rabbitmq`: RabbitMQ client library
- `github.com/seasbee/go-validatorx`: Configuration validation
- `github.com/creasty/defaults`: Default value setting
- `github.com/google/uuid`: UUID generation

## License

This project is licensed under the MIT License.
