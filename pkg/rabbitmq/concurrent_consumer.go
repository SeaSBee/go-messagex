// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/rabbitmq/amqp091-go"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// HandlerTask represents a message handling task
type HandlerTask struct {
	ctx         context.Context
	delivery    amqp091.Delivery
	msgDelivery messaging.Delivery
	handler     messaging.Handler
	consumer    *ConcurrentConsumer
	startTime   time.Time
	workerID    int
}

// ConsumerWorker represents a worker in the consumer pool
type ConsumerWorker struct {
	id       int
	consumer *ConcurrentConsumer
	taskChan chan *HandlerTask
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *ConsumerWorkerStats
}

// ConsumerWorkerStats tracks worker statistics
type ConsumerWorkerStats struct {
	TasksProcessed  uint64
	TasksFailed     uint64
	PanicsRecovered uint64
	LastActivity    time.Time
	mu              sync.RWMutex
}

// ConcurrentConsumer provides concurrent message consumption with worker pools
type ConcurrentConsumer struct {
	transport     *Transport
	config        *messaging.ConsumerConfig
	observability *messaging.ObservabilityContext
	dlq           *messaging.DeadLetterQueue

	// Worker pool
	workers  []*ConsumerWorker
	workerWg sync.WaitGroup

	// Semaphore for controlling concurrency
	semaphore chan struct{}

	// Task distribution - using round-robin approach
	taskQueue     chan *HandlerTask
	taskQueueSize int
	currentWorker int32 // Atomic counter for round-robin distribution

	// Message delivery
	deliveries <-chan amqp091.Delivery
	channel    *amqp091.Channel

	// Lifecycle
	mu      sync.RWMutex
	closed  bool
	started bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Statistics
	stats *ConsumerStats
}

// ConsumerStats tracks consumer statistics
type ConsumerStats struct {
	MessagesReceived  uint64
	MessagesProcessed uint64
	MessagesFailed    uint64
	MessagesRequeued  uint64
	MessagesSentToDLQ uint64
	PanicsRecovered   uint64
	ActiveWorkers     int
	QueuedTasks       int
	mu                sync.RWMutex
}

// NewConcurrentConsumer creates a new concurrent consumer with worker pools
func NewConcurrentConsumer(transport *Transport, config *messaging.ConsumerConfig, observability *messaging.ObservabilityContext) *ConcurrentConsumer {
	// Validate input parameters
	if transport == nil {
		panic("transport cannot be nil")
	}
	if config == nil {
		panic("config cannot be nil")
	}

	// Validate and set defaults
	maxConcurrent := config.MaxConcurrentHandlers
	if maxConcurrent <= 0 {
		maxConcurrent = 512
	}
	if maxConcurrent > 10000 {
		maxConcurrent = 10000
	}

	// Task queue size should be larger than concurrent handlers
	taskQueueSize := maxConcurrent * 2
	if taskQueueSize > 20000 {
		taskQueueSize = 20000
	}

	return &ConcurrentConsumer{
		transport:     transport,
		config:        config,
		observability: observability,
		semaphore:     make(chan struct{}, maxConcurrent),
		taskQueue:     make(chan *HandlerTask, taskQueueSize),
		taskQueueSize: taskQueueSize,
		stats:         &ConsumerStats{},
	}
}

// SetDLQ sets the dead letter queue for the consumer
func (cc *ConcurrentConsumer) SetDLQ(dlq *messaging.DeadLetterQueue) {
	cc.dlq = dlq
}

