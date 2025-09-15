package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/require"
)

// BenchmarkComprehensivePublisher runs comprehensive publisher benchmarks
func BenchmarkComprehensivePublisher(b *testing.B) {
	configs := []struct {
		name        string
		messageSize int
		publishers  int
		duration    time.Duration
	}{
		{"SmallMessages_1Publisher", 64, 1, 5 * time.Second},
		{"SmallMessages_4Publishers", 64, 4, 5 * time.Second},
		{"MediumMessages_1Publisher", 256, 1, 5 * time.Second},
		{"MediumMessages_4Publishers", 256, 4, 5 * time.Second},
		{"LargeMessages_1Publisher", 1024, 1, 5 * time.Second},
		{"LargeMessages_4Publishers", 1024, 4, 5 * time.Second},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			benchmarkConfig := &messaging.BenchmarkConfig{
				Duration:              config.duration,
				WarmupDuration:        1 * time.Second,
				PublisherCount:        config.publishers,
				ConsumerCount:         1, // Minimum required for validation
				MessageSize:           config.messageSize,
				BatchSize:             50,
				EnableLatencyTracking: true,
				EnableMemoryTracking:  true,
				EnableGCTracking:      true,
			}

			runBenchmarkScenario(b, benchmarkConfig)
		})
	}
}

// BenchmarkComprehensiveConsumer runs comprehensive consumer benchmarks
func BenchmarkComprehensiveConsumer(b *testing.B) {
	configs := []struct {
		name      string
		consumers int
		duration  time.Duration
	}{
		{"1Consumer", 1, 5 * time.Second},
		{"4Consumers", 4, 5 * time.Second},
		{"8Consumers", 8, 5 * time.Second},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			benchmarkConfig := &messaging.BenchmarkConfig{
				Duration:              config.duration,
				WarmupDuration:        1 * time.Second,
				PublisherCount:        1, // Minimum required for validation
				ConsumerCount:         config.consumers,
				MessageSize:           256,
				BatchSize:             50,
				EnableLatencyTracking: true,
				EnableMemoryTracking:  true,
				EnableGCTracking:      true,
			}

			runBenchmarkScenario(b, benchmarkConfig)
		})
	}
}

// BenchmarkEndToEnd runs end-to-end publisher-consumer benchmarks
func BenchmarkEndToEnd(b *testing.B) {
	configs := []struct {
		name        string
		publishers  int
		consumers   int
		messageSize int
		duration    time.Duration
	}{
		{"1P_1C_SmallMessages", 1, 1, 64, 10 * time.Second},
		{"1P_1C_MediumMessages", 1, 1, 256, 10 * time.Second},
		{"1P_1C_LargeMessages", 1, 1, 1024, 10 * time.Second},
		{"4P_4C_MediumMessages", 4, 4, 256, 10 * time.Second},
		{"8P_8C_MediumMessages", 8, 8, 256, 10 * time.Second},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			benchmarkConfig := &messaging.BenchmarkConfig{
				Duration:              config.duration,
				WarmupDuration:        2 * time.Second,
				PublisherCount:        config.publishers,
				ConsumerCount:         config.consumers,
				MessageSize:           config.messageSize,
				BatchSize:             50,
				EnableLatencyTracking: true,
				EnableMemoryTracking:  true,
				EnableGCTracking:      true,
			}

			runBenchmarkScenario(b, benchmarkConfig)
		})
	}
}

// BenchmarkHighThroughput tests high throughput scenarios
func BenchmarkHighThroughput(b *testing.B) {
	benchmarkConfig := &messaging.BenchmarkConfig{
		Duration:              15 * time.Second,
		WarmupDuration:        3 * time.Second,
		PublisherCount:        16,
		ConsumerCount:         16,
		MessageSize:           128,
		BatchSize:             500,
		EnableLatencyTracking: true,
		EnableMemoryTracking:  true,
		EnableGCTracking:      true,
	}

	runBenchmarkScenario(b, benchmarkConfig)
}

// BenchmarkLowLatency tests low latency scenarios
func BenchmarkLowLatency(b *testing.B) {
	benchmarkConfig := &messaging.BenchmarkConfig{
		Duration:              10 * time.Second,
		WarmupDuration:        2 * time.Second,
		PublisherCount:        1,
		ConsumerCount:         1,
		MessageSize:           64,
		BatchSize:             1,
		EnableLatencyTracking: true,
		EnableMemoryTracking:  true,
		EnableGCTracking:      true,
	}

	runBenchmarkScenario(b, benchmarkConfig)
}

