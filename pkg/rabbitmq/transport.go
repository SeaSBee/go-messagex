// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

// Transport implements the messaging.Transport interface for RabbitMQ.
type Transport struct {
	config        *messaging.RabbitMQConfig
	connection    *amqp091.Connection
	channel       *amqp091.Channel
	mu            sync.RWMutex
	closed        bool
	connected     bool
	observability *messaging.ObservabilityContext
}

// NewTransport creates a new RabbitMQ transport.
func NewTransport(config *messaging.RabbitMQConfig, observability *messaging.ObservabilityContext) *Transport {
	if config == nil {
		// Log warning but don't panic - let calling code handle the error
		if observability != nil {
			observability.Logger().Warn("NewTransport called with nil config")
		}
	}

	return &Transport{
		config:        config,
		observability: observability,
	}
}

// Connect establishes a connection to RabbitMQ.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return messaging.NewError(messaging.ErrorCodeConnection, "connect", "transport is closed")
	}

	if t.connected {
		return messaging.NewError(messaging.ErrorCodeConnection, "connect", "transport is already connected")
	}

	// Log connection attempt
	if t.observability != nil {
		t.observability.Logger().Info("Attempting to connect to RabbitMQ")
	}

	// Select URI to connect to
	uri, err := t.selectURI()
	if err != nil {
		return messaging.WrapError(messaging.ErrorCodeConnection, "connect", "failed to select URI", err)
	}

	// Use simple dial
	conn, err := amqp091.Dial(uri)
	if err != nil {
		if t.observability != nil {
			t.observability.Logger().Error("Failed to connect to RabbitMQ", logx.String("error", err.Error()))
		}
		return messaging.WrapError(messaging.ErrorCodeConnection, "connect", "failed to dial RabbitMQ", err)
	}

	t.connection = conn

	// Create channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		if t.observability != nil {
			t.observability.Logger().Error("Failed to create channel", logx.String("error", err.Error()))
		}
		return messaging.WrapError(messaging.ErrorCodeChannel, "connect", "failed to create channel", err)
	}

	t.channel = channel

	// Configure channel
	if err := t.configureChannel(); err != nil {
		channel.Close()
		conn.Close()
		if t.observability != nil {
			t.observability.Logger().Error("Failed to configure channel", logx.String("error", err.Error()))
		}
		return messaging.WrapError(messaging.ErrorCodeChannel, "connect", "failed to configure channel", err)
	}

	// Declare topology
	if err := t.declareTopology(); err != nil {
		channel.Close()
		conn.Close()
		if t.observability != nil {
			t.observability.Logger().Error("Failed to declare topology", logx.String("error", err.Error()))
		}
		return messaging.WrapError(messaging.ErrorCodeTransport, "connect", "failed to declare topology", err)
	}

	t.connected = true

	// Log successful connection
	if t.observability != nil {
		t.observability.Logger().Info("Successfully connected to RabbitMQ")
	}

	return nil
}

// Disconnect closes the connection to RabbitMQ.
func (t *Transport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil // Already disconnected
	}

	// Log disconnect attempt
	if t.observability != nil {
		t.observability.Logger().Info("Disconnecting from RabbitMQ")
	}

	var errs []error

	// Set up timeout for disconnect operations
	disconnectCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		// Set default timeout if none provided
		var cancel context.CancelFunc
		disconnectCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	// Close channel first
	if t.channel != nil {
		// Use context for channel close timeout
		done := make(chan error, 1)
		go func() {
			done <- t.channel.Close()
		}()

		select {
		case err := <-done:
			if err != nil {
				errs = append(errs, messaging.WrapError(messaging.ErrorCodeChannel, "disconnect", "failed to close channel", err))
			}
		case <-disconnectCtx.Done():
			errs = append(errs, messaging.NewError(messaging.ErrorCodeChannel, "disconnect", "timeout closing channel"))
		}
		t.channel = nil
	}

	// Close connection
	if t.connection != nil {
		// Use context for connection close timeout
		done := make(chan error, 1)
		go func() {
			done <- t.connection.Close()
		}()

		select {
		case err := <-done:
			if err != nil {
				errs = append(errs, messaging.WrapError(messaging.ErrorCodeConnection, "disconnect", "failed to close connection", err))
			}
		case <-disconnectCtx.Done():
			errs = append(errs, messaging.NewError(messaging.ErrorCodeConnection, "disconnect", "timeout closing connection"))
		}
		t.connection = nil
	}

	// Mark as closed after resource cleanup
	t.closed = true
	t.connected = false

	// Log disconnect result
	if t.observability != nil {
		if len(errs) > 0 {
			t.observability.Logger().Warn("Disconnected from RabbitMQ with errors", logx.Int("error_count", len(errs)))
		} else {
			t.observability.Logger().Info("Successfully disconnected from RabbitMQ")
		}
	}

	// Return all errors if any occurred
	if len(errs) > 0 {
		if len(errs) == 1 {
			return errs[0]
		}
		// Create a combined error message for multiple errors
		errMsgs := make([]string, len(errs))
		for i, err := range errs {
			errMsgs[i] = err.Error()
		}
		return messaging.NewError(messaging.ErrorCodeTransport, "disconnect",
			fmt.Sprintf("failed to disconnect cleanly: %s", strings.Join(errMsgs, "; ")))
	}

	return nil
}

