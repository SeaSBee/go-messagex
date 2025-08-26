# Profiling Guide for go-messagex

This guide provides comprehensive information about memory usage analysis and profiling capabilities in the go-messagex codebase.

## Table of Contents

1. [Memory Usage Analysis](#memory-usage-analysis)
2. [Profiling Infrastructure](#profiling-infrastructure)
3. [Profiling Tools](#profiling-tools)
4. [Memory Optimization](#memory-optimization)
5. [Production Monitoring](#production-monitoring)
6. [Best Practices](#best-practices)

## Memory Usage Analysis

### Current Memory Performance

Based on comprehensive benchmarking, the current memory usage patterns are:

#### Memory Efficiency Metrics
- **Peak Memory Usage**: 1.5-5GB depending on workload intensity
- **Memory Efficiency**: ~3.2MB per 1,000 messages/sec throughput
- **Garbage Collection**: Average 0.02-0.07ms per GC cycle
- **Memory Allocation**: 2.1-6.5GB per operation (with object pooling)

#### Memory Usage by Workload Type

| Workload Type | Peak Memory | Throughput | Memory/Msg Rate | GC Frequency |
|---------------|-------------|------------|-----------------|--------------|
| Small Messages (1P) | 804MB | 239,767 msgs/sec | 3.35MB/1K msgs | 0.03ms avg |
| Small Messages (4P) | 1,713MB | 539,744 msgs/sec | 3.17MB/1K msgs | 0.05ms avg |
| Medium Messages (1P) | 815MB | 238,677 msgs/sec | 3.41MB/1K msgs | 0.03ms avg |
| Large Messages (1P) | 789MB | 204,931 msgs/sec | 3.85MB/1K msgs | 0.04ms avg |
| High Throughput | 5,007MB | 579,911 msgs/sec | 8.63MB/1K msgs | 0.04ms avg |
| End-to-End (8P8C) | 3,271MB | 552,927 msgs/sec | 5.92MB/1K msgs | 0.04ms avg |

### Memory Allocation Analysis

#### Top Memory Allocators (from profiling)
1. **Timer Creation**: 26.5GB (16.49%) - `time.newTimer`
2. **Context Operations**: 25.3GB (15.73%) - `context.WithDeadlineCause`
3. **Message Creation**: 24.6GB (15.29%) - `createTestMessage`
4. **Publisher Operations**: 24.0GB (14.98%) - `PublishAsync`
5. **Receipt Creation**: 23.0GB (14.30%) - `NewMockReceipt`

#### Memory Optimization Opportunities
- **Timer Pooling**: Implement timer pooling to reduce `time.newTimer` allocations
- **Context Reuse**: Optimize context creation and reuse patterns
- **Message Pooling**: Expand object pooling for message-related structures
- **Batch Optimization**: Fine-tune batch sizes based on memory constraints

## Profiling Infrastructure

### Built-in Memory Monitoring

The codebase includes comprehensive memory profiling capabilities through the `PerformanceMonitor`:

```go
// Enable memory profiling in performance configuration
perfConfig := &messaging.PerformanceConfig{
    EnableMemoryProfiling:      true,
    EnableCPUProfiling:         true,
    PerformanceMetricsInterval: 5 * time.Second,
    MemoryLimit:                1024 * 1024 * 1024, // 1GB
    GCThreshold:                80.0,
}

// Create performance monitor with memory tracking
performanceMonitor := messaging.NewPerformanceMonitor(perfConfig, obsCtx)
defer performanceMonitor.Close(ctx)

// Get real-time memory metrics
metrics := performanceMonitor.GetMetrics()
logx.Info("Memory usage",
    logx.Uint64("heap_alloc", metrics.HeapAlloc),
    logx.Uint64("heap_sys", metrics.HeapSys),
    logx.Uint64("heap_objects", metrics.HeapObjects),
    logx.Float64("utilization_percent", metrics.MemoryUtilization),
    logx.Uint32("num_gc", metrics.NumGC),
    logx.Int("num_goroutines", metrics.NumGoroutines),
)
```

### Memory Health Checks

```go
// Configure memory health monitoring
healthManager := messaging.NewHealthManager()
healthManager.AddCheck(messaging.NewMemoryHealthChecker(85.0)) // 85% threshold
healthManager.AddCheck(messaging.NewGoroutineHealthChecker(1000)) // 1000 goroutines max

// Monitor memory health
report := healthManager.GetHealthReport()
if report.Status != messaging.HealthStatusHealthy {
    logx.Warn("Memory health check failed", logx.Any("details", report.Details))
}
```

## Profiling Tools

### Quick Start Commands

```bash
# Memory profiling
make profile-memory

# CPU profiling
make profile-cpu

# Comprehensive profiling (memory + CPU)
make profile-comprehensive

# Benchmark profiling
make profile-benchmarks

# Analyze existing profiles
make analyze-profiles

# Clean profile files
make clean-profiles

# Run full benchmark suite
make benchmark-suite
```

### Manual Profiling Commands

#### 1. Memory Profiling
```bash
# Generate memory profile during benchmarks
go test -bench=. -benchmem -memprofile=memory.prof ./tests/benchmarks/

# Analyze memory profile
go tool pprof -top memory.prof
go tool pprof -web memory.prof
go tool pprof -list=. memory.prof

# Generate memory allocation graph
go tool pprof -alloc_space -web memory.prof

# Analyze specific functions
go tool pprof -list=createTestMessage memory.prof
```

#### 2. CPU Profiling
```bash
# Generate CPU profile during benchmarks
go test -bench=. -cpuprofile=cpu.prof ./tests/benchmarks/

# Analyze CPU profile
go tool pprof -top cpu.prof
go tool pprof -web cpu.prof
go tool pprof -list=. cpu.prof

# Generate CPU usage graph
go tool pprof -cum -web cpu.prof

# Analyze specific functions
go tool pprof -list=PublishAsync cpu.prof
```

#### 3. Comprehensive Benchmarking
```bash
# Run full benchmark suite with profiling
./scripts/run_benchmarks.sh

# Run with custom profiling settings
BENCHMARK_MEMORY=true BENCHMARK_CPU=true ./scripts/run_benchmarks.sh

# Run specific memory efficiency tests
go test -bench=BenchmarkMemoryEfficiency -benchmem ./tests/benchmarks/
```

### Profile Analysis Commands

#### Memory Profile Analysis
```bash
# Top memory allocators
go tool pprof -top memory.prof

# Memory allocation by function
go tool pprof -list=. memory.prof

# Memory allocation graph (requires graphviz)
go tool pprof -web memory.prof

# Memory allocation space
go tool pprof -alloc_space -top memory.prof

# Memory allocation objects
go tool pprof -alloc_objects -top memory.prof
```

#### CPU Profile Analysis
```bash
# Top CPU consumers
go tool pprof -top cpu.prof

# CPU usage by function
go tool pprof -list=. cpu.prof

# CPU usage graph (requires graphviz)
go tool pprof -web cpu.prof

# Cumulative CPU usage
go tool pprof -cum -top cpu.prof

# CPU usage with samples
go tool pprof -sample_index=cpu -top cpu.prof
```

## Memory Optimization

### Object Pooling

```go
// Use object pools for frequently allocated objects
var messagePool = sync.Pool{
    New: func() interface{} {
        return &messaging.Message{}
    },
}

// Get and return objects to pool
msg := messagePool.Get().(*messaging.Message)
defer messagePool.Put(msg)

// Use pooled message in handlers
func pooledHandler(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
    msg := messagePool.Get().(*messaging.Message)
    defer messagePool.Put(msg)
    
    // Process message
    // ...
    
    return messaging.Ack, nil
}
```

### Batch Processing Optimization

```go
// Configure optimal batch sizes for memory efficiency
config := &messaging.PerformanceConfig{
    BatchSize:    100,  // Optimal for most workloads
    BatchTimeout: 100 * time.Millisecond,
}

// Monitor batch processing memory usage
func monitorBatchMemory() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        var memStats runtime.MemStats
        runtime.ReadMemStats(&memStats)
        
        logx.Info("Batch processing memory",
            logx.Uint64("heap_alloc_mb", memStats.HeapAlloc/1024/1024),
            logx.Uint64("heap_objects", memStats.HeapObjects),
            logx.Uint32("num_gc", memStats.NumGC),
        )
    }
}
```

### Connection Pool Management

```go
// Monitor connection pool memory usage
stats := transport.GetConnectionPool().GetStats()
logx.Info("Connection pool memory",
    logx.Int("active_connections", stats.ActiveConnections),
    logx.Int("idle_connections", stats.IdleConnections),
    logx.Int("total_channels", stats.TotalChannels),
)

// Optimize connection pool for memory efficiency
poolConfig := &messaging.ConnectionPoolConfig{
    Min: 2,  // Minimum connections
    Max: 8,  // Maximum connections (adjust based on memory constraints)
}
```

## Production Monitoring

### Real-time Memory Monitoring

```go
// Set up continuous memory monitoring
func setupMemoryMonitoring() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            var memStats runtime.MemStats
            runtime.ReadMemStats(&memStats)
            
            logx.Info("Memory status",
                logx.Uint64("heap_alloc_mb", memStats.HeapAlloc/1024/1024),
                logx.Uint64("heap_sys_mb", memStats.HeapSys/1024/1024),
                logx.Uint64("heap_objects", memStats.HeapObjects),
                logx.Uint32("num_gc", memStats.NumGC),
                logx.Int("num_goroutines", runtime.NumGoroutine()),
            )
            
            // Alert on high memory usage
            if memStats.HeapAlloc > 1024*1024*1024 { // 1GB
                logx.Warn("High memory usage detected",
                    logx.Uint64("heap_alloc_mb", memStats.HeapAlloc/1024/1024),
                )
            }
        }
    }()
}
```

### Memory Alerting

```go
// Configure memory-based alerting
func setupMemoryAlerts() {
    alertThreshold := uint64(1024 * 1024 * 1024) // 1GB
    
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for range ticker.C {
            var memStats runtime.MemStats
            runtime.ReadMemStats(&memStats)
            
            if memStats.HeapAlloc > alertThreshold {
                logx.Error("Memory usage exceeded threshold",
                    logx.Uint64("current_mb", memStats.HeapAlloc/1024/1024),
                    logx.Uint64("threshold_mb", alertThreshold/1024/1024),
                )
                
                // Trigger memory cleanup
                runtime.GC()
            }
        }
    }()
}
```

### Performance Metrics Collection

```go
// Collect performance metrics with memory tracking
func collectPerformanceMetrics() {
    perfConfig := &messaging.PerformanceConfig{
        EnableMemoryProfiling:      true,
        PerformanceMetricsInterval: 10 * time.Second,
    }
    
    monitor := messaging.NewPerformanceMonitor(perfConfig, obsCtx)
    defer monitor.Close(ctx)
    
    // Record operations for metrics
    monitor.RecordPublish(100*time.Millisecond, true)
    monitor.RecordConsume(50*time.Millisecond, true)
    
    // Get metrics
    metrics := monitor.GetMetrics()
    logx.Info("Performance metrics",
        logx.Float64("throughput", metrics.TotalThroughput),
        logx.Int64("latency_p95", metrics.PublishLatencyP95),
        logx.Uint64("memory_usage", metrics.HeapAlloc),
        logx.Float64("error_rate", metrics.ErrorRate),
    )
}
```

## Best Practices

### Profiling Best Practices

1. **Regular Profiling**: Run memory profiles during development and testing
2. **Baseline Comparison**: Compare memory usage before and after optimizations
3. **Load Testing**: Profile under realistic load conditions
4. **Long-running Tests**: Monitor memory usage over extended periods
5. **Garbage Collection Analysis**: Track GC frequency and pause times

### Memory Optimization Checklist

- [ ] Enable object pooling for frequently allocated objects
- [ ] Configure appropriate batch sizes for your workload
- [ ] Monitor connection pool memory usage
- [ ] Set up memory health checks and alerting
- [ ] Profile memory usage under production-like conditions
- [ ] Optimize message sizes based on requirements
- [ ] Implement memory cleanup strategies for long-running processes
- [ ] Monitor garbage collection patterns and optimize if needed

### Performance Monitoring Checklist

- [ ] Set up real-time memory monitoring
- [ ] Configure memory-based alerting
- [ ] Monitor garbage collection patterns
- [ ] Track connection pool memory usage
- [ ] Monitor goroutine count and memory usage
- [ ] Set up performance metrics collection
- [ ] Implement memory health checks
- [ ] Configure memory limits and thresholds

### Troubleshooting Memory Issues

#### High Memory Usage
1. Check for memory leaks using `go tool pprof -alloc_space`
2. Monitor garbage collection frequency
3. Review object pooling implementation
4. Check for unbounded goroutines
5. Analyze memory allocation patterns

#### High GC Pressure
1. Reduce object allocation frequency
2. Implement object pooling
3. Optimize batch sizes
4. Review memory-intensive operations
5. Monitor GC pause times

#### Memory Leaks
1. Use `go tool pprof -inuse_space` to identify retained objects
2. Check for goroutine leaks
3. Review connection pool management
4. Analyze context usage patterns
5. Monitor long-lived objects

## Advanced Profiling

### Custom Profiling

```go
// Custom memory profiling
func customMemoryProfile() {
    f, err := os.Create("custom_memory.prof")
    if err != nil {
        logx.Fatal("Could not create memory profile", logx.ErrorField(err))
    }
    defer f.Close()
    
    // Start profiling
    pprof.WriteHeapProfile(f)
    
    // Or profile specific operations
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()
    
    // Your operations here
    // ...
}
```

### Continuous Profiling

```go
// Set up continuous profiling
func setupContinuousProfiling() {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
            // Generate memory profile
            f, err := os.Create(fmt.Sprintf("memory_%d.prof", time.Now().Unix()))
            if err != nil {
                logx.Error("Failed to create memory profile", logx.ErrorField(err))
                continue
            }
            
            pprof.WriteHeapProfile(f)
            f.Close()
            
            logx.Info("Memory profile generated", logx.String("file", f.Name()))
        }
    }()
}
```

For more detailed performance optimization strategies, see [Performance Guide](PERFORMANCE.md).