// Start starts the concurrent consumer and worker pool
func (cc *ConcurrentConsumer) Start(ctx context.Context, handler messaging.Handler) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closed {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer is closed")
	}

	if cc.started {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "consumer already started")
	}

	// Validate context and handler
	if ctx == nil {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "context cannot be nil")
	}
	if handler == nil {
		return messaging.NewError(messaging.ErrorCodeConsume, "start", "handler cannot be nil")
	}

	if cc.transport == nil || !cc.transport.IsConnected() {
		return messaging.NewError(messaging.ErrorCodeConnection, "start", "transport is not connected")
	}

	// Get channel from transport
	channel := cc.transport.GetChannel()
	if channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "start", "failed to get channel")
	}
	cc.channel = channel

	// Set up QoS (prefetch)
	if err := cc.setupQoS(); err != nil {
		return messaging.WrapError(messaging.ErrorCodeChannel, "start", "failed to setup QoS", err)
	}

	// Start consuming
	deliveries, err := channel.Consume(
		cc.config.Queue,     // queue
		"",                  // consumer
		cc.config.AutoAck,   // auto-ack
		cc.config.Exclusive, // exclusive
		cc.config.NoLocal,   // no-local
		cc.config.NoWait,    // no-wait
		cc.config.Arguments, // args
	)
	if err != nil {
		return messaging.WrapError(messaging.ErrorCodeConsume, "start", "failed to start consuming", err)
	}

	cc.deliveries = deliveries

	// Create context for the consumer
	cc.ctx, cc.cancel = context.WithCancel(ctx)

	// Create and start workers
	if err := cc.startWorkers(); err != nil {
		cc.cancel()
		return messaging.WrapError(messaging.ErrorCodeConsume, "start", "failed to start workers", err)
	}

	// Start message delivery goroutine
	go cc.deliverMessages(cc.ctx, handler)

	cc.started = true

	cc.logInfo("concurrent consumer started",
		logx.String("queue", cc.config.Queue),
		logx.Int("max_concurrent", cc.config.MaxConcurrentHandlers),
		logx.Int("prefetch", cc.config.Prefetch),
		logx.Int("workers", len(cc.workers)))

	return nil
}

// setupQoS sets up Quality of Service (prefetch) settings
func (cc *ConcurrentConsumer) setupQoS() error {
	prefetch := cc.config.Prefetch
	if prefetch <= 0 {
		prefetch = 256
	}
	if prefetch > 65535 {
		prefetch = 65535
	}

	// Set QoS with prefetch count
	if err := cc.channel.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS prefetch=%d: %w", prefetch, err)
	}

	cc.logDebug("QoS configured", logx.Int("prefetch", prefetch))
	return nil
}

// startWorkers creates and starts the worker pool
func (cc *ConcurrentConsumer) startWorkers() error {
	// Determine worker count based on concurrent handlers and CPU count
	// Use a more dynamic approach based on workload characteristics
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}

	// Cap workers based on max concurrent handlers
	maxWorkers := cc.config.MaxConcurrentHandlers / 5 // 5 handlers per worker
	if maxWorkers < workerCount {
		workerCount = maxWorkers
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 100 {
		workerCount = 100
	}

	cc.workers = make([]*ConsumerWorker, workerCount)

	for i := 0; i < workerCount; i++ {
		workerCtx, workerCancel := context.WithCancel(cc.ctx)
		worker := &ConsumerWorker{
			id:       i,
			consumer: cc,
			taskChan: make(chan *HandlerTask, 5), // Small buffer per worker
			done:     make(chan struct{}),
			ctx:      workerCtx,
			cancel:   workerCancel,
			stats:    &ConsumerWorkerStats{},
		}
		cc.workers[i] = worker

		// Start worker
		cc.workerWg.Add(1)
		go worker.start()
	}

	cc.stats.mu.Lock()
	cc.stats.ActiveWorkers = len(cc.workers)
	cc.stats.mu.Unlock()

	return nil
}

// deliverMessages receives messages and distributes them to workers using round-robin
func (cc *ConcurrentConsumer) deliverMessages(ctx context.Context, handler messaging.Handler) {
	defer func() {
		if r := recover(); r != nil {
			cc.logError("panic in message delivery",
				logx.Any("panic", r),
				logx.String("stack", string(debug.Stack())))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			cc.logDebug("message delivery stopped due to context cancellation")
			return
		case delivery, ok := <-cc.deliveries:
			if !ok {
				cc.logDebug("message delivery stopped due to channel closure")
				return
			}

			cc.incrementStats("messages_received")

			// Create message delivery
			msgDelivery := cc.convertDelivery(delivery)

			// Create handler task
			task := &HandlerTask{
				ctx:         ctx,
				delivery:    delivery,
				msgDelivery: msgDelivery,
				handler:     handler,
				consumer:    cc,
				startTime:   time.Now(),
			}

			// Distribute task to worker using round-robin
			cc.distributeTask(task)
		}
	}
}

