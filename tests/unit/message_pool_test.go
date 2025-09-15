package unit

import (
	"fmt"
	"testing"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

func TestMessageObjectPooling(t *testing.T) {
	t.Run("NewMessageUsesPool", func(t *testing.T) {
		// Create a message using the pool
		msg1 := messaging.NewMessage([]byte("test message"), messaging.WithKey("test.key"))
		assert.NotNil(t, msg1)
		assert.Equal(t, "test message", string(msg1.Body))
		assert.Equal(t, "test.key", msg1.Key)

		// Return to pool
		msg1.ReturnToPool()

		// Create another message - should reuse from pool
		msg2 := messaging.NewMessage([]byte("another message"), messaging.WithKey("another.key"))
		assert.NotNil(t, msg2)
		assert.Equal(t, "another message", string(msg2.Body))
		assert.Equal(t, "another.key", msg2.Key)

		// Return to pool
		msg2.ReturnToPool()
	})

	t.Run("PooledMessageReset", func(t *testing.T) {
		// Create message with various fields
		msg := messaging.NewMessage(
			[]byte("test body"),
			messaging.WithID("test-id"),
			messaging.WithKey("test.key"),
			messaging.WithContentType("text/plain"),
			messaging.WithPriority(5),
			messaging.WithCorrelationID("corr-123"),
			messaging.WithHeaders(map[string]string{"header1": "value1"}),
		)

		// Verify message is properly set
		assert.Equal(t, "test-id", msg.ID)
		assert.Equal(t, "test.key", msg.Key)
		assert.Equal(t, "text/plain", msg.ContentType)
		assert.Equal(t, uint8(5), msg.Priority)
		assert.Equal(t, "corr-123", msg.CorrelationID)
		assert.Equal(t, "value1", msg.Headers["header1"])

		// Return to pool
		msg.ReturnToPool()

		// Create new message - should be reset
		newMsg := messaging.NewMessage([]byte("new body"), messaging.WithKey("new.key"))
		assert.NotEqual(t, "test-id", newMsg.ID) // Should have new ID
		assert.Equal(t, "new.key", newMsg.Key)
		assert.Equal(t, "application/json", newMsg.ContentType) // Should be default
		assert.Equal(t, uint8(0), newMsg.Priority)              // Should be reset
		assert.Equal(t, "", newMsg.CorrelationID)               // Should be reset
		assert.Empty(t, newMsg.Headers)                         // Should be reset

		newMsg.ReturnToPool()
	})

	t.Run("NilMessageReturnToPool", func(t *testing.T) {
		// Should not panic
		var nilMsg *messaging.Message
		nilMsg.ReturnToPool()
	})

	t.Run("ConcurrentPoolAccess", func(t *testing.T) {
		const numGoroutines = 10
		const messagesPerGoroutine = 100

		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				for j := 0; j < messagesPerGoroutine; j++ {
					msg := messaging.NewMessage(
						[]byte(fmt.Sprintf("message-%d-%d", id, j)),
						messaging.WithKey(fmt.Sprintf("key.%d.%d", id, j)),
					)
					assert.NotNil(t, msg)
					assert.Equal(t, fmt.Sprintf("message-%d-%d", id, j), string(msg.Body))
					assert.Equal(t, fmt.Sprintf("key.%d.%d", id, j), msg.Key)

					// Return to pool
					msg.ReturnToPool()
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})

	t.Run("NewPooledMessageFunction", func(t *testing.T) {
		// Test the convenience function
		msg := messaging.NewPooledMessage([]byte("pooled message"), messaging.WithKey("pooled.key"))
		assert.NotNil(t, msg)
		assert.Equal(t, "pooled message", string(msg.Body))
		assert.Equal(t, "pooled.key", msg.Key)

		msg.ReturnToPool()
	})

	t.Run("PoolWithValidationFailure", func(t *testing.T) {
		// This should panic but return the message to pool first
		assert.Panics(t, func() {
			messaging.NewMessage([]byte("test"), messaging.WithKey("")) // Empty key should fail validation
		})

		// Pool should still be functional after panic
		msg := messaging.NewMessage([]byte("valid message"), messaging.WithKey("valid.key"))
		assert.NotNil(t, msg)
		assert.Equal(t, "valid message", string(msg.Body))
		assert.Equal(t, "valid.key", msg.Key)

		msg.ReturnToPool()
	})
}

func BenchmarkMessagePooling(b *testing.B) {
	b.Run("NewMessageWithPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := messaging.NewMessage([]byte("benchmark message"), messaging.WithKey("benchmark.key"))
			msg.ReturnToPool()
		}
	})

	b.Run("NewMessageWithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create message without using pool (for comparison)
			msg := &messaging.Message{
				ID:          "benchmark-msg",
				Body:        []byte("benchmark message"),
				ContentType: "application/json",
				Key:         "benchmark.key",
				Headers:     make(map[string]string),
			}
			_ = msg
		}
	})

	b.Run("ConcurrentPoolAccess", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				msg := messaging.NewMessage([]byte("concurrent message"), messaging.WithKey("concurrent.key"))
				msg.ReturnToPool()
			}
		})
	})
}
