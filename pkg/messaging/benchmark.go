// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seasbee/go-logx"
)

// BenchmarkConfig defines benchmark configuration.
type BenchmarkConfig struct {
	// Duration is the benchmark duration
	Duration time.Duration `yaml:"duration" env:"MSG_BENCHMARK_DURATION" default:"60s"`

	// WarmupDuration is the warmup duration
	WarmupDuration time.Duration `yaml:"warmupDuration" env:"MSG_BENCHMARK_WARMUP_DURATION" default:"10s"`

	// PublisherCount is the number of concurrent publishers
	PublisherCount int `yaml:"publisherCount" env:"MSG_BENCHMARK_PUBLISHER_COUNT" validate:"min=1,max=100" default:"1"`

	// ConsumerCount is the number of concurrent consumers
	ConsumerCount int `yaml:"consumerCount" env:"MSG_BENCHMARK_CONSUMER_COUNT" validate:"min=1,max=100" default:"1"`

	// MessageSize is the size of messages in bytes
	MessageSize int `yaml:"messageSize" env:"MSG_BENCHMARK_MESSAGE_SIZE" validate:"min=1,max=1048576" default:"1024"`

	// BatchSize is the batch size for operations
	BatchSize int `yaml:"batchSize" env:"MSG_BENCHMARK_BATCH_SIZE" validate:"min=1,max=10000" default:"100"`

	// EnableLatencyTracking enables latency tracking
	EnableLatencyTracking bool `yaml:"enableLatencyTracking" env:"MSG_BENCHMARK_ENABLE_LATENCY_TRACKING" default:"true"`

	// EnableMemoryTracking enables memory tracking
	EnableMemoryTracking bool `yaml:"enableMemoryTracking" env:"MSG_BENCHMARK_ENABLE_MEMORY_TRACKING" default:"true"`

	// EnableGCTracking enables GC tracking
	EnableGCTracking bool `yaml:"enableGCTracking" env:"MSG_BENCHMARK_ENABLE_GC_TRACKING" default:"true"`
}

// BenchmarkResult contains benchmark results.
type BenchmarkResult struct {
	// Config is the benchmark configuration
	Config *BenchmarkConfig `json:"config"`

	// StartTime is when the benchmark started
	StartTime time.Time `json:"startTime"`

	// EndTime is when the benchmark ended
	EndTime time.Time `json:"endTime"`

	// Duration is the actual benchmark duration
	Duration time.Duration `json:"duration"`

	// PublisherResults contains publisher benchmark results
	PublisherResults *PublisherBenchmarkResult `json:"publisherResults"`

	// ConsumerResults contains consumer benchmark results
	ConsumerResults *ConsumerBenchmarkResult `json:"consumerResults"`

	// SystemResults contains system benchmark results
	SystemResults *SystemBenchmarkResult `json:"systemResults"`

	// Summary contains benchmark summary
	Summary *BenchmarkSummary `json:"summary"`
}

// PublisherBenchmarkResult contains publisher benchmark results.
type PublisherBenchmarkResult struct {
	// TotalMessages is the total number of messages published
	TotalMessages uint64 `json:"totalMessages"`

	// SuccessfulMessages is the number of successfully published messages
	SuccessfulMessages uint64 `json:"successfulMessages"`

	// FailedMessages is the number of failed messages
	FailedMessages uint64 `json:"failedMessages"`

	// Throughput is the messages per second
	Throughput float64 `json:"throughput"`

	// AverageLatency is the average latency in nanoseconds
	AverageLatency int64 `json:"averageLatency"`

	// LatencyP50 is the 50th percentile latency
	LatencyP50 int64 `json:"latencyP50"`

	// LatencyP95 is the 95th percentile latency
	LatencyP95 int64 `json:"latencyP95"`

	// LatencyP99 is the 99th percentile latency
	LatencyP99 int64 `json:"latencyP99"`

	// ErrorRate is the error rate percentage
	ErrorRate float64 `json:"errorRate"`
}

