// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Performance-related constants
const (
	// DefaultBatchSize is the default batch size for operations
	DefaultBatchSize = 100

	// DefaultBatchTimeout is the default timeout for batch operations
	DefaultBatchTimeout = 100 * time.Millisecond

	// DefaultObjectPoolSize is the default size for object pools
	DefaultObjectPoolSize = 100

	// DefaultMemoryLimit is the default memory limit in bytes (1GB)
	DefaultMemoryLimit = 1073741824

	// DefaultPerformanceMetricsInterval is the default interval for collecting performance metrics
	DefaultPerformanceMetricsInterval = 10 * time.Second

	// LatencyHistogramBuckets is the number of buckets for latency tracking
	LatencyHistogramBuckets = 1000

	// LatencyHistogramScalingFactor is the scaling factor for logarithmic bucket mapping
	// 1000 buckets / 9 decades (1ns to 1s) ≈ 111.11
	LatencyHistogramScalingFactor = 111.11

	// LatencyHistogramMaxDecades is the maximum number of decades for latency range
	LatencyHistogramMaxDecades = 9.0
)

// PerformanceConfig defines performance-related configuration.
type PerformanceConfig struct {
	// EnableObjectPooling enables object pooling for better performance
	EnableObjectPooling bool `yaml:"enableObjectPooling" env:"MSG_PERFORMANCE_ENABLE_OBJECT_POOLING" default:"true"`

	// ObjectPoolSize is the size of object pools
	ObjectPoolSize int `yaml:"objectPoolSize" env:"MSG_PERFORMANCE_OBJECT_POOL_SIZE" validate:"min=1,max=10000" default:"1000"`

	// EnableMemoryProfiling enables memory profiling
	EnableMemoryProfiling bool `yaml:"enableMemoryProfiling" env:"MSG_PERFORMANCE_ENABLE_MEMORY_PROFILING" default:"false"`

	// EnableCPUProfiling enables CPU profiling
	EnableCPUProfiling bool `yaml:"enableCPUProfiling" env:"MSG_PERFORMANCE_ENABLE_CPU_PROFILING" default:"false"`

	// PerformanceMetricsInterval is the interval for collecting performance metrics
	PerformanceMetricsInterval time.Duration `yaml:"performanceMetricsInterval" env:"MSG_PERFORMANCE_METRICS_INTERVAL" default:"10s"`

	// MemoryLimit is the memory limit in bytes
	MemoryLimit int64 `yaml:"memoryLimit" env:"MSG_PERFORMANCE_MEMORY_LIMIT" default:"1073741824"` // 1GB

	// GCThreshold is the garbage collection threshold percentage
	GCThreshold float64 `yaml:"gcThreshold" env:"MSG_PERFORMANCE_GC_THRESHOLD" validate:"min=0,max=100" default:"80"`

	// BatchSize is the default batch size for operations
	BatchSize int `yaml:"batchSize" env:"MSG_PERFORMANCE_BATCH_SIZE" validate:"min=1,max=10000" default:"100"`

	// BatchTimeout is the timeout for batch operations
	BatchTimeout time.Duration `yaml:"batchTimeout" env:"MSG_PERFORMANCE_BATCH_TIMEOUT" default:"100ms"`
}

// PerformanceMetrics contains performance metrics.
type PerformanceMetrics struct {
	// Timestamp is when the metrics were collected
	Timestamp time.Time `json:"timestamp"`

	// Throughput metrics
	PublishThroughput float64 `json:"publishThroughput"` // messages per second
	ConsumeThroughput float64 `json:"consumeThroughput"` // messages per second
	TotalThroughput   float64 `json:"totalThroughput"`   // total messages per second

	// Latency metrics (in nanoseconds)
	PublishLatencyP50 int64 `json:"publishLatencyP50"`
	PublishLatencyP95 int64 `json:"publishLatencyP95"`
	PublishLatencyP99 int64 `json:"publishLatencyP99"`
	ConsumeLatencyP50 int64 `json:"consumeLatencyP50"`
	ConsumeLatencyP95 int64 `json:"consumeLatencyP95"`
	ConsumeLatencyP99 int64 `json:"consumeLatencyP99"`

	// Memory metrics
	HeapAlloc         uint64  `json:"heapAlloc"`         // bytes
	HeapSys           uint64  `json:"heapSys"`           // bytes
	HeapIdle          uint64  `json:"heapIdle"`          // bytes
	HeapInuse         uint64  `json:"heapInuse"`         // bytes
	HeapReleased      uint64  `json:"heapReleased"`      // bytes
	HeapObjects       uint64  `json:"heapObjects"`       // number of objects
	MemoryUtilization float64 `json:"memoryUtilization"` // percentage

	// GC metrics
	GCPauseTotalNs uint64 `json:"gcPauseTotalNs"`
	NumGC          uint32 `json:"numGC"`
	NumGoroutines  int    `json:"numGoroutines"`

	// Error metrics
	ErrorRate   float64 `json:"errorRate"` // errors per second
	TotalErrors uint64  `json:"totalErrors"`

	// Resource utilization
	ActiveConnections int     `json:"activeConnections"`
	ActiveChannels    int     `json:"activeChannels"`
	PoolUtilization   float64 `json:"poolUtilization"` // percentage
}