// IsConnected returns true if the transport is connected.
func (t *Transport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || !t.connected {
		return false
	}

	if t.connection == nil || t.channel == nil {
		return false
	}

	return !t.connection.IsClosed() && !t.channel.IsClosed()
}

// selectURI selects a URI from the configured URIs.
func (t *Transport) selectURI() (string, error) {
	if t.config == nil {
		return "", messaging.NewError(messaging.ErrorCodeConfiguration, "select_uri", "transport config is nil")
	}

	if len(t.config.URIs) == 0 {
		return "", messaging.NewError(messaging.ErrorCodeConfiguration, "select_uri", "no URIs configured")
	}

	// For now, use the first URI
	// TODO: Implement load balancing and failover
	uri := t.config.URIs[0]

	// Validate URI
	if strings.TrimSpace(uri) == "" {
		return "", messaging.NewError(messaging.ErrorCodeConfiguration, "select_uri", "URI is empty")
	}

	// Basic URI validation
	if _, err := url.Parse(uri); err != nil {
		return "", messaging.WrapError(messaging.ErrorCodeConfiguration, "select_uri", "invalid URI format", err)
	}

	return uri, nil
}

// configureChannel configures the channel settings.
func (t *Transport) configureChannel() error {
	if t.channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "configure_channel", "channel is nil")
	}

	if t.config == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "configure_channel", "config is nil")
	}

	// Set QoS
	if t.config.Consumer != nil && t.config.Consumer.Prefetch > 0 {
		if err := t.channel.Qos(
			t.config.Consumer.Prefetch, // prefetch count
			0,                          // prefetch size
			false,                      // global
		); err != nil {
			return messaging.WrapError(messaging.ErrorCodeChannel, "configure_channel", "failed to set QoS", err)
		}
	}

	return nil
}

// declareTopology declares exchanges, queues, and bindings.
func (t *Transport) declareTopology() error {
	if t.channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "declare_topology", "channel is nil")
	}

	if t.config == nil || t.config.Topology == nil {
		return nil
	}

	// Declare exchanges
	if t.config.Topology.Exchanges != nil {
		for _, exchange := range t.config.Topology.Exchanges {
			if err := t.declareExchange(&exchange); err != nil {
				return messaging.WrapError(messaging.ErrorCodeTransport, "declare_topology",
					fmt.Sprintf("failed to declare exchange %s", exchange.Name), err)
			}
		}
	}

	// Declare queues
	if t.config.Topology.Queues != nil {
		for _, queue := range t.config.Topology.Queues {
			if err := t.declareQueue(&queue); err != nil {
				return messaging.WrapError(messaging.ErrorCodeTransport, "declare_topology",
					fmt.Sprintf("failed to declare queue %s", queue.Name), err)
			}
		}
	}

	// Declare bindings
	if t.config.Topology.Bindings != nil {
		for _, binding := range t.config.Topology.Bindings {
			if err := t.declareBinding(&binding); err != nil {
				return messaging.WrapError(messaging.ErrorCodeTransport, "declare_topology",
					fmt.Sprintf("failed to declare binding for queue %s to exchange %s", binding.Queue, binding.Exchange), err)
			}
		}
	}

	return nil
}

// declareExchange declares an exchange.
func (t *Transport) declareExchange(exchange *messaging.ExchangeConfig) error {
	if t.channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "declare_exchange", "channel is nil")
	}

	if exchange == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_exchange", "exchange config is nil")
	}

	if strings.TrimSpace(exchange.Name) == "" {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_exchange", "exchange name is empty")
	}

	return t.channel.ExchangeDeclare(
		exchange.Name,       // name
		exchange.Type,       // type
		exchange.Durable,    // durable
		exchange.AutoDelete, // auto-deleted
		false,               // internal
		false,               // no-wait
		exchange.Arguments,  // arguments
	)
}

// declareQueue declares a queue.
func (t *Transport) declareQueue(queue *messaging.QueueConfig) error {
	if t.channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "declare_queue", "channel is nil")
	}

	if queue == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_queue", "queue config is nil")
	}

	if strings.TrimSpace(queue.Name) == "" {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_queue", "queue name is empty")
	}

	_, err := t.channel.QueueDeclare(
		queue.Name,       // name
		queue.Durable,    // durable
		queue.AutoDelete, // auto-deleted
		queue.Exclusive,  // exclusive
		false,            // no-wait
		queue.Arguments,  // arguments
	)
	return err
}