// ConsumerBenchmarkResult contains consumer benchmark results.
type ConsumerBenchmarkResult struct {
	// TotalMessages is the total number of messages consumed
	TotalMessages uint64 `json:"totalMessages"`

	// SuccessfulMessages is the number of successfully consumed messages
	SuccessfulMessages uint64 `json:"successfulMessages"`

	// FailedMessages is the number of failed messages
	FailedMessages uint64 `json:"failedMessages"`

	// Throughput is the messages per second
	Throughput float64 `json:"throughput"`

	// AverageLatency is the average latency in nanoseconds
	AverageLatency int64 `json:"averageLatency"`

	// LatencyP50 is the 50th percentile latency
	LatencyP50 int64 `json:"latencyP50"`

	// LatencyP95 is the 95th percentile latency
	LatencyP95 int64 `json:"latencyP95"`

	// LatencyP99 is the 99th percentile latency
	LatencyP99 int64 `json:"latencyP99"`

	// ErrorRate is the error rate percentage
	ErrorRate float64 `json:"errorRate"`
}

// SystemBenchmarkResult contains system benchmark results.
type SystemBenchmarkResult struct {
	// PeakMemoryUsage is the peak memory usage in bytes
	PeakMemoryUsage uint64 `json:"peakMemoryUsage"`

	// AverageMemoryUsage is the average memory usage in bytes
	AverageMemoryUsage uint64 `json:"averageMemoryUsage"`

	// PeakGoroutines is the peak number of goroutines
	PeakGoroutines int `json:"peakGoroutines"`

	// AverageGoroutines is the average number of goroutines
	AverageGoroutines int `json:"averageGoroutines"`

	// TotalGCs is the total number of garbage collections
	TotalGCs uint32 `json:"totalGCs"`

	// TotalGCPauseTime is the total GC pause time in nanoseconds
	TotalGCPauseTime uint64 `json:"totalGCPauseTime"`

	// AverageGCPauseTime is the average GC pause time in nanoseconds
	AverageGCPauseTime uint64 `json:"averageGCPauseTime"`
}

// BenchmarkSummary contains benchmark summary.
type BenchmarkSummary struct {
	// TotalThroughput is the total throughput (publish + consume)
	TotalThroughput float64 `json:"totalThroughput"`

	// AverageLatency is the average latency across all operations
	AverageLatency int64 `json:"averageLatency"`

	// TotalErrorRate is the total error rate
	TotalErrorRate float64 `json:"totalErrorRate"`

	// Efficiency is the efficiency score (0-100)
	Efficiency float64 `json:"efficiency"`

	// Recommendations contains performance recommendations
	Recommendations []string `json:"recommendations"`
}

// BenchmarkRunner provides benchmark execution capabilities.
type BenchmarkRunner struct {
	config        *BenchmarkConfig
	observability *ObservabilityContext
	performance   *PerformanceMonitor
}

// NewBenchmarkRunner creates a new benchmark runner.
func NewBenchmarkRunner(config *BenchmarkConfig, observability *ObservabilityContext, performance *PerformanceMonitor) *BenchmarkRunner {
	return &BenchmarkRunner{
		config:        config,
		observability: observability,
		performance:   performance,
	}
}

// RunBenchmark runs a comprehensive benchmark.
func (br *BenchmarkRunner) RunBenchmark(ctx context.Context, publisher Publisher, consumer Consumer) (*BenchmarkResult, error) {
	startTime := time.Now()

	// Create result
	result := &BenchmarkResult{
		Config:    br.config,
		StartTime: startTime,
	}

	// Run warmup
	if err := br.runWarmup(ctx, publisher, consumer); err != nil {
		return nil, WrapError(ErrorCodeInternal, "run_benchmark", "warmup failed", err)
	}

	// Run benchmark
	if err := br.runBenchmark(ctx, publisher, consumer, result); err != nil {
		return nil, WrapError(ErrorCodeInternal, "run_benchmark", "benchmark failed", err)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Calculate summary
	br.calculateSummary(result)

	return result, nil
}

// runWarmup runs the warmup phase.
func (br *BenchmarkRunner) runWarmup(ctx context.Context, publisher Publisher, consumer Consumer) error {
	br.observability.Logger().Info("Starting warmup phase", logx.String("duration", br.config.WarmupDuration.String()))

	warmupCtx, cancel := context.WithTimeout(ctx, br.config.WarmupDuration)
	defer cancel()

	// Run warmup publishers
	var wg sync.WaitGroup
	for i := 0; i < br.config.PublisherCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			br.runWarmupPublisher(warmupCtx, publisher)
		}()
	}

	// Run warmup consumers
	for i := 0; i < br.config.ConsumerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			br.runWarmupConsumer(warmupCtx, consumer)
		}()
	}

	wg.Wait()
	br.observability.Logger().Info("Warmup phase completed")

	return nil
}

