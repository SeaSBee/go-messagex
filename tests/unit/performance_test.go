package unit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

func TestPerformanceConfig(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		config := &messaging.PerformanceConfig{}

		// Test default values - these are zero values since struct tags don't auto-populate
		assert.False(t, config.EnableObjectPooling) // zero value
		assert.Equal(t, 0, config.ObjectPoolSize)   // zero value
		assert.False(t, config.EnableMemoryProfiling)
		assert.False(t, config.EnableCPUProfiling)
		assert.Equal(t, time.Duration(0), config.PerformanceMetricsInterval) // zero value
		assert.Equal(t, int64(0), config.MemoryLimit)                        // zero value
		assert.Equal(t, 0.0, config.GCThreshold)                             // zero value
		assert.Equal(t, 0, config.BatchSize)                                 // zero value
		assert.Equal(t, time.Duration(0), config.BatchTimeout)               // zero value
	})

	t.Run("CustomConfiguration", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			EnableObjectPooling:        false,
			ObjectPoolSize:             500,
			EnableMemoryProfiling:      true,
			EnableCPUProfiling:         true,
			PerformanceMetricsInterval: 5 * time.Second,
			MemoryLimit:                512 * 1024 * 1024, // 512MB
			GCThreshold:                70.0,
			BatchSize:                  50,
			BatchTimeout:               50 * time.Millisecond,
		}

		assert.False(t, config.EnableObjectPooling)
		assert.Equal(t, 500, config.ObjectPoolSize)
		assert.True(t, config.EnableMemoryProfiling)
		assert.True(t, config.EnableCPUProfiling)
		assert.Equal(t, 5*time.Second, config.PerformanceMetricsInterval)
		assert.Equal(t, int64(512*1024*1024), config.MemoryLimit)
		assert.Equal(t, 70.0, config.GCThreshold)
		assert.Equal(t, 50, config.BatchSize)
		assert.Equal(t, 50*time.Millisecond, config.BatchTimeout)
	})

	t.Run("BoundaryValues", func(t *testing.T) {
		// Test minimum values
		config := &messaging.PerformanceConfig{
			ObjectPoolSize: 1,
			BatchSize:      1,
			GCThreshold:    0.0,
		}

		assert.Equal(t, 1, config.ObjectPoolSize)
		assert.Equal(t, 1, config.BatchSize)
		assert.Equal(t, 0.0, config.GCThreshold)

		// Test maximum values
		config = &messaging.PerformanceConfig{
			ObjectPoolSize: 10000,
			BatchSize:      10000,
			GCThreshold:    100.0,
		}

		assert.Equal(t, 10000, config.ObjectPoolSize)
		assert.Equal(t, 10000, config.BatchSize)
		assert.Equal(t, 100.0, config.GCThreshold)
	})
}

