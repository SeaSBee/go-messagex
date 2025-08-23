# Performance Tuning Guide

This guide provides comprehensive performance optimization strategies for go-messagex applications.

## Table of Contents

1. [Performance Targets](#performance-targets)
2. [Publisher Optimization](#publisher-optimization)
3. [Consumer Optimization](#consumer-optimization)
4. [Connection Pool Tuning](#connection-pool-tuning)
5. [Memory Management](#memory-management)
6. [Network Optimization](#network-optimization)
7. [Monitoring and Profiling](#monitoring-and-profiling)
8. [Benchmarking](#benchmarking)
9. [Common Performance Issues](#common-performance-issues)

## Performance Targets

### Baseline Targets
- **Throughput**: ≥ 50k messages/minute per process
- **Latency**: p95 publish confirm < 20ms on LAN
- **Memory**: < 100MB per 10k concurrent operations
- **CPU**: < 80% utilization under peak load

### High-Performance Targets
- **Throughput**: ≥ 100k messages/minute per process
- **Latency**: p95 publish confirm < 10ms on LAN
- **Memory**: < 50MB per 10k concurrent operations
- **CPU**: < 60% utilization under peak load

## Publisher Optimization

### Worker Pool Configuration

```go
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        Publisher: &messaging.PublisherConfig{
            // Optimal for high throughput
            WorkerCount: runtime.NumCPU() * 2,
            MaxInFlight: 10000,
            
            // For low latency
            PublishTimeout: 5 * time.Second,
            DropOnOverflow: false, // Block on overflow for consistency
            
            // Retry configuration
            Retry: &messaging.RetryConfig{
                MaxAttempts:       3,
                BaseBackoff:       50 * time.Millisecond,
                MaxBackoff:        500 * time.Millisecond,
                BackoffMultiplier: 2.0,
                Jitter:            true,
            },
        },
    },
}
```

### Batch Publishing

```go
// For high-throughput scenarios
func publishBatch(publisher messaging.Publisher, messages []messaging.Message) {
    receipts := make([]messaging.Receipt, len(messages))
    
    // Publish all messages asynchronously
    for i, msg := range messages {
        receipt, err := publisher.PublishAsync(ctx, "exchange", msg)
        if err != nil {
            log.Printf("Failed to queue message %d: %v", i, err)
            continue
        }
        receipts[i] = receipt
    }
    
    // Wait for all confirmations
    for i, receipt := range receipts {
        if receipt == nil {
            continue
        }
        
        select {
        case <-receipt.Done():
            result, err := receipt.Result()
            if err != nil {
                log.Printf("Message %d failed: %v", i, err)
            }
        case <-time.After(30 * time.Second):
            log.Printf("Message %d timed out", i)
        }
    }
}
```

### Message Optimization

```go
// Optimize message creation
msg := messaging.NewMessage(
    payload,
    messaging.WithID(generateOptimizedID()), // Use efficient ID generation
    messaging.WithContentType("application/json"),
    messaging.WithCompression(true), // Enable compression for large payloads
    messaging.WithPriority(priority),
)

// Reuse message objects for high-frequency publishing
type MessagePool struct {
    pool sync.Pool
}

func (mp *MessagePool) Get() *messaging.Message {
    if msg := mp.pool.Get(); msg != nil {
        return msg.(*messaging.Message)
    }
    return &messaging.Message{}
}

func (mp *MessagePool) Put(msg *messaging.Message) {
    // Reset message fields
    msg.Body = nil
    msg.Headers = nil
    mp.pool.Put(msg)
}
```

## Consumer Optimization

### Concurrency Configuration

```go
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        Consumer: &messaging.ConsumerConfig{
            // Optimal concurrency based on CPU cores
            MaxConcurrentHandlers: runtime.NumCPU() * 4,
            
            // Prefetch for optimal throughput
            Prefetch: 1000,
            
            // Handler timeout
            HandlerTimeout: 30 * time.Second,
            
            // Error handling
            RequeueOnError: true,
            MaxRetries:     3,
            
            // Panic recovery
            PanicRecovery: true,
        },
    },
}
```

### Efficient Message Processing

```go
// Optimize message handler
func optimizedHandler(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
    // Use object pools for frequently allocated objects
    var data map[string]interface{}
    if err := json.Unmarshal(delivery.Message.Body, &data); err != nil {
        return messaging.NackRequeue, err
    }
    
    // Process message efficiently
    result := processMessage(data)
    
    // Return early for success
    if result.Success {
        return messaging.Ack, nil
    }
    
    // Handle failures
    if result.Retryable {
        return messaging.NackRequeue, result.Error
    }
    
    return messaging.Nack, result.Error
}

// Use worker pools for CPU-intensive processing
type ProcessingPool struct {
    workers chan struct{}
}

func (pp *ProcessingPool) Process(ctx context.Context, data interface{}) error {
    select {
    case pp.workers <- struct{}{}:
        defer func() { <-pp.workers }()
        return processCPUIntensive(data)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Batch Processing

```go
// Batch message processing for efficiency
type BatchProcessor struct {
    batchSize int
    timeout   time.Duration
    messages  chan messaging.Delivery
    processor func([]messaging.Delivery) error
}

func (bp *BatchProcessor) Start(ctx context.Context) {
    ticker := time.NewTicker(bp.timeout)
    defer ticker.Stop()
    
    var batch []messaging.Delivery
    
    for {
        select {
        case msg := <-bp.messages:
            batch = append(batch, msg)
            if len(batch) >= bp.batchSize {
                bp.processBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                bp.processBatch(batch)
                batch = batch[:0]
            }
        case <-ctx.Done():
            return
        }
    }
}
```

## Connection Pool Tuning

### Optimal Pool Sizes

```go
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        ConnectionPool: &messaging.ConnectionPoolConfig{
            // Base pool size on expected load
            Min: 2,
            Max: runtime.NumCPU() * 2,
            
            // Health check intervals
            HealthCheckInterval: 30 * time.Second,
            ConnectionTimeout:   10 * time.Second,
            HeartbeatInterval:   10 * time.Second,
        },
        ChannelPool: &messaging.ChannelPoolConfig{
            // Channels per connection
            PerConnectionMin: 10,
            PerConnectionMax: 100,
            
            // Borrow timeout
            BorrowTimeout: 5 * time.Second,
            HealthCheckInterval: 30 * time.Second,
        },
    },
}
```

### Pool Monitoring

```go
// Monitor pool health
func monitorPoolHealth(transport *rabbitmq.Transport) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := transport.GetPoolStats()
        
        // Log pool metrics
        logx.Info("Pool health",
            logx.Int("active_connections", stats.ActiveConnections),
            logx.Int("idle_connections", stats.IdleConnections),
            logx.Int("total_channels", stats.TotalChannels),
            logx.Int("borrowed_channels", stats.BorrowedChannels),
        )
        
        // Alert on pool exhaustion
        if stats.ActiveConnections >= stats.MaxConnections*9/10 {
            logx.Warn("Connection pool nearly exhausted",
                logx.Int("active", stats.ActiveConnections),
                logx.Int("max", stats.MaxConnections),
            )
        }
    }
}
```

## Memory Management

### Message Pooling

```go
// Object pools for frequently allocated objects
var (
    messagePool = sync.Pool{
        New: func() interface{} {
            return &messaging.Message{}
        },
    }
    
    deliveryPool = sync.Pool{
        New: func() interface{} {
            return &messaging.Delivery{}
        },
    }
)

// Use pools in handlers
func pooledHandler(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
    // Get message from pool
    msg := messagePool.Get().(*messaging.Message)
    defer messagePool.Put(msg)
    
    // Process message
    // ...
    
    return messaging.Ack, nil
}
```

### Memory Profiling

```go
// Enable memory profiling
import _ "net/http/pprof"

func main() {
    // Start pprof server
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Your application code
    // ...
}

// Profile memory usage
func profileMemory() {
    f, err := os.Create("memory.prof")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    
    pprof.WriteHeapProfile(f)
}
```

## Network Optimization

### Connection Optimization

```go
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        URIs: []string{
            "amqp://localhost:5672/",
            "amqp://localhost:5673/", // Fallback
        },
        
        // Network timeouts
        ConnectionTimeout: 10 * time.Second,
        HeartbeatInterval: 10 * time.Second,
        
        // TLS optimization
        TLS: &messaging.TLSConfig{
            Enabled: true,
            // Use modern cipher suites
            MinVersion: tls.VersionTLS12,
        },
    },
}
```

### Message Compression

```go
// Enable compression for large messages
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        Publisher: &messaging.PublisherConfig{
            Serialization: &messaging.SerializationConfig{
                CompressionEnabled: true,
                CompressionLevel:   6, // Balance between speed and size
            },
        },
    },
}

// Use compression for large payloads
func createCompressedMessage(payload []byte) messaging.Message {
    if len(payload) > 1024 { // Compress messages > 1KB
        return messaging.NewMessage(
            payload,
            messaging.WithCompression(true),
        )
    }
    
    return messaging.NewMessage(payload)
}
```

## Monitoring and Profiling

### Performance Metrics

```go
// Custom performance metrics
type PerformanceMetrics struct {
    PublishLatency    prometheus.Histogram
    ConsumeLatency    prometheus.Histogram
    MessageSize       prometheus.Histogram
    ErrorRate         prometheus.Counter
    Throughput        prometheus.Counter
}

func (pm *PerformanceMetrics) RecordPublish(duration time.Duration, size int) {
    pm.PublishLatency.Observe(duration.Seconds())
    pm.MessageSize.Observe(float64(size))
    pm.Throughput.Inc()
}

func (pm *PerformanceMetrics) RecordConsume(duration time.Duration) {
    pm.ConsumeLatency.Observe(duration.Seconds())
}

func (pm *PerformanceMetrics) RecordError() {
    pm.ErrorRate.Inc()
}
```

### Real-time Monitoring

```go
// Monitor performance in real-time
func monitorPerformance(publisher messaging.Publisher, consumer messaging.Consumer) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // Publisher stats
        if pub, ok := publisher.(*rabbitmq.AsyncPublisher); ok {
            stats := pub.GetStats()
            logx.Info("Publisher performance",
                logx.Uint64("queued", stats.TasksQueued),
                logx.Uint64("processed", stats.TasksProcessed),
                logx.Uint64("failed", stats.TasksFailed),
                logx.Uint64("dropped", stats.TasksDropped),
            )
        }
        
        // Consumer stats
        if con, ok := consumer.(*rabbitmq.ConcurrentConsumer); ok {
            stats := con.GetStats()
            logx.Info("Consumer performance",
                logx.Uint64("processed", stats.MessagesProcessed),
                logx.Uint64("failed", stats.MessagesFailed),
                logx.Int("active_workers", stats.ActiveWorkers),
                logx.Int("queued_tasks", stats.QueuedTasks),
            )
        }
    }
}
```

## Benchmarking

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./pkg/rabbitmq/...

# Run specific benchmarks
go test -bench=BenchmarkPublisher ./pkg/rabbitmq/...

# Run with memory profiling
go test -bench=. -memprofile=mem.prof ./pkg/rabbitmq/...

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./pkg/rabbitmq/...

# Run benchmarks with different configurations
go test -bench=. -benchmem ./pkg/rabbitmq/...
```