// declareBinding declares a binding.
func (t *Transport) declareBinding(binding *messaging.BindingConfig) error {
	if t.channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "declare_binding", "channel is nil")
	}

	if binding == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_binding", "binding config is nil")
	}

	if strings.TrimSpace(binding.Queue) == "" {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_binding", "binding queue name is empty")
	}

	if strings.TrimSpace(binding.Exchange) == "" {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "declare_binding", "binding exchange name is empty")
	}

	return t.channel.QueueBind(
		binding.Queue,     // queue name
		binding.Key,       // routing key
		binding.Exchange,  // exchange name
		false,             // no-wait
		binding.Arguments, // arguments
	)
}

// GetChannel returns the underlying AMQP channel.
func (t *Transport) GetChannel() *amqp091.Channel {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.channel
}

// GetConnection returns the underlying AMQP connection.
func (t *Transport) GetConnection() *amqp091.Connection {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connection
}

// TransportFactory implements messaging.TransportFactory for RabbitMQ.
type TransportFactory struct{}

// NewTransportFactory creates a new RabbitMQ transport factory.
func NewTransportFactory() *TransportFactory {
	return &TransportFactory{}
}

// NewPublisher creates a new RabbitMQ publisher.
func (tf *TransportFactory) NewPublisher(ctx context.Context, cfg *messaging.Config) (messaging.Publisher, error) {
	if cfg == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_publisher", "configuration is nil")
	}

	if cfg.RabbitMQ == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_publisher", "RabbitMQ configuration is required")
	}

	// Create observability context
	var observability *messaging.ObservabilityProvider
	var err error

	if cfg.Telemetry != nil {
		observability, err = messaging.NewObservabilityProvider(cfg.Telemetry)
		if err != nil {
			return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_publisher", "failed to create observability provider", err)
		}
	}

	obsCtx := messaging.NewObservabilityContext(ctx, observability)

	// Create transport
	transport := NewTransport(cfg.RabbitMQ, obsCtx)

	// Create publisher
	publisher, err := NewPublisher(transport, cfg.RabbitMQ.Publisher, obsCtx)
	if err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_publisher", "failed to create publisher", err)
	}

	return publisher, nil
}

// NewConsumer creates a new RabbitMQ consumer.
func (tf *TransportFactory) NewConsumer(ctx context.Context, cfg *messaging.Config) (messaging.Consumer, error) {
	if cfg == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_consumer", "configuration is nil")
	}

	if cfg.RabbitMQ == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_consumer", "RabbitMQ configuration is required")
	}

	// Create observability context
	var observability *messaging.ObservabilityProvider
	var err error

	if cfg.Telemetry != nil {
		observability, err = messaging.NewObservabilityProvider(cfg.Telemetry)
		if err != nil {
			return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_consumer", "failed to create observability provider", err)
		}
	}

	obsCtx := messaging.NewObservabilityContext(ctx, observability)

	// Create transport
	transport := NewTransport(cfg.RabbitMQ, obsCtx)

	// Create consumer
	consumer, err := NewConsumer(transport, cfg.RabbitMQ.Consumer, obsCtx)
	if err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_consumer", "failed to create consumer", err)
	}

	return consumer, nil
}

// ValidateConfig validates the RabbitMQ configuration.
func (tf *TransportFactory) ValidateConfig(cfg *messaging.Config) error {
	if cfg == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "configuration is nil")
	}

	if cfg.RabbitMQ == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "RabbitMQ configuration is required")
	}

	if len(cfg.RabbitMQ.URIs) == 0 {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "at least one RabbitMQ URI is required")
	}

	// Validate URIs
	for i, uri := range cfg.RabbitMQ.URIs {
		if strings.TrimSpace(uri) == "" {
			return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config",
				fmt.Sprintf("URI at index %d is empty", i))
		}

		if _, err := url.Parse(uri); err != nil {
			return messaging.WrapError(messaging.ErrorCodeConfiguration, "validate_config",
				fmt.Sprintf("invalid URI at index %d", i), err)
		}
	}

	if cfg.RabbitMQ.Consumer != nil && strings.TrimSpace(cfg.RabbitMQ.Consumer.Queue) == "" {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "consumer queue is required")
	}

	return nil
}

// Name returns the transport name.
func (tf *TransportFactory) Name() string {
	return "rabbitmq"
}

// Register registers the RabbitMQ transport factory.
func Register() {
	messaging.RegisterTransport("rabbitmq", NewTransportFactory())
}
