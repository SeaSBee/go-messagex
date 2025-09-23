// Package messaging provides RabbitMQ producer implementation
package messaging

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wagslane/go-rabbitmq"
)

// Producer represents a RabbitMQ message producer
type Producer struct {
	conn        *rabbitmq.Conn
	publisher   *rabbitmq.Publisher
	config      *ProducerConfig
	mu          sync.RWMutex
	closed      bool
	stats       *ProducerStats
	batchBuffer []*Message
	batchTimer  *time.Timer
	batchMutex  sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// ProducerStats contains producer statistics
type ProducerStats struct {
	MessagesPublished int64
	BatchesPublished  int64
	PublishErrors     int64
	LastPublishTime   int64 // UnixNano for atomic access
	LastErrorTime     int64 // UnixNano for atomic access
	LastError         error
	mu                sync.RWMutex // Protects LastError
}

// UpdatePublishTime atomically updates the last publish time
func (s *ProducerStats) UpdatePublishTime() {
	atomic.StoreInt64(&s.LastPublishTime, time.Now().UnixNano())
}

// UpdateError atomically updates the last error time and error
func (s *ProducerStats) UpdateError(err error) {
	atomic.StoreInt64(&s.LastErrorTime, time.Now().UnixNano())
	s.mu.Lock()
	s.LastError = err
	s.mu.Unlock()
}

// GetLastError safely retrieves the last error
func (s *ProducerStats) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastError
}

// checkClosed returns an error if the producer is closed
func (p *Producer) checkClosed() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return NewPublishError("producer is closed", "", "", "", nil)
	}
	return nil
}

// NewProducer creates a new RabbitMQ producer
func NewProducer(conn *rabbitmq.Conn, config *ProducerConfig) (*Producer, error) {
	if conn == nil {
		return nil, NewConnectionError("connection is nil", "", 0, nil)
	}

	if config == nil {
		config = &ProducerConfig{}
	}

	publisher, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, NewPublishError("failed to create publisher", "", "", "", err)
	}
	if publisher == nil {
		return nil, NewPublishError("publisher is nil after creation", "", "", "", nil)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Ensure batch size is valid
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	producer := &Producer{
		conn:        conn,
		publisher:   publisher,
		config:      config,
		stats:       &ProducerStats{},
		batchBuffer: make([]*Message, 0, batchSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start batch timer if batching is enabled
	if config.BatchSize > 1 {
		producer.batchTimer = time.NewTimer(config.BatchTimeout)
		go producer.batchProcessor()
	}

	return producer, nil
}

// Publish publishes a single message
func (p *Producer) Publish(ctx context.Context, msg *Message) error {
	if msg == nil {
		return NewPublishError("message is nil", "", "", "", nil)
	}

	if err := p.checkClosed(); err != nil {
		return err
	}

	// If batching is enabled and batch size > 1, add to batch
	if p.config.BatchSize > 1 {
		return p.addToBatch(ctx, msg)
	}

	// Publish immediately
	return p.publishMessage(ctx, msg)
}

// PublishBatch publishes a batch of messages
func (p *Producer) PublishBatch(ctx context.Context, batch *BatchMessage) error {
	if batch == nil || batch.IsEmpty() {
		return NewBatchError("batch is nil or empty", 0, 0, 0, nil)
	}

	if err := p.checkClosed(); err != nil {
		return err
	}

	var successCount, failureCount int
	var lastErr error

	for _, msg := range batch.Messages {
		if err := p.publishMessage(ctx, msg); err != nil {
			failureCount++
			lastErr = err
		} else {
			successCount++
		}
	}

	if failureCount > 0 {
		return NewBatchError(
			fmt.Sprintf("batch publish failed: %d/%d messages failed", failureCount, batch.Count()),
			batch.Count(),
			successCount,
			failureCount,
			lastErr,
		)
	}

	// Only increment batch count on successful batch
	atomic.AddInt64(&p.stats.BatchesPublished, 1)
	return nil
}

// publishMessage publishes a single message to RabbitMQ
func (p *Producer) publishMessage(ctx context.Context, msg *Message) error {
	// Set default exchange and routing key if not specified
	exchange := msg.Properties.Exchange
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	routingKey := msg.Properties.RoutingKey
	if routingKey == "" {
		routingKey = p.config.DefaultRoutingKey
	}

	// Create publishing options
	options := []func(*rabbitmq.PublishOptions){
		rabbitmq.WithPublishOptionsExchange(exchange),
	}

	// Set message properties
	if p.config.Mandatory {
		options = append(options, rabbitmq.WithPublishOptionsMandatory)
	}

	if p.config.Immediate {
		options = append(options, rabbitmq.WithPublishOptionsImmediate)
	}

	if msg.Properties.Persistent {
		options = append(options, rabbitmq.WithPublishOptionsPersistentDelivery)
	}

	if msg.Properties.Expiration > 0 {
		options = append(options, rabbitmq.WithPublishOptionsExpiration(msg.Properties.Expiration.String()))
	}

	if msg.Properties.CorrelationID != "" {
		options = append(options, rabbitmq.WithPublishOptionsCorrelationID(msg.Properties.CorrelationID))
	}

	if msg.Properties.ReplyTo != "" {
		options = append(options, rabbitmq.WithPublishOptionsReplyTo(msg.Properties.ReplyTo))
	}

	if msg.Properties.MessageID != "" {
		options = append(options, rabbitmq.WithPublishOptionsMessageID(msg.Properties.MessageID))
	}

	if msg.Properties.UserID != "" {
		options = append(options, rabbitmq.WithPublishOptionsUserID(msg.Properties.UserID))
	}

	if msg.Properties.AppID != "" {
		options = append(options, rabbitmq.WithPublishOptionsAppID(msg.Properties.AppID))
	}

	// Add headers
	if len(msg.Headers) > 0 {
		headers := make(map[string]interface{})
		for k, v := range msg.Headers {
			headers[k] = v
		}
		options = append(options, rabbitmq.WithPublishOptionsHeaders(headers))
	}

	// Set content type
	if msg.ContentType != "" {
		options = append(options, rabbitmq.WithPublishOptionsContentType(msg.ContentType))
	}

	// Set priority
	if msg.Priority > 0 {
		options = append(options, rabbitmq.WithPublishOptionsPriority(uint8(msg.Priority)))
	}

	// Publish the message
	err := p.publisher.Publish(
		msg.Body,
		[]string{routingKey},
		options...,
	)

	if err != nil {
		atomic.AddInt64(&p.stats.PublishErrors, 1)
		p.stats.UpdateError(err)
		return NewPublishError("failed to publish message", msg.Properties.Queue, exchange, routingKey, err)
	}

	atomic.AddInt64(&p.stats.MessagesPublished, 1)
	p.stats.UpdatePublishTime()

	return nil
}

// addToBatch adds a message to the batch buffer
func (p *Producer) addToBatch(ctx context.Context, msg *Message) error {
	p.batchMutex.Lock()
	defer p.batchMutex.Unlock()

	p.batchBuffer = append(p.batchBuffer, msg)

	// If batch is full, publish immediately
	if len(p.batchBuffer) >= p.config.BatchSize {
		return p.flushBatch(ctx)
	}

	return nil
}

// batchProcessor processes batched messages
func (p *Producer) batchProcessor() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.batchTimer.C:
			p.mu.Lock()
			if !p.closed {
				p.batchMutex.Lock()
				if len(p.batchBuffer) > 0 {
					p.flushBatch(p.ctx)
				}
				p.batchMutex.Unlock()

				// Reset timer only if not closed
				p.batchTimer.Reset(p.config.BatchTimeout)
			}
			p.mu.Unlock()
		}
	}
}

