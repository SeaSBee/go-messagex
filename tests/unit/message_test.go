package unit

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessagePriority tests message priority constants and validation
func TestMessagePriority(t *testing.T) {
	t.Run("priority constants are defined correctly", func(t *testing.T) {
		assert.Equal(t, messaging.MessagePriority(0), messaging.PriorityLow)
		assert.Equal(t, messaging.MessagePriority(1), messaging.PriorityNormal)
		assert.Equal(t, messaging.MessagePriority(2), messaging.PriorityHigh)
		assert.Equal(t, messaging.MessagePriority(3), messaging.PriorityCritical)
	})
}

// TestNewMessage tests message creation functions
func TestNewMessage(t *testing.T) {
	t.Run("creates message with body", func(t *testing.T) {
		body := []byte("test message")
		msg := messaging.NewMessage(body)

		assert.NotNil(t, msg)
		assert.NotEmpty(t, msg.ID)
		assert.Equal(t, body, msg.Body)
		assert.NotNil(t, msg.Headers)
		assert.NotNil(t, msg.Metadata)
		assert.Equal(t, messaging.PriorityNormal, msg.Priority)
		assert.Equal(t, "application/octet-stream", msg.ContentType)
		assert.Equal(t, "utf-8", msg.Encoding)
		assert.False(t, msg.Timestamp.IsZero())
	})

	t.Run("creates message with specific ID", func(t *testing.T) {
		id := "test-id-123"
		body := []byte("test message")
		msg := messaging.NewMessageWithID(id, body)

		assert.NotNil(t, msg)
		assert.Equal(t, id, msg.ID)
		assert.Equal(t, body, msg.Body)
	})

	t.Run("creates text message", func(t *testing.T) {
		text := "hello world"
		msg := messaging.NewTextMessage(text)

		assert.NotNil(t, msg)
		assert.Equal(t, []byte(text), msg.Body)
		assert.Equal(t, "text/plain", msg.ContentType)
	})

	t.Run("creates JSON message", func(t *testing.T) {
		data := map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		msg, err := messaging.NewJSONMessage(data)

		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, "application/json", msg.ContentType)

		// Verify JSON content
		var result map[string]interface{}
		err = json.Unmarshal(msg.Body, &result)
		assert.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(30), result["age"]) // JSON numbers are float64
	})

	t.Run("returns error for nil JSON message", func(t *testing.T) {
		msg, err := messaging.NewJSONMessage(nil)

		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "cannot create JSON message from nil value")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		// Create a value that cannot be marshaled to JSON
		invalidData := make(chan int)
		msg, err := messaging.NewJSONMessage(invalidData)

		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "failed to marshal JSON")
	})
}

// TestMessage_Headers tests header operations
func TestMessage_Headers(t *testing.T) {
	t.Run("sets and gets headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetHeader("key1", "value1")
		msg.SetHeader("key2", 42)

		value1, exists1 := msg.GetHeader("key1")
		assert.True(t, exists1)
		assert.Equal(t, "value1", value1)

		value2, exists2 := msg.GetHeader("key2")
		assert.True(t, exists2)
		assert.Equal(t, 42, value2)

		_, exists3 := msg.GetHeader("nonexistent")
		assert.False(t, exists3)
	})

	t.Run("handles nil message for headers", func(t *testing.T) {
		var msg *messaging.Message

		msg.SetHeader("key", "value") // Should not panic

		value, exists := msg.GetHeader("key")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("gets all headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetHeader("key1", "value1")
		msg.SetHeader("key2", "value2")

		headers := msg.GetAllHeaders()
		assert.NotNil(t, headers)
		assert.Equal(t, "value1", headers["key1"])
		assert.Equal(t, "value2", headers["key2"])
	})

	t.Run("returns nil for nil message getAllHeaders", func(t *testing.T) {
		var msg *messaging.Message
		headers := msg.GetAllHeaders()
		assert.Nil(t, headers)
	})
}

