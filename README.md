# go-messagex

A production-grade, open-source Go module for asynchronous RabbitMQ messaging with extensibility to Kafka, featuring built-in observability, security, and fault tolerance.

## 🚀 Features

- **Async, Non-blocking Operations**: All network operations run in goroutines
- **Connection/Channel Pooling**: Efficient resource management with auto-healing
- **Built-in Observability**: Structured logging with `go-logx` and OpenTelemetry metrics/tracing
- **Security-First**: TLS/mTLS support with proper secret management
- **Production-Hardened**: Dead Letter Queues, priority messaging, graceful shutdown
- **Transport-Agnostic**: Core interfaces designed for Kafka extensibility
- **Configuration Management**: YAML + ENV with ENV precedence

## 📦 Installation

```bash
go get github.com/seasbee/go-messagex
```

## 🏗️ Architecture

### Core Principles
- **Transport-agnostic core** with pluggable implementations
- **Non-blocking async operations** with proper backpressure
- **Connection/channel pooling** with auto-healing
- **Built-in observability** (go-logx + OpenTelemetry)
- **Security-first** (TLS/mTLS, secret management)
- **Production-hardened** (DLQ, priority, graceful shutdown)

### Project Structure
```
/messaging/
  ├── go.mod
  ├── LICENSE
  ├── README.md
  ├── CONTRIBUTING.md
  ├── CODEOWNERS
  ├── Makefile
  ├── .golangci.yml
  ├── /cmd/
  │    ├── publisher/        # example CLI publisher
  │    └── consumer/         # example CLI consumer
  ├── /pkg/
  │    ├── messaging/        # transport-agnostic interfaces & types
  │    ├── rabbitmq/         # RabbitMQ implementation
  │    └── kafka/            # (stub) extension point for future Kafka impl
  ├── /configs/
  │    └── messaging.example.yaml
  ├── /internal/
  │    └── configloader/     # YAML+ENV loader
  └── /tests/unit/ 
       ├── rabbitmq_publisher_test.go
       ├── rabbitmq_consumer_test.go
       └── amqp_pool_test.go
```

## 🔧 Quick Start

### Publisher Example
```go
package main

import (
    "context"
    "time"
    
    "github.com/seasbee/go-messagex/pkg/rabbitmq"
    "github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
    ctx := context.Background()
    
    // Load configuration
    cfg := loadConfig()
    
    // Create publisher
    pub, err := rabbitmq.NewPublisher(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer pub.Close(ctx)
    
    // Publish message asynchronously
    msg := messaging.NewJSONMessage("events.user.created", payload,
        messaging.WithID("uuid-..."),
        messaging.WithPriority(7),
        messaging.WithIdempotencyKey("idemp-..."),
    )
    
    receipt := pub.PublishAsync(ctx, "app.topic", msg)
    
    select {
    case <-receipt.Done():
        result, err := receipt.Result()
        if err != nil {
            log.Printf("Publish failed: %v", err)
        } else {
            log.Printf("Message published successfully")
        }
    case <-time.After(time.Second):
        log.Printf("Publish timeout")
    }
}
```

### Consumer Example
```go
package main

import (
    "context"
    
    "github.com/seasbee/go-messagex/pkg/rabbitmq"
    "github.com/seasbee/go-messagex/pkg/messaging"
)

func main() {
    ctx := context.Background()
    
    // Load configuration
    cfg := loadConfig()
    
    // Create consumer
    consumer, err := rabbitmq.NewConsumer(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer consumer.Stop(ctx)
    
    // Start consuming
    err = consumer.Start(ctx, messaging.HandlerFunc(func(ctx context.Context, d messaging.Delivery) (messaging.AckDecision, error) {
        // Process message safely; must be idempotent
        log.Printf("Processing message: %s", string(d.Body))
        return messaging.Ack, nil
    }))
    if err != nil {
        log.Fatal(err)
    }
    
    // Keep running
    select {}
}
```

## ⚙️ Configuration

