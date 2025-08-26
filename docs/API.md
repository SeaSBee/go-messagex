# go-messagex API Documentation

## Overview

go-messagex is a production-grade, open-source Go module for asynchronous RabbitMQ messaging with extensibility to Kafka, featuring built-in observability, security, and fault tolerance.

## Table of Contents

1. [Installation](#installation)
2. [Quick Start](#quick-start)
3. [Core Concepts](#core-concepts)
4. [Configuration](#configuration)
5. [Publisher API](#publisher-api)
6. [Consumer API](#consumer-api)
7. [Transport API](#transport-api)
8. [Observability API](#observability-api)
9. [Error Handling](#error-handling)
10. [Testing API](#testing-api)
11. [Examples](#examples)
12. [Troubleshooting](#troubleshooting)

## Installation

```bash
go get github.com/seasbee/go-messagex
```

## Quick Start

### Basic Publisher

```go
package main

import (
    "context"
    
    "github.com/seasbee/go-logx"
    "github.com/seasbee/go-messagex/pkg/messaging"
    "github.com/seasbee/go-messagex/pkg/rabbitmq"
)

func main() {
    // Load configuration
    config := &messaging.Config{
        Transport: "rabbitmq",
        RabbitMQ: &messaging.RabbitMQConfig{
            URIs: []string{"amqp://localhost:5672/"},
            Publisher: &messaging.PublisherConfig{
                MaxInFlight: 1000,
                WorkerCount: 4,
            },
        },
    }
    
    // Create transport factory
    factory := &rabbitmq.TransportFactory{}
    
    // Create publisher
    publisher, err := factory.NewPublisher(context.Background(), config)
    if err != nil {
        logx.Fatal("Failed to create publisher", logx.Error(err))
    }
    defer publisher.Close(context.Background())
    
    // Publish message
    msg := messaging.NewMessage([]byte("Hello, World!"), messaging.WithKey("my.routing.key"))
    receipt := publisher.PublishAsync(context.Background(), "my.exchange", *msg)
    
    // Wait for confirmation
    <-receipt.Done()
    result, err := receipt.Result()
    if err != nil {
        logx.Error("Publish failed", logx.Error(err))
    } else {
        logx.Info("Message published successfully")
    }
}
```

### Basic Consumer

```go
package main

import (
    "context"
    
    "github.com/seasbee/go-logx"
    "github.com/seasbee/go-messagex/pkg/messaging"
    "github.com/seasbee/go-messagex/pkg/rabbitmq"
)

func main() {
    // Load configuration
    config := &messaging.Config{
        Transport: "rabbitmq",
        RabbitMQ: &messaging.RabbitMQConfig{
            URIs: []string{"amqp://localhost:5672/"},
            Consumer: &messaging.ConsumerConfig{
                Queue: "my.queue",
                Prefetch: 256,
            },
        },
    }
    
    // Create transport factory
    factory := &rabbitmq.TransportFactory{}
    
    // Create consumer
    consumer, err := factory.NewConsumer(context.Background(), config)
    if err != nil {
        logx.Fatal("Failed to create consumer", logx.Error(err))
    }
    defer consumer.Stop(context.Background())
    
    // Define message handler
    handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
        logx.Info("Received message", logx.String("body", string(delivery.Message.Body)))
        return messaging.Ack, nil
    })
    
    // Start consuming
    err = consumer.Start(context.Background(), handler)
    if err != nil {
        logx.Fatal("Failed to start consumer", logx.Error(err))
    }
    
    // Keep running
    select {}
}
```

## Core Concepts

### Message

A message is the basic unit of data transmitted through the messaging system.

```go
type Message struct {
    ID          string            `json:"id"`
    Body        []byte            `json:"body"`
    ContentType string            `json:"contentType"`
    Key         string            `json:"key"`
    Priority    uint8             `json:"priority"`
    Timestamp   time.Time         `json:"timestamp"`
    Headers     map[string]string `json:"headers"`
    CorrelationID string          `json:"correlationId"`
    IdempotencyKey string         `json:"idempotencyKey"`
}
```

### Receipt

A receipt represents the asynchronous result of a publish operation.

```go
type Receipt interface {
    Done() <-chan struct{}
    Result() (PublishResult, error)
}
```

### Delivery

A delivery represents a message received by a consumer.

```go
type Delivery interface {
    Message() Message
    Ack() error
    Nack(requeue bool) error
    Reject(requeue bool) error
}
```

## Configuration

### Main Configuration

```go
type Config struct {
    Transport string           `yaml:"transport" env:"MSG_TRANSPORT" default:"rabbitmq"`
    RabbitMQ  *RabbitMQConfig  `yaml:"rabbitmq"`
    Logging   *LoggingConfig   `yaml:"logging"`
    Telemetry *TelemetryConfig `yaml:"telemetry"`
}
```

### RabbitMQ Configuration

```go
type RabbitMQConfig struct {
    URIs            []string                `yaml:"uris" env:"MSG_RABBITMQ_URIS"`
    Publisher       *PublisherConfig        `yaml:"publisher"`
    Consumer        *ConsumerConfig         `yaml:"consumer"`
    ConnectionPool  *ConnectionPoolConfig   `yaml:"connectionPool"`
    ChannelPool     *ChannelPoolConfig      `yaml:"channelPool"`
    TLS             *TLSConfig              `yaml:"tls"`
    Security        *SecurityConfig         `yaml:"security"`
}
```

### Publisher Configuration

```go
type PublisherConfig struct {
    Confirms       bool         `yaml:"confirms" env:"MSG_RABBITMQ_PUBLISHER_CONFIRMS" default:"true"`
    Mandatory      bool         `yaml:"mandatory" env:"MSG_RABBITMQ_PUBLISHER_MANDATORY" default:"false"`
    MaxInFlight    int          `yaml:"maxInFlight" env:"MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT" validate:"min=1,max=100000" default:"10000"`
    WorkerCount    int          `yaml:"workerCount" env:"MSG_RABBITMQ_PUBLISHER_WORKERCOUNT" validate:"min=1,max=100" default:"4"`
    DropOnOverflow bool         `yaml:"dropOnOverflow" env:"MSG_RABBITMQ_PUBLISHER_DROPONOVERFLOW" default:"false"`
    PublishTimeout time.Duration `yaml:"publishTimeout" env:"MSG_RABBITMQ_PUBLISHER_TIMEOUT" default:"30s"`
    Retry          *RetryConfig `yaml:"retry"`
}
```

### Consumer Configuration

```go
type ConsumerConfig struct {
    Queue                 string        `yaml:"queue" env:"MSG_RABBITMQ_CONSUMER_QUEUE"`
    Prefetch              int           `yaml:"prefetch" env:"MSG_RABBITMQ_CONSUMER_PREFETCH" validate:"min=1,max=65535" default:"256"`
    MaxConcurrentHandlers int           `yaml:"maxConcurrentHandlers" env:"MSG_RABBITMQ_CONSUMER_MAXCONCURRENTHANDLERS" validate:"min=1,max=10000" default:"512"`
    RequeueOnError        bool          `yaml:"requeueOnError" env:"MSG_RABBITMQ_CONSUMER_REQUEUEONERROR" default:"true"`
    AckOnSuccess          bool          `yaml:"ackOnSuccess" env:"MSG_RABBITMQ_CONSUMER_ACKONSUCCESS" default:"true"`
    AutoAck               bool          `yaml:"autoAck" env:"MSG_RABBITMQ_CONSUMER_AUTOACK" default:"false"`
    HandlerTimeout        time.Duration `yaml:"handlerTimeout" env:"MSG_RABBITMQ_CONSUMER_HANDLERTIMEOUT" default:"30s"`
    PanicRecovery         bool          `yaml:"panicRecovery" env:"MSG_RABBITMQ_CONSUMER_PANICRECOVERY" default:"true"`
    MaxRetries            int           `yaml:"maxRetries" env:"MSG_RABBITMQ_CONSUMER_MAXRETRIES" validate:"min=0,max=10" default:"3"`
}
```

## Publisher API

### Creating a Publisher

```go
// Create publisher with configuration
publisher, err := rabbitmq.NewPublisher(ctx, config)
if err != nil {
    return err
}
defer publisher.Close(ctx)
```

### Publishing Messages

```go
// Create a message
msg := messaging.NewMessage(
    []byte("Hello, World!"),
    messaging.WithID("msg-123"),
    messaging.WithContentType("text/plain"),
    messaging.WithKey("my.key"),
    messaging.WithPriority(5),
    messaging.WithCorrelationID("corr-123"),
    messaging.WithIdempotencyKey("idemp-123"),
)

// Publish asynchronously
receipt, err := publisher.PublishAsync(ctx, "my.exchange", msg)
if err != nil {
    return err
}

// Wait for confirmation
<-receipt.Done()
result, err := receipt.Result()
if err != nil {
    return err
}
```

### Batch Publishing

```go
// Publish multiple messages
receipts := make([]messaging.Receipt, len(messages))
for i, msg := range messages {
    receipt, err := publisher.PublishAsync(ctx, "my.exchange", msg)
    if err != nil {
        return err
    }
    receipts[i] = receipt
}

// Wait for all confirmations
err := messaging.AwaitAll(ctx, receipts...)
if err != nil {
    return err
}
```

## Consumer API

### Creating a Consumer

```go
// Create consumer with configuration
consumer, err := rabbitmq.NewConsumer(ctx, config)
if err != nil {
    return err
}
defer consumer.Stop(ctx)
```

### Message Handler

```go
// Define message handler
handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
    // Process message
    msg := delivery.Message
    logx.Info("Processing message", logx.String("body", string(msg.Body)))
    
    // Acknowledge message
    return messaging.Ack, nil
})

// Start consuming
err = consumer.Start(ctx, handler)
if err != nil {
    return err
}
```

### Error Handling in Handlers

```go
handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
    msg := delivery.Message
    
    // Process message
    err := processMessage(msg)
    if err != nil {
        // Reject message (will be sent to DLQ if configured)
        return messaging.Reject, err
    }
    
    // Acknowledge successful processing
    return messaging.Ack, nil
})
```

## Transport API

### Creating a Transport

```go
// Create transport
transport := rabbitmq.NewTransport(config, observability)

// Connect to RabbitMQ
err := transport.Connect(ctx)
if err != nil {
    return err
}
defer transport.Disconnect(ctx)
```

### Creating Publishers and Consumers

```go
// Create publisher through transport
publisher := transport.NewPublisher(publisherConfig, observability)

// Create consumer through transport
consumer := transport.NewConsumer(consumerConfig, observability)
```

## Observability API

### Creating Observability Context

```go
// Create observability provider
provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
    MetricsEnabled: true,
    TracingEnabled: true,
    ServiceName:    "my-service",
})
if err != nil {
    return err
}

// Create observability context
obsCtx := messaging.NewObservabilityContext(ctx, provider)
```

### Logging

```go
// Get logger from observability context
logger := obsCtx.Logger()

// Log messages
logger.Info("Message published",
    logx.String("message_id", msg.ID),
    logx.String("exchange", exchange),
    logx.Int("body_size", len(msg.Body)),
)
```

### Metrics

```go
// Record metrics
obsCtx.RecordPublishMetrics("my_operation", "my.exchange", duration, success, errorMsg)
obsCtx.RecordConsumeMetrics("my_operation", "my.queue", duration, success, errorMsg)
```

### Tracing

```go
// Get tracer from observability context
tracer := obsCtx.Tracer()

// Create span
span := tracer.StartSpan("publish_message")
defer span.End()

// Add span attributes
span.SetAttributes(
    attribute.String("message.id", msg.ID),
    attribute.String("exchange", exchange),
)
```

## Error Handling

### Error Types

```go
// Messaging errors
err := messaging.NewError(messaging.ErrorCodePublish, "operation", "error message")

// Wrapped errors
wrappedErr := messaging.WrapError(messaging.ErrorCodeInternal, "operation", "wrapped", originalErr)

// Check error types
var msgErr *messaging.MessagingError
if errors.As(err, &msgErr) {
    logx.Error("Messaging error occurred",
        logx.String("error_code", string(msgErr.Code)),
        logx.String("operation", msgErr.Operation),
        logx.String("message", msgErr.Message),
    )
}
```

### Error Codes

- `ErrorCodeConnection`: Connection-related errors
- `ErrorCodeChannel`: Channel-related errors
- `ErrorCodePublish`: Publishing errors
- `ErrorCodeConsume`: Consumption errors
- `ErrorCodeInternal`: Internal errors
- `ErrorCodeValidation`: Validation errors
- `ErrorCodeTimeout`: Timeout errors

## Testing API

### Test Configuration

```go
// Create test configuration
config := messaging.NewTestConfig()
config.FailureRate = 0.1 // 10% failure rate
config.Latency = 10 * time.Millisecond

// Create mock transport
transport := messaging.NewMockTransport(config)
```

### Test Message Factory

```go
// Create test message factory
factory := messaging.NewTestMessageFactory()

// Create test messages
msg := factory.CreateMessage([]byte("test"))
jsonMsg := factory.CreateJSONMessage("test-data")
bulkMsgs := factory.CreateBulkMessages(10, `{"test": "message-%d"}`)
```

### Test Runner

```go
// Create test runner
runner := messaging.NewTestRunner(config)

// Run publish test
result, err := runner.RunPublishTest(ctx, publisher, 100)
if err != nil {
    return err
}

logx.Info("Test results",
    logx.Float64("success_rate", result.SuccessRate()),
    logx.Float64("throughput", result.Throughput),
)
```

## Examples

### Advanced Publisher with Retries

```go
config := &messaging.PublisherConfig{
    MaxInFlight: 1000,
    WorkerCount: 4,
    Retry: &messaging.RetryConfig{
        MaxAttempts:       3,
        BaseBackoff:       100 * time.Millisecond,
        MaxBackoff:        1 * time.Second,
        BackoffMultiplier: 2.0,
        Jitter:            true,
    },
}

publisher := rabbitmq.NewAsyncPublisher(transport, config, observability)
```

### Consumer with Dead Letter Queue

```go
config := &messaging.ConsumerConfig{
    Queue: "my.queue",
    Prefetch: 256,
    MaxConcurrentHandlers: 512,
    PanicRecovery: true,
    MaxRetries: 3,
}

// Configure DLQ
dlq := &messaging.DeadLetterQueue{
    Exchange: "dlx",
    RoutingKey: "my.queue.dlq",
    MaxRetries: 3,
}

consumer := rabbitmq.NewConcurrentConsumer(transport, config, observability)
consumer.SetDLQ(dlq)
```

### Performance Monitoring

```go
// Create performance monitor
perfConfig := &messaging.PerformanceConfig{
    EnableObjectPooling: true,
    ObjectPoolSize: 1000,
    EnableMemoryProfiling: true,
    PerformanceMetricsInterval: 10 * time.Second,
}

monitor := messaging.NewPerformanceMonitor(perfConfig, observability)
defer monitor.Close(ctx)

// Record performance metrics
monitor.RecordPublish(duration, success)
monitor.RecordConsume(duration, success)

// Get metrics
metrics := monitor.GetMetrics()
logx.Info("Performance metrics",
    logx.Float64("throughput", metrics.TotalThroughput),
    logx.Int64("latency_p95_ns", metrics.PublishLatencyP95),
)
```

## Troubleshooting

### Common Issues

#### Connection Issues

**Problem**: Cannot connect to RabbitMQ
```
Error: dial tcp localhost:5672: connect: connection refused
```

**Solution**: 
1. Ensure RabbitMQ is running
2. Check connection URI format
3. Verify network connectivity
4. Check firewall settings

#### Message Publishing Issues

**Problem**: Messages not being published
```
Error: queue full, message dropped
```

**Solution**:
1. Increase `MaxInFlight` setting
2. Increase `WorkerCount`
3. Enable `DropOnOverflow` if appropriate
4. Check RabbitMQ queue capacity

#### Consumer Issues

**Problem**: Consumer not receiving messages
```
Error: consumer not started
```

**Solution**:
1. Ensure consumer is started with `consumer.Start()`
2. Check queue name and routing
3. Verify exchange and binding configuration
4. Check consumer configuration

#### Performance Issues

**Problem**: Low throughput or high latency

**Solution**:
1. Increase worker count
2. Optimize message size
3. Use connection/channel pooling
4. Enable performance monitoring
5. Check RabbitMQ server performance

### Debugging

#### Enable Debug Logging

```go
config := &messaging.Config{
    Logging: &messaging.LoggingConfig{
        Level: "debug",
        JSON:  true,
    },
}
```

#### Performance Profiling

```go
perfConfig := &messaging.PerformanceConfig{
    EnableMemoryProfiling: true,
    EnableCPUProfiling: true,
    PerformanceMetricsInterval: 5 * time.Second,
}
```

#### Health Checks

```go
// Create health manager
healthManager := messaging.NewHealthManager()

// Add health checks
healthManager.AddCheck("rabbitmq", func() messaging.HealthStatus {
    // Check RabbitMQ connectivity
    return messaging.HealthStatusHealthy
})

// Get health report
report := healthManager.GetHealthReport()
if report.Status != messaging.HealthStatusHealthy {
    logx.Error("Health check failed", logx.Any("report", report))
}
```

### Best Practices

1. **Always use context with timeouts** for operations
2. **Handle errors properly** and implement retry logic
3. **Monitor performance** and set up alerts
4. **Use connection pooling** for better resource management
5. **Implement proper error handling** in message handlers
6. **Use observability features** for debugging and monitoring
7. **Test thoroughly** with mock transports
8. **Configure appropriate timeouts** for your use case
9. **Use dead letter queues** for failed message handling
10. **Monitor memory usage** and implement cleanup

### Performance Tuning

1. **Worker Count**: Adjust based on CPU cores and workload
2. **Queue Size**: Set `MaxInFlight` based on memory constraints
3. **Prefetch**: Optimize based on message processing time
4. **Connection Pool**: Use appropriate pool sizes
5. **Batch Processing**: Use batch operations when possible
6. **Message Size**: Optimize message payload size
7. **Compression**: Enable compression for large messages
8. **Monitoring**: Use performance metrics to identify bottlenecks
