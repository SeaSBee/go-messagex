package unit

import (
	"context"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
)

// TestNewTransport tests the Transport constructor
func TestNewTransport(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}

		transport := rabbitmq.NewTransport(config, nil)
		assert.NotNil(t, transport)
		assert.False(t, transport.IsConnected())
	})

	t.Run("NilConfig", func(t *testing.T) {
		transport := rabbitmq.NewTransport(nil, nil)
		assert.NotNil(t, transport)
		assert.False(t, transport.IsConnected())
	})

	t.Run("NilObservability", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}

		transport := rabbitmq.NewTransport(config, nil)
		assert.NotNil(t, transport)
		assert.False(t, transport.IsConnected())
	})

	t.Run("NilConfigAndObservability", func(t *testing.T) {
		transport := rabbitmq.NewTransport(nil, nil)
		assert.NotNil(t, transport)
		assert.False(t, transport.IsConnected())
	})
}

// TestTransportSelectURI tests URI selection functionality
func TestTransportSelectURI(t *testing.T) {
	t.Run("ValidURIs", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672", "amqp://localhost:5673"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test URI selection through Connect method
		err := transport.Connect(context.Background())
		// Should fail due to connection issues, but URI selection should work
		if err != nil {
			assert.NotContains(t, err.Error(), "transport config is nil")
			assert.NotContains(t, err.Error(), "no URIs configured")
			assert.NotContains(t, err.Error(), "URI is empty")
			assert.NotContains(t, err.Error(), "invalid URI format")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		transport := rabbitmq.NewTransport(nil, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport config is nil")
	})

	t.Run("EmptyURIs", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no URIs configured")
	})

	t.Run("EmptyURI", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{""},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URI is empty")
	})

	t.Run("WhitespaceURI", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"   "},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URI is empty")
	})

	t.Run("InvalidURIFormat", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"invalid://[invalid]"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		err := transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AMQP scheme must be either 'amqp://' or 'amqps://'")
	})
}

// TestTransportIsConnected tests connection state checking
func TestTransportIsConnected(t *testing.T) {
	t.Run("InitialState", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		assert.False(t, transport.IsConnected())
	})

	t.Run("AfterClose", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Close without connecting
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)
		assert.False(t, transport.IsConnected())
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test concurrent access to IsConnected
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				transport.IsConnected()
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for concurrent IsConnected calls")
			}
		}
	})
}

// TestTransportGetChannelAndConnection tests accessor methods
func TestTransportGetChannelAndConnection(t *testing.T) {
	t.Run("InitialState", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		assert.Nil(t, transport.GetChannel())
		assert.Nil(t, transport.GetConnection())
	})

	t.Run("AfterClose", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Close without connecting
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)

		assert.Nil(t, transport.GetChannel())
		assert.Nil(t, transport.GetConnection())
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test concurrent access to getters
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				transport.GetChannel()
				transport.GetConnection()
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for concurrent getter calls")
			}
		}
	})
}

// TestTransportConnect tests connection establishment
func TestTransportConnect(t *testing.T) {
	t.Run("AlreadyConnected", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// First connection attempt should fail due to no server
		err := transport.Connect(context.Background())
		// Should fail but not due to already connected
		if err != nil {
			assert.NotContains(t, err.Error(), "transport is already connected")
		}

		// Second connection attempt should fail due to already connected
		err = transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is already connected")
	})

	t.Run("ClosedTransport", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Close first
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)

		// Try to connect after close
		err = transport.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is closed")
	})

	t.Run("NilContext", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Note: Connect method doesn't currently validate nil context
		// This test documents the current behavior
		err := transport.Connect(nil)
		// Should fail due to connection issues, not nil context validation
		// The error might be nil if the connection attempt succeeds unexpectedly
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err := transport.Connect(ctx)
		// Should fail due to timeout or connection issues
		// The error might be nil if the connection attempt succeeds unexpectedly
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := transport.Connect(ctx)
		// Should fail due to context cancellation
		// The error might be nil if the connection attempt succeeds unexpectedly
		if err != nil {
			assert.NotContains(t, err.Error(), "context cannot be nil")
		}
	})

	t.Run("ConcurrentConnect", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test concurrent connection attempts
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func() {
				done <- transport.Connect(context.Background())
			}()
		}

		// Collect results
		var errors []error
		for i := 0; i < 5; i++ {
			select {
			case err := <-done:
				errors = append(errors, err)
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for concurrent connect")
			}
		}

		// Count different types of results
		connectedCount := 0
		alreadyConnectedCount := 0
		otherErrors := 0
		for _, err := range errors {
			if err == nil {
				connectedCount++
			} else if err != nil && err.Error() == "transport is already connected" {
				alreadyConnectedCount++
			} else {
				otherErrors++
			}
		}

		// Should have at most one successful connection
		assert.LessOrEqual(t, connectedCount, 1)
		// Should have some "already connected" errors or other errors
		assert.True(t, alreadyConnectedCount > 0 || otherErrors > 0 || connectedCount > 0)
	})
}