// runWarmupPublisher runs a warmup publisher.
func (br *BenchmarkRunner) runWarmupPublisher(ctx context.Context, publisher Publisher) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg := br.createTestMessage()
			receipt, err := publisher.PublishAsync(ctx, "benchmark.exchange", msg)
			if err != nil {
				continue
			}

			select {
			case <-receipt.Done():
				// Message published
			case <-time.After(1 * time.Second):
				// Timeout
			}
		}
	}
}

// runWarmupConsumer runs a warmup consumer.
func (br *BenchmarkRunner) runWarmupConsumer(ctx context.Context, consumer Consumer) {
	handler := HandlerFunc(func(ctx context.Context, delivery Delivery) (AckDecision, error) {
		return Ack, nil
	})

	if err := consumer.Start(ctx, handler); err != nil {
		return
	}
	defer consumer.Stop(ctx)

	<-ctx.Done()
}

// runBenchmark runs the actual benchmark.
func (br *BenchmarkRunner) runBenchmark(ctx context.Context, publisher Publisher, consumer Consumer, result *BenchmarkResult) error {
	br.observability.Logger().Info("Starting benchmark phase", logx.String("duration", br.config.Duration.String()))

	benchmarkCtx, cancel := context.WithTimeout(ctx, br.config.Duration)
	defer cancel()

	// Start system monitoring
	systemMonitor := br.startSystemMonitoring(benchmarkCtx)

	// Run publishers
	publisherResults := br.runPublishers(benchmarkCtx, publisher)

	// Run consumers
	consumerResults := br.runConsumers(benchmarkCtx, consumer)

	// Wait for completion
	<-benchmarkCtx.Done()

	// Stop system monitoring
	systemResults := br.stopSystemMonitoring(systemMonitor)

	// Set results
	result.PublisherResults = publisherResults
	result.ConsumerResults = consumerResults
	result.SystemResults = systemResults

	br.observability.Logger().Info("Benchmark phase completed")

	return nil
}

// runPublishers runs the publisher benchmarks.
func (br *BenchmarkRunner) runPublishers(ctx context.Context, publisher Publisher) *PublisherBenchmarkResult {
	var totalMessages, successfulMessages, failedMessages uint64
	var latencyHistogram *LatencyHistogram

	if br.config.EnableLatencyTracking {
		latencyHistogram = NewLatencyHistogram()
	}

	var wg sync.WaitGroup
	for i := 0; i < br.config.PublisherCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			br.runPublisher(ctx, publisher, &totalMessages, &successfulMessages, &failedMessages, latencyHistogram)
		}()
	}

	wg.Wait()

	// Calculate results
	duration := float64(br.config.Duration.Seconds())
	throughput := float64(successfulMessages) / duration
	errorRate := float64(failedMessages) / float64(totalMessages) * 100

	result := &PublisherBenchmarkResult{
		TotalMessages:      totalMessages,
		SuccessfulMessages: successfulMessages,
		FailedMessages:     failedMessages,
		Throughput:         throughput,
		ErrorRate:          errorRate,
	}

	if latencyHistogram != nil {
		result.AverageLatency = latencyHistogram.Percentile(50) // Use P50 as average for now
		result.LatencyP50 = latencyHistogram.Percentile(50)
		result.LatencyP95 = latencyHistogram.Percentile(95)
		result.LatencyP99 = latencyHistogram.Percentile(99)
	}

	return result
}

