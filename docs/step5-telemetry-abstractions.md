# Step 5: Telemetry Abstractions

## Overview

Step 5 implements advanced telemetry abstractions that provide correlation ID management, sampling, batching, and enhanced observability capabilities. This builds upon the foundation established in Step 4 and provides production-grade telemetry features for distributed messaging systems.

## Components Implemented

### 1. Correlation ID Management (`pkg/messaging/correlation.go`)

**Purpose**: Comprehensive correlation ID system for tracking message flow across distributed systems.

**Key Features**:
- **CorrelationID**: Unique identifier for tracking message flow
- **CorrelationContext**: Context-aware correlation management with metadata
- **CorrelationManager**: Centralized correlation context management
- **CorrelationPropagator**: Propagation across message boundaries
- **CorrelationMiddleware**: Automatic correlation ID management

**Correlation Context Features**:
- Parent-child relationship tracking
- Trace and span ID integration
- Metadata storage and propagation
- Context integration with Go contexts
- Automatic cleanup and lifecycle management

**Usage Example**:
```go
// Create correlation context
corrCtx := messaging.NewCorrelationContext()
corrCtx.SetMetadata("user_id", "123")
corrCtx.SetTraceID("trace-123")

// Convert to Go context
ctx := corrCtx.ToContext(context.Background())

// Extract from context
extractedCtx, exists := messaging.FromContext(ctx)
```

### 2. Advanced Telemetry Provider (`pkg/messaging/telemetry_advanced.go`)

**Purpose**: Advanced telemetry capabilities with sampling and batching support.

**Key Features**:
- **SamplingConfig**: Configurable sampling rates and seeds
- **BatchingConfig**: Configurable batching parameters
- **AdvancedTelemetryProvider**: Centralized advanced telemetry management
- **SampledMetrics**: Metrics collection with sampling
- **BatchedMetrics**: Metrics collection with batching
- **TelemetryMiddleware**: Automatic telemetry collection

**Sampling Features**:
- Configurable sampling rates (0.0 to 1.0)
- Deterministic sampling with seeds
- Per-operation sampling decisions
- Performance optimization for high-volume systems

**Batching Features**:
- Configurable batch sizes and intervals
- Automatic flush on batch full or timeout
- Background worker for periodic flushing
- Graceful shutdown with final flush

**Usage Example**:
```go
// Create advanced telemetry provider
samplingConfig := messaging.SamplingConfig{
    Enabled: true,
    Rate:    0.1, // 10% sampling
    Seed:    12345,
}

batchingConfig := messaging.BatchingConfig{
    Enabled:       true,
    BatchSize:     100,
    FlushInterval: 1 * time.Second,
    MaxWaitTime:   5 * time.Second,
}

provider := messaging.NewAdvancedTelemetryProvider(
    metrics, tracer, logger,
    samplingConfig, batchingConfig,
)

// Use sampled metrics
sampledMetrics := messaging.NewSampledMetrics(metrics, provider)
sampledMetrics.PublishTotal("rabbitmq", "orders.exchange")

// Use batched metrics
batchedMetrics := messaging.NewBatchedMetrics(metrics, batchingConfig)
defer batchedMetrics.Close()
batchedMetrics.PublishTotal("rabbitmq", "orders.exchange")
```

### 3. Correlation Propagation System

**Header Injection/Extraction**:
- Automatic correlation ID injection into message headers
- Trace and span ID propagation
- Metadata propagation with prefixing
- Parent-child correlation tracking

**Header Format**:
```
X-Correlation-ID: <correlation-id>
X-Parent-Correlation-ID: <parent-correlation-id>
X-Trace-ID: <trace-id>
X-Span-ID: <span-id>
X-Correlation-Metadata-<key>: <value>
```

**Middleware Integration**:
- **PublisherMiddleware**: Automatic correlation injection
- **ConsumerMiddleware**: Automatic correlation extraction
- **HandlerMiddleware**: Child correlation context creation

### 4. Telemetry Middleware System

**Publisher Telemetry Middleware**:
- Automatic span creation for publish operations
- Metrics recording for publish operations
- Telemetry context injection into messages
- Performance timing and attributes

**Consumer Telemetry Middleware**:
- Automatic span creation for consume operations
- Metrics recording for consume operations
- Telemetry context extraction from messages
- Delivery information tracking

**Handler Telemetry Middleware**:
- Automatic span creation for message handling
- Performance timing for handlers
- Context propagation for handlers
- Error tracking and reporting

## Design Principles

### 1. Performance Optimization
- **Sampling**: Reduce telemetry overhead in high-volume systems
- **Batching**: Reduce network calls and improve throughput
- **Async Processing**: Non-blocking telemetry operations
- **Configurable Limits**: Prevent resource exhaustion

### 2. Distributed Tracing
- **Correlation IDs**: Track message flow across services
- **Parent-Child Relationships**: Maintain causality chains
- **Metadata Propagation**: Share context across boundaries
- **Trace Integration**: Compatible with OpenTelemetry

### 3. Production Ready
- **Graceful Degradation**: System continues without telemetry
- **Resource Management**: Automatic cleanup and limits
- **Error Handling**: Comprehensive error scenarios
- **Thread Safety**: Concurrent operation support

