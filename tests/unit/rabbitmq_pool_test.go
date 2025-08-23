package unit

import (
	"context"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionPool(t *testing.T) {
	t.Run("NewConnectionPool", func(t *testing.T) {
		config := &messaging.ConnectionPoolConfig{
			Min:                 2,
			Max:                 8,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		}

		logger := messaging.NoOpLogger()
		metrics := messaging.NoOpMetrics{}

		pool := rabbitmq.NewConnectionPool(config, logger, metrics)
		assert.NotNil(t, pool)
	})

	t.Run("GetConnection", func(t *testing.T) {
		config := &messaging.ConnectionPoolConfig{
			Min:                 1,
			Max:                 2,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		}

		logger := messaging.NoOpLogger()
		metrics := messaging.NoOpMetrics{}

		pool := rabbitmq.NewConnectionPool(config, logger, metrics)

		// Test getting connection (will fail without real RabbitMQ)
		ctx := context.Background()
		conn, err := pool.GetConnection(ctx, "amqp://localhost:5672")

		// Should fail without real RabbitMQ, but pool should be created
		assert.Error(t, err)
		assert.Nil(t, conn)
	})

	t.Run("ConnectionPoolStats", func(t *testing.T) {
		config := &messaging.ConnectionPoolConfig{
			Min:                 1,
			Max:                 5,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		}

		logger := messaging.NoOpLogger()
		metrics := messaging.NoOpMetrics{}

		pool := rabbitmq.NewConnectionPool(config, logger, metrics)

		stats := pool.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, 0, stats["total_connections"])
		assert.Equal(t, 5, stats["max_connections"])
		assert.Equal(t, 1, stats["min_connections"])
		assert.False(t, stats["closed"].(bool))
	})

	t.Run("ConnectionPoolClose", func(t *testing.T) {
		config := &messaging.ConnectionPoolConfig{
			Min:                 1,
			Max:                 2,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		}

		logger := messaging.NoOpLogger()
		metrics := messaging.NoOpMetrics{}

		pool := rabbitmq.NewConnectionPool(config, logger, metrics)

		// Close the pool
		err := pool.Close()
		assert.NoError(t, err)

		// Verify pool is closed
		stats := pool.GetStats()
		assert.True(t, stats["closed"].(bool))
	})
}

func TestChannelPool(t *testing.T) {
	t.Run("NewChannelPool", func(t *testing.T) {
		config := &messaging.ChannelPoolConfig{
			PerConnectionMin:    5,
			PerConnectionMax:    20,
			BorrowTimeout:       5 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}

		// Create a mock connection (will be nil but sufficient for testing structure)
		var conn *amqp091.Connection

		pool := rabbitmq.NewChannelPool(config, conn)
		assert.NotNil(t, pool)
	})

	t.Run("ChannelPoolInitialize", func(t *testing.T) {
		config := &messaging.ChannelPoolConfig{
			PerConnectionMin:    2,
			PerConnectionMax:    10,
			BorrowTimeout:       5 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}

		var conn *amqp091.Connection
		pool := rabbitmq.NewChannelPool(config, conn)

		// Initialize should panic with nil connection
		assert.Panics(t, func() {
			pool.Initialize()
		})
	})

	t.Run("ChannelPoolBorrow", func(t *testing.T) {
		config := &messaging.ChannelPoolConfig{
			PerConnectionMin:    1,
			PerConnectionMax:    5,
			BorrowTimeout:       1 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}

		var conn *amqp091.Connection
		pool := rabbitmq.NewChannelPool(config, conn)

		ctx := context.Background()
		channel, err := pool.Borrow(ctx)

		// Should timeout without available channels
		assert.Error(t, err)
		assert.Nil(t, channel)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("ChannelPoolClose", func(t *testing.T) {
		config := &messaging.ChannelPoolConfig{
			PerConnectionMin:    1,
			PerConnectionMax:    5,
			BorrowTimeout:       5 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}

		var conn *amqp091.Connection
		pool := rabbitmq.NewChannelPool(config, conn)

		err := pool.Close()
		assert.NoError(t, err)
	})
}

func TestPooledTransport(t *testing.T) {
	t.Run("NewPooledTransport", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			ChannelPool: &messaging.ChannelPoolConfig{
				PerConnectionMin:    5,
				PerConnectionMax:    20,
				BorrowTimeout:       5 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewPooledTransport(config, obsCtx)
		assert.NotNil(t, transport)
	})

	t.Run("PooledTransportGetChannel", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			ChannelPool: &messaging.ChannelPoolConfig{
				PerConnectionMin:    5,
				PerConnectionMax:    20,
				BorrowTimeout:       5 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewPooledTransport(config, obsCtx)

		ctx := context.Background()
		channel, err := transport.GetChannel(ctx)

		// Should fail without real RabbitMQ
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("PooledTransportClose", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 1,
				Max:                 5,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			ChannelPool: &messaging.ChannelPoolConfig{
				PerConnectionMin:    5,
				PerConnectionMax:    20,
				BorrowTimeout:       5 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
		}

		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewPooledTransport(config, obsCtx)

		ctx := context.Background()
		err = transport.Close(ctx)
		assert.NoError(t, err)
	})
}

func TestConnectionState(t *testing.T) {
	t.Run("ConnectionStateString", func(t *testing.T) {
		states := []rabbitmq.ConnectionState{
			rabbitmq.ConnectionStateUnknown,
			rabbitmq.ConnectionStateConnecting,
			rabbitmq.ConnectionStateConnected,
			rabbitmq.ConnectionStateDisconnected,
			rabbitmq.ConnectionStateFailed,
			rabbitmq.ConnectionStateClosed,
		}

		for _, state := range states {
			str := state.String()
			assert.NotEmpty(t, str)
		}
	})
}

func TestConnectionLifecycle(t *testing.T) {
	t.Run("ConnectionLifecycleEvent", func(t *testing.T) {
		event := rabbitmq.ConnectionLifecycleEvent{
			Event:     "test",
			State:     rabbitmq.ConnectionStateConnected,
			Timestamp: time.Now(),
		}

		assert.Equal(t, "test", event.Event)
		assert.Equal(t, rabbitmq.ConnectionStateConnected, event.State)
		assert.False(t, event.Timestamp.IsZero())
	})
}