// runPublisher runs a single publisher.
func (br *BenchmarkRunner) runPublisher(ctx context.Context, publisher Publisher, totalMessages, successfulMessages, failedMessages *uint64, latencyHistogram *LatencyHistogram) {
	ticker := time.NewTicker(time.Microsecond) // High frequency publishing
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			atomic.AddUint64(totalMessages, 1)

			msg := br.createTestMessage()
			start := time.Now()

			receipt, err := publisher.PublishAsync(ctx, "benchmark.exchange", msg)
			if err != nil {
				atomic.AddUint64(failedMessages, 1)
				continue
			}

			select {
			case <-receipt.Done():
				duration := time.Since(start)
				atomic.AddUint64(successfulMessages, 1)

				if latencyHistogram != nil {
					latencyHistogram.Record(duration.Nanoseconds())
				}

				// Record in performance monitor
				if br.performance != nil {
					br.performance.RecordPublish(duration, true)
				}

			case <-time.After(1 * time.Second):
				atomic.AddUint64(failedMessages, 1)
				if br.performance != nil {
					br.performance.RecordPublish(time.Since(start), false)
				}
			}
		}
	}
}

// runConsumers runs the consumer benchmarks.
func (br *BenchmarkRunner) runConsumers(ctx context.Context, consumer Consumer) *ConsumerBenchmarkResult {
	var totalMessages, successfulMessages, failedMessages uint64
	var latencyHistogram *LatencyHistogram

	if br.config.EnableLatencyTracking {
		latencyHistogram = NewLatencyHistogram()
	}

	// Start consumer
	handler := HandlerFunc(func(ctx context.Context, delivery Delivery) (AckDecision, error) {
		atomic.AddUint64(&totalMessages, 1)
		start := time.Now()

		// Simulate processing
		time.Sleep(1 * time.Microsecond)

		duration := time.Since(start)
		atomic.AddUint64(&successfulMessages, 1)

		if latencyHistogram != nil {
			latencyHistogram.Record(duration.Nanoseconds())
		}

		if br.performance != nil {
			br.performance.RecordConsume(duration, true)
		}

		return Ack, nil
	})

	if err := consumer.Start(ctx, handler); err != nil {
		return &ConsumerBenchmarkResult{
			FailedMessages: 1,
			ErrorRate:      100,
		}
	}
	defer consumer.Stop(ctx)

	// Wait for completion
	<-ctx.Done()

	// Calculate results
	duration := float64(br.config.Duration.Seconds())
	throughput := float64(successfulMessages) / duration
	errorRate := float64(failedMessages) / float64(totalMessages) * 100

	result := &ConsumerBenchmarkResult{
		TotalMessages:      totalMessages,
		SuccessfulMessages: successfulMessages,
		FailedMessages:     failedMessages,
		Throughput:         throughput,
		ErrorRate:          errorRate,
	}

	if latencyHistogram != nil {
		result.AverageLatency = latencyHistogram.Percentile(50)
		result.LatencyP50 = latencyHistogram.Percentile(50)
		result.LatencyP95 = latencyHistogram.Percentile(95)
		result.LatencyP99 = latencyHistogram.Percentile(99)
	}

	return result
}