// TestMessage_Metadata tests metadata operations
func TestMessage_Metadata(t *testing.T) {
	t.Run("sets and gets metadata", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetMetadata("key1", "value1")
		msg.SetMetadata("key2", 42)

		value1, exists1 := msg.GetMetadata("key1")
		assert.True(t, exists1)
		assert.Equal(t, "value1", value1)

		value2, exists2 := msg.GetMetadata("key2")
		assert.True(t, exists2)
		assert.Equal(t, 42, value2)

		_, exists3 := msg.GetMetadata("nonexistent")
		assert.False(t, exists3)
	})

	t.Run("handles nil message for metadata", func(t *testing.T) {
		var msg *messaging.Message

		msg.SetMetadata("key", "value") // Should not panic

		value, exists := msg.GetMetadata("key")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("gets all metadata", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetMetadata("key1", "value1")
		msg.SetMetadata("key2", "value2")

		metadata := msg.GetAllMetadata()
		assert.NotNil(t, metadata)
		assert.Equal(t, "value1", metadata["key1"])
		assert.Equal(t, "value2", metadata["key2"])
	})

	t.Run("returns nil for nil message getAllMetadata", func(t *testing.T) {
		var msg *messaging.Message
		metadata := msg.GetAllMetadata()
		assert.Nil(t, metadata)
	})
}

// TestMessage_Properties tests message property operations
func TestMessage_Properties(t *testing.T) {
	t.Run("sets priority", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		err := msg.SetPriority(messaging.PriorityHigh)
		assert.NoError(t, err)
		assert.Equal(t, messaging.PriorityHigh, msg.Priority)
	})

	t.Run("returns error for invalid priority", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		err := msg.SetPriority(messaging.MessagePriority(5))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid priority")
	})

	t.Run("handles nil message for priority", func(t *testing.T) {
		var msg *messaging.Message
		err := msg.SetPriority(messaging.PriorityHigh)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set priority on nil message")
	})

	t.Run("sets TTL", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		ttl := 30 * time.Second
		err := msg.SetTTL(ttl)
		assert.NoError(t, err)
		assert.Equal(t, ttl, msg.Properties.TTL)
	})

	t.Run("returns error for negative TTL", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		err := msg.SetTTL(-1 * time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL cannot be negative")
	})

	t.Run("sets expiration", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		expiration := 60 * time.Second
		err := msg.SetExpiration(expiration)
		assert.NoError(t, err)
		assert.Equal(t, expiration, msg.Properties.Expiration)
	})

	t.Run("returns error for negative expiration", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		err := msg.SetExpiration(-1 * time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expiration cannot be negative")
	})

	t.Run("sets other properties", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetPersistent(true)
		msg.SetRoutingKey("test.key")
		msg.SetExchange("test.exchange")
		msg.SetQueue("test.queue")
		msg.SetCorrelationID("corr-123")
		msg.SetReplyTo("reply.queue")

		assert.True(t, msg.Properties.Persistent)
		assert.Equal(t, "test.key", msg.Properties.RoutingKey)
		assert.Equal(t, "test.exchange", msg.Properties.Exchange)
		assert.Equal(t, "test.queue", msg.Properties.Queue)
		assert.Equal(t, "corr-123", msg.Properties.CorrelationID)
		assert.Equal(t, "reply.queue", msg.Properties.ReplyTo)
	})

	t.Run("handles nil message for properties", func(t *testing.T) {
		var msg *messaging.Message

		msg.SetPersistent(true)     // Should not panic
		msg.SetRoutingKey("key")    // Should not panic
		msg.SetExchange("exchange") // Should not panic
		msg.SetQueue("queue")       // Should not panic
		msg.SetCorrelationID("id")  // Should not panic
		msg.SetReplyTo("reply")     // Should not panic
	})
}