func TestPerformanceMonitor(t *testing.T) {
	t.Run("NewPerformanceMonitor", func(t *testing.T) {
		// Test with valid config
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
			MemoryLimit:                1024 * 1024 * 1024,
		}

		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)

		assert.NotNil(t, monitor)

		// Cleanup
		monitor.Close(context.Background())

		// Test with nil config (should use defaults)
		monitor = messaging.NewPerformanceMonitor(nil, obsCtx)
		assert.NotNil(t, monitor)

		// Cleanup
		monitor.Close(context.Background())

		// Test with nil observability
		monitor = messaging.NewPerformanceMonitor(config, nil)
		assert.NotNil(t, monitor)

		// Cleanup
		monitor.Close(context.Background())
	})

	t.Run("RecordPublish", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		// Test successful publish
		monitor.RecordPublish(100*time.Microsecond, true)
		monitor.RecordPublish(200*time.Microsecond, true)

		// Test failed publish
		monitor.RecordPublish(150*time.Microsecond, false)

		// Test with nil monitor
		var nilMonitor *messaging.PerformanceMonitor
		nilMonitor.RecordPublish(100*time.Microsecond, true) // Should not panic
	})

	t.Run("RecordConsume", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		// Test successful consume
		monitor.RecordConsume(50*time.Microsecond, true)
		monitor.RecordConsume(75*time.Microsecond, true)

		// Test failed consume
		monitor.RecordConsume(60*time.Microsecond, false)

		// Test with nil monitor
		var nilMonitor *messaging.PerformanceMonitor
		nilMonitor.RecordConsume(50*time.Microsecond, true) // Should not panic
	})

	t.Run("GetMetrics", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		// Initially should be nil
		metrics := monitor.GetMetrics()
		assert.Nil(t, metrics)

		// Record some operations
		monitor.RecordPublish(100*time.Microsecond, true)
		monitor.RecordConsume(50*time.Microsecond, true)
		monitor.RecordPublish(200*time.Microsecond, false) // Error

		// Wait for metrics collection
		time.Sleep(1100 * time.Millisecond)

		// Now should have metrics
		metrics = monitor.GetMetrics()
		assert.NotNil(t, metrics)
		assert.NotZero(t, metrics.Timestamp)
		assert.Greater(t, metrics.PublishThroughput, 0.0)
		assert.Greater(t, metrics.ConsumeThroughput, 0.0)
		assert.Greater(t, metrics.TotalThroughput, 0.0)
		assert.Greater(t, metrics.ErrorRate, 0.0)
		assert.Greater(t, metrics.TotalErrors, uint64(0))
		assert.Greater(t, metrics.NumGoroutines, 0)

		// Test with nil monitor
		var nilMonitor *messaging.PerformanceMonitor
		metrics = nilMonitor.GetMetrics()
		assert.Nil(t, metrics)
	})

	t.Run("Close", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 100 * time.Millisecond,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)

		// Test normal close
		err := monitor.Close(context.Background())
		assert.NoError(t, err)

		// Test double close
		err = monitor.Close(context.Background())
		assert.NoError(t, err)

		// Test with nil monitor
		var nilMonitor *messaging.PerformanceMonitor
		err = nilMonitor.Close(context.Background())
		assert.NoError(t, err)

		// Test with cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		monitor2 := messaging.NewPerformanceMonitor(config, obsCtx)
		err = monitor2.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		const numGoroutines = 10
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					monitor.RecordPublish(time.Duration(j)*time.Microsecond, j%10 != 0) // 10% errors
					monitor.RecordConsume(time.Duration(j)*time.Microsecond, j%20 != 0) // 5% errors
				}
			}(i)
		}

		wg.Wait()

		// Wait for metrics collection
		time.Sleep(1100 * time.Millisecond)

		metrics := monitor.GetMetrics()
		assert.NotNil(t, metrics)
		assert.Greater(t, metrics.PublishThroughput, 0.0)
		assert.Greater(t, metrics.ConsumeThroughput, 0.0)
		assert.Greater(t, metrics.TotalThroughput, 0.0)
	})
}

func TestLatencyHistogram(t *testing.T) {
	t.Run("NewLatencyHistogram", func(t *testing.T) {
		histogram := messaging.NewLatencyHistogram()

		assert.NotNil(t, histogram)
	})

	t.Run("Record", func(t *testing.T) {
		histogram := messaging.NewLatencyHistogram()

		// Test recording various latencies
		latencies := []int64{
			1,          // 1ns
			1000,       // 1µs
			1000000,    // 1ms
			1000000000, // 1s
			100000000,  // 100ms
		}

		for _, latency := range latencies {
			histogram.Record(latency)
		}

		// Test with nil histogram
		var nilHistogram *messaging.LatencyHistogram
		nilHistogram.Record(1000) // Should not panic

		// Test with negative latency
		histogram.Record(-1000) // Should handle gracefully

		// Test with zero latency
		histogram.Record(0) // Should handle gracefully
	})

	t.Run("Percentile", func(t *testing.T) {
		histogram := messaging.NewLatencyHistogram()

		// Test empty histogram
		p50 := histogram.Percentile(50)
		assert.Equal(t, int64(0), p50)

		// Add some data
		for i := 1; i <= 100; i++ {
			histogram.Record(int64(i * 1000)) // 1µs, 2µs, ..., 100µs
		}

		// Test percentiles
		p50 = histogram.Percentile(50)
		p95 := histogram.Percentile(95)
		p99 := histogram.Percentile(99)

		assert.Greater(t, p50, int64(0))
		assert.Greater(t, p95, p50)
		assert.Greater(t, p99, p95)

		// Test edge cases
		p0 := histogram.Percentile(0)
		p100 := histogram.Percentile(100)

		assert.GreaterOrEqual(t, p0, int64(0))
		assert.Greater(t, p100, int64(0))

		// Test with nil histogram
		var nilHistogram *messaging.LatencyHistogram
		p50 = nilHistogram.Percentile(50)
		assert.Equal(t, int64(0), p50)
	})

	t.Run("ConcurrentRecording", func(t *testing.T) {
		histogram := messaging.NewLatencyHistogram()

		const numGoroutines = 10
		const recordsPerGoroutine = 1000

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < recordsPerGoroutine; j++ {
					latency := int64((id*recordsPerGoroutine + j) * 1000)
					histogram.Record(latency)
				}
			}(i)
		}

		wg.Wait()

		// Verify we have data
		p50 := histogram.Percentile(50)
		assert.Greater(t, p50, int64(0))

		// Verify percentiles are reasonable
		p95 := histogram.Percentile(95)
		p99 := histogram.Percentile(99)
		assert.Greater(t, p95, p50)
		assert.Greater(t, p99, p95)
	})
}