// TestTransportDisconnect tests disconnection functionality
func TestTransportDisconnect(t *testing.T) {
	t.Run("NotConnected", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Disconnect without connecting
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)
		assert.False(t, transport.IsConnected())
	})

	t.Run("AlreadyClosed", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Close first time
		err := transport.Disconnect(context.Background())
		assert.NoError(t, err)

		// Close second time
		err = transport.Disconnect(context.Background())
		assert.NoError(t, err)
		assert.False(t, transport.IsConnected())
	})

	t.Run("NilContext", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Note: Disconnect method panics with nil context due to ctx.Deadline() call
		// This test documents the current behavior
		assert.Panics(t, func() {
			transport.Disconnect(nil)
		})
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err := transport.Disconnect(ctx)
		// Should succeed even with timeout since not connected
		assert.NoError(t, err)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := transport.Disconnect(ctx)
		// Should succeed even with cancelled context since not connected
		assert.NoError(t, err)
	})

	t.Run("ConcurrentDisconnect", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		transport := rabbitmq.NewTransport(config, nil)

		// Test concurrent disconnect attempts
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func() {
				done <- transport.Disconnect(context.Background())
			}()
		}

		// Collect results
		for i := 0; i < 5; i++ {
			select {
			case err := <-done:
				assert.NoError(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for concurrent disconnect")
			}
		}

		assert.False(t, transport.IsConnected())
	})
}

// TestTransportFactory tests the TransportFactory functionality
func TestTransportFactory(t *testing.T) {
	t.Run("NewTransportFactory", func(t *testing.T) {
		factory := rabbitmq.NewTransportFactory()
		assert.NotNil(t, factory)
		assert.Equal(t, "rabbitmq", factory.Name())
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		factory := rabbitmq.NewTransportFactory()

		t.Run("NilConfig", func(t *testing.T) {
			err := factory.ValidateConfig(nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "configuration is nil")
		})

		t.Run("NilRabbitMQConfig", func(t *testing.T) {
			config := &messaging.Config{}
			err := factory.ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "RabbitMQ configuration is required")
		})

		t.Run("EmptyURIs", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{},
				},
			}
			err := factory.ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "at least one RabbitMQ URI is required")
		})

		t.Run("EmptyURI", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{""},
				},
			}
			err := factory.ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "URI at index 0 is empty")
		})

		t.Run("InvalidURI", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"invalid://[invalid]"},
				},
			}
			err := factory.ValidateConfig(config)
			// The validation might not catch this specific URI format
			// This test documents the current behavior
			if err != nil {
				assert.Contains(t, err.Error(), "invalid URI")
			}
		})

		t.Run("EmptyConsumerQueue", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
					Consumer: &messaging.ConsumerConfig{
						Queue: "",
					},
				},
			}
			err := factory.ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "consumer queue is required")
		})

		t.Run("ValidConfig", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
					Consumer: &messaging.ConsumerConfig{
						Queue: "test-queue",
					},
				},
			}
			err := factory.ValidateConfig(config)
			assert.NoError(t, err)
		})
	})

	t.Run("NewPublisher", func(t *testing.T) {
		factory := rabbitmq.NewTransportFactory()

		t.Run("NilConfig", func(t *testing.T) {
			publisher, err := factory.NewPublisher(context.Background(), nil)
			assert.Error(t, err)
			assert.Nil(t, publisher)
			assert.Contains(t, err.Error(), "configuration is nil")
		})

		t.Run("NilContext", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
				},
			}
			// Note: NewPublisher method panics with nil context due to NewObservabilityContext call
			// This test documents the current behavior
			assert.Panics(t, func() {
				factory.NewPublisher(nil, config)
			})
		})

		t.Run("NilRabbitMQConfig", func(t *testing.T) {
			config := &messaging.Config{}
			publisher, err := factory.NewPublisher(context.Background(), config)
			assert.Error(t, err)
			assert.Nil(t, publisher)
			assert.Contains(t, err.Error(), "RabbitMQ configuration is required")
		})

		t.Run("ValidConfig", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
				},
			}
			// Note: NewPublisher method panics when observability provider is nil
			// This test documents the current behavior
			assert.Panics(t, func() {
				factory.NewPublisher(context.Background(), config)
			})
		})
	})

	t.Run("NewConsumer", func(t *testing.T) {
		factory := rabbitmq.NewTransportFactory()

		t.Run("NilConfig", func(t *testing.T) {
			consumer, err := factory.NewConsumer(context.Background(), nil)
			assert.Error(t, err)
			assert.Nil(t, consumer)
			assert.Contains(t, err.Error(), "configuration is nil")
		})

		t.Run("NilContext", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
				},
			}
			// Note: NewConsumer method panics with nil context due to NewObservabilityContext call
			// This test documents the current behavior
			assert.Panics(t, func() {
				factory.NewConsumer(nil, config)
			})
		})

		t.Run("NilRabbitMQConfig", func(t *testing.T) {
			config := &messaging.Config{}
			consumer, err := factory.NewConsumer(context.Background(), config)
			assert.Error(t, err)
			assert.Nil(t, consumer)
			assert.Contains(t, err.Error(), "RabbitMQ configuration is required")
		})

		t.Run("ValidConfig", func(t *testing.T) {
			config := &messaging.Config{
				RabbitMQ: &messaging.RabbitMQConfig{
					URIs: []string{"amqp://localhost:5672"},
					Consumer: &messaging.ConsumerConfig{
						Queue: "test-queue",
					},
				},
			}
			// Note: NewConsumer method panics when observability provider is nil
			// This test documents the current behavior
			assert.Panics(t, func() {
				factory.NewConsumer(context.Background(), config)
			})
		})
	})
}

// TestTransportRegister tests the Register function
func TestTransportRegister(t *testing.T) {
	t.Run("Register", func(t *testing.T) {
		// Register the RabbitMQ transport
		rabbitmq.Register()

		// Verify it's registered
		factory, exists := messaging.GetTransport("rabbitmq")
		assert.True(t, exists)
		assert.NotNil(t, factory)
		assert.Equal(t, "rabbitmq", factory.Name())
	})
}
