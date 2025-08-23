// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"sync"

	"github.com/rabbitmq/amqp091-go"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

// Transport implements the messaging.Transport interface for RabbitMQ.
type Transport struct {
	config        *messaging.RabbitMQConfig
	connection    *amqp091.Connection
	channel       *amqp091.Channel
	mu            sync.RWMutex
	closed        bool
	observability *messaging.ObservabilityContext
}

// NewTransport creates a new RabbitMQ transport.
func NewTransport(config *messaging.RabbitMQConfig, observability *messaging.ObservabilityContext) *Transport {
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

	// Select URI to connect to
	uri, err := t.selectURI()
	if err != nil {
		return messaging.WrapError(messaging.ErrorCodeConnection, "connect", "failed to select URI", err)
	}

	// Establish connection
	conn, err := amqp091.Dial(uri)
	if err != nil {
		return messaging.WrapError(messaging.ErrorCodeConnection, "connect", "failed to dial RabbitMQ", err)
	}

	t.connection = conn

	// Create channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return messaging.WrapError(messaging.ErrorCodeChannel, "connect", "failed to create channel", err)
	}

	t.channel = channel

	// Configure channel
	if err := t.configureChannel(); err != nil {
		channel.Close()
		conn.Close()
		return messaging.WrapError(messaging.ErrorCodeChannel, "connect", "failed to configure channel", err)
	}

	// Declare topology
	if err := t.declareTopology(); err != nil {
		channel.Close()
		conn.Close()
		return messaging.WrapError(messaging.ErrorCodeTransport, "connect", "failed to declare topology", err)
	}

	return nil
}

// Disconnect closes the connection to RabbitMQ.
func (t *Transport) Disconnect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true

	var errs []error

	if t.channel != nil {
		if err := t.channel.Close(); err != nil {
			errs = append(errs, messaging.WrapError(messaging.ErrorCodeChannel, "disconnect", "failed to close channel", err))
		}
		t.channel = nil
	}

	if t.connection != nil {
		if err := t.connection.Close(); err != nil {
			errs = append(errs, messaging.WrapError(messaging.ErrorCodeConnection, "disconnect", "failed to close connection", err))
		}
		t.connection = nil
	}

	if len(errs) > 0 {
		return messaging.WrapError(messaging.ErrorCodeTransport, "disconnect", "failed to disconnect cleanly", errs[0])
	}

	return nil
}

// IsConnected returns true if the transport is connected.
func (t *Transport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.connection != nil && !t.connection.IsClosed() &&
		t.channel != nil && !t.channel.IsClosed() &&
		!t.closed
}

// selectURI selects a URI from the configured URIs.
func (t *Transport) selectURI() (string, error) {
	if len(t.config.URIs) == 0 {
		return "", messaging.NewError(messaging.ErrorCodeConfiguration, "select_uri", "no URIs configured")
	}

	// For now, use the first URI
	// TODO: Implement load balancing and failover
	return t.config.URIs[0], nil
}

// configureChannel configures the channel settings.
func (t *Transport) configureChannel() error {
	// Set QoS
	if t.config.Consumer != nil && t.config.Consumer.Prefetch > 0 {
		if err := t.channel.Qos(
			t.config.Consumer.Prefetch, // prefetch count
			0,                          // prefetch size
			false,                      // global
		); err != nil {
			return err
		}
	}

	return nil
}

// declareTopology declares exchanges, queues, and bindings.
func (t *Transport) declareTopology() error {
	if t.config.Topology == nil {
		return nil
	}

	// Declare exchanges
	if t.config.Topology.Exchanges != nil {
		for _, exchange := range t.config.Topology.Exchanges {
			if err := t.declareExchange(&exchange); err != nil {
				return err
			}
		}
	}

	// Declare queues
	if t.config.Topology.Queues != nil {
		for _, queue := range t.config.Topology.Queues {
			if err := t.declareQueue(&queue); err != nil {
				return err
			}
		}
	}

	// Declare bindings
	if t.config.Topology.Bindings != nil {
		for _, binding := range t.config.Topology.Bindings {
			if err := t.declareBinding(&binding); err != nil {
				return err
			}
		}
	}

	return nil
}

// declareExchange declares an exchange.
func (t *Transport) declareExchange(exchange *messaging.ExchangeConfig) error {
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
	if cfg.RabbitMQ == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_publisher", "RabbitMQ configuration is required")
	}

	// Create observability context
	observability, err := messaging.NewObservabilityProvider(cfg.Telemetry)
	if err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_publisher", "failed to create observability provider", err)
	}

	obsCtx := messaging.NewObservabilityContext(ctx, observability)

	// Create transport
	transport := NewTransport(cfg.RabbitMQ, obsCtx)

	// Create publisher
	publisher := NewPublisher(transport, cfg.RabbitMQ.Publisher, obsCtx)

	return publisher, nil
}

// NewConsumer creates a new RabbitMQ consumer.
func (tf *TransportFactory) NewConsumer(ctx context.Context, cfg *messaging.Config) (messaging.Consumer, error) {
	if cfg.RabbitMQ == nil {
		return nil, messaging.NewError(messaging.ErrorCodeConfiguration, "new_consumer", "RabbitMQ configuration is required")
	}

	// Create observability context
	observability, err := messaging.NewObservabilityProvider(cfg.Telemetry)
	if err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeConfiguration, "new_consumer", "failed to create observability provider", err)
	}

	obsCtx := messaging.NewObservabilityContext(ctx, observability)

	// Create transport
	transport := NewTransport(cfg.RabbitMQ, obsCtx)

	// Create consumer
	consumer := NewConsumer(transport, cfg.RabbitMQ.Consumer, obsCtx)

	return consumer, nil
}

// ValidateConfig validates the RabbitMQ configuration.
func (tf *TransportFactory) ValidateConfig(cfg *messaging.Config) error {
	if cfg.RabbitMQ == nil {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "RabbitMQ configuration is required")
	}

	if len(cfg.RabbitMQ.URIs) == 0 {
		return messaging.NewError(messaging.ErrorCodeConfiguration, "validate_config", "at least one RabbitMQ URI is required")
	}

	if cfg.RabbitMQ.Consumer != nil && cfg.RabbitMQ.Consumer.Queue == "" {
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
