// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/pkg/messaging"
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

	// Task distribution
	taskQueue     chan *HandlerTask
	taskQueueSize int

	// Message delivery
	deliveries <-chan amqp091.Delivery
	channel    *amqp091.Channel

	// Lifecycle
	mu      sync.RWMutex
	closed  bool
	started bool

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

	if !cc.transport.IsConnected() {
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

	// Create and start workers
	if err := cc.startWorkers(); err != nil {
		return messaging.WrapError(messaging.ErrorCodeConsume, "start", "failed to start workers", err)
	}

	// Start message delivery goroutine
	go cc.deliverMessages(ctx, handler)

	cc.started = true

	if cc.observability != nil && cc.observability.Logger() != nil {
		cc.observability.Logger().Info("concurrent consumer started",
			logx.String("queue", cc.config.Queue),
			logx.Int("max_concurrent", cc.config.MaxConcurrentHandlers),
			logx.Int("prefetch", cc.config.Prefetch),
			logx.Int("workers", len(cc.workers)))
	}

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

	if cc.observability != nil && cc.observability.Logger() != nil {
		cc.observability.Logger().Debug("QoS configured",
			logx.Int("prefetch", prefetch))
	}

	return nil
}

// startWorkers creates and starts the worker pool
func (cc *ConcurrentConsumer) startWorkers() error {
	// Determine worker count based on concurrent handlers and CPU count
	workerCount := cc.config.MaxConcurrentHandlers / 10 // 10 handlers per worker
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > runtime.NumCPU()*4 {
		workerCount = runtime.NumCPU() * 4
	}
	if workerCount > 100 {
		workerCount = 100
	}

	cc.workers = make([]*ConsumerWorker, workerCount)

	for i := 0; i < workerCount; i++ {
		worker := &ConsumerWorker{
			id:       i,
			consumer: cc,
			taskChan: make(chan *HandlerTask, 10), // Small buffer per worker
			done:     make(chan struct{}),
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

// deliverMessages receives messages and distributes them to workers
func (cc *ConcurrentConsumer) deliverMessages(ctx context.Context, handler messaging.Handler) {
	defer func() {
		if r := recover(); r != nil {
			if cc.observability != nil && cc.observability.Logger() != nil {
				cc.observability.Logger().Error("panic in message delivery",
					logx.Any("panic", r),
					logx.String("stack", string(debug.Stack())))
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if cc.observability != nil && cc.observability.Logger() != nil {
				cc.observability.Logger().Debug("message delivery stopped due to context cancellation")
			}
			return
		case delivery, ok := <-cc.deliveries:
			if !ok {
				if cc.observability != nil && cc.observability.Logger() != nil {
					cc.observability.Logger().Debug("message delivery stopped due to channel closure")
				}
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

			// Try to queue the task
			select {
			case cc.taskQueue <- task:
				cc.incrementStats("queued_tasks")
			default:
				// Task queue is full, drop message or handle overflow
				cc.handleTaskQueueOverflow(task)
			}
		}
	}
}

// convertDelivery converts AMQP delivery to messaging delivery
func (cc *ConcurrentConsumer) convertDelivery(amqpDelivery amqp091.Delivery) messaging.Delivery {
	headers := make(map[string]string)
	for k, v := range amqpDelivery.Headers {
		if str, ok := v.(string); ok {
			headers[k] = str
		} else {
			headers[k] = fmt.Sprintf("%v", v)
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

// handleTaskQueueOverflow handles task queue overflow
func (cc *ConcurrentConsumer) handleTaskQueueOverflow(task *HandlerTask) {
	// For now, we'll nack the message to requeue it
	// In production, you might want different strategies
	task.delivery.Nack(false, true) // Requeue
	cc.incrementStats("messages_requeued")

	if cc.observability != nil && cc.observability.Logger() != nil {
		cc.observability.Logger().Warn("task queue overflow, message requeued",
			logx.String("message_id", task.msgDelivery.Message.ID))
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

	// Close task queue to stop accepting new tasks
	close(cc.taskQueue)

	// Stop all workers
	for _, worker := range cc.workers {
		close(worker.done)
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
	case <-time.After(30 * time.Second):
		// Timeout waiting for workers
		if cc.observability != nil && cc.observability.Logger() != nil {
			cc.observability.Logger().Warn("timeout waiting for workers to finish")
		}
	}

	cc.started = false

	if cc.observability != nil && cc.observability.Logger() != nil {
		cc.observability.Logger().Info("concurrent consumer stopped")
	}

	return nil
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
		// This is handled dynamically in GetStats
	}
}

// start starts the consumer worker
func (cw *ConsumerWorker) start() {
	defer cw.consumer.workerWg.Done()

	if cw.consumer.observability != nil && cw.consumer.observability.Logger() != nil {
		cw.consumer.observability.Logger().Debug("consumer worker started",
			logx.Int("worker_id", cw.id))
	}

	for {
		select {
		case task := <-cw.consumer.taskQueue:
			if task != nil {
				cw.processTask(task)
			}
		case <-cw.done:
			if cw.consumer.observability != nil && cw.consumer.observability.Logger() != nil {
				cw.consumer.observability.Logger().Debug("consumer worker stopped",
					logx.Int("worker_id", cw.id))
			}
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

	// Log panic
	if cw.consumer.observability != nil && cw.consumer.observability.Logger() != nil {
		cw.consumer.observability.Logger().Error("panic recovered in message handler",
			logx.Int("worker_id", cw.id),
			logx.String("message_id", task.msgDelivery.Message.ID),
			logx.Any("panic", r),
			logx.String("stack", string(debug.Stack())))
	}

	// Handle panic according to configuration
	if cw.consumer.config.PanicRecovery {
		// Send to DLQ if configured
		if cw.consumer.dlq != nil {
			panicMsg := fmt.Sprintf("panic in handler: %v", r)
			err := cw.consumer.dlq.SendToDLQ(task.ctx, &task.msgDelivery.Message, panicMsg)
			if err != nil {
				if cw.consumer.observability != nil && cw.consumer.observability.Logger() != nil {
					cw.consumer.observability.Logger().Error("failed to send panic message to DLQ",
						logx.ErrorField(err))
				}
				// If DLQ fails, nack and requeue
				task.delivery.Nack(false, true)
				cw.consumer.incrementStats("messages_requeued")
			} else {
				// Successfully sent to DLQ, ack the message
				task.delivery.Ack(false)
				cw.consumer.incrementStats("messages_sent_to_dlq")
			}
		} else {
			// No DLQ configured, nack and requeue
			task.delivery.Nack(false, true)
			cw.consumer.incrementStats("messages_requeued")
		}
	} else {
		// Panic recovery disabled, nack without requeue
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
	case messaging.NackRequeue:
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
	case messaging.NackDLQ:
		cw.handleDLQRouting(task, "explicit DLQ routing")
	default:
		// Default to ack if decision is unclear
		task.delivery.Ack(false)
		cw.consumer.incrementStats("messages_processed")
	}
}

// handleProcessingError handles errors that occur during message processing
func (cw *ConsumerWorker) handleProcessingError(task *HandlerTask, err error) {
	cw.stats.mu.Lock()
	cw.stats.TasksFailed++
	cw.stats.mu.Unlock()

	// Check if we should retry or send to DLQ
	retryCount := cw.getRetryCount(task)
	maxRetries := cw.consumer.config.MaxRetries

	if maxRetries > 0 && retryCount >= maxRetries {
		// Max retries exceeded, send to DLQ
		cw.handleDLQRouting(task, fmt.Sprintf("max retries exceeded (%d): %v", maxRetries, err))
	} else if retryCount < maxRetries && maxRetries > 0 {
		// Increment retry count and requeue
		cw.incrementRetryCount(task)
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
	} else if cw.consumer.config.RequeueOnError {
		// Requeue on error (no retry limit or retry disabled)
		task.delivery.Nack(false, true) // Requeue
		cw.consumer.incrementStats("messages_requeued")
	} else {
		// Don't requeue, just nack
		task.delivery.Nack(false, false)
		cw.consumer.incrementStats("messages_failed")
	}
}

// handleDLQRouting handles routing messages to the dead letter queue
func (cw *ConsumerWorker) handleDLQRouting(task *HandlerTask, reason string) {
	if cw.consumer.dlq != nil {
		err := cw.consumer.dlq.SendToDLQ(task.ctx, &task.msgDelivery.Message, reason)
		if err != nil {
			if cw.consumer.observability != nil && cw.consumer.observability.Logger() != nil {
				cw.consumer.observability.Logger().Error("failed to send message to DLQ",
					logx.ErrorField(err),
					logx.String("reason", reason))
			}
			// If DLQ fails, nack without requeue
			task.delivery.Nack(false, false)
			cw.consumer.incrementStats("messages_failed")
		} else {
			// Successfully sent to DLQ, ack the message
			task.delivery.Ack(false)
			cw.consumer.incrementStats("messages_sent_to_dlq")
		}
	} else {
		// No DLQ configured, nack without requeue
		task.delivery.Nack(false, false)
		cw.consumer.incrementStats("messages_failed")
	}
}

// getRetryCount gets the retry count from message headers
func (cw *ConsumerWorker) getRetryCount(task *HandlerTask) int {
	if retryCountStr, exists := task.msgDelivery.Message.Headers["x-retry-count"]; exists {
		var retryCount int
		if n, err := fmt.Sscanf(retryCountStr, "%d", &retryCount); err == nil && n > 0 {
			return retryCount
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

// String returns a string representation of the concurrent consumer
func (cc *ConcurrentConsumer) String() string {
	stats := cc.GetStats()
	return fmt.Sprintf("ConcurrentConsumer{queue:%s, workers:%d, received:%d, processed:%d, failed:%d}",
		cc.config.Queue, stats.ActiveWorkers, stats.MessagesReceived, stats.MessagesProcessed, stats.MessagesFailed)
}