// startSystemMonitoring starts system monitoring.
func (br *BenchmarkRunner) startSystemMonitoring(ctx context.Context) chan *SystemBenchmarkResult {
	resultChan := make(chan *SystemBenchmarkResult, 1)

	go func() {
		var peakMemory, totalMemory uint64
		var peakGoroutines, totalGoroutines int
		var totalGCs uint32
		var totalGCPauseTime uint64
		var sampleCount int

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var lastGCStats runtime.MemStats
		runtime.ReadMemStats(&lastGCStats)

		for {
			select {
			case <-ctx.Done():
				// Calculate averages
				var avgMemory, avgGoroutines uint64
				var avgGCPauseTime uint64

				if sampleCount > 0 {
					avgMemory = totalMemory / uint64(sampleCount)
					avgGoroutines = uint64(totalGoroutines / sampleCount)
					avgGCPauseTime = totalGCPauseTime / uint64(sampleCount)
				}

				result := &SystemBenchmarkResult{
					PeakMemoryUsage:    peakMemory,
					AverageMemoryUsage: avgMemory,
					PeakGoroutines:     peakGoroutines,
					AverageGoroutines:  int(avgGoroutines),
					TotalGCs:           totalGCs,
					TotalGCPauseTime:   totalGCPauseTime,
					AverageGCPauseTime: avgGCPauseTime,
				}

				resultChan <- result
				return

			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				// Track memory usage
				if memStats.HeapInuse > peakMemory {
					peakMemory = memStats.HeapInuse
				}
				totalMemory += memStats.HeapInuse

				// Track goroutines
				goroutines := runtime.NumGoroutine()
				if goroutines > peakGoroutines {
					peakGoroutines = goroutines
				}
				totalGoroutines += goroutines

				// Track GC
				if memStats.NumGC > lastGCStats.NumGC {
					totalGCs += memStats.NumGC - lastGCStats.NumGC
					totalGCPauseTime += memStats.PauseTotalNs - lastGCStats.PauseTotalNs
				}
				lastGCStats = memStats

				sampleCount++
			}
		}
	}()

	return resultChan
}

// stopSystemMonitoring stops system monitoring.
func (br *BenchmarkRunner) stopSystemMonitoring(resultChan chan *SystemBenchmarkResult) *SystemBenchmarkResult {
	result := <-resultChan
	return result
}

// calculateSummary calculates benchmark summary.
func (br *BenchmarkRunner) calculateSummary(result *BenchmarkResult) {
	// Calculate total throughput
	totalThroughput := result.PublisherResults.Throughput + result.ConsumerResults.Throughput

	// Calculate average latency
	totalLatency := result.PublisherResults.AverageLatency + result.ConsumerResults.AverageLatency
	averageLatency := totalLatency / 2

	// Calculate total error rate
	totalErrors := result.PublisherResults.FailedMessages + result.ConsumerResults.FailedMessages
	totalMessages := result.PublisherResults.TotalMessages + result.ConsumerResults.TotalMessages
	totalErrorRate := float64(totalErrors) / float64(totalMessages) * 100

	// Calculate efficiency (0-100)
	efficiency := 100.0
	if totalErrorRate > 0 {
		efficiency -= totalErrorRate
	}
	if averageLatency > 1000000 { // 1ms
		efficiency -= 10
	}

	// Generate recommendations
	var recommendations []string
	if totalErrorRate > 1 {
		recommendations = append(recommendations, "High error rate detected. Check network connectivity and server capacity.")
	}
	if averageLatency > 1000000 {
		recommendations = append(recommendations, "High latency detected. Consider optimizing network or reducing load.")
	}
	if result.SystemResults.PeakMemoryUsage > 100*1024*1024 { // 100MB
		recommendations = append(recommendations, "High memory usage detected. Consider reducing batch sizes or message sizes.")
	}
	if result.SystemResults.PeakGoroutines > 1000 {
		recommendations = append(recommendations, "High goroutine count detected. Consider reducing concurrency.")
	}

	result.Summary = &BenchmarkSummary{
		TotalThroughput: totalThroughput,
		AverageLatency:  averageLatency,
		TotalErrorRate:  totalErrorRate,
		Efficiency:      efficiency,
		Recommendations: recommendations,
	}
}

// createTestMessage creates a test message.
func (br *BenchmarkRunner) createTestMessage() Message {
	// Create message with specified size
	body := make([]byte, br.config.MessageSize)
	for i := range body {
		body[i] = byte(i % 256)
	}

	return NewMessage(
		body,
		WithID(fmt.Sprintf("benchmark-%d", time.Now().UnixNano())),
		WithContentType("application/octet-stream"),
		WithKey("benchmark.key"),
	)
}