// PerformanceMonitor provides performance monitoring capabilities.
type PerformanceMonitor struct {
	config        *PerformanceConfig
	metrics       *PerformanceMetrics
	mu            sync.RWMutex
	closed        bool
	observability *ObservabilityContext

	// Counters
	publishCount   uint64
	consumeCount   uint64
	errorCount     uint64
	publishLatency *LatencyHistogram
	consumeLatency *LatencyHistogram

	// Memory tracking
	lastGCStats runtime.MemStats
	lastGC      uint32

	// Goroutine management
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPerformanceMonitor creates a new performance monitor.
func NewPerformanceMonitor(config *PerformanceConfig, observability *ObservabilityContext) *PerformanceMonitor {
	// Validate input parameters
	if config == nil {
		config = &PerformanceConfig{
			PerformanceMetricsInterval: DefaultPerformanceMetricsInterval,
			MemoryLimit:                DefaultMemoryLimit,
		}
	}

	pm := &PerformanceMonitor{
		config:         config,
		observability:  observability,
		publishLatency: NewLatencyHistogram(),
		consumeLatency: NewLatencyHistogram(),
		stopChan:       make(chan struct{}),
	}

	// Start metrics collection if enabled
	if config.PerformanceMetricsInterval > 0 {
		pm.wg.Add(1)
		go pm.collectMetrics()
	}

	return pm
}

// RecordPublish records a publish operation.
func (pm *PerformanceMonitor) RecordPublish(duration time.Duration, success bool) {
	if pm == nil {
		return
	}

	atomic.AddUint64(&pm.publishCount, 1)
	pm.publishLatency.Record(duration.Nanoseconds())

	if !success {
		atomic.AddUint64(&pm.errorCount, 1)
	}
}

// RecordConsume records a consume operation.
func (pm *PerformanceMonitor) RecordConsume(duration time.Duration, success bool) {
	if pm == nil {
		return
	}

	atomic.AddUint64(&pm.consumeCount, 1)
	pm.consumeLatency.Record(duration.Nanoseconds())

	if !success {
		atomic.AddUint64(&pm.errorCount, 1)
	}
}

// GetMetrics returns the current performance metrics.
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	if pm == nil {
		return nil
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.metrics
}

// collectMetrics collects performance metrics periodically.
func (pm *PerformanceMonitor) collectMetrics() {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.PerformanceMetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if pm.closed {
				return
			}
			pm.updateMetrics()
		case <-pm.stopChan:
			return
		}
	}
}

// updateMetrics updates the performance metrics.
func (pm *PerformanceMonitor) updateMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate throughput with protection against division by zero
	interval := pm.config.PerformanceMetricsInterval.Seconds()
	if interval <= 0 {
		interval = 1.0 // Default to 1 second to avoid division by zero
	}

	publishCount := atomic.LoadUint64(&pm.publishCount)
	consumeCount := atomic.LoadUint64(&pm.consumeCount)
	errorCount := atomic.LoadUint64(&pm.errorCount)

	publishThroughput := float64(publishCount) / interval
	consumeThroughput := float64(consumeCount) / interval
	totalThroughput := publishThroughput + consumeThroughput
	errorRate := float64(errorCount) / interval

	// Calculate latency percentiles
	publishP50 := pm.publishLatency.Percentile(50)
	publishP95 := pm.publishLatency.Percentile(95)
	publishP99 := pm.publishLatency.Percentile(99)
	consumeP50 := pm.consumeLatency.Percentile(50)
	consumeP95 := pm.consumeLatency.Percentile(95)
	consumeP99 := pm.consumeLatency.Percentile(99)

	// Calculate memory utilization with protection against division by zero
	memoryUtilization := 0.0
	if pm.config.MemoryLimit > 0 {
		memoryUtilization = float64(memStats.HeapInuse) / float64(pm.config.MemoryLimit) * 100
	}

	// Calculate GC metrics
	gcPauseDelta := memStats.PauseTotalNs - pm.lastGCStats.PauseTotalNs
	numGCDelta := memStats.NumGC - pm.lastGC

	pm.metrics = &PerformanceMetrics{
		Timestamp:         time.Now(),
		PublishThroughput: publishThroughput,
		ConsumeThroughput: consumeThroughput,
		TotalThroughput:   totalThroughput,
		PublishLatencyP50: publishP50,
		PublishLatencyP95: publishP95,
		PublishLatencyP99: publishP99,
		ConsumeLatencyP50: consumeP50,
		ConsumeLatencyP95: consumeP95,
		ConsumeLatencyP99: consumeP99,
		HeapAlloc:         memStats.HeapAlloc,
		HeapSys:           memStats.HeapSys,
		HeapIdle:          memStats.HeapIdle,
		HeapInuse:         memStats.HeapInuse,
		HeapReleased:      memStats.HeapReleased,
		HeapObjects:       memStats.HeapObjects,
		MemoryUtilization: memoryUtilization,
		GCPauseTotalNs:    gcPauseDelta,
		NumGC:             numGCDelta,
		NumGoroutines:     runtime.NumGoroutine(),
		ErrorRate:         errorRate,
		TotalErrors:       errorCount,
	}

	// Update last GC stats
	pm.lastGCStats = memStats
	pm.lastGC = memStats.NumGC

	// Reset counters
	atomic.StoreUint64(&pm.publishCount, 0)
	atomic.StoreUint64(&pm.consumeCount, 0)
	atomic.StoreUint64(&pm.errorCount, 0)

	// Record metrics in observability if available
	if pm.observability != nil {
		pm.observability.RecordPerformanceMetrics(pm.metrics)
	}
}