// BenchmarkMemoryEfficiency tests memory efficiency
func BenchmarkMemoryEfficiency(b *testing.B) {
	benchmarkConfig := &messaging.BenchmarkConfig{
		Duration:              10 * time.Second,
		WarmupDuration:        2 * time.Second,
		PublisherCount:        4,
		ConsumerCount:         4,
		MessageSize:           512, // Medium messages
		BatchSize:             50,
		EnableLatencyTracking: false, // Disable to reduce memory overhead
		EnableMemoryTracking:  true,
		EnableGCTracking:      true,
	}

	runBenchmarkScenario(b, benchmarkConfig)
}

// runBenchmarkScenario runs a specific benchmark scenario
func runBenchmarkScenario(b *testing.B, config *messaging.BenchmarkConfig) {
	// Create test configuration
	testConfig := messaging.NewTestConfig()
	testConfig.FailureRate = 0.0 // No failures for benchmark
	testConfig.Latency = 0       // No artificial latency

	// Create mock transport
	transport := messaging.NewMockTransport(testConfig)

	// Connect transport
	ctx := context.Background()
	err := transport.Connect(ctx)
	require.NoError(b, err)
	defer transport.Disconnect(ctx)

	// Create observability
	telemetryConfig := &messaging.TelemetryConfig{
		MetricsEnabled: true,
		TracingEnabled: false, // Disable for benchmark
		ServiceName:    "benchmark-test",
	}

	observabilityProvider, err := messaging.NewObservabilityProvider(telemetryConfig)
	require.NoError(b, err)
	// Note: ObservabilityProvider doesn't have Close method in current implementation

	obsCtx := messaging.NewObservabilityContext(ctx, observabilityProvider)

	// Create performance monitor
	perfConfig := &messaging.PerformanceConfig{
		EnableObjectPooling:        true,
		ObjectPoolSize:             1000,
		EnableMemoryProfiling:      config.EnableMemoryTracking,
		PerformanceMetricsInterval: 1 * time.Second,
	}

	performanceMonitor := messaging.NewPerformanceMonitor(perfConfig, obsCtx)
	defer performanceMonitor.Close(ctx)

	// Create benchmark runner
	runner, err := messaging.NewBenchmarkRunner(config, obsCtx, performanceMonitor)
	if err != nil {
		b.Fatalf("Failed to create benchmark runner: %v", err)
	}

	// Create publisher and consumer
	var publisher messaging.Publisher
	var consumer messaging.Consumer

	if config.PublisherCount > 0 {
		publisherConfig := &messaging.PublisherConfig{
			MaxInFlight: config.BatchSize * config.PublisherCount,
			WorkerCount: config.PublisherCount,
		}
		publisher = transport.NewPublisher(publisherConfig, obsCtx)
	}

	if config.ConsumerCount > 0 {
		consumerConfig := &messaging.ConsumerConfig{
			Queue:                 "benchmark.queue",
			Prefetch:              256,
			MaxConcurrentHandlers: config.ConsumerCount * 64,
		}
		consumer = transport.NewConsumer(consumerConfig, obsCtx)
	}

	// Reset timer and run benchmark
	b.ResetTimer()

	// Run the actual benchmark
	for i := 0; i < b.N; i++ {
		// Handle nil publisher/consumer case by using mock implementations
		if publisher == nil {
			publisherConfig := &messaging.PublisherConfig{
				MaxInFlight: 100,
				WorkerCount: 1,
			}
			publisher = transport.NewPublisher(publisherConfig, obsCtx)
		}

		if consumer == nil {
			consumerConfig := &messaging.ConsumerConfig{
				Queue:                 "benchmark.queue",
				Prefetch:              256,
				MaxConcurrentHandlers: 64,
			}
			consumer = transport.NewConsumer(consumerConfig, obsCtx)
		}

		result, err := runner.RunBenchmark(ctx, publisher, consumer)
		require.NoError(b, err)

		// Report custom metrics
		if result.PublisherResults != nil {
			b.ReportMetric(result.PublisherResults.Throughput, "pub_msgs/sec")
			b.ReportMetric(float64(result.PublisherResults.LatencyP95)/1e6, "pub_p95_ms")
			b.ReportMetric(result.PublisherResults.ErrorRate, "pub_error_%")
		}

		if result.ConsumerResults != nil {
			b.ReportMetric(result.ConsumerResults.Throughput, "con_msgs/sec")
			b.ReportMetric(float64(result.ConsumerResults.LatencyP95)/1e6, "con_p95_ms")
			b.ReportMetric(result.ConsumerResults.ErrorRate, "con_error_%")
		}

		if result.SystemResults != nil {
			b.ReportMetric(float64(result.SystemResults.PeakMemoryUsage)/1024/1024, "peak_mem_MB")
			b.ReportMetric(float64(result.SystemResults.PeakGoroutines), "peak_goroutines")
			b.ReportMetric(float64(result.SystemResults.AverageGCPauseTime)/1e6, "avg_gc_ms")
		}

		if result.Summary != nil {
			b.ReportMetric(result.Summary.TotalThroughput, "total_msgs/sec")
			b.ReportMetric(result.Summary.Efficiency, "efficiency_%")
		}

		// Log results for analysis
		b.Logf("Benchmark Results:")
		if result.PublisherResults != nil {
			b.Logf("  Publisher: %.2f msgs/sec, P95: %.2fms, Errors: %.2f%%",
				result.PublisherResults.Throughput,
				float64(result.PublisherResults.LatencyP95)/1e6,
				result.PublisherResults.ErrorRate)
		}
		if result.ConsumerResults != nil {
			b.Logf("  Consumer: %.2f msgs/sec, P95: %.2fms, Errors: %.2f%%",
				result.ConsumerResults.Throughput,
				float64(result.ConsumerResults.LatencyP95)/1e6,
				result.ConsumerResults.ErrorRate)
		}
		if result.SystemResults != nil {
			b.Logf("  System: %.2fMB peak memory, %d peak goroutines, %.2fms avg GC",
				float64(result.SystemResults.PeakMemoryUsage)/1024/1024,
				result.SystemResults.PeakGoroutines,
				float64(result.SystemResults.AverageGCPauseTime)/1e6)
		}
		if result.Summary != nil {
			b.Logf("  Summary: %.2f total msgs/sec, %.1f%% efficiency",
				result.Summary.TotalThroughput,
				result.Summary.Efficiency)

			if len(result.Summary.Recommendations) > 0 {
				b.Logf("  Recommendations:")
				for _, rec := range result.Summary.Recommendations {
					b.Logf("    - %s", rec)
				}
			}
		}
	}
}

