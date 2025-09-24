// Package messaging provides RabbitMQ client implementation using github.com/wagslane/go-rabbitmq
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/wagslane/go-rabbitmq"
)

// Client represents a RabbitMQ messaging client
type Client struct {
	config   *Config
	conn     *rabbitmq.Conn
	producer *Producer
	consumer *Consumer
	health   *HealthChecker
	logger   *logx.Logger
	mu       sync.RWMutex
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewClient creates a new RabbitMQ client with a custom logger configuration
func NewClient(config *Config, logger *logx.Logger) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.ValidateAndSetDefaults(); err != nil {
		return nil, NewValidationError("invalid configuration", "config", config, "validation", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		cancel()
		return nil, fmt.Errorf("logger is mandatory and cannot be nil")
	}

	client := &Client{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize connection
	client.logger.Info("connecting to RabbitMQ",
		logx.String("url", config.URL),
		logx.Int("max_connections", config.MaxConnections),
		logx.Int("max_channels", config.MaxChannels),
	)

	if err := client.connect(); err != nil {
		client.logger.Error("failed to connect to RabbitMQ",
			logx.String("url", config.URL),
			logx.ErrorField(err),
		)
		cancel()
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	client.logger.Info("successfully connected to RabbitMQ",
		logx.String("url", config.URL),
	)

	// Initialize producer
	client.logger.Info("initializing producer")
	producer, err := NewProducer(client.conn, &config.ProducerConfig, client.logger)
	if err != nil {
		client.logger.Error("failed to create producer", logx.ErrorField(err))
		client.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	client.producer = producer
	client.logger.Info("producer initialized successfully")

	// Initialize consumer
	client.logger.Info("initializing consumer")
	consumer, err := NewConsumer(client.conn, &config.ConsumerConfig, client.logger)
	if err != nil {
		client.logger.Error("failed to create consumer", logx.ErrorField(err))
		client.Close()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	client.consumer = consumer
	client.logger.Info("consumer initialized successfully")

	// Initialize health checker
	client.logger.Info("initializing health checker",
		logx.String("interval", config.HealthCheckInterval.String()),
	)
	client.health = NewHealthChecker(client.conn, config.HealthCheckInterval, nil)
	client.logger.Info("health checker initialized successfully")

	return client, nil
}

// connect establishes connection to RabbitMQ
func (c *Client) connect() error {
	c.logger.Debug("establishing RabbitMQ connection",
		logx.String("url", c.config.URL),
	)

	conn, err := rabbitmq.NewConn(
		c.config.URL,
		rabbitmq.WithConnectionOptionsLogging,
	)
	if err != nil {
		c.logger.Error("connection failed",
			logx.String("url", c.config.URL),
			logx.ErrorField(err),
		)
		return NewConnectionError("failed to connect to RabbitMQ", c.config.URL, 1, err)
	}

	c.conn = conn
	c.logger.Debug("connection established successfully")
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
	c.logger.Debug("publishing message",
		logx.String("message_id", msg.ID),
		logx.String("routing_key", msg.Properties.RoutingKey),
		logx.Int("size_bytes", len(msg.Body)),
	)

	component, err := c.checkClosedAndGetComponent("producer")
	if err != nil {
		c.logger.Error("failed to get producer component", logx.ErrorField(err))
		return err
	}

	producer := component.(*Producer)
	err = producer.Publish(ctx, msg)
	if err != nil {
		c.logger.Error("message publish failed",
			logx.String("message_id", msg.ID),
			logx.String("routing_key", msg.Properties.RoutingKey),
			logx.ErrorField(err),
		)
	} else {
		c.logger.Debug("message published successfully",
			logx.String("message_id", msg.ID),
			logx.String("routing_key", msg.Properties.RoutingKey),
		)
	}
	return err
}

// PublishBatch publishes a batch of messages to RabbitMQ
func (c *Client) PublishBatch(ctx context.Context, batch *BatchMessage) error {
	c.logger.Debug("publishing batch",
		logx.Int("batch_size", len(batch.Messages)),
	)

	component, err := c.checkClosedAndGetComponent("producer")
	if err != nil {
		c.logger.Error("failed to get producer component", logx.ErrorField(err))
		return err
	}

	producer := component.(*Producer)
	err = producer.PublishBatch(ctx, batch)
	if err != nil {
		c.logger.Error("batch publish failed",
			logx.Int("batch_size", len(batch.Messages)),
			logx.ErrorField(err),
		)
	} else {
		c.logger.Debug("batch published successfully",
			logx.Int("batch_size", len(batch.Messages)),
		)
	}
	return err
}

// Consume starts consuming messages from a queue
func (c *Client) Consume(ctx context.Context, queue string, handler MessageHandler) error {
	c.logger.Info("starting message consumption",
		logx.String("queue", queue),
	)

	component, err := c.checkClosedAndGetComponent("consumer")
	if err != nil {
		c.logger.Error("failed to get consumer component", logx.ErrorField(err))
		return err
	}

	consumer := component.(*Consumer)
	err = consumer.Consume(ctx, queue, handler)
	if err != nil {
		c.logger.Error("failed to start consuming",
			logx.String("queue", queue),
			logx.ErrorField(err),
		)
	} else {
		c.logger.Info("message consumption started successfully",
			logx.String("queue", queue),
		)
	}
	return err
}

// ConsumeWithOptions starts consuming messages with custom options
func (c *Client) ConsumeWithOptions(ctx context.Context, queue string, handler MessageHandler, options *ConsumeOptions) error {
	c.logger.Info("starting message consumption with options",
		logx.String("queue", queue),
		logx.Any("options", options),
	)

	component, err := c.checkClosedAndGetComponent("consumer")
	if err != nil {
		c.logger.Error("failed to get consumer component", logx.ErrorField(err))
		return err
	}

	consumer := component.(*Consumer)
	err = consumer.ConsumeWithOptions(ctx, queue, handler, options)
	if err != nil {
		c.logger.Error("failed to start consuming with options",
			logx.String("queue", queue),
			logx.ErrorField(err),
		)
	} else {
		c.logger.Info("message consumption with options started successfully",
			logx.String("queue", queue),
		)
	}
	return err
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
		c.logger.Debug("client already closed")
		return nil
	}

	c.logger.Info("closing client and all resources")
	c.closed = true
	c.cancel()

	var errs []error

	// Close producer
	if c.producer != nil {
		c.logger.Debug("closing producer")
		if err := c.producer.Close(); err != nil {
			c.logger.Error("failed to close producer", logx.ErrorField(err))
			errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
		} else {
			c.logger.Debug("producer closed successfully")
		}
	}

	// Close consumer
	if c.consumer != nil {
		c.logger.Debug("closing consumer")
		if err := c.consumer.Close(); err != nil {
			c.logger.Error("failed to close consumer", logx.ErrorField(err))
			errs = append(errs, fmt.Errorf("failed to close consumer: %w", err))
		} else {
			c.logger.Debug("consumer closed successfully")
		}
	}

	// Close health checker
	if c.health != nil {
		c.logger.Debug("closing health checker")
		if err := c.health.Close(); err != nil {
			c.logger.Error("failed to close health checker", logx.ErrorField(err))
			errs = append(errs, fmt.Errorf("failed to close health checker: %w", err))
		} else {
			c.logger.Debug("health checker closed successfully")
		}
	}

	// Close connection
	if c.conn != nil {
		c.logger.Debug("closing connection")
		if err := c.conn.Close(); err != nil {
			c.logger.Error("failed to close connection", logx.ErrorField(err))
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		} else {
			c.logger.Debug("connection closed successfully")
		}
	}

	if len(errs) > 0 {
		c.logger.Error("errors occurred during close",
			logx.Int("error_count", len(errs)),
			logx.Any("errors", errs),
		)
		return fmt.Errorf("errors during close: %v", errs)
	}

	c.logger.Info("client closed successfully")
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
	producer, err := NewProducer(c.conn, &c.config.ProducerConfig, c.logger)
	if err != nil {
		return NewConnectionError("failed to recreate producer", c.config.URL, 1, err)
	}
	c.producer = producer

	// Recreate consumer
	consumer, err := NewConsumer(c.conn, &c.config.ConsumerConfig, c.logger)
	if err != nil {
		return NewConnectionError("failed to recreate consumer", c.config.URL, 1, err)
	}
	c.consumer = consumer

	// Recreate health checker
	c.health = NewHealthChecker(c.conn, c.config.HealthCheckInterval, nil)

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