// Close closes the performance monitor.
func (pm *PerformanceMonitor) Close(ctx context.Context) error {
	if pm == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return nil
	}

	pm.closed = true
	close(pm.stopChan)

	// Wait for goroutines to finish
	pm.wg.Wait()

	return nil
}

// LatencyHistogram provides latency histogram functionality.
type LatencyHistogram struct {
	buckets []uint64
	mu      sync.RWMutex
}

// NewLatencyHistogram creates a new latency histogram.
func NewLatencyHistogram() *LatencyHistogram {
	return &LatencyHistogram{
		buckets: make([]uint64, LatencyHistogramBuckets),
	}
}

// Record records a latency value.
func (lh *LatencyHistogram) Record(latencyNs int64) {
	if lh == nil {
		return
	}

	lh.mu.Lock()
	defer lh.mu.Unlock()

	// Convert nanoseconds to bucket index (logarithmic scale)
	bucket := lh.latencyToBucket(latencyNs)
	if bucket >= 0 && bucket < len(lh.buckets) {
		lh.buckets[bucket]++
	}
}

// Percentile calculates the percentile latency.
func (lh *LatencyHistogram) Percentile(p float64) int64 {
	if lh == nil {
		return 0
	}

	lh.mu.RLock()
	defer lh.mu.RUnlock()

	total := uint64(0)
	for _, count := range lh.buckets {
		total += count
	}

	if total == 0 {
		return 0
	}

	target := uint64(float64(total) * p / 100.0)
	current := uint64(0)

	for i, count := range lh.buckets {
		current += count
		if current >= target {
			return lh.bucketToLatency(i)
		}
	}

	return lh.bucketToLatency(len(lh.buckets) - 1)
}

// latencyToBucket converts latency to bucket index.
func (lh *LatencyHistogram) latencyToBucket(latencyNs int64) int {
	if latencyNs <= 0 {
		return 0
	}

	// Proper logarithmic scale: log10(latency_ns)
	logLatency := math.Log10(float64(latencyNs))

	// Map to bucket (0-999) with proper scaling
	// Assuming latency range from 1ns to 1s (1e9 ns)
	// log10(1) = 0, log10(1e9) = 9
	bucket := int(logLatency * LatencyHistogramScalingFactor)

	if bucket < 0 {
		bucket = 0
	}
	if bucket >= len(lh.buckets) {
		bucket = len(lh.buckets) - 1
	}

	return bucket
}

// bucketToLatency converts bucket index to latency.
func (lh *LatencyHistogram) bucketToLatency(bucket int) int64 {
	// Reverse of latencyToBucket
	// bucket / LatencyHistogramScalingFactor gives us the log10 value
	logLatency := float64(bucket) / LatencyHistogramScalingFactor
	latency := math.Pow(10, logLatency)
	return int64(latency)
}

// ObjectPool provides object pooling for better performance.
type ObjectPool struct {
	pool      chan interface{}
	newFunc   func() interface{}
	resetFunc func(interface{})
	mu        sync.RWMutex
	closed    bool
}

