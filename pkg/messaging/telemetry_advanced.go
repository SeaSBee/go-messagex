// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/seasbee/go-logx"
)

// Custom context key types to avoid collisions
type (
	telemetryStartKey        struct{}
	telemetryDeliveryKey     struct{}
	telemetryHandlerStartKey struct{}
)

// SamplingConfig defines sampling configuration for telemetry.
type SamplingConfig struct {
	// Enabled enables sampling
	Enabled bool

	// Rate is the sampling rate (0.0 to 1.0)
	Rate float64

	// Seed is the random seed for sampling
	Seed int64
}

// BatchingConfig defines batching configuration for telemetry.
type BatchingConfig struct {
	// Enabled enables batching
	Enabled bool

	// BatchSize is the maximum batch size
	BatchSize int

	// FlushInterval is the interval between batch flushes
	FlushInterval time.Duration

	// MaxWaitTime is the maximum time to wait before flushing
	MaxWaitTime time.Duration
}

// AdvancedTelemetryProvider provides advanced telemetry capabilities.
type AdvancedTelemetryProvider struct {
	metrics Metrics
	tracer  Tracer
	logger  *logx.Logger

	samplingConfig SamplingConfig
	batchingConfig BatchingConfig
	rand           *mathrand.Rand
	mu             sync.RWMutex
}

// NewAdvancedTelemetryProvider creates a new advanced telemetry provider.
func NewAdvancedTelemetryProvider(
	metrics Metrics,
	tracer Tracer,
	logger *logx.Logger,
	samplingConfig SamplingConfig,
	batchingConfig BatchingConfig,
) *AdvancedTelemetryProvider {
	var r *mathrand.Rand
	if samplingConfig.Enabled {
		if samplingConfig.Seed == 0 {
			samplingConfig.Seed = time.Now().UnixNano()
		}
		r = mathrand.New(mathrand.NewSource(samplingConfig.Seed))
	}

	return &AdvancedTelemetryProvider{
		metrics:        metrics,
		tracer:         tracer,
		logger:         logger,
		samplingConfig: samplingConfig,
		batchingConfig: batchingConfig,
		rand:           r,
	}
}

// ShouldSample determines if an operation should be sampled.
func (atp *AdvancedTelemetryProvider) ShouldSample() bool {
	atp.mu.RLock()
	defer atp.mu.RUnlock()

	if !atp.samplingConfig.Enabled {
		return true
	}

	return atp.rand.Float64() <= atp.samplingConfig.Rate
}

// SampledMetrics provides sampled metrics collection.
type SampledMetrics struct {
	metrics Metrics
	sampler *AdvancedTelemetryProvider
}

// NewSampledMetrics creates a new sampled metrics implementation.
func NewSampledMetrics(metrics Metrics, sampler *AdvancedTelemetryProvider) *SampledMetrics {
	return &SampledMetrics{
		metrics: metrics,
		sampler: sampler,
	}
}

// PublishTotal increments the total number of published messages (sampled).
func (sm *SampledMetrics) PublishTotal(transport, exchange string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.PublishTotal(transport, exchange)
	}
}

// PublishSuccess increments the number of successfully published messages (sampled).
func (sm *SampledMetrics) PublishSuccess(transport, exchange string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.PublishSuccess(transport, exchange)
	}
}

// PublishFailure increments the number of failed published messages (sampled).
func (sm *SampledMetrics) PublishFailure(transport, exchange, reason string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.PublishFailure(transport, exchange, reason)
	}
}

// PublishDuration records the duration of a publish operation (sampled).
func (sm *SampledMetrics) PublishDuration(transport, exchange string, duration time.Duration) {
	if sm.sampler.ShouldSample() {
		sm.metrics.PublishDuration(transport, exchange, duration)
	}
}

// ConsumeTotal increments the total number of consumed messages (sampled).
func (sm *SampledMetrics) ConsumeTotal(transport, queue string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ConsumeTotal(transport, queue)
	}
}

// ConsumeSuccess increments the number of successfully consumed messages (sampled).
func (sm *SampledMetrics) ConsumeSuccess(transport, queue string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ConsumeSuccess(transport, queue)
	}
}