// TestMessage_JSON tests JSON serialization/deserialization
func TestMessage_JSON(t *testing.T) {
	t.Run("converts message to JSON", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test message"))
		msg.SetHeader("test-header", "test-value")
		msg.SetMetadata("test-metadata", "test-value")

		jsonData, err := msg.ToJSON()
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonData)

		// Verify it's valid JSON
		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		assert.NoError(t, err)
		// Body is base64 encoded in JSON
		assert.Equal(t, "dGVzdCBtZXNzYWdl", result["body"])
	})

	t.Run("returns error for nil message toJSON", func(t *testing.T) {
		var msg *messaging.Message
		jsonData, err := msg.ToJSON()
		assert.Error(t, err)
		assert.Nil(t, jsonData)
		assert.Contains(t, err.Error(), "cannot convert nil message to JSON")
	})

	t.Run("creates message from JSON", func(t *testing.T) {
		originalMsg := messaging.NewMessage([]byte("test message"))
		originalMsg.SetHeader("test-header", "test-value")
		originalMsg.SetMetadata("test-metadata", "test-value")

		jsonData, err := originalMsg.ToJSON()
		require.NoError(t, err)

		msg, err := messaging.FromJSON(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, originalMsg.ID, msg.ID)
		assert.Equal(t, originalMsg.Body, msg.Body)
		assert.Equal(t, originalMsg.ContentType, msg.ContentType)
		assert.Equal(t, originalMsg.Encoding, msg.Encoding)
	})

	t.Run("returns error for empty JSON data", func(t *testing.T) {
		msg, err := messaging.FromJSON([]byte{})
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "cannot create message from empty JSON data")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		msg, err := messaging.FromJSON([]byte("invalid json"))
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "failed to unmarshal message from JSON")
	})
}

// TestMessage_Clone tests message cloning
func TestMessage_Clone(t *testing.T) {
	t.Run("clones message with all properties", func(t *testing.T) {
		original := messaging.NewMessage([]byte("test message"))
		original.SetHeader("header1", "value1")
		original.SetMetadata("metadata1", "value1")
		original.SetPriority(messaging.PriorityHigh)
		original.SetTTL(30 * time.Second)
		original.SetExpiration(60 * time.Second)
		original.SetPersistent(true)
		original.SetRoutingKey("test.key")
		original.SetExchange("test.exchange")
		original.SetQueue("test.queue")
		original.SetCorrelationID("corr-123")
		original.SetReplyTo("reply.queue")

		clone := original.Clone()

		assert.NotNil(t, clone)
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.Body, clone.Body)
		assert.Equal(t, original.Priority, clone.Priority)
		assert.Equal(t, original.ContentType, clone.ContentType)
		assert.Equal(t, original.Encoding, clone.Encoding)
		assert.Equal(t, original.Properties, clone.Properties)

		// Verify headers and metadata are copied
		headers := clone.GetAllHeaders()
		assert.Equal(t, "value1", headers["header1"])

		metadata := clone.GetAllMetadata()
		assert.Equal(t, "value1", metadata["metadata1"])

		// Verify it's a deep copy (modifying clone doesn't affect original)
		clone.SetHeader("header2", "value2")
		originalHeaders := original.GetAllHeaders()
		_, exists := originalHeaders["header2"]
		assert.False(t, exists)
	})

	t.Run("returns nil for nil message clone", func(t *testing.T) {
		var msg *messaging.Message
		clone := msg.Clone()
		assert.Nil(t, clone)
	})
}

// TestMessage_Validation tests message validation
func TestMessage_Validation(t *testing.T) {
	t.Run("validates valid message", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		err := msg.Validate()
		assert.NoError(t, err)
	})

	t.Run("returns error for nil message validation", func(t *testing.T) {
		var msg *messaging.Message
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message is nil")
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.ID = ""
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message ID cannot be empty")
	})

	t.Run("returns error for invalid priority", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.Priority = messaging.MessagePriority(5)
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid priority")
	})

	t.Run("returns error for negative TTL", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.Properties.TTL = -1 * time.Second
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL cannot be negative")
	})

	t.Run("returns error for negative expiration", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.Properties.Expiration = -1 * time.Second
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expiration cannot be negative")
	})

	t.Run("returns error for empty content type", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.ContentType = ""
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content type cannot be empty")
	})

	t.Run("returns error for empty encoding", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.Encoding = ""
		err := msg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "encoding cannot be empty")
	})
}