// distributeTask distributes a task to a worker using round-robin approach
func (cc *ConcurrentConsumer) distributeTask(task *HandlerTask) {
	// Use round-robin distribution to avoid race conditions
	cc.mu.RLock()
	workerCount := len(cc.workers)
	if workerCount == 0 {
		cc.mu.RUnlock()
		cc.logError("no workers available for task distribution")
		cc.handleTaskQueueOverflow(task)
		return
	}
	cc.mu.RUnlock()

	workerIndex := atomic.AddInt32(&cc.currentWorker, 1) % int32(workerCount)

	cc.mu.RLock()
	worker := cc.workers[workerIndex]
	cc.mu.RUnlock()

	// Try to send to worker's task channel
	select {
	case worker.taskChan <- task:
		cc.incrementStats("queued_tasks")
	default:
		// Worker's channel is full, try next worker
		cc.handleTaskDistributionOverflow(task)
	}
}

// handleTaskDistributionOverflow handles overflow when worker channels are full
func (cc *ConcurrentConsumer) handleTaskDistributionOverflow(task *HandlerTask) {
	// Try to find an available worker
	cc.mu.RLock()
	workerCount := len(cc.workers)
	cc.mu.RUnlock()

	for i := 0; i < workerCount; i++ {
		workerIndex := (atomic.AddInt32(&cc.currentWorker, 1) % int32(workerCount))

		cc.mu.RLock()
		worker := cc.workers[workerIndex]
		cc.mu.RUnlock()

		select {
		case worker.taskChan <- task:
			cc.incrementStats("queued_tasks")
			return
		default:
			continue
		}
	}

	// All workers are busy, implement backoff strategy
	cc.handleTaskQueueOverflow(task)
}

// convertDelivery converts AMQP delivery to messaging delivery
func (cc *ConcurrentConsumer) convertDelivery(amqpDelivery amqp091.Delivery) messaging.Delivery {
	headers := make(map[string]string)
	for k, v := range amqpDelivery.Headers {
		// Validate header key is not empty
		if k != "" {
			if str, ok := v.(string); ok {
				headers[k] = str
			} else {
				headers[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return messaging.Delivery{
		Message: messaging.Message{
			ID:             amqpDelivery.MessageId,
			Key:            amqpDelivery.RoutingKey,
			Body:           amqpDelivery.Body,
			Headers:        headers,
			ContentType:    amqpDelivery.ContentType,
			Timestamp:      amqpDelivery.Timestamp,
			Priority:       uint8(amqpDelivery.Priority),
			CorrelationID:  amqpDelivery.CorrelationId,
			ReplyTo:        amqpDelivery.ReplyTo,
			IdempotencyKey: "", // Can be set from headers if needed
		},
		DeliveryTag: amqpDelivery.DeliveryTag,
		Exchange:    amqpDelivery.Exchange,
		RoutingKey:  amqpDelivery.RoutingKey,
		Queue:       cc.config.Queue,
		Redelivered: amqpDelivery.Redelivered,
		ConsumerTag: amqpDelivery.ConsumerTag,
	}
}

// handleTaskQueueOverflow handles task queue overflow with backoff strategy
func (cc *ConcurrentConsumer) handleTaskQueueOverflow(task *HandlerTask) {
	// Implement exponential backoff for requeuing
	retryCount := cc.getRetryCount(task)
	maxBackoffRetries := 3

	if retryCount < maxBackoffRetries {
		// Increment retry count and requeue with backoff
		cc.incrementRetryCount(task)
		task.delivery.Nack(false, true) // Requeue
		cc.incrementStats("messages_requeued")

		cc.logWarn("task queue overflow, message requeued with backoff",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.Int("retry_count", retryCount),
			logx.String("queue", task.msgDelivery.Queue))
	} else {
		// Max backoff retries exceeded, send to DLQ or drop
		if cc.dlq != nil {
			cc.handleDLQRouting(task, "task queue overflow after backoff retries")
		} else {
			// No DLQ, nack without requeue
			task.delivery.Nack(false, false)
			cc.incrementStats("messages_failed")
		}

		cc.logError("task queue overflow, message dropped after backoff retries",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue),
			logx.Int("max_backoff_retries", maxBackoffRetries))
	}
}

// getRetryCount gets the retry count from message headers
func (cc *ConcurrentConsumer) getRetryCount(task *HandlerTask) int {
	if task.msgDelivery.Message.Headers != nil {
		if retryCountStr, exists := task.msgDelivery.Message.Headers["x-retry-count"]; exists {
			var retryCount int
			if n, err := fmt.Sscanf(retryCountStr, "%d", &retryCount); err == nil && n > 0 {
				return retryCount
			}
		}
	}
	return 0
}

// incrementRetryCount increments the retry count in message headers
func (cc *ConcurrentConsumer) incrementRetryCount(task *HandlerTask) {
	retryCount := cc.getRetryCount(task) + 1
	if task.msgDelivery.Message.Headers == nil {
		task.msgDelivery.Message.Headers = make(map[string]string)
	}
	task.msgDelivery.Message.Headers["x-retry-count"] = fmt.Sprintf("%d", retryCount)
}

// handleDLQRouting handles routing messages to the dead letter queue
func (cc *ConcurrentConsumer) handleDLQRouting(task *HandlerTask, reason string) {
	if task == nil {
		cc.logError("cannot route nil task to DLQ")
		return
	}

	if cc.dlq != nil {
		err := cc.dlq.SendToDLQ(task.ctx, &task.msgDelivery.Message, reason)
		if err != nil {
			cc.logError("failed to send message to DLQ",
				logx.ErrorField(err),
				logx.String("reason", reason),
				logx.String("message_id", task.msgDelivery.Message.ID),
				logx.String("queue", task.msgDelivery.Queue))
			// If DLQ fails, nack without requeue
			task.delivery.Nack(false, false)
			cc.incrementStats("messages_failed")
		} else {
			// Successfully sent to DLQ, ack the message
			task.delivery.Ack(false)
			cc.incrementStats("messages_sent_to_dlq")
			cc.logDebug("message successfully sent to DLQ",
				logx.String("message_id", task.msgDelivery.Message.ID),
				logx.String("reason", reason),
				logx.String("queue", task.msgDelivery.Queue))
		}
	} else {
		// No DLQ configured, nack without requeue
		cc.logWarn("no DLQ configured, dropping message",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("reason", reason),
			logx.String("queue", task.msgDelivery.Queue))
		task.delivery.Nack(false, false)
		cc.incrementStats("messages_failed")
	}
}

// Stop gracefully stops the concurrent consumer
func (cc *ConcurrentConsumer) Stop(ctx context.Context) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if !cc.started || cc.closed {
		return nil
	}

	cc.closed = true

	// Cancel context to stop all goroutines
	if cc.cancel != nil {
		cc.cancel()
	}

	// Use provided context for timeout if available, otherwise use default
	timeout := 30 * time.Second
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
			if timeout <= 0 {
				timeout = 5 * time.Second // Minimum timeout
			}
		}
	}

	// Stop all workers with timeout before closing task queue
	cc.stopWorkersWithTimeout(timeout)

	// Close task queue after workers are stopped
	select {
	case <-cc.taskQueue:
		// Channel is already closed
	default:
		close(cc.taskQueue)
	}

	cc.started = false

	cc.logInfo("concurrent consumer stopped", logx.String("timeout", timeout.String()))
	return nil
}

