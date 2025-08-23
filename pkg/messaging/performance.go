// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
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
}

// NewPerformanceMonitor creates a new performance monitor.
func NewPerformanceMonitor(config *PerformanceConfig, observability *ObservabilityContext) *PerformanceMonitor {
	pm := &PerformanceMonitor{
		config:         config,
		observability:  observability,
		publishLatency: NewLatencyHistogram(),
		consumeLatency: NewLatencyHistogram(),
	}

	// Start metrics collection if enabled
	if config.PerformanceMetricsInterval > 0 {
		go pm.collectMetrics()
	}

	return pm
}

// RecordPublish records a publish operation.
func (pm *PerformanceMonitor) RecordPublish(duration time.Duration, success bool) {
	atomic.AddUint64(&pm.publishCount, 1)
	pm.publishLatency.Record(duration.Nanoseconds())

	if !success {
		atomic.AddUint64(&pm.errorCount, 1)
	}
}

// RecordConsume records a consume operation.
func (pm *PerformanceMonitor) RecordConsume(duration time.Duration, success bool) {
	atomic.AddUint64(&pm.consumeCount, 1)
	pm.consumeLatency.Record(duration.Nanoseconds())

	if !success {
		atomic.AddUint64(&pm.errorCount, 1)
	}
}

// GetMetrics returns the current performance metrics.
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.metrics
}

// collectMetrics collects performance metrics periodically.
func (pm *PerformanceMonitor) collectMetrics() {
	ticker := time.NewTicker(pm.config.PerformanceMetricsInterval)
	defer ticker.Stop()

	for range ticker.C {
		if pm.closed {
			return
		}

		pm.updateMetrics()
	}
}

// updateMetrics updates the performance metrics.
func (pm *PerformanceMonitor) updateMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate throughput
	interval := float64(pm.config.PerformanceMetricsInterval.Seconds())
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

	// Calculate memory utilization
	memoryUtilization := float64(memStats.HeapInuse) / float64(pm.config.MemoryLimit) * 100

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

	// Record metrics in observability
	pm.observability.RecordPerformanceMetrics(pm.metrics)
}

// Close closes the performance monitor.
func (pm *PerformanceMonitor) Close(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.closed = true
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
		buckets: make([]uint64, 1000), // 1000 buckets for latency tracking
	}
}

// Record records a latency value.
func (lh *LatencyHistogram) Record(latencyNs int64) {
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

	// Logarithmic scale: log10(latency_ns)
	logLatency := float64(latencyNs)
	if logLatency > 0 {
		logLatency = float64(int64(logLatency))
	}

	// Map to bucket (0-999)
	bucket := int(logLatency / 1000.0) // Adjust scale as needed
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
	latency := float64(bucket) * 1000.0
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
	return &ObjectPool{
		pool:      make(chan interface{}, size),
		newFunc:   newFunc,
		resetFunc: resetFunc,
	}
}

// Get gets an object from the pool.
func (op *ObjectPool) Get() interface{} {
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
	op.mu.RLock()
	defer op.mu.RUnlock()

	if op.closed || obj == nil {
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
	op.mu.Lock()
	defer op.mu.Unlock()

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
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(batchSize int, batchTimeout time.Duration, processor func([]interface{}) error) *BatchProcessor {
	bp := &BatchProcessor{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		processor:    processor,
		items:        make([]interface{}, 0, batchSize),
	}

	return bp
}

// Add adds an item to the batch.
func (bp *BatchProcessor) Add(item interface{}) error {
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
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if len(bp.items) > 0 {
		return bp.processBatch()
	}

	return nil
}

// Close closes the batch processor.
func (bp *BatchProcessor) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.closed = true

	if bp.timer != nil {
		bp.timer.Stop()
	}

	return bp.processBatch()
}

// process processes the batch (called by timer).
func (bp *BatchProcessor) process() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if len(bp.items) > 0 {
		bp.processBatch()
	}
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

	// Process batch
	items := make([]interface{}, len(bp.items))
	copy(items, bp.items)
	bp.items = bp.items[:0]

	// Process in goroutine to avoid blocking
	go func() {
		if err := bp.processor(items); err != nil {
			// Log error but don't block
			// TODO: Add proper error handling
		}
	}()

	return nil
}
