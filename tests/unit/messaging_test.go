package unit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

func TestMessage(t *testing.T) {
	t.Run("NewMessage", func(t *testing.T) {
		body := []byte(`{"test": "data"}`)
		msg := messaging.NewMessage(body)

		assert.NotNil(t, msg)
		assert.Equal(t, body, msg.Body)
		assert.NotEmpty(t, msg.ID)
		assert.NotZero(t, msg.Timestamp)
	})

	t.Run("MessageWithOptions", func(t *testing.T) {
		body := []byte(`{"test": "data"}`)
		timestamp := time.Now()

		msg := messaging.NewMessage(body,
			messaging.WithID("test-123"),
			messaging.WithContentType("application/json"),
			messaging.WithKey("test.key"),
			messaging.WithTimestamp(timestamp),
			messaging.WithHeader("custom", "value"),
		)

		assert.Equal(t, "test-123", msg.ID)
		assert.Equal(t, "application/json", msg.ContentType)
		assert.Equal(t, "test.key", msg.Key)
		assert.Equal(t, timestamp.Unix(), msg.Timestamp.Unix())
		assert.Equal(t, "value", msg.Headers["custom"])
	})

	t.Run("MessageSerialization", func(t *testing.T) {
		body := []byte(`{"test": "data"}`)
		msg := messaging.NewMessage(body,
			messaging.WithID("test-123"),
			messaging.WithContentType("application/json"),
		)

		// Test that message has expected properties
		assert.Equal(t, "test-123", msg.ID)
		assert.Equal(t, body, msg.Body)
		assert.Equal(t, "application/json", msg.ContentType)
	})
}

func TestTestMessageFactory(t *testing.T) {
	factory := messaging.NewTestMessageFactory()

	t.Run("CreateMessage", func(t *testing.T) {
		body := []byte(`{"test": "data"}`)
		msg := factory.CreateMessage(body)

		assert.NotNil(t, msg)
		assert.Equal(t, body, msg.Body)
		assert.Contains(t, msg.ID, "test-msg-")
		assert.Equal(t, "application/json", msg.ContentType)
	})

	t.Run("CreateJSONMessage", func(t *testing.T) {
		msg := factory.CreateJSONMessage("test-data")

		assert.NotNil(t, msg)
		assert.Contains(t, string(msg.Body), "test-data")
		assert.Contains(t, msg.ID, "test-msg-")
	})

	t.Run("CreateBulkMessages", func(t *testing.T) {
		count := 10
		messages := factory.CreateBulkMessages(count, `{"test": "message-%d"}`)

		assert.Len(t, messages, count)
		for i, msg := range messages {
			assert.Contains(t, string(msg.Body), "message-")
			assert.Contains(t, msg.ID, "test-msg-")

			// Ensure unique IDs
			for j, otherMsg := range messages {
				if i != j {
					assert.NotEqual(t, msg.ID, otherMsg.ID)
				}
			}
		}
	})
}

func TestMockTransport(t *testing.T) {
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)

	t.Run("Connect", func(t *testing.T) {
		ctx := context.Background()
		err := transport.Connect(ctx)
		assert.NoError(t, err)
		assert.True(t, transport.IsConnected())
	})

	t.Run("Disconnect", func(t *testing.T) {
		ctx := context.Background()
		err := transport.Connect(ctx)
		require.NoError(t, err)

		err = transport.Disconnect(ctx)
		assert.NoError(t, err)
		assert.False(t, transport.IsConnected())
	})

	t.Run("ConnectWithFailure", func(t *testing.T) {
		ctx := context.Background()
		config.FailureRate = 1.0 // Always fail

		err := transport.Connect(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "simulated connection failure")
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := transport.GetStats()
		assert.Contains(t, stats, "publishers")
		assert.Contains(t, stats, "consumers")
		assert.Contains(t, stats, "connected")
	})
}

func TestMockPublisher(t *testing.T) {
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)
	publisher := transport.NewPublisher(nil, config.Observability)

	t.Run("PublishAsync", func(t *testing.T) {
		ctx := context.Background()
		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")

		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		assert.NoError(t, err)
		assert.NotNil(t, receipt)
		assert.Equal(t, msg.ID, receipt.ID())

		// Wait for completion
		select {
		case <-receipt.Done():
			result, err := receipt.Result()
			assert.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, msg.ID, result.MessageID)
		case <-time.After(2 * time.Second):
			t.Fatal("Receipt not completed within timeout")
		}
	})

	t.Run("PublishAsyncWithFailure", func(t *testing.T) {
		ctx := context.Background()
		config.FailureRate = 1.0 // Always fail

		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")

		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "simulated publish failure")
	})

	t.Run("PublishAsyncClosed", func(t *testing.T) {
		ctx := context.Background()

		// Close publisher
		err := publisher.Close(ctx)
		assert.NoError(t, err)

		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")

		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "publisher is closed")
	})
}