### 4. Extensibility
- **Pluggable Middleware**: Custom telemetry logic
- **Configurable Sampling**: Adaptive sampling strategies
- **Custom Metrics**: Extensible metrics collection
- **Integration Points**: Easy integration with existing systems

## Configuration

### Sampling Configuration
```yaml
telemetry:
  sampling:
    enabled: true
    rate: 0.1  # 10% sampling
    seed: 12345
```

### Batching Configuration
```yaml
telemetry:
  batching:
    enabled: true
    batchSize: 100
    flushInterval: 1s
    maxWaitTime: 5s
```

### Correlation Configuration
```yaml
telemetry:
  correlation:
    maxActive: 1000
    cleanupInterval: 5m
    maxAge: 1h
```

## Testing

Comprehensive unit tests cover:
- Correlation ID generation and validation
- Correlation context operations and metadata
- Correlation manager registration and cleanup
- Correlation propagation and extraction
- Sampling configuration and behavior
- Batching configuration and operations
- Telemetry middleware functionality
- Error scenarios and edge cases

**Test Coverage**:
- ✅ Correlation ID generation and validation
- ✅ Correlation context operations
- ✅ Parent-child relationship tracking
- ✅ Metadata storage and retrieval
- ✅ Context integration and extraction
- ✅ Correlation manager operations
- ✅ Correlation propagation system
- ✅ Middleware functionality
- ✅ Sampling configuration and behavior
- ✅ Batching configuration and operations
- ✅ Telemetry middleware integration
- ✅ Error handling and edge cases

## Integration Points

### With Transport Implementations
- **Automatic Correlation**: Seamless correlation ID management
- **Header Propagation**: Automatic header injection/extraction
- **Metrics Integration**: Comprehensive metrics collection
- **Tracing Integration**: Distributed tracing support

### With Observability Foundation (Step 4)
- **Health Check Integration**: Correlation-aware health checks
- **Metrics Enhancement**: Sampled and batched metrics
- **Tracing Enhancement**: Correlation-aware tracing
- **Logging Enhancement**: Correlation-aware logging

### With Configuration System (Step 2)
- **Telemetry Configuration**: YAML and environment variable support
- **Validation**: Configuration validation and defaults
- **Override Support**: Environment variable precedence

## Performance Considerations

### Sampling Benefits
- **Reduced Overhead**: Lower CPU and memory usage
- **Network Efficiency**: Fewer telemetry data transmissions
- **Storage Optimization**: Reduced telemetry data storage
- **Cost Reduction**: Lower infrastructure costs

### Batching Benefits
- **Network Efficiency**: Reduced network round trips
- **Throughput Improvement**: Higher telemetry throughput
- **Resource Optimization**: Better resource utilization
- **Latency Reduction**: Lower per-operation latency

### Correlation Management
- **Memory Efficiency**: Automatic cleanup of old correlations
- **Concurrent Safety**: Thread-safe operations
- **Resource Limits**: Configurable limits to prevent exhaustion
- **Performance Monitoring**: Built-in performance metrics

## Security Considerations

### Correlation ID Security
- **UUID Generation**: Cryptographically secure correlation IDs
- **Metadata Filtering**: Sensitive data filtering in metadata
- **Access Control**: Correlation data access control
- **Audit Trail**: Correlation-based audit trails

### Telemetry Security
- **Data Encryption**: Encrypted telemetry data transmission
- **Authentication**: Telemetry endpoint authentication
- **Authorization**: Telemetry data access authorization
- **Compliance**: GDPR and privacy compliance support

## Future Enhancements

### Advanced Sampling
- **Adaptive Sampling**: Dynamic sampling rate adjustment
- **Conditional Sampling**: Context-aware sampling decisions
- **Priority Sampling**: Priority-based sampling strategies
- **Custom Sampling**: User-defined sampling logic

### Enhanced Batching
- **Smart Batching**: Intelligent batch size adjustment
- **Priority Batching**: Priority-based batch processing
- **Compression**: Telemetry data compression
- **Persistence**: Batch persistence for reliability

### Correlation Enhancements
- **Correlation Analytics**: Advanced correlation analysis
- **Correlation Visualization**: Correlation flow visualization
- **Correlation Alerts**: Correlation-based alerting
- **Correlation Metrics**: Correlation-specific metrics

## Conclusion

Step 5 provides advanced telemetry abstractions that:
- ✅ Enable comprehensive correlation tracking across distributed systems
- ✅ Provide performance optimization through sampling and batching
- ✅ Support production-grade observability with middleware integration
- ✅ Offer configurable and extensible telemetry capabilities
- ✅ Maintain high performance with minimal overhead
- ✅ Ensure thread safety and resource management
- ✅ Provide comprehensive test coverage
- ✅ Support security and compliance requirements

This telemetry abstraction layer provides the foundation for advanced observability features and will be essential for the RabbitMQ implementation in the next steps. The correlation system, sampling, and batching capabilities ensure that the messaging system can handle high-volume production workloads while maintaining comprehensive observability.