// TestMessage_Size tests message size calculation
func TestMessage_Size(t *testing.T) {
	t.Run("calculates size correctly", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.SetHeader("header1", "value1")
		msg.SetMetadata("metadata1", "value1")

		size := msg.Size()
		assert.Greater(t, size, 0)
		assert.Equal(t, len("test")+len("header1")+len("value1")+len("metadata1")+len("value1"), size)
	})

	t.Run("caches size calculation", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		size1 := msg.Size()
		size2 := msg.Size()
		assert.Equal(t, size1, size2)

		// Modify message to invalidate cache
		msg.SetHeader("new", "header")
		size3 := msg.Size()
		assert.Greater(t, size3, size1)
	})

	t.Run("returns 0 for nil message size", func(t *testing.T) {
		var msg *messaging.Message
		size := msg.Size()
		assert.Equal(t, 0, size)
	})
}

// TestMessage_Utility tests utility methods
func TestMessage_Utility(t *testing.T) {
	t.Run("string representation", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		str := msg.String()

		assert.Contains(t, str, "Message{")
		assert.Contains(t, str, msg.ID)
		assert.Contains(t, str, "4 bytes") // len("test")
		assert.Contains(t, str, fmt.Sprintf("%d", msg.Priority))
	})

	t.Run("returns nil string for nil message", func(t *testing.T) {
		var msg *messaging.Message
		str := msg.String()
		assert.Equal(t, "Message{nil}", str)
	})

	t.Run("checks if empty", func(t *testing.T) {
		msg1 := messaging.NewMessage([]byte("test"))
		assert.False(t, msg1.IsEmpty())

		msg2 := messaging.NewMessage([]byte{})
		assert.True(t, msg2.IsEmpty())

		var msg3 *messaging.Message
		assert.True(t, msg3.IsEmpty())
	})
}

// TestMessageBuilder tests the message builder pattern
func TestMessageBuilder(t *testing.T) {
	t.Run("builds message with all properties", func(t *testing.T) {
		builder := messaging.NewMessageBuilder()

		msg, err := builder.
			WithBody([]byte("test body")).
			WithHeader("header1", "value1").
			WithMetadata("metadata1", "value1").
			WithPriority(messaging.PriorityHigh).
			WithTTL(30 * time.Second).
			WithExpiration(60 * time.Second).
			WithRoutingKey("test.key").
			WithExchange("test.exchange").
			WithQueue("test.queue").
			WithCorrelationID("corr-123").
			WithReplyTo("reply.queue").
			WithPersistent(true).
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, []byte("test body"), msg.Body)
		assert.Equal(t, "value1", msg.Headers["header1"])
		assert.Equal(t, "value1", msg.Metadata["metadata1"])
		assert.Equal(t, messaging.PriorityHigh, msg.Priority)
		assert.Equal(t, 30*time.Second, msg.Properties.TTL)
		assert.Equal(t, 60*time.Second, msg.Properties.Expiration)
		assert.Equal(t, "test.key", msg.Properties.RoutingKey)
		assert.Equal(t, "test.exchange", msg.Properties.Exchange)
		assert.Equal(t, "test.queue", msg.Properties.Queue)
		assert.Equal(t, "corr-123", msg.Properties.CorrelationID)
		assert.Equal(t, "reply.queue", msg.Properties.ReplyTo)
		assert.True(t, msg.Properties.Persistent)
	})

	t.Run("builds text message", func(t *testing.T) {
		builder := messaging.NewMessageBuilder()

		msg, err := builder.
			WithTextBody("hello world").
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, []byte("hello world"), msg.Body)
		assert.Equal(t, "text/plain", msg.ContentType)
	})

	t.Run("builds JSON message", func(t *testing.T) {
		builder := messaging.NewMessageBuilder()

		data := map[string]interface{}{
			"name": "test",
			"age":  30,
		}

		msg, err := builder.
			WithJSONBody(data).
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, "application/json", msg.ContentType)

		// Verify JSON content
		var result map[string]interface{}
		err = json.Unmarshal(msg.Body, &result)
		assert.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(30), result["age"])
	})

	t.Run("handles JSON marshaling error", func(t *testing.T) {
		builder := messaging.NewMessageBuilder()

		// Create a value that cannot be marshaled to JSON
		invalidData := make(chan int)

		msg, err := builder.
			WithJSONBody(invalidData).
			Build()

		assert.NoError(t, err) // Builder should not fail, error is stored in metadata
		assert.NotNil(t, msg)
		assert.Contains(t, msg.Metadata, "json_error")
	})

	t.Run("validates message during build", func(t *testing.T) {
		builder := messaging.NewMessageBuilder()

		// Create invalid message with invalid priority
		msg, err := builder.
			WithBody([]byte("test")).
			WithPriority(messaging.MessagePriority(5)).
			Build()

		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "invalid priority")
	})
}

