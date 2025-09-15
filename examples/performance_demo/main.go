package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

func performanceDemo() {
	fmt.Println("Performance Optimization and Benchmarking Demo")
	fmt.Println(strings.Repeat("=", 60))

	// Create observability provider
	obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: true,
	})
	if err != nil {
		logx.Fatal("Failed to create observability provider", logx.ErrorField(err))
	}

	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	// Create performance configuration
	perfConfig := &messaging.PerformanceConfig{
		EnableObjectPooling:        true,
		ObjectPoolSize:             1000,
		EnableMemoryProfiling:      true,
		EnableCPUProfiling:         true,
		PerformanceMetricsInterval: 5 * time.Second,
		MemoryLimit:                1024 * 1024 * 1024, // 1GB
		GCThreshold:                80.0,
		BatchSize:                  100,
		BatchTimeout:               100 * time.Millisecond,
	}

	// Create performance monitor
	performanceMonitor := messaging.NewPerformanceMonitor(perfConfig, obsCtx)
	defer performanceMonitor.Close(context.Background())

	// Demonstrate object pooling
	demonstrateObjectPooling()

	// Demonstrate batch processing
	demonstrateBatchProcessing()

	// Demonstrate performance monitoring
	demonstratePerformanceMonitoring(performanceMonitor)

	// Demonstrate latency histogram
	demonstrateLatencyHistogram()

	fmt.Println("\nPerformance demo completed!")
}

func demonstrateObjectPooling() {
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("OBJECT POOLING DEMONSTRATION")
	fmt.Println(strings.Repeat("-", 60))

	// Create object pool for messages
	pool := messaging.NewObjectPool(
		100,
		func() interface{} {
			return messaging.NewMessage([]byte("pooled message"))
		},
		func(obj interface{}) {
			// Reset object for reuse
			if msg, ok := obj.(messaging.Message); ok {
				// Note: In a real implementation, you'd reset the message
				_ = msg
			}
		},
	)

	// Demonstrate object reuse
	fmt.Println("Demonstrating object pooling...")

	start := time.Now()
	for i := 0; i < 10000; i++ {
		obj := pool.Get()
		// Use object
		_ = obj
		pool.Put(obj)
	}
	duration := time.Since(start)

	fmt.Printf("Processed 10,000 objects in %v (%.2f ops/sec)\n",
		duration, float64(10000)/duration.Seconds())

	pool.Close()
}

func demonstrateBatchProcessing() {
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("BATCH PROCESSING DEMONSTRATION")
	fmt.Println(strings.Repeat("-", 60))

	// Create batch processor
	batchProcessor := messaging.NewBatchProcessor(
		100,
		100*time.Millisecond,
		func(items []interface{}) error {
			fmt.Printf("Processing batch of %d items\n", len(items))
			return nil
		},
	)

	// Add items to batch
	fmt.Println("Adding items to batch processor...")
	for i := 0; i < 250; i++ {
		err := batchProcessor.Add(fmt.Sprintf("item-%d", i))
		if err != nil {
			fmt.Printf("Error adding item: %v\n", err)
		}
	}

	// Flush remaining items
	err := batchProcessor.Flush()
	if err != nil {
		fmt.Printf("Error flushing batch: %v\n", err)
	}

	batchProcessor.Close()
}

func demonstratePerformanceMonitoring(monitor *messaging.PerformanceMonitor) {
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("PERFORMANCE MONITORING DEMONSTRATION")
	fmt.Println(strings.Repeat("-", 60))

	// Record some performance metrics
	fmt.Println("Recording performance metrics...")

	// Simulate some operations
	for i := 0; i < 100; i++ {
		monitor.RecordPublish(100*time.Microsecond, true)
		monitor.RecordConsume(50*time.Microsecond, true)

		if i%10 == 0 {
			monitor.RecordPublish(1*time.Millisecond, false) // Simulate error
		}
	}

	// Wait for metrics collection
	time.Sleep(6 * time.Second)

	// Get current metrics
	metrics := monitor.GetMetrics()
	if metrics != nil {
		fmt.Printf("Current Performance Metrics:\n")
		fmt.Printf("  Publish Throughput: %.2f msg/sec\n", metrics.PublishThroughput)
		fmt.Printf("  Consume Throughput: %.2f msg/sec\n", metrics.ConsumeThroughput)
		fmt.Printf("  Total Throughput: %.2f msg/sec\n", metrics.TotalThroughput)
		fmt.Printf("  Publish Latency P50: %d ns\n", metrics.PublishLatencyP50)
		fmt.Printf("  Publish Latency P95: %d ns\n", metrics.PublishLatencyP95)
		fmt.Printf("  Publish Latency P99: %d ns\n", metrics.PublishLatencyP99)
		fmt.Printf("  Consume Latency P50: %d ns\n", metrics.ConsumeLatencyP50)
		fmt.Printf("  Consume Latency P95: %d ns\n", metrics.ConsumeLatencyP95)
		fmt.Printf("  Consume Latency P99: %d ns\n", metrics.ConsumeLatencyP99)
		fmt.Printf("  Memory Usage: %.2f MB\n", float64(metrics.HeapInuse)/1024/1024)
		fmt.Printf("  Goroutines: %d\n", metrics.NumGoroutines)
		fmt.Printf("  Error Rate: %.2f errors/sec\n", metrics.ErrorRate)
	} else {
		fmt.Println("No metrics available yet")
	}
}

func demonstrateLatencyHistogram() {
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("LATENCY HISTOGRAM DEMONSTRATION")
	fmt.Println(strings.Repeat("-", 60))

	// Create latency histogram
	histogram := messaging.NewLatencyHistogram()

	// Record various latencies
	fmt.Println("Recording latency samples...")

	// Simulate various latency distributions
	for i := 0; i < 1000; i++ {
		// Simulate normal distribution around 1ms
		latency := time.Duration(1000000 + (i%100)*10000) // 1ms ± 0.5ms
		histogram.Record(latency.Nanoseconds())
	}

	// Calculate percentiles
	p50 := histogram.Percentile(50)
	p95 := histogram.Percentile(95)
	p99 := histogram.Percentile(99)

	fmt.Printf("Latency Percentiles:\n")
	fmt.Printf("  P50: %d ns (%.2f ms)\n", p50, float64(p50)/1000000)
	fmt.Printf("  P95: %d ns (%.2f ms)\n", p95, float64(p95)/1000000)
	fmt.Printf("  P99: %d ns (%.2f ms)\n", p99, float64(p99)/1000000)
}

func main() {
	performanceDemo()
}