// flushBatch publishes all messages in the batch buffer
func (p *Producer) flushBatch(ctx context.Context) error {
	if len(p.batchBuffer) == 0 {
		return nil
	}

	batch := NewBatchMessage(p.batchBuffer)
	err := p.PublishBatch(ctx, batch)

	// Clear the batch buffer
	p.batchBuffer = p.batchBuffer[:0]

	return err
}

// Flush flushes any pending batched messages
func (p *Producer) Flush() error {
	p.batchMutex.Lock()
	defer p.batchMutex.Unlock()

	return p.flushBatch(context.Background())
}

// GetStats returns producer statistics
func (p *Producer) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	lastPublishTime := atomic.LoadInt64(&p.stats.LastPublishTime)
	lastErrorTime := atomic.LoadInt64(&p.stats.LastErrorTime)

	var lastPublishTimeStr, lastErrorTimeStr string
	if lastPublishTime > 0 {
		lastPublishTimeStr = time.Unix(0, lastPublishTime).Format(time.RFC3339)
	}
	if lastErrorTime > 0 {
		lastErrorTimeStr = time.Unix(0, lastErrorTime).Format(time.RFC3339)
	}

	return map[string]interface{}{
		"messages_published": atomic.LoadInt64(&p.stats.MessagesPublished),
		"batches_published":  atomic.LoadInt64(&p.stats.BatchesPublished),
		"publish_errors":     atomic.LoadInt64(&p.stats.PublishErrors),
		"last_publish_time":  lastPublishTimeStr,
		"last_error_time":    lastErrorTimeStr,
		"last_error":         p.stats.GetLastError(),
		"batch_size":         len(p.batchBuffer),
		"closed":             p.closed,
	}
}

// IsHealthy returns true if the producer is healthy
func (p *Producer) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return !p.closed && p.publisher != nil
}

// Close closes the producer
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Cancel context to signal batch processor to stop
	p.cancel()

	// Flush any pending messages
	p.batchMutex.Lock()
	if len(p.batchBuffer) > 0 {
		p.flushBatch(context.Background())
	}
	p.batchMutex.Unlock()

	// Stop batch timer
	if p.batchTimer != nil {
		p.batchTimer.Stop()
	}

	// Close publisher
	if p.publisher != nil {
		p.publisher.Close()
	}

	return nil
}

// String returns a string representation of the producer
func (p *Producer) String() string {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	return fmt.Sprintf("Producer{Messages: %d, Errors: %d, Closed: %v}",
		atomic.LoadInt64(&p.stats.MessagesPublished),
		atomic.LoadInt64(&p.stats.PublishErrors),
		closed)
}