func TestMockConsumer(t *testing.T) {
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)
	consumer := transport.NewConsumer(nil, config.Observability)

	t.Run("StartAndStop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var receivedMessages []messaging.Message
		var mu sync.Mutex
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			mu.Lock()
			receivedMessages = append(receivedMessages, delivery.Message)
			mu.Unlock()
			return messaging.Ack, nil
		})

		err := consumer.Start(ctx, handler)
		assert.NoError(t, err)

		// Let it run for a short time
		time.Sleep(500 * time.Millisecond)

		err = consumer.Stop(ctx)
		assert.NoError(t, err)

		// Should have received some messages
		mu.Lock()
		messageCount := len(receivedMessages)
		mu.Unlock()
		assert.NotEmpty(t, messageCount)
	})

	t.Run("SimulateMessage", func(t *testing.T) {
		ctx := context.Background()

		var receivedMsg messaging.Message
		var mu sync.Mutex
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			mu.Lock()
			receivedMsg = delivery.Message
			mu.Unlock()
			return messaging.Ack, nil
		})

		err := consumer.Start(ctx, handler)
		require.NoError(t, err)

		factory := messaging.NewTestMessageFactory()
		testMsg := factory.CreateJSONMessage("test-data")

		mockConsumer := consumer.(*messaging.MockConsumer)
		err = mockConsumer.SimulateMessage(testMsg)
		assert.NoError(t, err)

		// Wait for message processing
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		msg := receivedMsg
		mu.Unlock()
		assert.NotNil(t, msg)
		assert.Equal(t, testMsg.ID, msg.ID)

		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("StartAlreadyRunning", func(t *testing.T) {
		ctx := context.Background()
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			return messaging.Ack, nil
		})

		err := consumer.Start(ctx, handler)
		require.NoError(t, err)

		err = consumer.Start(ctx, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")

		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestMockDelivery(t *testing.T) {
	config := messaging.NewTestConfig()
	factory := messaging.NewTestMessageFactory()
	msg := factory.CreateJSONMessage("test-data")
	delivery := messaging.NewMockDelivery(msg, config)

	t.Run("Message", func(t *testing.T) {
		assert.Equal(t, msg, delivery.Message)
	})

	t.Run("Ack", func(t *testing.T) {
		err := delivery.Ack()
		assert.NoError(t, err)
		assert.True(t, delivery.IsAcked())
		assert.False(t, delivery.IsNacked())
		assert.False(t, delivery.IsRejected())
	})

	t.Run("Nack", func(t *testing.T) {
		config2 := messaging.NewTestConfig()
		delivery2 := messaging.NewMockDelivery(msg, config2)

		err := delivery2.Nack(true)
		assert.NoError(t, err)
		assert.False(t, delivery2.IsAcked())
		assert.True(t, delivery2.IsNacked())
		assert.False(t, delivery2.IsRejected())
	})

	t.Run("Reject", func(t *testing.T) {
		config3 := messaging.NewTestConfig()
		delivery3 := messaging.NewMockDelivery(msg, config3)

		err := delivery3.Nack(false)
		assert.NoError(t, err)
		assert.False(t, delivery3.IsAcked())
		assert.False(t, delivery3.IsNacked())
		assert.True(t, delivery3.IsRejected())
	})

	t.Run("DoubleAck", func(t *testing.T) {
		config4 := messaging.NewTestConfig()
		delivery4 := messaging.NewMockDelivery(msg, config4)

		err := delivery4.Ack()
		require.NoError(t, err)

		err = delivery4.Ack()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already acknowledged")
	})
}

func TestTestRunner(t *testing.T) {
	config := messaging.NewTestConfig()
	runner := messaging.NewTestRunner(config)
	transport := messaging.NewMockTransport(config)

	t.Run("RunPublishTest", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		publisher := transport.NewPublisher(nil, config.Observability)
		messageCount := 10

		result, err := runner.RunPublishTest(ctx, publisher, messageCount)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint64(messageCount), result.MessageCount)
		assert.Equal(t, uint64(messageCount), result.SuccessCount)
		assert.Equal(t, uint64(0), result.ErrorCount)
		assert.Equal(t, 100.0, result.SuccessRate())
		assert.Equal(t, 0.0, result.ErrorRate())
		assert.Greater(t, result.Throughput, 0.0)
	})

	t.Run("RunPublishTestWithErrors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		config.FailureRate = 0.5 // 50% failure rate
		publisher := transport.NewPublisher(nil, config.Observability)
		messageCount := 10

		result, err := runner.RunPublishTest(ctx, publisher, messageCount)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint64(messageCount), result.MessageCount)
		assert.Greater(t, result.ErrorCount, uint64(0))
		assert.Less(t, result.SuccessRate(), 100.0)
		assert.Greater(t, result.ErrorRate(), 0.0)
	})
}

func TestObservabilityIntegration(t *testing.T) {
	config := messaging.NewTestConfig()

	t.Run("ObservabilityProvider", func(t *testing.T) {
		assert.NotNil(t, config.Observability)
		assert.NotNil(t, config.Observability.Logger())
		assert.NotNil(t, config.Observability.Tracer())
	})

	t.Run("MetricsRecording", func(t *testing.T) {
		// Test that observability context can record metrics without errors
		config.Observability.RecordPublishMetrics("test", "test.exchange", time.Millisecond, true, "")
		config.Observability.RecordConsumeMetrics("test", "test.queue", time.Millisecond, true, "")
		config.Observability.RecordConnectionMetrics("test", 1, 1, 0)

		// No assertion needed - just ensure no panics
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("MessagingError", func(t *testing.T) {
		err := messaging.NewError(messaging.ErrorCodePublish, "test", "test error")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")

		var msgErr *messaging.MessagingError
		ok := errors.As(err, &msgErr)
		assert.True(t, ok)
		assert.Equal(t, messaging.ErrorCodePublish, msgErr.Code)
		assert.Equal(t, "test", msgErr.Operation)
		assert.Equal(t, "test error", msgErr.Message)
	})

	t.Run("WrappedError", func(t *testing.T) {
		originalErr := assert.AnError
		wrappedErr := messaging.WrapError(messaging.ErrorCodeInternal, "test", "wrapped", originalErr)

		assert.Error(t, wrappedErr)
		assert.Contains(t, wrappedErr.Error(), "wrapped")

		var msgErr *messaging.MessagingError
		ok := errors.As(wrappedErr, &msgErr)
		assert.True(t, ok)
		assert.Equal(t, messaging.ErrorCodeInternal, msgErr.Code)
		assert.Equal(t, originalErr, msgErr.Cause)
	})
}
