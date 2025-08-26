# Troubleshooting Guide

This guide provides solutions to common issues encountered when using go-messagex.

## Table of Contents

1. [Connection Issues](#connection-issues)
2. [Publisher Issues](#publisher-issues)
3. [Consumer Issues](#consumer-issues)
4. [Performance Issues](#performance-issues)
5. [Configuration Issues](#configuration-issues)
6. [Error Handling](#error-handling)
7. [Debugging Techniques](#debugging-techniques)
8. [Monitoring and Metrics](#monitoring-and-metrics)

## Connection Issues

### Cannot Connect to RabbitMQ

**Symptoms:**
- `dial tcp localhost:5672: connect: connection refused`
- `connection reset by peer`
- `timeout waiting for connection`

**Diagnosis:**
```bash
# Check if RabbitMQ is running
rabbitmqctl status

# Check if port is accessible
telnet localhost 5672

# Check RabbitMQ logs
tail -f /var/log/rabbitmq/rabbit@hostname.log
```

**Solutions:**

1. **RabbitMQ Not Running**
   ```bash
   # Start RabbitMQ
   sudo systemctl start rabbitmq-server
   
   # Enable auto-start
   sudo systemctl enable rabbitmq-server
   ```

2. **Incorrect Connection URI**
   ```go
   // Correct format
   config := &messaging.Config{
       RabbitMQ: &messaging.RabbitMQConfig{
           URIs: []string{"amqp://username:password@localhost:5672/vhost"},
       },
   }
   ```

3. **Authentication Issues**
   ```bash
   # Create user
   rabbitmqctl add_user myuser mypassword
   rabbitmqctl set_permissions -p / myuser ".*" ".*" ".*"
   ```

4. **Network/Firewall Issues**
   ```bash
   # Check firewall
   sudo ufw status
   
   # Allow RabbitMQ port
   sudo ufw allow 5672
   ```

### Connection Pool Issues

**Symptoms:**
- `connection pool exhausted`
- `timeout waiting for connection from pool`

**Solutions:**

1. **Increase Pool Size**
   ```go
   config := &messaging.Config{
       RabbitMQ: &messaging.RabbitMQConfig{
           ConnectionPool: &messaging.ConnectionPoolConfig{
               MinConnections: 5,
               MaxConnections: 20,
           },
       },
   }
   ```

2. **Check Connection Leaks**
   ```go
   // Always close connections
   defer transport.Disconnect(ctx)
   defer publisher.Close(ctx)
   defer consumer.Stop(ctx)
   ```

## Publisher Issues

### Messages Not Being Published

**Symptoms:**
- `queue full, message dropped`
- `publisher is closed`
- No confirmations received

**Diagnosis:**
```go
// Check publisher stats
stats := publisher.GetStats()
logx.Info("Publisher stats", 
    logx.Int("tasks_queued", stats.TasksQueued),
    logx.Int("tasks_dropped", stats.TasksDropped),
    logx.Int("tasks_processed", stats.TasksProcessed))
```

**Solutions:**

1. **Queue Full**
   ```go
   // Increase queue size
   config := &messaging.PublisherConfig{
       MaxInFlight: 5000, // Increase from default 10000
       WorkerCount: 8,    // Increase from default 4
   }
   ```

2. **Enable Drop on Overflow**
   ```go
   config := &messaging.PublisherConfig{
       DropOnOverflow: true, // Drop messages instead of blocking
   }
   ```

3. **Check Receipt Confirmations**
   ```go
   receipt, err := publisher.PublishAsync(ctx, exchange, msg)
   if err != nil {
       return err
   }
   
   // Wait for confirmation
   <-receipt.Done()
   result, err := receipt.Result()
   if err != nil {
       logx.Error("Publish failed", logx.Error(err))
   }
   ```

### Publisher Performance Issues

**Symptoms:**
- Low throughput
- High latency
- Memory usage growing

**Solutions:**

1. **Optimize Worker Configuration**
   ```go
   config := &messaging.PublisherConfig{
       WorkerCount: runtime.NumCPU() * 2, // Scale with CPU cores
       MaxInFlight: 10000,
   }
   ```

2. **Use Batch Publishing**
   ```go
   // Publish multiple messages efficiently
   receipts := make([]messaging.Receipt, len(messages))
   for i, msg := range messages {
       receipt, err := publisher.PublishAsync(ctx, exchange, msg)
       if err != nil {
           return err
       }
       receipts[i] = receipt
   }
   
   // Wait for all confirmations
   err := messaging.AwaitAll(ctx, receipts...)
   ```

3. **Enable Performance Monitoring**
   ```go
   perfConfig := &messaging.PerformanceConfig{
       EnableObjectPooling: true,
       ObjectPoolSize: 1000,
       PerformanceMetricsInterval: 10 * time.Second,
   }
   
   monitor := messaging.NewPerformanceMonitor(perfConfig, observability)
   defer monitor.Close(ctx)
   ```

## Consumer Issues

### Consumer Not Receiving Messages

**Symptoms:**
- No messages processed
- Consumer appears idle
- Queue has messages but consumer not processing

**Diagnosis:**
```go
// Check consumer stats
stats := consumer.GetStats()
logx.Info("Consumer stats", 
    logx.Int("messages_processed", stats.MessagesProcessed),
    logx.Int("messages_failed", stats.MessagesFailed),
    logx.Int("active_workers", stats.ActiveWorkers))
```

**Solutions:**

1. **Consumer Not Started**
   ```go
   // Ensure consumer is started
   err := consumer.Start(ctx, handler)
   if err != nil {
       return err
   }
   
   // Keep consumer running
   select {}
   ```

2. **Incorrect Queue Configuration**
   ```go
   config := &messaging.ConsumerConfig{
       Queue: "my.queue", // Ensure queue name is correct
       Prefetch: 256,     // Adjust based on processing speed
   }
   ```

3. **Handler Errors**
   ```go
   handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
       // Add logging to debug
       logx.Info("Processing message", logx.String("message_id", delivery.Message.ID))
       
       // Process message
       err := processMessage(delivery.Message)
       if err != nil {
           logx.Error("Handler error", logx.Error(err))
           return messaging.Reject, err
       }
       
       return messaging.Ack, nil
   })
   ```

### Consumer Performance Issues

**Symptoms:**
- Slow message processing
- High memory usage
- Worker starvation

**Solutions:**

1. **Optimize Concurrency**
   ```go
   config := &messaging.ConsumerConfig{
       MaxConcurrentHandlers: runtime.NumCPU() * 4, // Scale with CPU
       Prefetch: 512, // Increase for faster processing
   }
   ```

2. **Add Timeouts**
   ```go
   config := &messaging.ConsumerConfig{
       HandlerTimeout: 30 * time.Second, // Prevent hanging handlers
   }
   ```

3. **Enable Panic Recovery**
   ```go
   config := &messaging.ConsumerConfig{
       PanicRecovery: true, // Recover from panics
       MaxRetries: 3,       // Retry failed messages
   }
   ```

## Performance Issues

### Low Throughput

**Symptoms:**
- Messages per second below expected
- High latency
- Resource underutilization

**Diagnosis:**
```go
// Monitor performance metrics
metrics := monitor.GetMetrics()
logx.Info("Performance metrics", 
    logx.Float64("throughput_msg_per_sec", metrics.TotalThroughput),
    logx.Int64("latency_p95_ns", metrics.PublishLatencyP95),
    logx.Int("memory_usage_mb", metrics.MemoryUsageMB))
```

**Solutions:**

1. **Optimize Configuration**
   ```go
   // Publisher optimization
   publisherConfig := &messaging.PublisherConfig{
       WorkerCount: runtime.NumCPU() * 2,
       MaxInFlight: 20000,
       DropOnOverflow: false,
   }
   
   // Consumer optimization
   consumerConfig := &messaging.ConsumerConfig{
       MaxConcurrentHandlers: runtime.NumCPU() * 4,
       Prefetch: 1024,
       HandlerTimeout: 10 * time.Second,
   }
   ```

2. **Use Connection Pooling**
   ```go
   config := &messaging.Config{
       RabbitMQ: &messaging.RabbitMQConfig{
           ConnectionPool: &messaging.ConnectionPoolConfig{
               MinConnections: 10,
               MaxConnections: 50,
               ConnectionTimeout: 30 * time.Second,
           },
           ChannelPool: &messaging.ChannelPoolConfig{
               MinChannels: 20,
               MaxChannels: 100,
               ChannelTimeout: 10 * time.Second,
           },
       },
   }
   ```

3. **Enable Object Pooling**
   ```go
   perfConfig := &messaging.PerformanceConfig{
       EnableObjectPooling: true,
       ObjectPoolSize: 5000,
   }
   ```

### High Memory Usage

**Symptoms:**
- Memory usage growing over time
- Out of memory errors
- Garbage collection pressure

**Solutions:**

1. **Check for Memory Leaks**
   ```go
   // Enable memory profiling
   perfConfig := &messaging.PerformanceConfig{
       EnableMemoryProfiling: true,
       MemoryProfilingInterval: 30 * time.Second,
   }
   ```

2. **Optimize Message Size**
   ```go
   // Use compression for large messages
   msg := messaging.NewMessage(
       compressData(largeData),
       messaging.WithContentType("application/gzip"),
   )
   ```

3. **Implement Proper Cleanup**
   ```go
   // Always close resources
   defer func() {
       publisher.Close(ctx)
       consumer.Stop(ctx)
       transport.Disconnect(ctx)
   }()
   ```

## Configuration Issues

### Invalid Configuration

**Symptoms:**
- `invalid configuration` errors
- Validation failures
- Unexpected behavior

**Solutions:**

1. **Validate Configuration**
   ```go
   // Validate configuration before use
   if err := config.Validate(); err != nil {
       logx.Fatal("Invalid configuration", logx.Error(err))
   }
   ```

2. **Use Environment Variables**
   ```bash
   # Set configuration via environment
   export MSG_TRANSPORT=rabbitmq
   export MSG_RABBITMQ_URIS=amqp://localhost:5672/
   export MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT=5000
   ```

3. **Check Configuration Precedence**
   ```go
   // Environment variables override YAML
   // YAML overrides defaults
   config := &messaging.Config{
       Transport: "rabbitmq", // Can be overridden by MSG_TRANSPORT
   }
   ```

### Configuration Loading Issues

**Symptoms:**
- Configuration not loaded
- Default values not applied
- Environment variables ignored

**Solutions:**

1. **Use Config Loader**
   ```go
   // Load from file
   config, err := configloader.LoadFromFile("config.yaml")
   if err != nil {
       logx.Fatal("Failed to load config", logx.Error(err))
   }
   
   // Load from environment
   config, err := configloader.LoadFromEnvironment()
   if err != nil {
       logx.Fatal("Failed to load config", logx.Error(err))
   }
   ```

2. **Check File Paths**
   ```go
   // Use absolute paths or check working directory
   config, err := configloader.LoadFromFile("/path/to/config.yaml")
   ```

## Error Handling

### Common Error Types

1. **Connection Errors**
   ```go
   var connErr *messaging.MessagingError
   if errors.As(err, &connErr) && connErr.Code == messaging.ErrorCodeConnection {
       // Handle connection error
       logx.Error("Connection error", logx.Error(err))
   }
   ```

2. **Publish Errors**
   ```go
   var publishErr *messaging.MessagingError
   if errors.As(err, &publishErr) && publishErr.Code == messaging.ErrorCodePublish {
       // Handle publish error
       logx.Error("Publish error", logx.Error(err))
   }
   ```

3. **Timeout Errors**
   ```go
   var timeoutErr *messaging.MessagingError
   if errors.As(err, &timeoutErr) && timeoutErr.Code == messaging.ErrorCodeTimeout {
       // Handle timeout error
       logx.Error("Timeout error", logx.Error(err))
   }
   ```

### Implementing Retry Logic

```go
func publishWithRetry(ctx context.Context, publisher messaging.Publisher, msg messaging.Message, maxRetries int) error {
    for attempt := 0; attempt <= maxRetries; attempt++ {
        receipt, err := publisher.PublishAsync(ctx, "my.exchange", msg)
        if err != nil {
            if attempt == maxRetries {
                return err
            }
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        
        <-receipt.Done()
        result, err := receipt.Result()
        if err != nil {
            if attempt == maxRetries {
                return err
            }
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        
        return nil
    }
    return errors.New("max retries exceeded")
}
```

## Debugging Techniques

### Enable Debug Logging

```go
config := &messaging.Config{
    Logging: &messaging.LoggingConfig{
        Level: "debug",
        JSON:  true,
        Fields: map[string]string{
            "service": "my-service",
            "version": "1.0.0",
        },
    },
}
```

### Add Custom Logging

```go
logger := obsCtx.Logger()

logger.Debug("Publishing message",
    logx.String("message_id", msg.ID),
    logx.String("exchange", exchange),
    logx.Int("body_size", len(msg.Body)),
)

logger.Info("Message published successfully",
    logx.String("message_id", msg.ID),
    logx.Duration("duration", duration),
)
```

### Use Health Checks

```go
// Create health manager
healthManager := messaging.NewHealthManager()

// Add custom health checks
healthManager.AddCheck("rabbitmq_connection", func() messaging.HealthStatus {
    // Check RabbitMQ connectivity
    return messaging.HealthStatusHealthy
})

healthManager.AddCheck("publisher_health", func() messaging.HealthStatus {
    stats := publisher.GetStats()
    if stats.TasksFailed > 100 {
        return messaging.HealthStatusUnhealthy
    }
    return messaging.HealthStatusHealthy
})

// Get health report
report := healthManager.GetHealthReport()
if report.Status != messaging.HealthStatusHealthy {
    logx.Error("Health check failed", logx.Any("report", report))
}
```

### Performance Profiling

```go
perfConfig := &messaging.PerformanceConfig{
    EnableMemoryProfiling: true,
    EnableCPUProfiling: true,
    PerformanceMetricsInterval: 5 * time.Second,
    MemoryProfilingInterval: 30 * time.Second,
}

monitor := messaging.NewPerformanceMonitor(perfConfig, observability)
defer monitor.Close(ctx)

// Get performance metrics
metrics := monitor.GetMetrics()
logx.Info("Performance metrics", 
    logx.Float64("throughput_msg_per_sec", metrics.TotalThroughput),
    logx.Int64("latency_p95_ns", metrics.PublishLatencyP95),
    logx.Int("memory_usage_mb", metrics.MemoryUsageMB))
```

## Monitoring and Metrics

### Key Metrics to Monitor

1. **Throughput**
   - Messages per second
   - Bytes per second
   - Success/failure rates

2. **Latency**
   - Publish latency (P50, P95, P99)
   - Consumer processing time
   - End-to-end latency

3. **Resource Usage**
   - Memory usage
   - CPU usage
   - Connection pool utilization

4. **Error Rates**
   - Connection errors
   - Publish failures
   - Consumer errors

### Setting Up Alerts

```go
// Monitor error rates
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := publisher.GetStats()
        if stats.TasksFailed > 100 {
            // Send alert
            sendAlert("High publish failure rate detected")
        }
        
        consumerStats := consumer.GetStats()
        if consumerStats.MessagesFailed > 50 {
            // Send alert
            sendAlert("High consumer failure rate detected")
        }
    }
}()
```

### Integration with Monitoring Systems

```go
// Prometheus metrics
import "github.com/prometheus/client_golang/prometheus"

var (
    publishCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "messages_published_total",
            Help: "Total number of messages published",
        },
        []string{"exchange", "status"},
    )
    
    publishLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "publish_latency_seconds",
            Help: "Publish latency in seconds",
        },
        []string{"exchange"},
    )
)

// Record metrics
publishCounter.WithLabelValues("my.exchange", "success").Inc()
publishLatency.WithLabelValues("my.exchange").Observe(duration.Seconds())
```

## Best Practices Summary

1. **Always use context with timeouts**
2. **Implement proper error handling and retry logic**
3. **Monitor performance and set up alerts**
4. **Use connection and channel pooling**
5. **Implement proper cleanup in defer statements**
6. **Use observability features for debugging**
7. **Test thoroughly with mock transports**
8. **Configure appropriate timeouts for your use case**
9. **Use dead letter queues for failed message handling**
10. **Monitor memory usage and implement cleanup**
11. **Scale worker counts based on CPU cores**
12. **Optimize message size and use compression when needed**
13. **Use batch operations for better performance**
14. **Implement health checks for production systems**
15. **Set up comprehensive monitoring and alerting**
