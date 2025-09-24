// Package messaging provides RabbitMQ consumer implementation
package messaging

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creasty/defaults"
	"github.com/seasbee/go-logx"
	"github.com/wagslane/go-rabbitmq"
)

// updateConsumeTime atomically updates consume time statistics
func (s *ConsumerStats) updateConsumeTime() {
	now := time.Now().UnixNano()
	atomic.StoreInt64(&s.LastConsumeTime, now)

	// Set first consume time if not set
	atomic.CompareAndSwapInt64(&s.FirstConsumeTime, 0, now)
}

// UpdateError atomically updates error statistics
func (s *ConsumerStats) UpdateError(err error) {
	atomic.AddInt64(&s.ConsumeErrors, 1)
	atomic.StoreInt64(&s.LastErrorTime, time.Now().UnixNano())
	s.mu.Lock()
	s.LastError = err
	s.mu.Unlock()
}

// updateProcessTime updates message processing time statistics.
// This method atomically updates the total processing time and calculates
// the average processing time per message.
//
// Parameters:
//   - processTime: Duration spent processing a single message
//
// Thread Safety: This method is thread-safe and uses atomic operations.
func (s *ConsumerStats) updateProcessTime(processTime time.Duration) {
	atomic.AddInt64(&s.TotalConsumeTime, int64(processTime))

	// Calculate average processing time
	totalTime := atomic.LoadInt64(&s.TotalConsumeTime)
	totalMessages := atomic.LoadInt64(&s.MessagesConsumed)
	if totalMessages > 0 {
		atomic.StoreInt64(&s.AvgProcessTime, totalTime/totalMessages)
	}
}

// updateThroughput calculates and updates throughput metrics.
// This method calculates the current messages per second and updates
// the peak throughput if the current rate exceeds the previous peak.
//
// Thread Safety: This method is thread-safe and uses atomic operations.
func (s *ConsumerStats) updateThroughput() {
	now := time.Now().UnixNano()
	firstTime := atomic.LoadInt64(&s.FirstConsumeTime)
	totalMessages := atomic.LoadInt64(&s.MessagesConsumed)

	if firstTime > 0 && totalMessages > 0 {
		elapsed := now - firstTime
		if elapsed > 0 {
			// Calculate messages per second (multiply by 1e9 for nanoseconds to seconds)
			mps := (totalMessages * 1e9) / elapsed
			atomic.StoreInt64(&s.MessagesPerSecond, mps)

			// Update peak throughput
			peak := atomic.LoadInt64(&s.PeakThroughput)
			if mps > peak {
				atomic.StoreInt64(&s.PeakThroughput, mps)
			}
		}
	}
}

// GetLastError safely retrieves the last error
func (s *ConsumerStats) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastError
}

// createConsumerOptions creates RabbitMQ consumer options from ConsumeOptions.
// This method converts the internal ConsumeOptions to the format expected
// by the go-rabbitmq library.
//
// Parameters:
//   - options: Consumer options containing AutoAck, PrefetchCount, etc.
//
// Returns:
//   - []func(*rabbitmq.ConsumerOptions): Slice of option functions for RabbitMQ consumer
//
// Thread Safety: This method is thread-safe and has no side effects.
func (c *Consumer) createConsumerOptions(options *ConsumeOptions) []func(*rabbitmq.ConsumerOptions) {
	consumerOptions := []func(*rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsConsumerAutoAck(options.AutoAck),
		rabbitmq.WithConsumerOptionsQOSPrefetch(options.PrefetchCount),
	}

	if options.Exclusive {
		consumerOptions = append(consumerOptions, rabbitmq.WithConsumerOptionsConsumerExclusive)
	}

	if options.NoWait {
		consumerOptions = append(consumerOptions, rabbitmq.WithConsumerOptionsConsumerNoWait)
	}

	if options.ConsumerTag != "" {
		consumerOptions = append(consumerOptions, rabbitmq.WithConsumerOptionsConsumerName(options.ConsumerTag))
	}

	return consumerOptions
}