// stopWorkersWithTimeout stops workers with a timeout
func (cc *ConcurrentConsumer) stopWorkersWithTimeout(timeout time.Duration) {
	// Cancel all worker contexts
	for _, worker := range cc.workers {
		if worker != nil && worker.cancel != nil {
			worker.cancel()
		}
		if worker != nil {
			select {
			case <-worker.done:
				// Channel is already closed
			default:
				close(worker.done)
			}
		}
	}

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		cc.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished gracefully
		cc.logDebug("all workers stopped gracefully")
	case <-time.After(timeout):
		// Timeout waiting for workers
		cc.logWarn("timeout waiting for workers to finish", logx.String("timeout", timeout.String()))
	}
}

// GetStats returns consumer statistics
func (cc *ConcurrentConsumer) GetStats() *ConsumerStats {
	cc.stats.mu.RLock()
	defer cc.stats.mu.RUnlock()

	// Create new stats without copying the mutex
	stats := &ConsumerStats{
		MessagesReceived:  cc.stats.MessagesReceived,
		MessagesProcessed: cc.stats.MessagesProcessed,
		MessagesFailed:    cc.stats.MessagesFailed,
		MessagesRequeued:  cc.stats.MessagesRequeued,
		MessagesSentToDLQ: cc.stats.MessagesSentToDLQ,
		PanicsRecovered:   cc.stats.PanicsRecovered,
		ActiveWorkers:     cc.stats.ActiveWorkers,
		QueuedTasks:       len(cc.taskQueue),
	}

	return stats
}

