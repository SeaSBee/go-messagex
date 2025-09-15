package unit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/SeaSBee/go-messagex/pkg/rabbitmq"
)

// TestNewPublisherComprehensive tests the NewPublisher constructor comprehensively
func TestNewPublisherComprehensive(t *testing.T) {
	t.Run("ValidParameters", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{
			MaxInFlight:    1000,
			WorkerCount:    4,
			DropOnOverflow: false,
			PublishTimeout: 5 * time.Second,
		}

		// Create publisher
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		assert.NoError(t, err)
		assert.NotNil(t, publisher)
	})

	t.Run("NilTransport", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create config
		config := &messaging.PublisherConfig{}

		// Should return error with nil transport
		publisher, err := rabbitmq.NewPublisher(nil, config, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Contains(t, err.Error(), "transport cannot be nil")
	})

	t.Run("NilConfig", func(t *testing.T) {
		// Create observability context
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Should return error with nil config
		publisher, err := rabbitmq.NewPublisher(transport, nil, obsCtx)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("NilObservability", func(t *testing.T) {
		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, nil)

		// Create config
		config := &messaging.PublisherConfig{}

		// Should return error with nil observability
		publisher, err := rabbitmq.NewPublisher(transport, config, nil)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Contains(t, err.Error(), "observability cannot be nil")
	})

	t.Run("AsyncPublisherCreationFailure", func(t *testing.T) {
		// This test would require mocking the async publisher creation
		// For now, we test the basic error handling structure
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
			MetricsEnabled: false,
			TracingEnabled: false,
		})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)

		// Create transport
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)

		// Create config
		config := &messaging.PublisherConfig{}

		// Create publisher
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		assert.NoError(t, err)
		assert.NotNil(t, publisher)
	})
}