// ConsumeFailure increments the number of failed consumed messages (sampled).
func (sm *SampledMetrics) ConsumeFailure(transport, queue, reason string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ConsumeFailure(transport, queue, reason)
	}
}

// ConsumeDuration records the duration of a consume operation (sampled).
func (sm *SampledMetrics) ConsumeDuration(transport, queue string, duration time.Duration) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ConsumeDuration(transport, queue, duration)
	}
}

// ConnectionsActive records the number of active connections (sampled).
func (sm *SampledMetrics) ConnectionsActive(transport string, count int) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ConnectionsActive(transport, count)
	}
}

// ChannelsActive records the number of active channels (sampled).
func (sm *SampledMetrics) ChannelsActive(transport string, count int) {
	if sm.sampler.ShouldSample() {
		sm.metrics.ChannelsActive(transport, count)
	}
}

// MessagesInFlight records the number of in-flight messages (sampled).
func (sm *SampledMetrics) MessagesInFlight(transport string, count int) {
	if sm.sampler.ShouldSample() {
		sm.metrics.MessagesInFlight(transport, count)
	}
}

// BackpressureTotal increments the backpressure counter (sampled).
func (sm *SampledMetrics) BackpressureTotal(transport, mode string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.BackpressureTotal(transport, mode)
	}
}

// RetryTotal increments the retry counter (sampled).
func (sm *SampledMetrics) RetryTotal(transport, stage string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.RetryTotal(transport, stage)
	}
}

// DroppedTotal increments the dropped messages counter (sampled).
func (sm *SampledMetrics) DroppedTotal(transport, reason string) {
	if sm.sampler.ShouldSample() {
		sm.metrics.DroppedTotal(transport, reason)
	}
}

// BatchedMetrics provides batched metrics collection.
type BatchedMetrics struct {
	metrics Metrics
	config  BatchingConfig
	batch   []metricOperation
	mu      sync.Mutex
	ticker  *time.Ticker
	done    chan struct{}
}

// metricOperation represents a single metric operation.
type metricOperation struct {
	opType    string
	params    map[string]interface{}
	timestamp time.Time
}

// NewBatchedMetrics creates a new batched metrics implementation.
func NewBatchedMetrics(metrics Metrics, config BatchingConfig) *BatchedMetrics {
	bm := &BatchedMetrics{
		metrics: metrics,
		config:  config,
		batch:   make([]metricOperation, 0, config.BatchSize),
		done:    make(chan struct{}),
	}

	if config.Enabled {
		bm.ticker = time.NewTicker(config.FlushInterval)
		go bm.flushWorker()
	}

	return bm
}

// flushWorker periodically flushes the metric batch.
func (bm *BatchedMetrics) flushWorker() {
	for {
		select {
		case <-bm.ticker.C:
			bm.flush()
		case <-bm.done:
			bm.ticker.Stop()
			bm.flush() // Final flush
			return
		}
	}
}

// flush processes the current batch of metrics.
func (bm *BatchedMetrics) flush() {
	bm.mu.Lock()
	if len(bm.batch) == 0 {
		bm.mu.Unlock()
		return
	}

	batch := bm.batch
	bm.batch = make([]metricOperation, 0, bm.config.BatchSize)
	bm.mu.Unlock()

	// Process batch
	for _, op := range batch {
		bm.processOperation(op)
	}
}

// processOperation processes a single metric operation.
func (bm *BatchedMetrics) processOperation(op metricOperation) {
	switch op.opType {
	case "publish_total":
		transport := op.params["transport"].(string)
		exchange := op.params["exchange"].(string)
		bm.metrics.PublishTotal(transport, exchange)
	case "publish_success":
		transport := op.params["transport"].(string)
		exchange := op.params["exchange"].(string)
		bm.metrics.PublishSuccess(transport, exchange)
	case "publish_failure":
		transport := op.params["transport"].(string)
		exchange := op.params["exchange"].(string)
		reason := op.params["reason"].(string)
		bm.metrics.PublishFailure(transport, exchange, reason)
	case "publish_duration":
		transport := op.params["transport"].(string)
		exchange := op.params["exchange"].(string)
		duration := op.params["duration"].(time.Duration)
		bm.metrics.PublishDuration(transport, exchange, duration)
		// Add other operation types as needed
	}
}