// GetWorkerStats returns statistics for all workers
func (cc *ConcurrentConsumer) GetWorkerStats() []*ConsumerWorkerStats {
	stats := make([]*ConsumerWorkerStats, len(cc.workers))
	for i, worker := range cc.workers {
		if worker != nil && worker.stats != nil {
			worker.stats.mu.RLock()
			// Create new stats without copying the mutex
			workerStats := &ConsumerWorkerStats{
				TasksProcessed:  worker.stats.TasksProcessed,
				TasksFailed:     worker.stats.TasksFailed,
				PanicsRecovered: worker.stats.PanicsRecovered,
				LastActivity:    worker.stats.LastActivity,
			}
			worker.stats.mu.RUnlock()
			stats[i] = workerStats
		}
	}
	return stats
}

// incrementStats increments a statistics counter
func (cc *ConcurrentConsumer) incrementStats(stat string) {
	cc.stats.mu.Lock()
	defer cc.stats.mu.Unlock()

	switch stat {
	case "messages_received":
		cc.stats.MessagesReceived++
	case "messages_processed":
		cc.stats.MessagesProcessed++
	case "messages_failed":
		cc.stats.MessagesFailed++
	case "messages_requeued":
		cc.stats.MessagesRequeued++
	case "messages_sent_to_dlq":
		cc.stats.MessagesSentToDLQ++
	case "panics_recovered":
		cc.stats.PanicsRecovered++
	case "queued_tasks":
		// This is handled dynamically in GetStats, no action needed
	default:
		// Unknown stat, log for debugging
		cc.logDebug("unknown stat increment requested", logx.String("stat", stat))
	}
}

// start starts the consumer worker
func (cw *ConsumerWorker) start() {
	defer cw.consumer.workerWg.Done()

	cw.consumer.logDebug("consumer worker started", logx.Int("worker_id", cw.id))

	for {
		select {
		case task := <-cw.taskChan:
			if task != nil {
				cw.processTask(task)
			}
		case <-cw.ctx.Done():
			cw.consumer.logDebug("consumer worker stopped due to context cancellation", logx.Int("worker_id", cw.id))
			return
		case <-cw.done:
			cw.consumer.logDebug("consumer worker stopped", logx.Int("worker_id", cw.id))
			return
		}
	}
}

// processTask processes a single message handling task with semaphore control
func (cw *ConsumerWorker) processTask(task *HandlerTask) {
	// Acquire semaphore to limit concurrent handlers
	cw.consumer.semaphore <- struct{}{}
	defer func() { <-cw.consumer.semaphore }()

	// Update worker stats
	cw.stats.mu.Lock()
	cw.stats.TasksProcessed++
	cw.stats.LastActivity = time.Now()
	cw.stats.mu.Unlock()

	// Set worker ID for tracking
	task.workerID = cw.id

	// Process with panic recovery
	cw.processWithPanicRecovery(task)
}

// processWithPanicRecovery processes a task with panic recovery
func (cw *ConsumerWorker) processWithPanicRecovery(task *HandlerTask) {
	var decision messaging.AckDecision
	var err error

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			cw.handlePanic(task, r)
			return
		}

		// Handle the result
		cw.handleTaskResult(task, decision, err)
	}()

	// Validate handler
	if task.handler == nil {
		err = fmt.Errorf("handler is nil")
		return
	}

	// Set up timeout for handler
	handlerCtx := task.ctx
	if cw.consumer.config.HandlerTimeout > 0 {
		var cancel context.CancelFunc
		handlerCtx, cancel = context.WithTimeout(task.ctx, cw.consumer.config.HandlerTimeout)
		defer cancel()
	}

	// Process the message
	decision, err = task.handler.Process(handlerCtx, task.msgDelivery)
}

