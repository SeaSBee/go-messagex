# Go MessageX - Documentation

## Overview

Go MessageX is a comprehensive, transport-agnostic messaging library for Go applications. It provides a unified interface for working with different messaging systems while offering enterprise-grade features like observability, error handling, validation, and configuration management.

## Key Features

- **Transport Agnostic**: Unified interface for different messaging systems (RabbitMQ, Kafka, etc.)
- **Enterprise Ready**: Production-ready with observability, error handling, and validation
- **High Performance**: Efficient connection pooling, async operations, and resource management
- **Comprehensive Configuration**: Flexible configuration system with environment variable support
- **Observability**: Built-in metrics, tracing, and logging with correlation IDs
- **Error Handling**: Rich error model with categorization and recovery strategies
- **Thread Safe**: Thread-safe operations with proper concurrency handling
- **Extensible**: Plugin architecture for custom transports and handlers

## Architecture

The library follows a layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                   Messaging Interface                       │
├─────────────────────────────────────────────────────────────┤
│                  Transport Layer                            │
├─────────────────────────────────────────────────────────────┤
│                Observability Layer                          │
├─────────────────────────────────────────────────────────────┤
│                Configuration Layer                          │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Steps

1. [Step 1: Project Structure and Foundation](./step1-project-structure.md)
2. [Step 2: Configuration System](./step2-configuration-system.md)
3. [Step 3: Core Messaging Interfaces](./step3-core-interfaces.md)
4. [Step 4: Observability Foundation](./step4-observability-foundation.md)
5. [Step 5: Advanced Observability](./step5-telemetry-abstractions.md)
6. [Step 6: Error Model and Validation](./step6-error-model-validation.md)
7. [Step 7: RabbitMQ Core Implementation](./step7-rabbitmq-core-implementation.md)
8. [Step 9: Advanced Features Implementation](./step9-advanced-features.md)
9. [Step 10: Performance Optimizations and Benchmarking](./step10-performance-optimizations.md)
10. [Step 11: Testing Framework and Test Coverage](./step11-testing-framework.md)

## Quick Start

### Installation

```bash
go get github.com/SeaSBee/go-messagex
```

### Basic Usage

```go
package main

import (
    "context"
    
    "github.com/SeaSBee/go-logx"
    "github.com/SeaSBee/go-messagex/pkg/messaging"
    "github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

func main() {
    ctx := context.Background()
    
    // Create configuration
    config := &messaging.Config{
        Transport: "rabbitmq",
        RabbitMQ: &messaging.RabbitMQConfig{
            URIs: []string{"amqp://localhost:5672"},
        },
    }
    
    // Create publisher
    publisher, err := messaging.NewPublisher(ctx, config)
    if err != nil {
        logx.Fatal(err)
    }
    defer publisher.Close(ctx)
    
    // Create message
    msg := messaging.NewMessage(
        []byte(`{"key": "value"}`),
        messaging.WithID("msg-123"),
        messaging.WithContentType("application/json"),
        messaging.WithKey("routing.key"),
    )
    
    // Publish message
    receipt, err := publisher.PublishAsync(ctx, "exchange.name", msg)
    if err != nil {
        logx.Fatal(err)
    }
    
    // Wait for confirmation
    select {
    case <-receipt.Done():
        result, err := receipt.Result()
        if err != nil {
            logx.Errorf("Publish failed: %v", err)
        } else {
            logx.Info("Message published successfully")
        }
    case <-ctx.Done():
        logx.Warn("Publish timeout")
    }
}
```

## Supported Transports

### RabbitMQ
- Full AMQP 0.9.1 support
- Connection and channel pooling
- Publisher confirms
- Consumer acknowledgments
- Automatic topology declaration
- TLS/SSL support

### Future Transports
- Apache Kafka
- Apache Pulsar
- AWS SQS/SNS
- Google Cloud Pub/Sub
- Azure Service Bus

## Configuration

The library supports comprehensive configuration through YAML files, environment variables, and programmatic configuration:

```yaml
transport: "rabbitmq"
rabbitmq:
  uris:
    - "amqp://localhost:5672"
  
  connectionPool:
    min: 2
    max: 8
  
  publisher:
    confirms: true
    mandatory: true
  
  consumer:
    prefetch: 256
    autoAck: false
```

## Observability

Built-in observability features include:

- **Metrics**: Publish/consume rates, latency, error rates
- **Tracing**: Distributed tracing with correlation IDs
- **Logging**: Structured logging with context
- **Health Checks**: Connection and transport health monitoring

## Error Handling

Comprehensive error handling with:

- **Error Categorization**: Network, validation, configuration errors
- **Error Context**: Rich error context with details
- **Recovery Strategies**: Automatic retry and recovery
- **Error Reporting**: Integration with observability systems

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For support and questions:
- Create an issue on GitHub
- Check the documentation
- Review the examples

## Roadmap

- [ ] Apache Kafka transport
- [ ] Apache Pulsar transport
- [ ] AWS SQS/SNS transport
- [ ] Google Cloud Pub/Sub transport
- [ ] Azure Service Bus transport
- [ ] Message persistence
- [ ] Dead letter queues
- [ ] Message transformation
- [ ] Advanced routing
- [ ] Message compression
- [ ] Batch operations
- [ ] Stream processing
- [ ] Event sourcing support
- [ ] Saga pattern support
- [ ] Circuit breaker integration
- [ ] Rate limiting
- [ ] Message encryption
- [ ] Schema validation
- [ ] Message versioning
- [ ] Multi-region support