// validateConsumerConfig validates the consumer configuration
func (c *Consumer) validateConsumerConfig(config *ConsumerConfig) error {
	if config == nil {
		return NewConsumeError("consumer config is nil", "", "", nil)
	}

	// Validate MaxConsumers
	if config.MaxConsumers <= 0 {
		return NewConsumeError("MaxConsumers must be greater than 0", "", "", nil)
	}
	if config.MaxConsumers > 1000 {
		return NewConsumeError("MaxConsumers cannot exceed 1000", "", "", nil)
	}

	// Validate PrefetchCount
	if config.PrefetchCount < 0 {
		return NewConsumeError("PrefetchCount cannot be negative", "", "", nil)
	}
	if config.PrefetchCount > 10000 {
		return NewConsumeError("PrefetchCount cannot exceed 10000", "", "", nil)
	}

	// Validate PrefetchSize
	if config.PrefetchSize < 0 {
		return NewConsumeError("PrefetchSize cannot be negative", "", "", nil)
	}

	// Validate ConsumerTag length
	if len(config.ConsumerTag) > 255 {
		return NewConsumeError("ConsumerTag cannot exceed 255 characters", "", "", nil)
	}

	// Validate ExchangeType
	validExchangeTypes := map[string]bool{
		"direct":  true,
		"fanout":  true,
		"topic":   true,
		"headers": true,
	}
	if config.ExchangeType != "" && !validExchangeTypes[config.ExchangeType] {
		return NewConsumeError("invalid ExchangeType: "+config.ExchangeType, "", "", nil)
	}

	return nil
}

// Consumer represents a RabbitMQ message consumer
type Consumer struct {
	conn        *rabbitmq.Conn
	consumer    *rabbitmq.Consumer
	config      *ConsumerConfig
	logger      *logx.Logger
	mu          sync.RWMutex
	closed      bool
	stats       *ConsumerStats
	consumers   map[string]*rabbitmq.Consumer
	consumersMu sync.RWMutex
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// ConsumerStats contains comprehensive consumer statistics
type ConsumerStats struct {
	// Message processing statistics
	MessagesConsumed int64
	MessagesAcked    int64
	MessagesNacked   int64
	MessagesRejected int64
	ConsumeErrors    int64

	// Timing statistics (Unix timestamps for atomic access)
	LastConsumeTime  int64
	LastErrorTime    int64
	FirstConsumeTime int64 // Time when first message was consumed
	TotalConsumeTime int64 // Total time spent processing messages (nanoseconds)

	// Error tracking
	LastError error // Protected by mutex

	// Performance metrics
	ActiveConsumers int64
	MaxConsumers    int64 // Maximum consumers reached
	AvgProcessTime  int64 // Average message processing time (nanoseconds)

	// Throughput metrics
	MessagesPerSecond int64 // Current messages per second
	PeakThroughput    int64 // Peak messages per second

	// Resource usage
	MemoryUsage    int64 // Estimated memory usage in bytes
	GoroutineCount int64 // Number of active goroutines

	mu sync.RWMutex // Protects LastError and other non-atomic fields
}

// ConsumeOptions contains options for consuming messages
type ConsumeOptions struct {
	Queue         string
	Exchange      string
	RoutingKey    string
	ExchangeType  string
	AutoAck       bool
	Exclusive     bool
	NoLocal       bool
	NoWait        bool
	PrefetchCount int
	PrefetchSize  int
	ConsumerTag   string
}

// QueueOptions contains options for declaring a queue
type QueueOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       map[string]interface{}
}