// TestBatchMessage tests batch message functionality
func TestBatchMessage(t *testing.T) {
	t.Run("creates batch message", func(t *testing.T) {
		msg1 := messaging.NewMessage([]byte("message 1"))
		msg2 := messaging.NewMessage([]byte("message 2"))
		msg3 := messaging.NewMessage([]byte("message 3"))

		batch := messaging.NewBatchMessage([]*messaging.Message{msg1, msg2, msg3})

		assert.NotNil(t, batch)
		assert.Equal(t, 3, batch.Count())
		assert.False(t, batch.IsEmpty())
		assert.Greater(t, batch.Size, 0)
		assert.False(t, batch.Created.IsZero())
	})

	t.Run("creates empty batch message", func(t *testing.T) {
		batch := messaging.NewBatchMessage([]*messaging.Message{})

		assert.NotNil(t, batch)
		assert.Equal(t, 0, batch.Count())
		assert.True(t, batch.IsEmpty())
		assert.Equal(t, 0, batch.Size)
	})

	t.Run("adds message to batch", func(t *testing.T) {
		batch := messaging.NewBatchMessage([]*messaging.Message{})
		msg := messaging.NewMessage([]byte("new message"))

		initialSize := batch.Size
		batch.AddMessage(msg)

		assert.Equal(t, 1, batch.Count())
		assert.False(t, batch.IsEmpty())
		assert.Greater(t, batch.Size, initialSize)
	})

	t.Run("handles nil message in add", func(t *testing.T) {
		batch := messaging.NewBatchMessage([]*messaging.Message{})
		initialCount := batch.Count()

		batch.AddMessage(nil)

		assert.Equal(t, initialCount, batch.Count())
	})

	t.Run("handles nil batch in add", func(t *testing.T) {
		var batch *messaging.BatchMessage
		msg := messaging.NewMessage([]byte("test"))

		batch.AddMessage(msg) // Should not panic
	})

	t.Run("clears batch", func(t *testing.T) {
		msg1 := messaging.NewMessage([]byte("message 1"))
		msg2 := messaging.NewMessage([]byte("message 2"))
		batch := messaging.NewBatchMessage([]*messaging.Message{msg1, msg2})

		assert.Equal(t, 2, batch.Count())
		assert.False(t, batch.IsEmpty())

		batch.Clear()

		assert.Equal(t, 0, batch.Count())
		assert.True(t, batch.IsEmpty())
		assert.Equal(t, 0, batch.Size)
	})

	t.Run("handles nil batch in clear", func(t *testing.T) {
		var batch *messaging.BatchMessage
		batch.Clear() // Should not panic
	})

	t.Run("handles nil batch in isEmpty", func(t *testing.T) {
		var batch *messaging.BatchMessage
		assert.True(t, batch.IsEmpty())
	})

	t.Run("handles nil batch in count", func(t *testing.T) {
		var batch *messaging.BatchMessage
		assert.Equal(t, 0, batch.Count())
	})
}