// addToBatch adds a metric operation to the batch.
func (bm *BatchedMetrics) addToBatch(opType string, params map[string]interface{}) {
	if !bm.config.Enabled {
		// Process immediately if batching is disabled
		bm.processOperation(metricOperation{
			opType:    opType,
			params:    params,
			timestamp: time.Now(),
		})
		return
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	op := metricOperation{
		opType:    opType,
		params:    params,
		timestamp: time.Now(),
	}

	bm.batch = append(bm.batch, op)

	// Flush if batch is full
	if len(bm.batch) >= bm.config.BatchSize {
		batch := bm.batch
		bm.batch = make([]metricOperation, 0, bm.config.BatchSize)
		bm.mu.Unlock()

		// Process batch
		for _, op := range batch {
			bm.processOperation(op)
		}
		bm.mu.Lock()
	}
}

// PublishTotal adds publish total to batch.
func (bm *BatchedMetrics) PublishTotal(transport, exchange string) {
	bm.addToBatch("publish_total", map[string]interface{}{
		"transport": transport,
		"exchange":  exchange,
	})
}

// PublishSuccess adds publish success to batch.
func (bm *BatchedMetrics) PublishSuccess(transport, exchange string) {
	bm.addToBatch("publish_success", map[string]interface{}{
		"transport": transport,
		"exchange":  exchange,
	})
}

// PublishFailure adds publish failure to batch.
func (bm *BatchedMetrics) PublishFailure(transport, exchange, reason string) {
	bm.addToBatch("publish_failure", map[string]interface{}{
		"transport": transport,
		"exchange":  exchange,
		"reason":    reason,
	})
}

// PublishDuration adds publish duration to batch.
func (bm *BatchedMetrics) PublishDuration(transport, exchange string, duration time.Duration) {
	bm.addToBatch("publish_duration", map[string]interface{}{
		"transport": transport,
		"exchange":  exchange,
		"duration":  duration,
	})
}

// ConsumeTotal adds consume total to batch.
func (bm *BatchedMetrics) ConsumeTotal(transport, queue string) {
	bm.addToBatch("consume_total", map[string]interface{}{
		"transport": transport,
		"queue":     queue,
	})
}

// ConsumeSuccess adds consume success to batch.
func (bm *BatchedMetrics) ConsumeSuccess(transport, queue string) {
	bm.addToBatch("consume_success", map[string]interface{}{
		"transport": transport,
		"queue":     queue,
	})
}

// ConsumeFailure adds consume failure to batch.
func (bm *BatchedMetrics) ConsumeFailure(transport, queue, reason string) {
	bm.addToBatch("consume_failure", map[string]interface{}{
		"transport": transport,
		"queue":     queue,
		"reason":    reason,
	})
}

// ConsumeDuration adds consume duration to batch.
func (bm *BatchedMetrics) ConsumeDuration(transport, queue string, duration time.Duration) {
	bm.addToBatch("consume_duration", map[string]interface{}{
		"transport": transport,
		"queue":     queue,
		"duration":  duration,
	})
}

// ConnectionsActive adds connections active to batch.
func (bm *BatchedMetrics) ConnectionsActive(transport string, count int) {
	bm.addToBatch("connections_active", map[string]interface{}{
		"transport": transport,
		"count":     count,
	})
}

// ChannelsActive adds channels active to batch.
func (bm *BatchedMetrics) ChannelsActive(transport string, count int) {
	bm.addToBatch("channels_active", map[string]interface{}{
		"transport": transport,
		"count":     count,
	})
}

// MessagesInFlight adds messages in flight to batch.
func (bm *BatchedMetrics) MessagesInFlight(transport string, count int) {
	bm.addToBatch("messages_in_flight", map[string]interface{}{
		"transport": transport,
		"count":     count,
	})
}

// BackpressureTotal adds backpressure total to batch.
func (bm *BatchedMetrics) BackpressureTotal(transport, mode string) {
	bm.addToBatch("backpressure_total", map[string]interface{}{
		"transport": transport,
		"mode":      mode,
	})
}

// RetryTotal adds retry total to batch.
func (bm *BatchedMetrics) RetryTotal(transport, stage string) {
	bm.addToBatch("retry_total", map[string]interface{}{
		"transport": transport,
		"stage":     stage,
	})
}

// DroppedTotal adds dropped total to batch.
func (bm *BatchedMetrics) DroppedTotal(transport, reason string) {
	bm.addToBatch("dropped_total", map[string]interface{}{
		"transport": transport,
		"reason":    reason,
	})
}

// Close closes the batched metrics and performs final flush.
func (bm *BatchedMetrics) Close() {
	if bm.config.Enabled {
		close(bm.done)
	}
}

// TelemetryMiddleware provides middleware for automatic telemetry collection.
type TelemetryMiddleware struct {
	provider *AdvancedTelemetryProvider
	metrics  Metrics
	tracer   Tracer
	logger   *logx.Logger
}

// NewTelemetryMiddleware creates a new telemetry middleware.
func NewTelemetryMiddleware(provider *AdvancedTelemetryProvider) *TelemetryMiddleware {
	return &TelemetryMiddleware{
		provider: provider,
		metrics:  provider.metrics,
		tracer:   provider.tracer,
		logger:   provider.logger,
	}
}

// PublisherTelemetryMiddleware returns middleware for publisher telemetry.
func (tm *TelemetryMiddleware) PublisherTelemetryMiddleware() func(context.Context, string, Message) (context.Context, Message) {
	return func(ctx context.Context, topic string, msg Message) (context.Context, Message) {
		start := time.Now()

		// Create span for publish operation
		ctx, span := tm.tracer.StartSpan(ctx, "publish_message",
			WithAttributes(map[string]interface{}{
				"topic":        topic,
				"message_id":   msg.ID,
				"content_type": msg.ContentType,
				"size":         msg.Size(),
			}),
		)
		defer span.End()

		// Record metrics
		tm.metrics.PublishTotal("unknown", topic)

		// Add telemetry context to message
		if msg.Headers == nil {
			msg.Headers = make(map[string]string)
		}
		msg.Headers["X-Telemetry-Start"] = start.Format(time.RFC3339Nano)

		return ctx, msg
	}
}

// ConsumerTelemetryMiddleware returns middleware for consumer telemetry.
func (tm *TelemetryMiddleware) ConsumerTelemetryMiddleware() func(context.Context, Delivery) context.Context {
	return func(ctx context.Context, delivery Delivery) context.Context {
		start := time.Now()

		// Create span for consume operation
		ctx, span := tm.tracer.StartSpan(ctx, "consume_message",
			WithAttributes(map[string]interface{}{
				"queue":        delivery.Queue,
				"exchange":     delivery.Exchange,
				"routing_key":  delivery.RoutingKey,
				"message_id":   delivery.ID,
				"redelivered":  delivery.Redelivered,
				"delivery_tag": delivery.DeliveryTag,
			}),
		)
		defer span.End()

		// Record metrics
		tm.metrics.ConsumeTotal("unknown", delivery.Queue)

		// Add telemetry context
		ctx = context.WithValue(ctx, telemetryStartKey{}, start)
		ctx = context.WithValue(ctx, telemetryDeliveryKey{}, delivery)

		return ctx
	}
}

// HandlerTelemetryMiddleware returns middleware for handler telemetry.
func (tm *TelemetryMiddleware) HandlerTelemetryMiddleware() func(context.Context, Delivery) (context.Context, Delivery) {
	return func(ctx context.Context, delivery Delivery) (context.Context, Delivery) {
		start := time.Now()

		// Create span for handler operation
		ctx, span := tm.tracer.StartSpan(ctx, "handle_message",
			WithAttributes(map[string]interface{}{
				"queue":        delivery.Queue,
				"message_id":   delivery.ID,
				"content_type": delivery.ContentType,
				"size":         delivery.Size(),
			}),
		)
		defer span.End()

		// Add telemetry context
		ctx = context.WithValue(ctx, telemetryHandlerStartKey{}, start)

		return ctx, delivery
	}
}