// ExchangeOptions contains options for declaring an exchange
type ExchangeOptions struct {
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       map[string]interface{}
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(conn *rabbitmq.Conn, config *ConsumerConfig, logger *logx.Logger) (*Consumer, error) {
	return NewConsumerWithLogger(conn, config, logger)
}

// NewConsumerWithLogger creates a new RabbitMQ consumer with a custom logger configuration
func NewConsumerWithLogger(conn *rabbitmq.Conn, config *ConsumerConfig, logger *logx.Logger) (*Consumer, error) {
	if conn == nil {
		return nil, NewConnectionError("connection is nil", "", 0, nil)
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is mandatory and cannot be nil")
	}

	if config == nil {
		config = &ConsumerConfig{}
		// Apply defaults to ensure all fields are properly initialized
		if err := defaults.Set(config); err != nil {
			// Log error but continue with manual defaults
			logger.Warn("failed to apply defaults to consumer config", logx.ErrorField(err))
		}
	}

	// Create a temporary consumer instance to validate config
	tempConsumer := &Consumer{}
	if err := tempConsumer.validateConsumerConfig(config); err != nil {
		logger.Error("consumer config validation failed", logx.ErrorField(err))
		return nil, err
	}

	logger.Debug("creating RabbitMQ consumer")
	consumer, err := rabbitmq.NewConsumer(
		conn,
		"",
		rabbitmq.WithConsumerOptionsLogging,
	)
	if err != nil {
		logger.Error("failed to create consumer", logx.ErrorField(err))
		return nil, NewConsumeError("failed to create consumer", "", "", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger.Info("consumer created successfully",
		logx.Bool("auto_ack", config.AutoAck),
		logx.Bool("exclusive", config.Exclusive),
		logx.Int("prefetch_count", config.PrefetchCount),
	)

	return &Consumer{
		conn:      conn,
		consumer:  consumer,
		config:    config,
		logger:    logger,
		stats:     &ConsumerStats{},
		consumers: make(map[string]*rabbitmq.Consumer),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// Consume starts consuming messages from a queue
func (c *Consumer) Consume(ctx context.Context, queue string, handler MessageHandler) error {
	if c == nil {
		return fmt.Errorf("consumer is nil")
	}

	c.logger.Info("starting message consumption",
		logx.String("queue", queue),
	)

	if c.config == nil {
		c.logger.Warn("consumer config is nil, using default options",
			logx.String("queue", queue),
		)
		// Use default options if config is nil
		options := &ConsumeOptions{
			Queue:         queue,
			AutoAck:       false,
			Exclusive:     false,
			NoLocal:       false,
			NoWait:        false,
			PrefetchCount: 10,
			PrefetchSize:  0,
			ConsumerTag:   "",
		}
		return c.ConsumeWithOptions(ctx, queue, handler, options)
	}

	options := &ConsumeOptions{
		Queue:         queue,
		AutoAck:       c.config.AutoAck,
		Exclusive:     c.config.Exclusive,
		NoLocal:       c.config.NoLocal,
		NoWait:        c.config.NoWait,
		PrefetchCount: c.config.PrefetchCount,
		PrefetchSize:  c.config.PrefetchSize,
		ConsumerTag:   c.config.ConsumerTag,
	}

	return c.ConsumeWithOptions(ctx, queue, handler, options)
}

// ConsumeWithOptions starts consuming messages with custom options.
// This method performs the following operations atomically:
// 1. Validates input parameters (handler, options)
// 2. Checks if consumer is closed
// 3. Validates consumer limits and prevents duplicates
// 4. Creates and configures RabbitMQ consumer
// 5. Starts message processing goroutines
// 6. Handles context cancellation
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - queue: Queue name to consume from
//   - handler: Message handler function
//   - options: Consumer options (AutoAck, PrefetchCount, etc.)
//
// Returns:
//   - error: Returns error if validation fails, consumer limit exceeded, or consumer creation fails
//
// Thread Safety: This method is thread-safe and can be called concurrently.
// However, only one consumer per queue is allowed.
func (c *Consumer) ConsumeWithOptions(ctx context.Context, queue string, handler MessageHandler, options *ConsumeOptions) error {
	if c == nil {
		return fmt.Errorf("consumer is nil")
	}
	if handler == nil {
		return NewConsumeError("handler is nil", queue, "", nil)
	}

	if options == nil {
		return NewConsumeError("options is nil", queue, "", nil)
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return NewConsumeError("consumer is closed", queue, "", nil)
	}
	c.mu.RUnlock()

	// Check consumer limit and reserve slot atomically
	c.consumersMu.Lock()
	if c.config != nil && len(c.consumers) >= c.config.MaxConsumers {
		c.consumersMu.Unlock()
		return NewConsumeError("maximum consumers reached", queue, "", nil)
	}
	// Check if queue already exists
	if _, exists := c.consumers[queue]; exists {
		c.consumersMu.Unlock()
		return NewConsumeError("consumer already exists for queue", queue, "", nil)
	}
	// Reserve the slot by adding a placeholder
	c.consumers[queue] = nil
	c.consumersMu.Unlock()

	// Create consumer options
	consumerOptions := c.createConsumerOptions(options)

	// Create the consumer
	consumer, err := rabbitmq.NewConsumer(
		c.conn,
		queue,
		consumerOptions...,
	)
	if err != nil {
		// Remove the placeholder on error
		c.consumersMu.Lock()
		delete(c.consumers, queue)
		c.consumersMu.Unlock()
		c.stats.UpdateError(err)
		return NewConsumeError("failed to create consumer", queue, "", err)
	}

	// Store the actual consumer
	c.consumersMu.Lock()
	c.consumers[queue] = consumer
	c.consumersMu.Unlock()

	// Update active consumers and max consumers atomically
	currentActive := atomic.AddInt64(&c.stats.ActiveConsumers, 1)

	// Update max consumers metric using atomic compare-and-swap
	for {
		maxConsumers := atomic.LoadInt64(&c.stats.MaxConsumers)
		if currentActive <= maxConsumers {
			break
		}
		if atomic.CompareAndSwapInt64(&c.stats.MaxConsumers, maxConsumers, currentActive) {
			break
		}
	}

	// Start consuming with the handler in a goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		err := consumer.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
			return c.handleDelivery(d, handler)
		})
		if err != nil {
			c.stats.UpdateError(err)
		}
	}()

	// Start context cancellation handler in a goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		select {
		case <-ctx.Done():
			c.stopConsumer(queue)
		case <-c.ctx.Done():
			c.stopConsumer(queue)
		}
	}()

	return nil
}