// TestMessage_Concurrency tests concurrent access to message
func TestMessage_Concurrency(t *testing.T) {
	t.Run("concurrent header operations", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		var wg sync.WaitGroup
		numGoroutines := 10

		// Test concurrent header writes
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg.SetHeader(fmt.Sprintf("key%d", id), fmt.Sprintf("value%d", id))
			}(i)
		}

		// Test concurrent header reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg.GetHeader(fmt.Sprintf("key%d", id))
			}(i)
		}

		wg.Wait()

		// Verify all headers were set
		headers := msg.GetAllHeaders()
		assert.Len(t, headers, numGoroutines)
	})

	t.Run("concurrent metadata operations", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		var wg sync.WaitGroup
		numGoroutines := 10

		// Test concurrent metadata writes
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg.SetMetadata(fmt.Sprintf("key%d", id), fmt.Sprintf("value%d", id))
			}(i)
		}

		// Test concurrent metadata reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg.GetMetadata(fmt.Sprintf("key%d", id))
			}(i)
		}

		wg.Wait()

		// Verify all metadata was set
		metadata := msg.GetAllMetadata()
		assert.Len(t, metadata, numGoroutines)
	})

	t.Run("concurrent size calculations", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		var wg sync.WaitGroup
		numGoroutines := 10

		// Test concurrent size calculations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				size := msg.Size()
				assert.Greater(t, size, 0)
			}()
		}

		wg.Wait()
	})
}

// TestMessage_EdgeCases tests edge cases and boundary conditions
func TestMessage_EdgeCases(t *testing.T) {
	t.Run("message with very large body", func(t *testing.T) {
		largeBody := make([]byte, 1024*1024) // 1MB
		for i := range largeBody {
			largeBody[i] = byte(i % 256)
		}

		msg := messaging.NewMessage(largeBody)
		assert.NotNil(t, msg)
		assert.Equal(t, len(largeBody), len(msg.Body))
		assert.Equal(t, len(largeBody), msg.Size())
	})

	t.Run("message with many headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		// Add many headers
		for i := 0; i < 1000; i++ {
			msg.SetHeader(fmt.Sprintf("header%d", i), fmt.Sprintf("value%d", i))
		}

		headers := msg.GetAllHeaders()
		assert.Len(t, headers, 1000)
		assert.Greater(t, msg.Size(), 1000) // Should include header sizes
	})

	t.Run("message with many metadata entries", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		// Add many metadata entries
		for i := 0; i < 1000; i++ {
			msg.SetMetadata(fmt.Sprintf("meta%d", i), fmt.Sprintf("value%d", i))
		}

		metadata := msg.GetAllMetadata()
		assert.Len(t, metadata, 1000)
		assert.Greater(t, msg.Size(), 1000) // Should include metadata sizes
	})

	t.Run("message with special characters in headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		specialChars := []string{
			"key with spaces",
			"key-with-dashes",
			"key_with_underscores",
			"key.with.dots",
			"key/with/slashes",
			"key\\with\\backslashes",
			"key:with:colons",
			"key;with;semicolons",
		}

		for i, key := range specialChars {
			msg.SetHeader(key, fmt.Sprintf("value%d", i))
		}

		headers := msg.GetAllHeaders()
		assert.Len(t, headers, len(specialChars))

		for i, key := range specialChars {
			value, exists := msg.GetHeader(key)
			assert.True(t, exists)
			assert.Equal(t, fmt.Sprintf("value%d", i), value)
		}
	})

	t.Run("message with complex data types in headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		// Test various data types
		msg.SetHeader("string", "test")
		msg.SetHeader("int", 42)
		msg.SetHeader("float", 3.14)
		msg.SetHeader("bool", true)
		msg.SetHeader("slice", []string{"a", "b", "c"})
		msg.SetHeader("map", map[string]interface{}{"key": "value"})

		headers := msg.GetAllHeaders()
		assert.Len(t, headers, 6)

		// Verify types are preserved
		assert.Equal(t, "test", headers["string"])
		assert.Equal(t, 42, headers["int"])
		assert.Equal(t, 3.14, headers["float"])
		assert.Equal(t, true, headers["bool"])
		assert.Equal(t, []string{"a", "b", "c"}, headers["slice"])
		assert.Equal(t, map[string]interface{}{"key": "value"}, headers["map"])
	})
}