// TestPublisherPublishAsyncComprehensive tests the PublishAsync method comprehensively
func TestPublisherPublishAsyncComprehensive(t *testing.T) {
	t.Run("NilPublisher", func(t *testing.T) {
		var publisher *rabbitmq.Publisher
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "test.topic", *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "publisher is nil")
	})

	t.Run("ClosedPublisher", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Close publisher
		err = publisher.Close(context.Background())
		assert.NoError(t, err)

		// Try to publish
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "test.topic", *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "publisher is closed")
	})

	t.Run("NilContext", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish with nil context
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(nil, "test.topic", *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("EmptyTopic", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Try to publish with empty topic
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "", *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "topic cannot be empty")
	})

	t.Run("TopicTooLong", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Create topic longer than MaxTopicLength
		longTopic := strings.Repeat("a", messaging.MaxTopicLength+1)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), longTopic, *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "topic too long")
	})

	t.Run("InvalidTopicName", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test various invalid topic names
		invalidTopics := []string{
			"invalid@topic",
			"invalid#topic",
			"invalid*topic",
			"invalid&topic",
			"invalid!topic",
			"invalid%topic",
			"invalid+topic",
			"invalid=topic",
			"invalid/topic",
			"invalid\\topic",
			"invalid|topic",
			"invalid<topic",
			"invalid>topic",
			"invalid[topic",
			"invalid]topic",
			"invalid{topic",
			"invalid}topic",
			"invalid`topic",
			"invalid~topic",
			"invalid^topic",
		}

		for _, topic := range invalidTopics {
			t.Run(fmt.Sprintf("Topic_%s", topic), func(t *testing.T) {
				msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
				receipt, err := publisher.PublishAsync(context.Background(), topic, *msg)
				assert.Error(t, err)
				assert.Nil(t, receipt)
				assert.Contains(t, err.Error(), "invalid topic name")
			})
		}
	})

	t.Run("ValidTopicNames", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test various valid topic names
		validTopics := []string{
			"valid.topic",
			"valid-topic",
			"valid_topic",
			"valid123topic",
			"topic123",
			"123topic",
			"topic.name.with.dots",
			"topic-name-with-dashes",
			"topic_name_with_underscores",
			"topic123name456",
			"a",
			"ab",
			"abc",
			"topic",
		}

		for _, topic := range validTopics {
			t.Run(fmt.Sprintf("Topic_%s", topic), func(t *testing.T) {
				msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
				_, err := publisher.PublishAsync(context.Background(), topic, *msg)
				// Should not fail on topic validation, but may fail on transport
				if err != nil {
					// Transport error is expected, but topic validation should pass
					assert.NotContains(t, err.Error(), "invalid topic name")
					assert.NotContains(t, err.Error(), "topic cannot be empty")
					assert.NotContains(t, err.Error(), "topic too long")
				}
			})
		}
	})

	t.Run("MessageValidation", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		t.Run("EmptyMessageID", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "", // Empty ID
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "message ID cannot be empty")
		})

		t.Run("MessageIDTooLong", func(t *testing.T) {
			longID := strings.Repeat("a", messaging.MaxMessageIDLength+1)
			msg := messaging.Message{
				ID:          longID,
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "message ID too long")
		})

		t.Run("InvalidMessageID", func(t *testing.T) {
			invalidIDs := []string{
				"invalid@id",
				"invalid#id",
				"invalid*id",
				"invalid&id",
				"invalid!id",
				"invalid%id",
				"invalid+id",
				"invalid=id",
				"invalid/id",
				"invalid\\id",
				"invalid|id",
				"invalid<id",
				"invalid>id",
				"invalid[id",
				"invalid]id",
				"invalid{id",
				"invalid}id",
				"invalid`id",
				"invalid~id",
				"invalid^id",
			}

			for _, id := range invalidIDs {
				t.Run(fmt.Sprintf("ID_%s", id), func(t *testing.T) {
					msg := messaging.Message{
						ID:          id,
						Key:         "test.key",
						Body:        []byte("test"),
						ContentType: "application/json",
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "invalid message ID format")
				})
			}
		})

		t.Run("EmptyMessageBody", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte{}, // Empty body
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "message body cannot be empty")
		})

		t.Run("MessageTooLarge", func(t *testing.T) {
			largeBody := make([]byte, messaging.MaxMessageSize+1)
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        largeBody,
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "message too large")
		})

		t.Run("EmptyRoutingKey", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "", // Empty routing key
				Body:        []byte("test"),
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "routing key cannot be empty")
		})

		t.Run("RoutingKeyTooLong", func(t *testing.T) {
			longKey := strings.Repeat("a", messaging.MaxRoutingKeyLength+1)
			msg := messaging.Message{
				ID:          "test-id",
				Key:         longKey,
				Body:        []byte("test"),
				ContentType: "application/json",
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "routing key too long")
		})

		t.Run("InvalidRoutingKey", func(t *testing.T) {
			invalidKeys := []string{
				"invalid@key",
				"invalid#key",
				"invalid*key",
				"invalid&key",
				"invalid!key",
				"invalid%key",
				"invalid+key",
				"invalid=key",
				"invalid/key",
				"invalid\\key",
				"invalid|key",
				"invalid<key",
				"invalid>key",
				"invalid[key",
				"invalid]key",
				"invalid{key",
				"invalid}key",
				"invalid`key",
				"invalid~key",
				"invalid^key",
			}

			for _, key := range invalidKeys {
				t.Run(fmt.Sprintf("Key_%s", key), func(t *testing.T) {
					msg := messaging.Message{
						ID:          "test-id",
						Key:         key,
						Body:        []byte("test"),
						ContentType: "application/json",
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "invalid routing key format")
				})
			}
		})

		t.Run("EmptyContentType", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "", // Empty content type
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "content type cannot be empty")
		})

		t.Run("InvalidContentType", func(t *testing.T) {
			invalidContentTypes := []string{
				"invalid/type",
				"image/jpeg",
				"video/mp4",
				"audio/mpeg",
				"multipart/form-data",
				"application/pdf",
				"text/css",
				"application/javascript",
			}

			for _, contentType := range invalidContentTypes {
				t.Run(fmt.Sprintf("ContentType_%s", contentType), func(t *testing.T) {
					msg := messaging.Message{
						ID:          "test-id",
						Key:         "test.key",
						Body:        []byte("test"),
						ContentType: contentType,
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "unsupported content type")
				})
			}
		})

		t.Run("ValidContentTypes", func(t *testing.T) {
			validContentTypes := messaging.SupportedContentTypes()
			for _, contentType := range validContentTypes {
				t.Run(fmt.Sprintf("ContentType_%s", contentType), func(t *testing.T) {
					msg := messaging.Message{
						ID:          "test-id",
						Key:         "test.key",
						Body:        []byte("test"),
						ContentType: contentType,
					}
					_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					// Should not fail on content type validation, but may fail on transport
					if err != nil {
						assert.NotContains(t, err.Error(), "unsupported content type")
						assert.NotContains(t, err.Error(), "content type cannot be empty")
					}
				})
			}
		})

		t.Run("TooManyHeaders", func(t *testing.T) {
			headers := make(map[string]string, 101)
			for i := 0; i < 101; i++ {
				headers[fmt.Sprintf("header-%d", i)] = "value"
			}
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Headers:     headers,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "too many headers")
		})

		t.Run("HeaderKeyTooLong", func(t *testing.T) {
			longKey := strings.Repeat("a", 256)
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Headers: map[string]string{
					longKey: "value",
				},
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "header key too long")
		})

		t.Run("HeaderValueTooLarge", func(t *testing.T) {
			largeValue := strings.Repeat("a", messaging.MaxHeaderSize+1)
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Headers: map[string]string{
					"header": largeValue,
				},
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "header value too large")
		})

		t.Run("PriorityTooHigh", func(t *testing.T) {
			// Since MaxPriority is 255 (uint8), we can't test overflow directly
			// Instead, we test that the validation logic works correctly
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Priority:    messaging.MaxPriority, // Use max valid priority
			}
			_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			// Should not fail on priority validation, but may fail on transport
			if err != nil {
				assert.NotContains(t, err.Error(), "priority too high")
			}
		})

		t.Run("CorrelationIDTooLong", func(t *testing.T) {
			longCorrelationID := strings.Repeat("a", messaging.MaxCorrelationIDLength+1)
			msg := messaging.Message{
				ID:            "test-id",
				Key:           "test.key",
				Body:          []byte("test"),
				ContentType:   "application/json",
				CorrelationID: longCorrelationID,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "correlation ID too long")
		})

		t.Run("InvalidCorrelationID", func(t *testing.T) {
			invalidCorrelationIDs := []string{
				"invalid@id",
				"invalid#id",
				"invalid*id",
				"invalid&id",
				"invalid!id",
				"invalid%id",
				"invalid+id",
				"invalid=id",
				"invalid/id",
				"invalid\\id",
				"invalid|id",
				"invalid<id",
				"invalid>id",
				"invalid[id",
				"invalid]id",
				"invalid{id",
				"invalid}id",
				"invalid`id",
				"invalid~id",
				"invalid^id",
			}

			for _, id := range invalidCorrelationIDs {
				t.Run(fmt.Sprintf("CorrelationID_%s", id), func(t *testing.T) {
					msg := messaging.Message{
						ID:            "test-id",
						Key:           "test.key",
						Body:          []byte("test"),
						ContentType:   "application/json",
						CorrelationID: id,
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "invalid correlation ID format")
				})
			}
		})

		t.Run("ReplyToTooLong", func(t *testing.T) {
			longReplyTo := strings.Repeat("a", messaging.MaxReplyToLength+1)
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				ReplyTo:     longReplyTo,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "reply-to too long")
		})

		t.Run("InvalidReplyTo", func(t *testing.T) {
			invalidReplyTos := []string{
				"invalid@reply",
				"invalid#reply",
				"invalid*reply",
				"invalid&reply",
				"invalid!reply",
				"invalid%reply",
				"invalid+reply",
				"invalid=reply",
				"invalid/reply",
				"invalid\\reply",
				"invalid|reply",
				"invalid<reply",
				"invalid>reply",
				"invalid[reply",
				"invalid]reply",
				"invalid{reply",
				"invalid}reply",
				"invalid`reply",
				"invalid~reply",
				"invalid^reply",
			}

			for _, replyTo := range invalidReplyTos {
				t.Run(fmt.Sprintf("ReplyTo_%s", replyTo), func(t *testing.T) {
					msg := messaging.Message{
						ID:          "test-id",
						Key:         "test.key",
						Body:        []byte("test"),
						ContentType: "application/json",
						ReplyTo:     replyTo,
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "invalid reply-to format")
				})
			}
		})

		t.Run("IdempotencyKeyTooLong", func(t *testing.T) {
			longIdempotencyKey := strings.Repeat("a", messaging.MaxIdempotencyKeyLength+1)
			msg := messaging.Message{
				ID:             "test-id",
				Key:            "test.key",
				Body:           []byte("test"),
				ContentType:    "application/json",
				IdempotencyKey: longIdempotencyKey,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "idempotency key too long")
		})

		t.Run("InvalidIdempotencyKey", func(t *testing.T) {
			invalidIdempotencyKeys := []string{
				"invalid@key",
				"invalid#key",
				"invalid*key",
				"invalid&key",
				"invalid!key",
				"invalid%key",
				"invalid+key",
				"invalid=key",
				"invalid/key",
				"invalid\\key",
				"invalid|key",
				"invalid<key",
				"invalid>key",
				"invalid[key",
				"invalid]key",
				"invalid{key",
				"invalid}key",
				"invalid`key",
				"invalid~key",
				"invalid^key",
			}

			for _, key := range invalidIdempotencyKeys {
				t.Run(fmt.Sprintf("IdempotencyKey_%s", key), func(t *testing.T) {
					msg := messaging.Message{
						ID:             "test-id",
						Key:            "test.key",
						Body:           []byte("test"),
						ContentType:    "application/json",
						IdempotencyKey: key,
					}
					receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
					assert.Error(t, err)
					assert.Nil(t, receipt)
					assert.Contains(t, err.Error(), "invalid idempotency key format")
				})
			}
		})

		t.Run("NegativeExpiration", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Expiration:  -1 * time.Second,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "expiration cannot be negative")
		})

		t.Run("ExpirationTooLong", func(t *testing.T) {
			msg := messaging.Message{
				ID:          "test-id",
				Key:         "test.key",
				Body:        []byte("test"),
				ContentType: "application/json",
				Expiration:  messaging.MaxTimeout + time.Second,
			}
			receipt, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
			assert.Error(t, err)
			assert.Nil(t, receipt)
			assert.Contains(t, err.Error(), "expiration too long")
		})
	})
}