// handleDelivery handles a message delivery from RabbitMQ.
// This method is called for each message received and performs:
// 1. Updates consumption statistics atomically
// 2. Converts RabbitMQ delivery to internal Message format
// 3. Calls the user-provided handler function
// 4. Handles acknowledgment based on AutoAck configuration
// 5. Updates error statistics on handler failure
// 6. Tracks processing time and throughput metrics
//
// Parameters:
//   - d: RabbitMQ delivery containing message data
//   - handler: User-provided message handler function
//
// Returns:
//   - rabbitmq.Action: Ack, Nack, or NackRequeue based on handler result and AutoAck setting
//
// Thread Safety: This method is called by RabbitMQ consumer goroutines and is thread-safe.
// All statistics updates use atomic operations.
func (c *Consumer) handleDelivery(d rabbitmq.Delivery, handler MessageHandler) rabbitmq.Action {
	startTime := time.Now()

	atomic.AddInt64(&c.stats.MessagesConsumed, 1)
	c.stats.updateConsumeTime()

	// Convert rabbitmq.Delivery to our Delivery type
	delivery := &Delivery{
		Message:      c.convertDelivery(d),
		Acknowledger: &RabbitMQAcknowledger{delivery: d},
		Tag:          fmt.Sprintf("%d", d.DeliveryTag),
		Redelivered:  d.Redelivered,
		Exchange:     d.Exchange,
		RoutingKey:   d.RoutingKey,
	}

	// Call the handler
	if err := handler(delivery); err != nil {
		c.stats.UpdateError(err)
		return rabbitmq.NackRequeue
	}

	// Update processing time and throughput metrics
	processTime := time.Since(startTime)
	c.stats.updateProcessTime(processTime)
	c.stats.updateThroughput()

	// Handle acknowledgment based on auto-ack setting
	if c.config != nil && c.config.AutoAck {
		// Auto-ack is enabled, message is already acked by RabbitMQ
		// We don't need to return Ack since it's already handled
		atomic.AddInt64(&c.stats.MessagesAcked, 1)
		return rabbitmq.Ack // This is correct - we still need to return Ack for the handler
	}

	// Auto-ack is disabled, we need to ack manually
	atomic.AddInt64(&c.stats.MessagesAcked, 1)
	return rabbitmq.Ack
}