// TestMessage_Integration tests integration scenarios
func TestMessage_Integration(t *testing.T) {
	t.Run("full message lifecycle", func(t *testing.T) {
		// Create message
		msg := messaging.NewMessage([]byte("integration test"))
		msg.SetHeader("source", "test")
		msg.SetMetadata("version", "1.0")
		msg.SetPriority(messaging.PriorityHigh)
		msg.SetTTL(30 * time.Second)
		msg.SetPersistent(true)
		msg.SetRoutingKey("test.key")
		msg.SetExchange("test.exchange")

		// Validate
		err := msg.Validate()
		assert.NoError(t, err)

		// Clone
		clone := msg.Clone()
		assert.NotNil(t, clone)
		assert.Equal(t, msg.ID, clone.ID)
		assert.Equal(t, msg.Body, clone.Body)

		// Convert to JSON
		jsonData, err := msg.ToJSON()
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonData)

		// Create from JSON
		msgFromJSON, err := messaging.FromJSON(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, msgFromJSON)
		assert.Equal(t, msg.ID, msgFromJSON.ID)
		assert.Equal(t, msg.Body, msgFromJSON.Body)

		// Verify size calculation
		size := msg.Size()
		assert.Greater(t, size, 0)

		// Verify string representation
		str := msg.String()
		assert.Contains(t, str, msg.ID)
		assert.Contains(t, str, "16 bytes") // Body length, not content
	})

	t.Run("message builder integration", func(t *testing.T) {
		// Build complex message
		msg, err := messaging.NewMessageBuilder().
			WithTextBody("Hello, World!").
			WithHeader("content-type", "text/plain").
			WithMetadata("source", "integration-test").
			WithPriority(messaging.PriorityHigh).
			WithTTL(5 * time.Minute).
			WithExpiration(10 * time.Minute).
			WithRoutingKey("hello.world").
			WithExchange("messages").
			WithQueue("hello-queue").
			WithCorrelationID("corr-123").
			WithReplyTo("reply-queue").
			WithPersistent(true).
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, msg)

		// Validate
		err = msg.Validate()
		assert.NoError(t, err)

		// Verify all properties
		assert.Equal(t, "Hello, World!", string(msg.Body))
		assert.Equal(t, "text/plain", msg.ContentType)
		assert.Equal(t, "text/plain", msg.Headers["content-type"])
		assert.Equal(t, "integration-test", msg.Metadata["source"])
		assert.Equal(t, messaging.PriorityHigh, msg.Priority)
		assert.Equal(t, 5*time.Minute, msg.Properties.TTL)
		assert.Equal(t, 10*time.Minute, msg.Properties.Expiration)
		assert.Equal(t, "hello.world", msg.Properties.RoutingKey)
		assert.Equal(t, "messages", msg.Properties.Exchange)
		assert.Equal(t, "hello-queue", msg.Properties.Queue)
		assert.Equal(t, "corr-123", msg.Properties.CorrelationID)
		assert.Equal(t, "reply-queue", msg.Properties.ReplyTo)
		assert.True(t, msg.Properties.Persistent)
	})
}
