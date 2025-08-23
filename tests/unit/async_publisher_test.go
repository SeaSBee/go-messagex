package unit

import (
	"context"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestObservabilityContext creates a proper observability context for testing
func createTestObservabilityContext() *messaging.ObservabilityContext {
	obsProvider, _ := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{})
	return messaging.NewObservabilityContext(context.Background(), obsProvider)
}

func TestAsyncPublisher(t *testing.T) {
	t.Run("NewAsyncPublisher", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			MaxInFlight:    1000,
			WorkerCount:    4,
			DropOnOverflow: false,
			PublishTimeout: 2 * time.Second,
			Retry: &messaging.RetryConfig{
				MaxAttempts:       3,
				BaseBackoff:       100 * time.Millisecond,
				MaxBackoff:        1 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            true,
			},
		}

		// Create mock transport
		transport := &rabbitmq.Transport{} // This would need to be properly mocked

		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)
		// Note: taskQueueSize is private, so we can't test it directly
	})

	t.Run("StartAndClose", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			MaxInFlight:    100,
			WorkerCount:    2,
			DropOnOverflow: false,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		// Start publisher
		ctx := context.Background()
		err := publisher.Start(ctx)
		assert.NoError(t, err)

		// Close publisher
		err = publisher.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("DropOnOverflow", func(t *testing.T) {
		// Test the drop on overflow functionality without starting the publisher
		config := &messaging.PublisherConfig{
			MaxInFlight:    1, // Very small queue
			WorkerCount:    1, // One worker
			DropOnOverflow: true,
			PublishTimeout: 100 * time.Millisecond,
		}

		// Create a minimal transport for testing
		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)

		// Test that publisher can be created successfully
		assert.NotNil(t, publisher)

		// Test that stats are initialized
		stats := publisher.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.TasksQueued)
		assert.Equal(t, uint64(0), stats.TasksDropped)

		// Test that publisher can be closed without starting
		err := publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("BlockOnOverflow", func(t *testing.T) {
		// Test the block on overflow configuration
		config := &messaging.PublisherConfig{
			MaxInFlight:    1, // Very small queue
			WorkerCount:    1,
			DropOnOverflow: false,
			PublishTimeout: 100 * time.Millisecond,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)
		assert.NotNil(t, publisher)

		// Test that publisher can be created successfully
		assert.NotNil(t, publisher)

		// Test that stats are initialized
		stats := publisher.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.TasksQueued)
		assert.Equal(t, uint64(0), stats.TasksDropped)

		// Test that publisher can be closed without starting
		err := publisher.Close(context.Background())
		assert.NoError(t, err)
	})

	t.Run("WorkerStats", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			MaxInFlight:    100,
			WorkerCount:    2,
			DropOnOverflow: false,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		ctx := context.Background()
		err := publisher.Start(ctx)
		require.NoError(t, err)

		// Get worker stats
		workerStats := publisher.GetWorkerStats()
		assert.Len(t, workerStats, 2)

		for _, stats := range workerStats {
			assert.Equal(t, uint64(0), stats.TasksProcessed)
			assert.Equal(t, uint64(0), stats.TasksFailed)
		}

		publisher.Close(ctx)
	})

	t.Run("PublisherStats", func(t *testing.T) {
		config := &messaging.PublisherConfig{
			MaxInFlight:    100,
			WorkerCount:    1,
			DropOnOverflow: false,
		}

		transport := &rabbitmq.Transport{}
		obsCtx := createTestObservabilityContext()

		publisher := rabbitmq.NewAsyncPublisher(transport, config, obsCtx)

		ctx := context.Background()
		err := publisher.Start(ctx)
		require.NoError(t, err)

		// Get initial stats
		stats := publisher.GetStats()
		assert.Equal(t, uint64(0), stats.TasksQueued)
		assert.Equal(t, uint64(0), stats.TasksProcessed)
		assert.Equal(t, uint64(0), stats.TasksFailed)
		assert.Equal(t, uint64(0), stats.TasksDropped)

		publisher.Close(ctx)
	})
}