// convertDelivery converts rabbitmq.Delivery to our Message type
func (c *Consumer) convertDelivery(d rabbitmq.Delivery) *Message {
	msg := &Message{
		ID:          d.MessageId,
		Body:        d.Body,
		Headers:     d.Headers,
		Properties:  MessageProperties{},
		Timestamp:   time.Now(),
		ContentType: d.ContentType,
		Encoding:    d.ContentEncoding,
		Metadata:    make(map[string]interface{}),
	}

	// Set properties from delivery
	msg.Properties.Exchange = d.Exchange
	msg.Properties.RoutingKey = d.RoutingKey
	msg.Properties.CorrelationID = d.CorrelationId
	msg.Properties.ReplyTo = d.ReplyTo
	msg.Properties.MessageID = d.MessageId
	msg.Properties.UserID = d.UserId
	msg.Properties.AppID = d.AppId

	// Set priority
	msg.Priority = MessagePriority(d.Priority)

	// Set expiration
	if d.Expiration != "" {
		if duration, err := time.ParseDuration(d.Expiration); err == nil {
			msg.Properties.Expiration = duration
		}
	}

	return msg
}

// stopConsumer stops a consumer for a specific queue
func (c *Consumer) stopConsumer(queue string) {
	c.consumersMu.Lock()
	defer c.consumersMu.Unlock()

	if consumer, exists := c.consumers[queue]; exists {
		consumer.Close()
		delete(c.consumers, queue)
		atomic.AddInt64(&c.stats.ActiveConsumers, -1)
	}
}

// GetStats returns comprehensive consumer statistics.
// This method provides real-time metrics for monitoring consumer performance:
// - Message counts (consumed, acked, nacked, rejected)
// - Error statistics and last error information
// - Timing information (last consume time, last error time)
// - Active consumer count and consumer state
//
// Returns:
//   - map[string]interface{}: Statistics map with the following keys:
//   - "messages_consumed": Total messages consumed (int64)
//   - "messages_acked": Total messages acknowledged (int64)
//   - "messages_nacked": Total messages negatively acknowledged (int64)
//   - "messages_rejected": Total messages rejected (int64)
//   - "consume_errors": Total consumption errors (int64)
//   - "last_consume_time": Time of last message consumption (time.Time)
//   - "last_error_time": Time of last error (time.Time)
//   - "last_error": Last error that occurred (error)
//   - "active_consumers": Number of active consumers (int64)
//   - "closed": Whether consumer is closed (bool)
//
// Thread Safety: This method is thread-safe and can be called concurrently.
// All statistics are read atomically to ensure consistency.
func (c *Consumer) GetStats() map[string]interface{} {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	lastConsumeTime := atomic.LoadInt64(&c.stats.LastConsumeTime)
	lastErrorTime := atomic.LoadInt64(&c.stats.LastErrorTime)

	var lastConsumeTimeFormatted time.Time
	var lastErrorTimeFormatted time.Time

	if lastConsumeTime > 0 {
		lastConsumeTimeFormatted = time.Unix(0, lastConsumeTime)
	}
	if lastErrorTime > 0 {
		lastErrorTimeFormatted = time.Unix(0, lastErrorTime)
	}

	// Get additional timing information
	firstConsumeTime := atomic.LoadInt64(&c.stats.FirstConsumeTime)
	var firstConsumeTimeFormatted time.Time
	if firstConsumeTime > 0 {
		firstConsumeTimeFormatted = time.Unix(0, firstConsumeTime)
	}

	// Calculate uptime
	var uptime time.Duration
	if firstConsumeTime > 0 {
		uptime = time.Since(time.Unix(0, firstConsumeTime))
	}

	return map[string]interface{}{
		// Message processing statistics
		"messages_consumed": atomic.LoadInt64(&c.stats.MessagesConsumed),
		"messages_acked":    atomic.LoadInt64(&c.stats.MessagesAcked),
		"messages_nacked":   atomic.LoadInt64(&c.stats.MessagesNacked),
		"messages_rejected": atomic.LoadInt64(&c.stats.MessagesRejected),
		"consume_errors":    atomic.LoadInt64(&c.stats.ConsumeErrors),

		// Timing information
		"last_consume_time":  lastConsumeTimeFormatted,
		"last_error_time":    lastErrorTimeFormatted,
		"first_consume_time": firstConsumeTimeFormatted,
		"uptime":             uptime,
		"avg_process_time":   time.Duration(atomic.LoadInt64(&c.stats.AvgProcessTime)),
		"total_process_time": time.Duration(atomic.LoadInt64(&c.stats.TotalConsumeTime)),

		// Performance metrics
		"active_consumers":    atomic.LoadInt64(&c.stats.ActiveConsumers),
		"max_consumers":       atomic.LoadInt64(&c.stats.MaxConsumers),
		"messages_per_second": atomic.LoadInt64(&c.stats.MessagesPerSecond),
		"peak_throughput":     atomic.LoadInt64(&c.stats.PeakThroughput),

		// Resource usage
		"memory_usage":    atomic.LoadInt64(&c.stats.MemoryUsage),
		"goroutine_count": atomic.LoadInt64(&c.stats.GoroutineCount),

		// Error information
		"last_error": c.stats.GetLastError(),

		// Consumer state
		"closed": c.closed,
	}
}