// BenchmarkScalability tests scalability characteristics
func BenchmarkScalability(b *testing.B) {
	scales := []struct {
		name       string
		publishers int
		consumers  int
	}{
		{"1P_1C", 1, 1},
		{"2P_2C", 2, 2},
		{"4P_4C", 4, 4},
		{"8P_8C", 8, 8},
		{"16P_16C", 16, 16},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			benchmarkConfig := &messaging.BenchmarkConfig{
				Duration:              8 * time.Second,
				WarmupDuration:        2 * time.Second,
				PublisherCount:        scale.publishers,
				ConsumerCount:         scale.consumers,
				MessageSize:           256,
				BatchSize:             50,
				EnableLatencyTracking: true,
				EnableMemoryTracking:  true,
				EnableGCTracking:      true,
			}

			runBenchmarkScenario(b, benchmarkConfig)
		})
	}
}

// BenchmarkMessageSizes tests different message sizes
func BenchmarkMessageSizes(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"32B", 32},
		{"64B", 64},
		{"128B", 128},
		{"256B", 256},
		{"512B", 512},
		{"1KB", 1024},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			benchmarkConfig := &messaging.BenchmarkConfig{
				Duration:              5 * time.Second,
				WarmupDuration:        1 * time.Second,
				PublisherCount:        4,
				ConsumerCount:         4,
				MessageSize:           size.size,
				BatchSize:             50,
				EnableLatencyTracking: true,
				EnableMemoryTracking:  true,
				EnableGCTracking:      true,
			}

			runBenchmarkScenario(b, benchmarkConfig)
		})
	}
}