// NewObjectPool creates a new object pool.
func NewObjectPool(size int, newFunc func() interface{}, resetFunc func(interface{})) *ObjectPool {
	if size <= 0 {
		size = DefaultObjectPoolSize
	}

	if newFunc == nil {
		return nil // Cannot create pool without newFunc
	}

	return &ObjectPool{
		pool:      make(chan interface{}, size),
		newFunc:   newFunc,
		resetFunc: resetFunc,
	}
}

// Get gets an object from the pool.
func (op *ObjectPool) Get() interface{} {
	if op == nil || op.newFunc == nil {
		return nil
	}

	op.mu.RLock()
	defer op.mu.RUnlock()

	if op.closed {
		return op.newFunc()
	}

	select {
	case obj := <-op.pool:
		return obj
	default:
		return op.newFunc()
	}
}

// Put puts an object back to the pool.
func (op *ObjectPool) Put(obj interface{}) {
	if op == nil || obj == nil {
		return
	}

	op.mu.RLock()
	defer op.mu.RUnlock()

	if op.closed {
		return
	}

	if op.resetFunc != nil {
		op.resetFunc(obj)
	}

	select {
	case op.pool <- obj:
		// Successfully put back to pool
	default:
		// Pool is full, discard object
	}
}

// Close closes the object pool.
func (op *ObjectPool) Close() {
	if op == nil {
		return
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	if op.closed {
		return
	}

	op.closed = true
	close(op.pool)
}

// BatchProcessor provides batch processing capabilities.
type BatchProcessor struct {
	batchSize    int
	batchTimeout time.Duration
	processor    func([]interface{}) error
	items        []interface{}
	mu           sync.Mutex
	timer        *time.Timer
	closed       bool
	errorHandler func(error)
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(batchSize int, batchTimeout time.Duration, processor func([]interface{}) error) *BatchProcessor {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	if batchTimeout <= 0 {
		batchTimeout = DefaultBatchTimeout
	}

	if processor == nil {
		return nil // Cannot create processor without processor function
	}

	bp := &BatchProcessor{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		processor:    processor,
		items:        make([]interface{}, 0, batchSize),
	}

	return bp
}

// SetErrorHandler sets the error handler for batch processing errors.
func (bp *BatchProcessor) SetErrorHandler(handler func(error)) {
	if bp != nil {
		bp.errorHandler = handler
	}
}

// Add adds an item to the batch.
func (bp *BatchProcessor) Add(item interface{}) error {
	if bp == nil {
		return NewError(ErrorCodeInternal, "batch_processor", "batch processor is nil")
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return NewError(ErrorCodeInternal, "batch_processor", "batch processor is closed")
	}

	bp.items = append(bp.items, item)

	// Start timer if this is the first item
	if len(bp.items) == 1 {
		bp.timer = time.AfterFunc(bp.batchTimeout, bp.process)
	}

	// Process if batch is full
	if len(bp.items) >= bp.batchSize {
		return bp.processBatch()
	}

	return nil
}

// Flush flushes the current batch.
func (bp *BatchProcessor) Flush() error {
	if bp == nil {
		return NewError(ErrorCodeInternal, "batch_processor", "batch processor is nil")
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if len(bp.items) > 0 {
		return bp.processBatch()
	}

	return nil
}

// Close closes the batch processor.
func (bp *BatchProcessor) Close() error {
	if bp == nil {
		return nil
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return nil
	}

	bp.closed = true

	if bp.timer != nil {
		bp.timer.Stop()
		bp.timer = nil
	}

	return bp.processBatch()
}

// process processes the batch (called by timer).
func (bp *BatchProcessor) process() {
	if bp == nil {
		return
	}

	bp.mu.Lock()

	// Check if we have items to process
	if len(bp.items) == 0 {
		bp.mu.Unlock()
		return
	}

	// Stop timer
	if bp.timer != nil {
		bp.timer.Stop()
		bp.timer = nil
	}

	// Process batch using shared logic
	bp.processBatchInternal()

	bp.mu.Unlock()
}

// processBatch processes the current batch.
func (bp *BatchProcessor) processBatch() error {
	if len(bp.items) == 0 {
		return nil
	}

	// Stop timer
	if bp.timer != nil {
		bp.timer.Stop()
		bp.timer = nil
	}

	// Process batch using shared logic
	bp.processBatchInternal()

	return nil
}

// processBatchInternal contains the shared logic for processing batches.
func (bp *BatchProcessor) processBatchInternal() {
	// Process batch
	items := make([]interface{}, len(bp.items))
	copy(items, bp.items)
	bp.items = bp.items[:0]

	// Process in goroutine to avoid blocking
	go func() {
		if err := bp.processor(items); err != nil {
			// Use error handler if available
			if bp.errorHandler != nil {
				bp.errorHandler(err)
			}
		}
	}()
}
