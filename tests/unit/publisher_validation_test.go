package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

// TestValidationHelperFunctions tests the validation helper functions used in publisher.go
// Since the validation functions are private, we test them indirectly through the publisher
func TestValidationHelperFunctions(t *testing.T) {
	// Create a publisher for testing validation
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

	t.Run("TopicNameValidation", func(t *testing.T) {
		// Test valid topic names
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
			"TOPIC",
			"Topic",
			"topic123",
			"123topic",
		}

		for _, topic := range validTopics {
			t.Run(fmt.Sprintf("Valid_%s", topic), func(t *testing.T) {
				msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
				_, err := publisher.PublishAsync(context.Background(), topic, *msg)
				// Should not fail on topic validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid topic name")
					assert.NotContains(t, err.Error(), "topic cannot be empty")
					assert.NotContains(t, err.Error(), "topic too long")
				}
			})
		}

		// Test invalid topic names
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
			t.Run(fmt.Sprintf("Invalid_%s", topic), func(t *testing.T) {
				msg := messaging.NewMessage([]byte("test"), messaging.WithKey("test.key"))
				receipt, err := publisher.PublishAsync(context.Background(), topic, *msg)
				assert.Error(t, err)
				assert.Nil(t, receipt)
				assert.Contains(t, err.Error(), "invalid topic name")
			})
		}
	})

	t.Run("MessageIDValidation", func(t *testing.T) {
		// Test valid message IDs
		validIDs := []string{
			"valid-id",
			"valid_id",
			"valid123id",
			"id123",
			"123id",
			"id123name456",
			"a",
			"ab",
			"abc",
			"id",
			"ID",
			"Id",
			"id123",
			"123id",
		}

		for _, id := range validIDs {
			t.Run(fmt.Sprintf("Valid_%s", id), func(t *testing.T) {
				msg := messaging.Message{
					ID:          id,
					Key:         "test.key",
					Body:        []byte("test"),
					ContentType: "application/json",
				}
				_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
				// Should not fail on message ID validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid message ID format")
					assert.NotContains(t, err.Error(), "message ID cannot be empty")
					assert.NotContains(t, err.Error(), "message ID too long")
				}
			})
		}

		// Test invalid message IDs
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
			t.Run(fmt.Sprintf("Invalid_%s", id), func(t *testing.T) {
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

	t.Run("RoutingKeyValidation", func(t *testing.T) {
		// Test valid routing keys
		validKeys := []string{
			"valid.key",
			"valid-key",
			"valid_key",
			"valid123key",
			"key123",
			"123key",
			"key.name.with.dots",
			"key-name-with-dashes",
			"key_name_with_underscores",
			"key123name456",
			"a",
			"ab",
			"abc",
			"key",
			"KEY",
			"Key",
			"key123",
			"123key",
		}

		for _, key := range validKeys {
			t.Run(fmt.Sprintf("Valid_%s", key), func(t *testing.T) {
				msg := messaging.Message{
					ID:          "test-id",
					Key:         key,
					Body:        []byte("test"),
					ContentType: "application/json",
				}
				_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
				// Should not fail on routing key validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid routing key format")
					assert.NotContains(t, err.Error(), "routing key cannot be empty")
					assert.NotContains(t, err.Error(), "routing key too long")
				}
			})
		}

		// Test invalid routing keys
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
			t.Run(fmt.Sprintf("Invalid_%s", key), func(t *testing.T) {
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

	t.Run("ContentTypeValidation", func(t *testing.T) {
		// Test valid content types
		validContentTypes := messaging.SupportedContentTypes()
		for _, contentType := range validContentTypes {
			t.Run(fmt.Sprintf("Valid_%s", contentType), func(t *testing.T) {
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

		// Test invalid content types
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
			t.Run(fmt.Sprintf("Invalid_%s", contentType), func(t *testing.T) {
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

	t.Run("CorrelationIDValidation", func(t *testing.T) {
		// Test valid correlation IDs
		validCorrelationIDs := []string{
			"valid-id",
			"valid_id",
			"valid123id",
			"id123",
			"123id",
			"id123name456",
			"a",
			"ab",
			"abc",
			"id",
			"ID",
			"Id",
			"id123",
			"123id",
		}

		for _, id := range validCorrelationIDs {
			t.Run(fmt.Sprintf("Valid_%s", id), func(t *testing.T) {
				msg := messaging.Message{
					ID:            "test-id",
					Key:           "test.key",
					Body:          []byte("test"),
					ContentType:   "application/json",
					CorrelationID: id,
				}
				_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
				// Should not fail on correlation ID validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid correlation ID format")
					assert.NotContains(t, err.Error(), "correlation ID too long")
				}
			})
		}

		// Test invalid correlation IDs
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
			t.Run(fmt.Sprintf("Invalid_%s", id), func(t *testing.T) {
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

	t.Run("ReplyToValidation", func(t *testing.T) {
		// Test valid reply-to values
		validReplyTos := []string{
			"valid.reply",
			"valid-reply",
			"valid_reply",
			"valid123reply",
			"reply123",
			"123reply",
			"reply.name.with.dots",
			"reply-name-with-dashes",
			"reply_name_with_underscores",
			"reply123name456",
			"a",
			"ab",
			"abc",
			"reply",
			"REPLY",
			"Reply",
			"reply123",
			"123reply",
		}

		for _, replyTo := range validReplyTos {
			t.Run(fmt.Sprintf("Valid_%s", replyTo), func(t *testing.T) {
				msg := messaging.Message{
					ID:          "test-id",
					Key:         "test.key",
					Body:        []byte("test"),
					ContentType: "application/json",
					ReplyTo:     replyTo,
				}
				_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
				// Should not fail on reply-to validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid reply-to format")
					assert.NotContains(t, err.Error(), "reply-to too long")
				}
			})
		}

		// Test invalid reply-to values
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
			t.Run(fmt.Sprintf("Invalid_%s", replyTo), func(t *testing.T) {
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

	t.Run("IdempotencyKeyValidation", func(t *testing.T) {
		// Test valid idempotency keys
		validIdempotencyKeys := []string{
			"valid-key",
			"valid_key",
			"valid123key",
			"key123",
			"123key",
			"key123name456",
			"a",
			"ab",
			"abc",
			"key",
			"KEY",
			"Key",
			"key123",
			"123key",
		}

		for _, key := range validIdempotencyKeys {
			t.Run(fmt.Sprintf("Valid_%s", key), func(t *testing.T) {
				msg := messaging.Message{
					ID:             "test-id",
					Key:            "test.key",
					Body:           []byte("test"),
					ContentType:    "application/json",
					IdempotencyKey: key,
				}
				_, err := publisher.PublishAsync(context.Background(), "test.topic", msg)
				// Should not fail on idempotency key validation, but may fail on transport
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid idempotency key format")
					assert.NotContains(t, err.Error(), "idempotency key too long")
				}
			})
		}

		// Test invalid idempotency keys
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
			t.Run(fmt.Sprintf("Invalid_%s", key), func(t *testing.T) {
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
}