// TestPublisherCloseComprehensive tests the Close method comprehensively
func TestPublisherCloseComprehensive(t *testing.T) {
	t.Run("NilPublisher", func(t *testing.T) {
		var publisher *rabbitmq.Publisher
		err := publisher.Close(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "publisher is nil")
	})

	t.Run("NilContext", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Try to close with nil context
		err = publisher.Close(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("DoubleClose", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Close publisher twice
		err1 := publisher.Close(context.Background())
		err2 := publisher.Close(context.Background())

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should not error on second close
	})

	t.Run("CloseWithTimeout", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Close with timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = publisher.Close(timeoutCtx)
		assert.NoError(t, err)
	})

	t.Run("CloseAfterPublish", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Try to publish a message (will fail due to transport not connected)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Transport error expected

		// Close publisher
		err = publisher.Close(context.Background())
		assert.NoError(t, err)
	})
}

// TestPublisherConcurrency tests concurrent operations on the publisher
func TestPublisherConcurrency(t *testing.T) {
	t.Run("ConcurrentPublish", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test concurrent publish operations
		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
				_, err := publisher.PublishAsync(context.Background(), "test.topic", *msg)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// All errors should be transport-related, not validation errors
		for err := range errors {
			assert.NotContains(t, err.Error(), "invalid topic name")
			assert.NotContains(t, err.Error(), "topic cannot be empty")
			assert.NotContains(t, err.Error(), "topic too long")
		}
	})

	t.Run("ConcurrentClose", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Test concurrent close operations
		const numGoroutines = 5
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := publisher.Close(context.Background())
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Should not have any errors from concurrent close
		for err := range errors {
			assert.NoError(t, err)
		}
	})

	t.Run("PublishAfterClose", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)

		// Close publisher
		err = publisher.Close(context.Background())
		assert.NoError(t, err)

		// Try to publish after close
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		receipt, err := publisher.PublishAsync(context.Background(), "test.topic", *msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "publisher is closed")
	})
}