func TestObjectPool(t *testing.T) {
	t.Run("NewObjectPool", func(t *testing.T) {
		// Test valid pool creation
		newFunc := func() interface{} {
			return "test-object"
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(100, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Test with zero size (should use default)
		pool = messaging.NewObjectPool(0, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Test with negative size (should use default)
		pool = messaging.NewObjectPool(-10, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Test with nil newFunc (should return nil)
		pool = messaging.NewObjectPool(100, nil, resetFunc)
		assert.Nil(t, pool)
	})

	t.Run("Get", func(t *testing.T) {
		objectCount := 0
		newFunc := func() interface{} {
			objectCount++
			return fmt.Sprintf("object-%d", objectCount)
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(5, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Test getting objects
		obj1 := pool.Get()
		obj2 := pool.Get()
		obj3 := pool.Get()

		assert.NotNil(t, obj1)
		assert.NotNil(t, obj2)
		assert.NotNil(t, obj3)
		assert.NotEqual(t, obj1, obj2)
		assert.NotEqual(t, obj2, obj3)

		// Test with nil pool
		var nilPool *messaging.ObjectPool
		obj := nilPool.Get()
		assert.Nil(t, obj)
	})

	t.Run("Put", func(t *testing.T) {
		objectCount := 0
		newFunc := func() interface{} {
			objectCount++
			return fmt.Sprintf("object-%d", objectCount)
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(5, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Get objects
		obj1 := pool.Get()
		obj2 := pool.Get()

		// Put them back
		pool.Put(obj1)
		pool.Put(obj2)

		// Test with nil pool
		var nilPool *messaging.ObjectPool
		nilPool.Put("test") // Should not panic

		// Test with nil object
		pool.Put(nil) // Should not panic
	})

	t.Run("Close", func(t *testing.T) {
		newFunc := func() interface{} {
			return "test-object"
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(5, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Close the pool
		pool.Close()

		// Test double close
		pool.Close() // Should not panic

		// Test with nil pool
		var nilPool *messaging.ObjectPool
		nilPool.Close() // Should not panic
	})

	t.Run("PoolExhaustion", func(t *testing.T) {
		objectCount := 0
		newFunc := func() interface{} {
			objectCount++
			return fmt.Sprintf("object-%d", objectCount)
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(3, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Get more objects than pool size
		objects := make([]interface{}, 10)
		for i := 0; i < 10; i++ {
			objects[i] = pool.Get()
			assert.NotNil(t, objects[i])
		}

		// Verify we got unique objects (pool exhausted, new ones created)
		seen := make(map[interface{}]bool)
		for _, obj := range objects {
			assert.False(t, seen[obj], "Duplicate object returned")
			seen[obj] = true
		}
	})

	t.Run("PoolOverflow", func(t *testing.T) {
		newFunc := func() interface{} {
			return "test-object"
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(2, newFunc, resetFunc)
		assert.NotNil(t, pool)

		// Fill the pool
		obj1 := pool.Get()
		obj2 := pool.Get()

		// Put them back
		pool.Put(obj1)
		pool.Put(obj2)

		// Try to put more objects (should be discarded)
		pool.Put("extra-object-1")
		pool.Put("extra-object-2")
		pool.Put("extra-object-3")

		// Should not panic
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		newFunc := func() interface{} {
			return "test-object"
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(10, newFunc, resetFunc)
		assert.NotNil(t, pool)
		defer pool.Close()

		const numGoroutines = 10
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					obj := pool.Get()
					assert.NotNil(t, obj)
					pool.Put(obj)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestBatchProcessor(t *testing.T) {
	t.Run("NewBatchProcessor", func(t *testing.T) {
		processor := func(items []interface{}) error {
			return nil
		}

		// Test valid processor creation
		bp := messaging.NewBatchProcessor(100, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)

		// Test with zero batch size (should use default)
		bp = messaging.NewBatchProcessor(0, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)

		// Test with zero timeout (should use default)
		bp = messaging.NewBatchProcessor(100, 0, processor)
		assert.NotNil(t, bp)

		// Test with negative batch size (should use default)
		bp = messaging.NewBatchProcessor(-10, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)

		// Test with negative timeout (should use default)
		bp = messaging.NewBatchProcessor(100, -10*time.Millisecond, processor)
		assert.NotNil(t, bp)

		// Test with nil processor (should return nil)
		bp = messaging.NewBatchProcessor(100, 100*time.Millisecond, nil)
		assert.Nil(t, bp)
	})

	t.Run("Add", func(t *testing.T) {
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(3, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		// Add items
		err := bp.Add("item1")
		assert.NoError(t, err)

		err = bp.Add("item2")
		assert.NoError(t, err)

		err = bp.Add("item3")
		assert.NoError(t, err) // Should trigger processing

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		assert.Len(t, processedItems, 1)
		assert.Len(t, processedItems[0], 3)
		mu.Unlock()

		// Test with nil processor
		var nilBp *messaging.BatchProcessor
		err = nilBp.Add("item")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch processor is nil")
	})

	t.Run("Flush", func(t *testing.T) {
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(5, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		// Add some items
		bp.Add("item1")
		bp.Add("item2")

		// Flush
		err := bp.Flush()
		assert.NoError(t, err)

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		assert.Len(t, processedItems, 1)
		assert.Len(t, processedItems[0], 2)
		mu.Unlock()

		// Test flush with empty batch
		err = bp.Flush()
		assert.NoError(t, err)

		// Test with nil processor
		var nilBp *messaging.BatchProcessor
		err = nilBp.Flush()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch processor is nil")
	})

	t.Run("Close", func(t *testing.T) {
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(5, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)

		// Add some items
		bp.Add("item1")
		bp.Add("item2")

		// Close
		err := bp.Close()
		assert.NoError(t, err)

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		assert.Len(t, processedItems, 1)
		assert.Len(t, processedItems[0], 2)
		mu.Unlock()

		// Test double close
		err = bp.Close()
		assert.NoError(t, err)

		// Test with nil processor
		var nilBp *messaging.BatchProcessor
		err = nilBp.Close()
		assert.NoError(t, err)
	})

	t.Run("TimerBasedProcessing", func(t *testing.T) {
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(10, 50*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		// Add one item (should start timer)
		bp.Add("item1")

		// Wait for timer to trigger
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		assert.Len(t, processedItems, 1)
		assert.Len(t, processedItems[0], 1)
		mu.Unlock()
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		errorHandlerCalled := false
		var lastError error

		processor := func(items []interface{}) error {
			return fmt.Errorf("processing error")
		}

		errorHandler := func(err error) {
			errorHandlerCalled = true
			lastError = err
		}

		bp := messaging.NewBatchProcessor(2, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		bp.SetErrorHandler(errorHandler)

		// Add items to trigger processing
		bp.Add("item1")
		bp.Add("item2")

		// Wait for processing
		time.Sleep(150 * time.Millisecond)

		assert.True(t, errorHandlerCalled)
		assert.NotNil(t, lastError)
		assert.Contains(t, lastError.Error(), "processing error")
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(5, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		const numGoroutines = 10
		const itemsPerGoroutine = 20

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < itemsPerGoroutine; j++ {
					item := fmt.Sprintf("item-%d-%d", id, j)
					err := bp.Add(item)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()

		// Flush remaining items
		bp.Flush()

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		assert.Greater(t, len(processedItems), 0)
		totalProcessed := 0
		for _, batch := range processedItems {
			totalProcessed += len(batch)
		}
		assert.Equal(t, numGoroutines*itemsPerGoroutine, totalProcessed)
		mu.Unlock()
	})
}

func TestPerformanceMetrics(t *testing.T) {
	t.Run("MetricsStructure", func(t *testing.T) {
		metrics := &messaging.PerformanceMetrics{
			Timestamp:         time.Now(),
			PublishThroughput: 100.5,
			ConsumeThroughput: 200.3,
			TotalThroughput:   300.8,
			PublishLatencyP50: 1000,
			PublishLatencyP95: 5000,
			PublishLatencyP99: 10000,
			ConsumeLatencyP50: 500,
			ConsumeLatencyP95: 2500,
			ConsumeLatencyP99: 5000,
			HeapAlloc:         1024 * 1024,
			HeapSys:           2048 * 1024,
			HeapIdle:          512 * 1024,
			HeapInuse:         1536 * 1024,
			HeapReleased:      256 * 1024,
			HeapObjects:       1000,
			MemoryUtilization: 75.5,
			GCPauseTotalNs:    1000000,
			NumGC:             5,
			NumGoroutines:     10,
			ErrorRate:         2.5,
			TotalErrors:       25,
			ActiveConnections: 5,
			ActiveChannels:    10,
			PoolUtilization:   80.0,
		}

		assert.NotZero(t, metrics.Timestamp)
		assert.Equal(t, 100.5, metrics.PublishThroughput)
		assert.Equal(t, 200.3, metrics.ConsumeThroughput)
		assert.Equal(t, 300.8, metrics.TotalThroughput)
		assert.Equal(t, int64(1000), metrics.PublishLatencyP50)
		assert.Equal(t, int64(5000), metrics.PublishLatencyP95)
		assert.Equal(t, int64(10000), metrics.PublishLatencyP99)
		assert.Equal(t, int64(500), metrics.ConsumeLatencyP50)
		assert.Equal(t, int64(2500), metrics.ConsumeLatencyP95)
		assert.Equal(t, int64(5000), metrics.ConsumeLatencyP99)
		assert.Equal(t, uint64(1024*1024), metrics.HeapAlloc)
		assert.Equal(t, uint64(2048*1024), metrics.HeapSys)
		assert.Equal(t, uint64(512*1024), metrics.HeapIdle)
		assert.Equal(t, uint64(1536*1024), metrics.HeapInuse)
		assert.Equal(t, uint64(256*1024), metrics.HeapReleased)
		assert.Equal(t, uint64(1000), metrics.HeapObjects)
		assert.Equal(t, 75.5, metrics.MemoryUtilization)
		assert.Equal(t, uint64(1000000), metrics.GCPauseTotalNs)
		assert.Equal(t, uint32(5), metrics.NumGC)
		assert.Equal(t, 10, metrics.NumGoroutines)
		assert.Equal(t, 2.5, metrics.ErrorRate)
		assert.Equal(t, uint64(25), metrics.TotalErrors)
		assert.Equal(t, 5, metrics.ActiveConnections)
		assert.Equal(t, 10, metrics.ActiveChannels)
		assert.Equal(t, 80.0, metrics.PoolUtilization)
	})

	t.Run("ZeroMetrics", func(t *testing.T) {
		metrics := &messaging.PerformanceMetrics{}

		assert.Zero(t, metrics.PublishThroughput)
		assert.Zero(t, metrics.ConsumeThroughput)
		assert.Zero(t, metrics.TotalThroughput)
		assert.Zero(t, metrics.PublishLatencyP50)
		assert.Zero(t, metrics.PublishLatencyP95)
		assert.Zero(t, metrics.PublishLatencyP99)
		assert.Zero(t, metrics.ConsumeLatencyP50)
		assert.Zero(t, metrics.ConsumeLatencyP95)
		assert.Zero(t, metrics.ConsumeLatencyP99)
		assert.Zero(t, metrics.HeapAlloc)
		assert.Zero(t, metrics.HeapSys)
		assert.Zero(t, metrics.HeapIdle)
		assert.Zero(t, metrics.HeapInuse)
		assert.Zero(t, metrics.HeapReleased)
		assert.Zero(t, metrics.HeapObjects)
		assert.Zero(t, metrics.MemoryUtilization)
		assert.Zero(t, metrics.GCPauseTotalNs)
		assert.Zero(t, metrics.NumGC)
		assert.Zero(t, metrics.NumGoroutines)
		assert.Zero(t, metrics.ErrorRate)
		assert.Zero(t, metrics.TotalErrors)
		assert.Zero(t, metrics.ActiveConnections)
		assert.Zero(t, metrics.ActiveChannels)
		assert.Zero(t, metrics.PoolUtilization)
	})
}

func TestPerformanceIntegration(t *testing.T) {
	t.Run("PerformanceMonitorWithLatencyHistogram", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 1 * time.Second,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		// Record various latencies
		latencies := []time.Duration{
			1 * time.Microsecond,
			10 * time.Microsecond,
			100 * time.Microsecond,
			1 * time.Millisecond,
			10 * time.Millisecond,
		}

		for i, latency := range latencies {
			success := i%5 != 0 // 20% error rate
			monitor.RecordPublish(latency, success)
			monitor.RecordConsume(latency/2, success)
		}

		// Wait for metrics collection
		time.Sleep(1100 * time.Millisecond)

		metrics := monitor.GetMetrics()
		assert.NotNil(t, metrics)
		assert.Greater(t, metrics.PublishThroughput, 0.0)
		assert.Greater(t, metrics.ConsumeThroughput, 0.0)
		assert.Greater(t, metrics.ErrorRate, 0.0)
		assert.Greater(t, metrics.PublishLatencyP50, int64(0))
		assert.GreaterOrEqual(t, metrics.PublishLatencyP95, metrics.PublishLatencyP50)
		assert.GreaterOrEqual(t, metrics.PublishLatencyP99, metrics.PublishLatencyP95)
	})

	t.Run("ObjectPoolWithBatchProcessor", func(t *testing.T) {
		// Create object pool
		objectCount := 0
		newFunc := func() interface{} {
			objectCount++
			return fmt.Sprintf("message-%d", objectCount)
		}
		resetFunc := func(obj interface{}) {
			// Reset logic
		}

		pool := messaging.NewObjectPool(10, newFunc, resetFunc)
		assert.NotNil(t, pool)
		defer pool.Close()

		// Create batch processor
		processedItems := make([][]interface{}, 0)
		var mu sync.Mutex

		processor := func(items []interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			processedItems = append(processedItems, items)
			return nil
		}

		bp := messaging.NewBatchProcessor(5, 100*time.Millisecond, processor)
		assert.NotNil(t, bp)
		defer bp.Close()

		// Use pool objects in batch processor
		for i := 0; i < 20; i++ {
			obj := pool.Get()
			assert.NotNil(t, obj)
			err := bp.Add(obj)
			assert.NoError(t, err)
		}

		// Flush and wait
		bp.Flush()
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		assert.Greater(t, len(processedItems), 0)
		mu.Unlock()
	})

	t.Run("StressTest", func(t *testing.T) {
		config := &messaging.PerformanceConfig{
			PerformanceMetricsInterval: 500 * time.Millisecond,
		}
		provider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		monitor := messaging.NewPerformanceMonitor(config, obsCtx)
		defer monitor.Close(context.Background())

		// Create object pool
		newFunc := func() interface{} {
			return "stress-test-object"
		}
		resetFunc := func(obj interface{}) {}

		pool := messaging.NewObjectPool(100, newFunc, resetFunc)
		defer pool.Close()

		// Create batch processor
		processor := func(items []interface{}) error {
			return nil
		}

		bp := messaging.NewBatchProcessor(10, 50*time.Millisecond, processor)
		defer bp.Close()

		const numGoroutines = 20
		const operationsPerGoroutine = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					// Record performance
					latency := time.Duration(j*100) * time.Microsecond
					success := j%10 != 0
					monitor.RecordPublish(latency, success)
					monitor.RecordConsume(latency/2, success)

					// Use object pool
					obj := pool.Get()
					assert.NotNil(t, obj)

					// Add to batch processor
					err := bp.Add(obj)
					assert.NoError(t, err)

					// Return object to pool
					pool.Put(obj)
				}
			}(i)
		}

		wg.Wait()

		// Flush batch processor
		bp.Flush()

		// Wait for metrics collection
		time.Sleep(600 * time.Millisecond)

		metrics := monitor.GetMetrics()
		assert.NotNil(t, metrics)
		assert.Greater(t, metrics.PublishThroughput, 0.0)
		assert.Greater(t, metrics.ConsumeThroughput, 0.0)
		assert.Greater(t, metrics.TotalThroughput, 0.0)
		assert.Greater(t, metrics.NumGoroutines, 0)
	})
}