// handlePanic handles panics that occur during message processing
func (cw *ConsumerWorker) handlePanic(task *HandlerTask, r interface{}) {
	// Update stats
	cw.stats.mu.Lock()
	cw.stats.PanicsRecovered++
	cw.stats.mu.Unlock()

	cw.consumer.incrementStats("panics_recovered")

	// Log panic with more context
	cw.consumer.logError("panic recovered in message handler",
		logx.Int("worker_id", cw.id),
		logx.String("message_id", task.msgDelivery.Message.ID),
		logx.String("queue", task.msgDelivery.Queue),
		logx.String("routing_key", task.msgDelivery.RoutingKey),
		logx.Any("panic", r),
		logx.String("stack", string(debug.Stack())))

	// Handle panic according to configuration
	if cw.consumer.config.PanicRecovery {
		// Send to DLQ if configured
		if cw.consumer.dlq != nil {
			panicMsg := fmt.Sprintf("panic in handler: %v", r)
			err := cw.consumer.dlq.SendToDLQ(task.ctx, &task.msgDelivery.Message, panicMsg)
			if err != nil {
				cw.consumer.logError("failed to send panic message to DLQ",
					logx.ErrorField(err),
					logx.String("message_id", task.msgDelivery.Message.ID),
					logx.String("queue", task.msgDelivery.Queue))
				// If DLQ fails, nack and requeue
				task.delivery.Nack(false, true)
				cw.consumer.incrementStats("messages_requeued")
			} else {
				// Successfully sent to DLQ, ack the message
				task.delivery.Ack(false)
				cw.consumer.incrementStats("messages_sent_to_dlq")
				cw.consumer.logDebug("panic message successfully sent to DLQ",
					logx.String("message_id", task.msgDelivery.Message.ID),
					logx.String("queue", task.msgDelivery.Queue))
			}
		} else {
			// No DLQ configured, nack and requeue
			cw.consumer.logWarn("no DLQ configured for panic recovery, requeuing message",
				logx.String("message_id", task.msgDelivery.Message.ID),
				logx.String("queue", task.msgDelivery.Queue))
			task.delivery.Nack(false, true)
			cw.consumer.incrementStats("messages_requeued")
		}
	} else {
		// Panic recovery disabled, nack without requeue
		cw.consumer.logWarn("panic recovery disabled, dropping message",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue))
		task.delivery.Nack(false, false)
		cw.consumer.incrementStats("messages_failed")
	}
}

// handleTaskResult handles the result of message processing
func (cw *ConsumerWorker) handleTaskResult(task *HandlerTask, decision messaging.AckDecision, err error) {
	if err != nil {
		cw.handleProcessingError(task, err)
		return
	}

	// Handle successful processing based on decision
	switch decision {
	case messaging.Ack:
		task.delivery.Ack(false)
		cw.consumer.incrementStats("messages_processed")
		cw.consumer.logDebug("message processed successfully",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.Int("worker_id", cw.id),
			logx.String("queue", task.msgDelivery.Queue))
	case messaging.NackRequeue:
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
		cw.consumer.logDebug("message requeued by handler",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.Int("worker_id", cw.id),
			logx.String("queue", task.msgDelivery.Queue))
	case messaging.NackDLQ:
		cw.handleDLQRouting(task, "explicit DLQ routing")
	default:
		// Default to ack if decision is unclear
		cw.consumer.logWarn("unknown ack decision, defaulting to ack",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue),
			logx.Any("decision", decision))
		task.delivery.Ack(false)
		cw.consumer.incrementStats("messages_processed")
	}
}

// handleProcessingError handles errors that occur during message processing
func (cw *ConsumerWorker) handleProcessingError(task *HandlerTask, err error) {
	cw.stats.mu.Lock()
	cw.stats.TasksFailed++
	cw.stats.mu.Unlock()

	// Log the processing error with context
	cw.consumer.logError("message processing failed",
		logx.String("message_id", task.msgDelivery.Message.ID),
		logx.Int("worker_id", cw.id),
		logx.String("queue", task.msgDelivery.Queue),
		logx.String("routing_key", task.msgDelivery.RoutingKey),
		logx.ErrorField(err))

	// Check if we should retry or send to DLQ
	retryCount := cw.getRetryCount(task)
	maxRetries := cw.consumer.config.MaxRetries

	if maxRetries > 0 && retryCount >= maxRetries {
		// Max retries exceeded, send to DLQ
		cw.consumer.logWarn("max retries exceeded, sending to DLQ",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue),
			logx.Int("retry_count", retryCount),
			logx.Int("max_retries", maxRetries))
		cw.handleDLQRouting(task, fmt.Sprintf("max retries exceeded (%d): %v", maxRetries, err))
	} else if retryCount < maxRetries && maxRetries > 0 {
		// Increment retry count and requeue
		cw.incrementRetryCount(task)
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
		cw.consumer.logDebug("message requeued for retry",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue),
			logx.Int("retry_count", retryCount+1),
			logx.Int("max_retries", maxRetries))
	} else if cw.consumer.config.RequeueOnError {
		// Requeue on error (no retry limit or retry disabled)
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
		cw.consumer.logDebug("message requeued due to requeue on error setting",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue))
	} else {
		// Don't requeue, just nack
		task.delivery.Nack(false, false)
		cw.consumer.incrementStats("messages_failed")
		cw.consumer.logDebug("message dropped due to error",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("queue", task.msgDelivery.Queue))
	}
}