### YAML Configuration
```yaml
transport: rabbitmq
rabbitmq:
  uris: ["amqps://user:pass@rmq-1:5671/vhost", "amqps://user:pass@rmq-2:5671/vhost"]
  connectionPool:
    min: 2
    max: 8
  channelPool:
    perConnectionMin: 10
    perConnectionMax: 100
  topology:
    exchanges:
      - name: app.topic
        type: topic
        durable: true
    queues:
      - name: app.events
        durable: true
        args:
          x-dead-letter-exchange: app.dlx
          x-max-priority: 10
    bindings:
      - exchange: app.topic
        queue: app.events
        key: "events.*"
  publisher:
    confirms: true
    mandatory: true
    maxInFlight: 10000
    dropOnOverflow: false
    retry:
      maxAttempts: 5
      baseBackoffMs: 100
      maxBackoffMs: 5000
  consumer:
    queue: app.events
    prefetch: 256
    maxConcurrentHandlers: 512
  tls:
    enabled: true
    caFile: /etc/ssl/ca.pem
    certFile: /etc/ssl/client.crt
    keyFile: /etc/ssl/client.key
logging:
  level: info
  json: true
```

### Environment Variables
Environment variables override YAML configuration:
```bash
export MSG_TRANSPORT=rabbitmq
export MSG_RABBITMQ_URIS=amqps://user:pass@host:5671/vh1,amqps://user:pass@host:5671/vh2
export MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT=20000
export MSG_RABBITMQ_CONSUMER_PREFETCH=512
export MSG_LOGGING_LEVEL=debug
```

## 🔒 Security

- **TLS/mTLS**: End-to-end encryption with certificate management
- **Hostname Verification**: Enabled by default for security
- **Secret Management**: Credentials only via environment variables
- **Message Signing**: Optional HMAC verification support
- **Principle of Least Privilege**: Minimal required permissions

For comprehensive security guidance, see [Security Guide](docs/SECURITY.md).

## 📊 Observability

### Structured Logging
Direct integration with `go-logx` for consistent structured logging:
- `transport`, `exchange`, `routing_key`, `queue`, `delivery_tag`
- `attempt`, `latency_ms`, `size_bytes`, `result`, `error`
- `conn_id`, `chan_id`, `idempotency_key`, `correlation_id`

### Metrics (OpenTelemetry)
- **Counters**: `messaging_publish_total`, `messaging_consume_total`, `messaging_failures_total`
- **Gauges**: `messaging_pool_connections_active`, `messaging_publish_inflight`
- **Histograms**: `messaging_publish_duration_ms`, `messaging_consume_duration_ms`

### Distributed Tracing
- Automatic span propagation via AMQP headers
- Trace context preservation across message boundaries
- Performance monitoring and debugging support

## 🚀 Performance

### Performance Targets
- **Throughput**: ≥ 50k messages/minute per process
- **Latency**: p95 publish confirm < 20ms on LAN
- **Memory**: < 100MB per 10k concurrent operations
- **CPU**: < 80% utilization under peak load

### Performance Optimization
For detailed performance tuning guidance, see [Performance Guide](docs/PERFORMANCE.md).

## 🔒 Security

### Security Features
- **TLS/mTLS**: End-to-end encryption with certificate management
- **Hostname Verification**: Enabled by default for security
- **Secret Management**: Credentials only via environment variables
- **Message Signing**: Optional HMAC verification support
- **Principle of Least Privilege**: Minimal required permissions

### Security Best Practices
For comprehensive security guidance, see [Security Guide](docs/SECURITY.md).

## 🚀 Performance Targets

- **Throughput**: ≥ 50k msgs/minute per process
- **Latency**: p95 publish confirm under 20ms on LAN
- **Reliability**: Graceful shutdown with in-flight message handling

For detailed performance optimization strategies, see [Performance Guide](docs/PERFORMANCE.md).

## 🧪 Testing

```bash
# Run all tests with race detector
make test

# Run specific test suites
go test -race ./pkg/rabbitmq/...
go test -race ./internal/configloader/...

# Run benchmarks
go test -bench=. ./pkg/rabbitmq/...
```

## 🤝 Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔮 Roadmap

- [ ] Kafka transport implementation
- [ ] Message compression support
- [ ] Advanced routing patterns
- [ ] Message persistence strategies
- [ ] Cluster-aware load balancing

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/seasbee/go-messagex/issues)
- **Security**: [SECURITY.md](SECURITY.md)
- **Documentation**: [docs/](docs/)
- **Troubleshooting**: [Troubleshooting Guide](docs/TROUBLESHOOTING.md)
- **API Reference**: [API Documentation](docs/API.md)
- **CLI Applications**: [CLI Guide](cmd/README.md)

---

**Built with ❤️ by the SeaSBee team**