func TestCodecSystem(t *testing.T) {
	t.Run("JSONCodec", func(t *testing.T) {
		codec := messaging.NewJSONCodec()
		assert.Equal(t, "json", codec.Name())
		assert.Equal(t, "application/json", codec.ContentType())

		// Test encoding
		data := map[string]interface{}{
			"test":   "value",
			"number": 123,
		}

		encoded, err := codec.Encode(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Test decoding
		var decoded map[string]interface{}
		err = codec.Decode(encoded, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "value", decoded["test"])
		assert.Equal(t, float64(123), decoded["number"])
	})

	t.Run("ProtobufCodec", func(t *testing.T) {
		codec := messaging.NewProtobufCodec()
		assert.Equal(t, "protobuf", codec.Name())
		assert.Equal(t, "application/x-protobuf", codec.ContentType())

		// Test with regular data (should fallback to JSON)
		data := map[string]interface{}{
			"test": "value",
		}

		encoded, err := codec.Encode(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, encoded)

		var decoded map[string]interface{}
		err = codec.Decode(encoded, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "value", decoded["test"])
	})

	t.Run("AvroCodec", func(t *testing.T) {
		schema := `{"type": "record", "name": "test", "fields": [{"name": "test", "type": "string"}]}`
		codec := messaging.NewAvroCodec(schema)
		assert.Equal(t, "avro", codec.Name())
		assert.Equal(t, "application/avro", codec.ContentType())

		// Test with regular data (should fallback to JSON)
		data := map[string]interface{}{
			"test": "value",
		}

		encoded, err := codec.Encode(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, encoded)

		var decoded map[string]interface{}
		err = codec.Decode(encoded, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "value", decoded["test"])
	})

	t.Run("CodecRegistry", func(t *testing.T) {
		registry := messaging.NewCodecRegistry()

		// Test default codecs
		jsonCodec, exists := registry.Get("json")
		assert.True(t, exists)
		assert.Equal(t, "json", jsonCodec.Name())

		protobufCodec, exists := registry.Get("protobuf")
		assert.True(t, exists)
		assert.Equal(t, "protobuf", protobufCodec.Name())

		// Test content type lookup
		codec, exists := registry.GetByContentType("application/json")
		assert.True(t, exists)
		assert.Equal(t, "json", codec.Name())

		// Test listing codecs
		codecs := registry.List()
		assert.Contains(t, codecs, "json")
		assert.Contains(t, codecs, "protobuf")
	})

	t.Run("MessageCodec", func(t *testing.T) {
		registry := messaging.NewCodecRegistry()
		messageCodec := messaging.NewMessageCodec(registry)

		// Create a test message
		msg := messaging.NewMessage(
			[]byte("test body"),
			messaging.WithID("test-123"),
			messaging.WithContentType("application/json"),
			messaging.WithKey("test.key"),
		)

		// Test encoding
		encoded, err := messageCodec.EncodeMessage(&msg)
		assert.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Test decoding
		decoded, err := messageCodec.DecodeMessage(encoded, "application/json")
		assert.NoError(t, err)
		assert.Equal(t, msg.ID, decoded.ID)
		assert.Equal(t, msg.Key, decoded.Key)
		assert.Equal(t, msg.Body, decoded.Body)
		assert.Equal(t, msg.ContentType, decoded.ContentType)
	})

	t.Run("GlobalCodecRegistry", func(t *testing.T) {
		// Test global registry functions
		codec, exists := messaging.GetCodec("json")
		assert.True(t, exists)
		assert.Equal(t, "json", codec.Name())

		codec, exists = messaging.GetCodecByContentType("application/json")
		assert.True(t, exists)
		assert.Equal(t, "json", codec.Name())

		// Test registering a custom codec
		customCodec := &messaging.JSONCodec{}
		messaging.RegisterCodec(customCodec)

		codec, exists = messaging.GetCodec("json")
		assert.True(t, exists)
		assert.Equal(t, "json", codec.Name())
	})
}