// handleDLQRouting handles routing messages to the dead letter queue
func (cw *ConsumerWorker) handleDLQRouting(task *HandlerTask, reason string) {
	if task == nil {
		cw.consumer.logError("cannot route nil task to DLQ")
		return
	}

	if cw.consumer.dlq != nil {
		err := cw.consumer.dlq.SendToDLQ(task.ctx, &task.msgDelivery.Message, reason)
		if err != nil {
			cw.consumer.logError("failed to send message to DLQ",
				logx.ErrorField(err),
				logx.String("reason", reason),
				logx.String("message_id", task.msgDelivery.Message.ID),
				logx.String("queue", task.msgDelivery.Queue),
				logx.Int("worker_id", cw.id))
			// If DLQ fails, nack without requeue
			task.delivery.Nack(false, false)
			cw.consumer.incrementStats("messages_failed")
		} else {
			// Successfully sent to DLQ, ack the message
			task.delivery.Ack(false)
			cw.consumer.incrementStats("messages_sent_to_dlq")
			cw.consumer.logDebug("message successfully sent to DLQ",
				logx.String("message_id", task.msgDelivery.Message.ID),
				logx.String("reason", reason),
				logx.String("queue", task.msgDelivery.Queue),
				logx.Int("worker_id", cw.id))
		}
	} else {
		// No DLQ configured, nack without requeue
		cw.consumer.logWarn("no DLQ configured, dropping message",
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.String("reason", reason),
			logx.String("queue", task.msgDelivery.Queue),
			logx.Int("worker_id", cw.id))
		task.delivery.Nack(false, false)
		cw.consumer.incrementStats("messages_failed")
	}
}

// getRetryCount gets the retry count from message headers
func (cw *ConsumerWorker) getRetryCount(task *HandlerTask) int {
	if task.msgDelivery.Message.Headers != nil {
		if retryCountStr, exists := task.msgDelivery.Message.Headers["x-retry-count"]; exists {
			var retryCount int
			if n, err := fmt.Sscanf(retryCountStr, "%d", &retryCount); err == nil && n > 0 {
				return retryCount
			}
		}
	}
	return 0
}

// incrementRetryCount increments the retry count in message headers
func (cw *ConsumerWorker) incrementRetryCount(task *HandlerTask) {
	retryCount := cw.getRetryCount(task) + 1
	if task.msgDelivery.Message.Headers == nil {
		task.msgDelivery.Message.Headers = make(map[string]string)
	}
	task.msgDelivery.Message.Headers["x-retry-count"] = fmt.Sprintf("%d", retryCount)
}

// Helper methods for safe logging
func (cc *ConcurrentConsumer) logInfo(msg string, fields ...logx.Field) {
	if cc.observability != nil {
		if logger := cc.observability.Logger(); logger != nil {
			logger.Info(msg, fields...)
		}
	}
}

func (cc *ConcurrentConsumer) logDebug(msg string, fields ...logx.Field) {
	if cc.observability != nil {
		if logger := cc.observability.Logger(); logger != nil {
			logger.Debug(msg, fields...)
		}
	}
}

func (cc *ConcurrentConsumer) logWarn(msg string, fields ...logx.Field) {
	if cc.observability != nil {
		if logger := cc.observability.Logger(); logger != nil {
			logger.Warn(msg, fields...)
		}
	}
}

func (cc *ConcurrentConsumer) logError(msg string, fields ...logx.Field) {
	if cc.observability != nil {
		if logger := cc.observability.Logger(); logger != nil {
			logger.Error(msg, fields...)
		}
	}
}

// String returns a string representation of the concurrent consumer
func (cc *ConcurrentConsumer) String() string {
	stats := cc.GetStats()
	return fmt.Sprintf("ConcurrentConsumer{queue:%s, workers:%d, received:%d, processed:%d, failed:%d}",
		cc.config.Queue, stats.ActiveWorkers, stats.MessagesReceived, stats.MessagesProcessed, stats.MessagesFailed)
}