// TestPublisherIntegration tests integration scenarios
func TestPublisherIntegration(t *testing.T) {
	t.Run("ValidMessagePublish", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Create a valid message with all fields
		msg := messaging.Message{
			ID:          "test-message-123",
			Key:         "test.routing.key",
			Body:        []byte(`{"test": "data"}`),
			ContentType: "application/json",
			Headers: map[string]string{
				"header1": "value1",
				"header2": "value2",
			},
			Priority:       10,
			CorrelationID:  "corr-123",
			ReplyTo:        "reply.queue",
			IdempotencyKey: "idemp-123",
			Expiration:     30 * time.Second,
		}

		// Try to publish (will fail due to transport not connected, but validates the flow)
		receipt, err := publisher.PublishAsync(context.Background(), "test.exchange", msg)
		if err != nil {
			// Transport error expected, but validation should pass
			assert.NotContains(t, err.Error(), "invalid topic name")
			assert.NotContains(t, err.Error(), "message ID cannot be empty")
			assert.NotContains(t, err.Error(), "routing key cannot be empty")
			assert.NotContains(t, err.Error(), "content type cannot be empty")
		} else {
			// Success case - receipt should be valid
			assert.NotNil(t, receipt)
			assert.Equal(t, msg.ID, receipt.ID())
		}
	})

	t.Run("BoundaryValues", func(t *testing.T) {
		// Create publisher
		provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
		require.NoError(t, err)
		obsCtx := messaging.NewObservabilityContext(context.Background(), provider)
		transport := rabbitmq.NewTransport(&messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672"},
		}, obsCtx)
		config := &messaging.PublisherConfig{}
		publisher, err := rabbitmq.NewPublisher(transport, config, obsCtx)
		require.NoError(t, err)
		defer publisher.Close(context.Background())

		// Test boundary values for topic length
		maxTopic := strings.Repeat("a", messaging.MaxTopicLength)
		msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
		_, err = publisher.PublishAsync(context.Background(), maxTopic, *msg)
		// Should not fail on topic validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "topic too long")
		}

		// Test boundary values for message ID length
		maxID := strings.Repeat("a", messaging.MaxMessageIDLength)
		msg.ID = maxID
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail on message ID validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "message ID too long")
		}

		// Test boundary values for routing key length
		maxKey := strings.Repeat("a", messaging.MaxRoutingKeyLength)
		msg.Key = maxKey
		_, err = publisher.PublishAsync(context.Background(), "test.topic", *msg)
		// Should not fail on routing key validation, but may fail on transport
		if err != nil {
			assert.NotContains(t, err.Error(), "routing key too long")
		}
	})
}
