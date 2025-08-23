package unit

import (
	"context"
	"testing"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
)

func TestRabbitMQTransport(t *testing.T) {
	t.Run("NewTransport", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		assert.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewTransport(config, obsCtx)
		assert.NotNil(t, transport)
	})

	t.Run("NewPublisher", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		assert.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewTransport(config, obsCtx)

		publisherConfig := &messaging.PublisherConfig{}
		publisher := rabbitmq.NewPublisher(transport, publisherConfig, obsCtx)
		assert.NotNil(t, publisher)
	})

	t.Run("NewConsumer", func(t *testing.T) {
		config := &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}
		obsProvider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		assert.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

		transport := rabbitmq.NewTransport(config, obsCtx)

		consumerConfig := &messaging.ConsumerConfig{
			Queue: "test.queue",
		}
		consumer := rabbitmq.NewConsumer(transport, consumerConfig, obsCtx)
		assert.NotNil(t, consumer)
	})
}
