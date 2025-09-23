// Package messaging provides RabbitMQ client implementation using github.com/wagslane/go-rabbitmq
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wagslane/go-rabbitmq"
)

// Client represents a RabbitMQ messaging client
type Client struct {
	config   *Config
	conn     *rabbitmq.Conn
	producer *Producer
	consumer *Consumer
	health   *HealthChecker
	mu       sync.RWMutex
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewClient creates a new RabbitMQ client
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.ValidateAndSetDefaults(); err != nil {
		return nil, NewValidationError("invalid configuration", "config", config, "validation", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize connection
	if err := client.connect(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Initialize producer
	producer, err := NewProducer(client.conn, &config.ProducerConfig)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	client.producer = producer

	// Initialize consumer
	consumer, err := NewConsumer(client.conn, &config.ConsumerConfig)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	client.consumer = consumer

	// Initialize health checker
	client.health = NewHealthChecker(client.conn, config.HealthCheckInterval)

	return client, nil
}

// connect establishes connection to RabbitMQ
func (c *Client) connect() error {
	conn, err := rabbitmq.NewConn(
		c.config.URL,
		rabbitmq.WithConnectionOptionsLogging,
	)
	if err != nil {
		return NewConnectionError("failed to connect to RabbitMQ", c.config.URL, 1, err)
	}

	c.conn = conn
	return nil
}

// GetProducer returns the producer instance
func (c *Client) GetProducer() *Producer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.producer
}

// GetConsumer returns the consumer instance
func (c *Client) GetConsumer() *Consumer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consumer
}

// GetHealthChecker returns the health checker instance
func (c *Client) GetHealthChecker() *HealthChecker {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.health
}

// checkClosedAndGetComponent checks if client is closed and returns the specified component
func (c *Client) checkClosedAndGetComponent(componentName string) (interface{}, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, NewConnectionError("client is closed", c.config.URL, 0, nil)
	}

	var component interface{}
	switch componentName {
	case "producer":
		component = c.producer
	case "consumer":
		component = c.consumer
	default:
		c.mu.RUnlock()
		return nil, NewConnectionError("unknown component", c.config.URL, 0, nil)
	}
	c.mu.RUnlock()

	if component == nil {
		return nil, NewConnectionError(componentName+" is not available", c.config.URL, 0, nil)
	}

	return component, nil
}

// Publish publishes a message to RabbitMQ
func (c *Client) Publish(ctx context.Context, msg *Message) error {
	component, err := c.checkClosedAndGetComponent("producer")
	if err != nil {
		return err
	}

	producer := component.(*Producer)
	return producer.Publish(ctx, msg)
}

// PublishBatch publishes a batch of messages to RabbitMQ
func (c *Client) PublishBatch(ctx context.Context, batch *BatchMessage) error {
	component, err := c.checkClosedAndGetComponent("producer")
	if err != nil {
		return err
	}

	producer := component.(*Producer)
	return producer.PublishBatch(ctx, batch)
}

// Consume starts consuming messages from a queue
func (c *Client) Consume(ctx context.Context, queue string, handler MessageHandler) error {
	component, err := c.checkClosedAndGetComponent("consumer")
	if err != nil {
		return err
	}

	consumer := component.(*Consumer)
	return consumer.Consume(ctx, queue, handler)
}

// ConsumeWithOptions starts consuming messages with custom options
func (c *Client) ConsumeWithOptions(ctx context.Context, queue string, handler MessageHandler, options *ConsumeOptions) error {
	component, err := c.checkClosedAndGetComponent("consumer")
	if err != nil {
		return err
	}

	consumer := component.(*Consumer)
	return consumer.ConsumeWithOptions(ctx, queue, handler, options)
}

// IsHealthy returns true if the client is healthy
func (c *Client) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.health == nil {
		return false
	}

	return c.health.IsHealthy()
}

// GetStats returns client statistics
func (c *Client) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := map[string]interface{}{
		"closed": c.closed,
		"config": map[string]interface{}{
			"url":             c.config.URL,
			"max_connections": c.config.MaxConnections,
			"max_channels":    c.config.MaxChannels,
			"metrics_enabled": c.config.MetricsEnabled,
		},
	}

	if c.producer != nil {
		stats["producer"] = c.producer.GetStats()
	}

	if c.consumer != nil {
		stats["consumer"] = c.consumer.GetStats()
	}

	if c.health != nil {
		stats["health"] = c.health.GetStatsMap()
	}

	return stats
}

// Close closes the client and all its resources
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.cancel()

	var errs []error

	// Close producer
	if c.producer != nil {
		if err := c.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
		}
	}

	// Close consumer
	if c.consumer != nil {
		if err := c.consumer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close consumer: %w", err))
		}
	}

	// Close health checker
	if c.health != nil {
		if err := c.health.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close health checker: %w", err))
		}
	}

	// Close connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// WaitForConnection waits for the connection to be established
func (c *Client) WaitForConnection(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewTimeoutError("connection timeout", timeout, "wait_for_connection", ctx.Err())
		case <-c.ctx.Done():
			return NewConnectionError("client context cancelled", c.config.URL, 0, c.ctx.Err())
		case <-ticker.C:
			if c.IsHealthy() {
				return nil
			}
		}
	}
}

// Reconnect attempts to reconnect to RabbitMQ
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		return NewConnectionError("client is not closed, cannot reconnect", c.config.URL, 0, nil)
	}

	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
	}

	// Reconnect
	if err := c.connect(); err != nil {
		return NewConnectionError("failed to reconnect", c.config.URL, 1, err)
	}

	c.closed = false

	// Recreate producer
	producer, err := NewProducer(c.conn, &c.config.ProducerConfig)
	if err != nil {
		return NewConnectionError("failed to recreate producer", c.config.URL, 1, err)
	}
	c.producer = producer

	// Recreate consumer
	consumer, err := NewConsumer(c.conn, &c.config.ConsumerConfig)
	if err != nil {
		return NewConnectionError("failed to recreate consumer", c.config.URL, 1, err)
	}
	c.consumer = consumer

	// Recreate health checker
	c.health = NewHealthChecker(c.conn, c.config.HealthCheckInterval)

	return nil
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *Config {
	return c.config
}

// String returns a string representation of the client
func (c *Client) String() string {
	c.mu.RLock()
	closed := c.closed
	config := c.config
	c.mu.RUnlock()

	return fmt.Sprintf("RabbitMQClient{URL: %s, Closed: %v, Healthy: %v}",
		config.URL, closed, c.IsHealthy())
}