// IsHealthy returns true if the consumer is healthy
func (c *Consumer) IsHealthy() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	return !c.closed && c.consumer != nil
}

// Close gracefully shuts down the consumer and all its resources.
// This method performs the following operations in order:
// 1. Marks the consumer as closed to prevent new operations
// 2. Cancels the internal context to signal all goroutines to stop
// 3. Closes all active consumers for individual queues
// 4. Closes the main consumer connection
// 5. Waits for all goroutines to finish using WaitGroup
//
// Returns:
//   - error: Always returns nil (for compatibility with io.Closer interface)
//
// Thread Safety: This method is thread-safe and can be called multiple times safely.
// Subsequent calls will return immediately without error.
//
// Note: This method blocks until all goroutines have finished, ensuring clean shutdown.
func (c *Consumer) Close() error {
	if c == nil {
		return fmt.Errorf("consumer is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Cancel context to signal all goroutines to stop
	c.cancel()

	// Close all consumers
	c.consumersMu.Lock()
	for queue, consumer := range c.consumers {
		consumer.Close()
		delete(c.consumers, queue)
	}
	c.consumersMu.Unlock()

	// Close main consumer
	if c.consumer != nil {
		c.consumer.Close()
	}

	// Wait for all goroutines to finish
	c.wg.Wait()

	return nil
}

// String returns a string representation of the consumer
func (c *Consumer) String() string {
	return fmt.Sprintf("Consumer{Messages: %d, Errors: %d, Active: %d, Closed: %v}",
		atomic.LoadInt64(&c.stats.MessagesConsumed),
		atomic.LoadInt64(&c.stats.ConsumeErrors),
		atomic.LoadInt64(&c.stats.ActiveConsumers),
		c.closed)
}

// RabbitMQAcknowledger implements the Acknowledger interface for RabbitMQ
type RabbitMQAcknowledger struct {
	delivery rabbitmq.Delivery
}

// Ack acknowledges the message
func (a *RabbitMQAcknowledger) Ack() error {
	return a.delivery.Ack(false)
}

// Nack negatively acknowledges the message
func (a *RabbitMQAcknowledger) Nack(requeue bool) error {
	return a.delivery.Nack(false, requeue)
}

// Reject rejects the message
func (a *RabbitMQAcknowledger) Reject(requeue bool) error {
	return a.delivery.Nack(false, requeue)
}