### Benchmark Examples

```go
// Publisher benchmark
func BenchmarkPublisher(b *testing.B) {
    config := createTestConfig()
    publisher, err := rabbitmq.NewPublisher(context.Background(), config)
    if err != nil {
        b.Fatal(err)
    }
    defer publisher.Close(context.Background())
    
    msg := messaging.NewMessage([]byte("test message"))
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            receipt, err := publisher.PublishAsync(context.Background(), "test.exchange", msg)
            if err != nil {
                b.Fatal(err)
            }
            
            <-receipt.Done()
            if _, err := receipt.Result(); err != nil {
                b.Fatal(err)
            }
        }
    })
}

// Consumer benchmark
func BenchmarkConsumer(b *testing.B) {
    config := createTestConfig()
    consumer, err := rabbitmq.NewConsumer(context.Background(), config)
    if err != nil {
        b.Fatal(err)
    }
    defer consumer.Stop(context.Background())
    
    handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
        return messaging.Ack, nil
    })
    
    err = consumer.Start(context.Background(), handler)
    if err != nil {
        b.Fatal(err)
    }
    
    b.ResetTimer()
    // Benchmark consumer processing
    // ...
}
```

## Common Performance Issues

### High Memory Usage

**Symptoms:**
- Memory usage growing over time
- Frequent garbage collection
- Out of memory errors

**Solutions:**
- Use object pools for frequently allocated objects
- Enable message compression
- Monitor and limit message sizes
- Profile memory usage with pprof

### Low Throughput

**Symptoms:**
- Messages queuing up
- High latency
- Worker pool exhaustion

**Solutions:**
- Increase worker pool size
- Optimize message processing
- Use batch processing
- Enable connection pooling

### High Latency

**Symptoms:**
- Slow message delivery
- Timeout errors
- Network delays

**Solutions:**
- Optimize network configuration
- Use connection pooling
- Enable message compression
- Monitor network metrics

### Connection Issues

**Symptoms:**
- Connection timeouts
- Pool exhaustion
- Network errors

**Solutions:**
- Increase connection pool size
- Optimize connection timeouts
- Use connection health checks
- Implement retry mechanisms

## Performance Checklist

Before deploying to production:

- [ ] Run performance benchmarks
- [ ] Profile memory usage
- [ ] Monitor CPU utilization
- [ ] Test with expected load
- [ ] Configure appropriate pool sizes
- [ ] Enable compression for large messages
- [ ] Set up monitoring and alerting
- [ ] Document performance baselines
- [ ] Plan for scaling
- [ ] Test failure scenarios
