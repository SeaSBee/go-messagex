package unit

import (
	"context"
	"testing"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
)

// TestTransportTopology tests topology declaration functionality
func TestTransportTopology(t *testing.T) {
	t.Run("NilTopology", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test that nil topology doesn't cause issues
		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to topology
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to declare topology")
		}
	})

	t.Run("EmptyTopology", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs:     []string{"amqp://localhost:5672"},
			Topology: &messaging.TopologyConfig{},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to topology
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to declare topology")
		}
	})

	t.Run("TopologyWithExchanges", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Topology: &messaging.TopologyConfig{
				Exchanges: []messaging.ExchangeConfig{
					{
						Name:       "test-exchange",
						Type:       "direct",
						Durable:    true,
						AutoDelete: false,
						Arguments:  map[string]interface{}{},
					},
				},
			},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to exchange declaration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to declare exchange")
		}
	})

	t.Run("TopologyWithQueues", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Topology: &messaging.TopologyConfig{
				Queues: []messaging.QueueConfig{
					{
						Name:       "test-queue",
						Durable:    true,
						AutoDelete: false,
						Exclusive:  false,
						Arguments:  map[string]interface{}{},
					},
				},
			},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to queue declaration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to declare queue")
		}
	})

	t.Run("TopologyWithBindings", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Topology: &messaging.TopologyConfig{
				Exchanges: []messaging.ExchangeConfig{
					{
						Name:       "test-exchange",
						Type:       "direct",
						Durable:    true,
						AutoDelete: false,
						Arguments:  map[string]interface{}{},
					},
				},
				Queues: []messaging.QueueConfig{
					{
						Name:       "test-queue",
						Durable:    true,
						AutoDelete: false,
						Exclusive:  false,
						Arguments:  map[string]interface{}{},
					},
				},
				Bindings: []messaging.BindingConfig{
					{
						Queue:     "test-queue",
						Exchange:  "test-exchange",
						Key:       "test-key",
						Arguments: map[string]interface{}{},
					},
				},
			},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to binding declaration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to declare binding")
		}
	})
}

// TestTransportChannelConfiguration tests channel configuration functionality
func TestTransportChannelConfiguration(t *testing.T) {
	t.Run("NilConsumerConfig", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to channel configuration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to configure channel")
		}
	})

	t.Run("ConsumerConfigWithPrefetch", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Consumer: &messaging.ConsumerConfig{
				Queue:    "test-queue",
				Prefetch: 10,
			},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to QoS configuration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to set QoS")
		}
	})

	t.Run("ConsumerConfigWithZeroPrefetch", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			Consumer: &messaging.ConsumerConfig{
				Queue:    "test-queue",
				Prefetch: 0,
			},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but not due to QoS configuration
		if err != nil {
			assert.NotContains(t, err.Error(), "failed to set QoS")
		}
	})
}

// TestTransportErrorHandling tests error handling scenarios
func TestTransportErrorHandling(t *testing.T) {
	t.Run("ConnectionFailure", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://invalid-host:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to dial RabbitMQ")
	})

	t.Run("ChannelCreationFailure", func(t *testing.T) {
		// This test is difficult to trigger without a real connection
		// It's documented here for completeness
		t.Skip("Requires real RabbitMQ connection to test channel creation failure")
	})

	t.Run("ExchangeDeclarationFailure", func(t *testing.T) {
		// This test is difficult to trigger without a real connection
		// It's documented here for completeness
		t.Skip("Requires real RabbitMQ connection to test exchange declaration failure")
	})

	t.Run("QueueDeclarationFailure", func(t *testing.T) {
		// This test is difficult to trigger without a real connection
		// It's documented here for completeness
		t.Skip("Requires real RabbitMQ connection to test queue declaration failure")
	})

	t.Run("BindingDeclarationFailure", func(t *testing.T) {
		// This test is difficult to trigger without a real connection
		// It's documented here for completeness
		t.Skip("Requires real RabbitMQ connection to test binding declaration failure")
	})
}

// TestTransportObservability tests observability integration
func TestTransportObservability(t *testing.T) {
	t.Run("WithoutObservability", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but should not panic
		// The error might be nil if the connection attempt succeeds unexpectedly
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})
}

// TestTransportResourceManagement tests resource cleanup
func TestTransportResourceManagement(t *testing.T) {
	t.Run("DisconnectWithoutConnect", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Disconnect without connecting should succeed
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)
		assert.False(t, transport.IsConnected())
	})

	t.Run("MultipleDisconnect", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Multiple disconnects should succeed
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)

		err = transport.Disconnect(context.Background())
		assert.NoError(t, err)

		assert.False(t, transport.IsConnected())
	})

	t.Run("ConnectAfterDisconnect", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Disconnect first
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)

		// Try to connect after disconnect should fail
		err = transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is closed")
	})
}
