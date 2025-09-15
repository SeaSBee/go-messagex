package unit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransportInterfaces tests the core transport interfaces
func TestTransportInterfaces(t *testing.T) {
	t.Run("PublisherInterface", func(t *testing.T) {
		// Test that MockPublisher implements Publisher interface
		config := messaging.NewTestConfig()
		transport := messaging.NewMockTransport(config)
		publisher := transport.NewPublisher(nil, config.Observability)

		// Verify interface compliance
		var _ messaging.Publisher = publisher

		// Test basic functionality
		ctx := context.Background()
		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")

		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		assert.NoError(t, err)
		assert.NotNil(t, receipt)

		// Test close functionality
		err = publisher.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("ConsumerInterface", func(t *testing.T) {
		// Test that MockConsumer implements Consumer interface
		config := messaging.NewTestConfig()
		transport := messaging.NewMockTransport(config)
		consumer := transport.NewConsumer(nil, config.Observability)

		// Verify interface compliance
		var _ messaging.Consumer = consumer

		// Test basic functionality
		ctx := context.Background()
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			return messaging.Ack, nil
		})

		err := consumer.Start(ctx, handler)
		assert.NoError(t, err)

		err = consumer.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("HandlerInterface", func(t *testing.T) {
		// Test that HandlerFunc implements Handler interface
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			return messaging.Ack, nil
		})

		// Verify interface compliance
		var _ messaging.Handler = handler

		// Test handler execution
		ctx := context.Background()
		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")
		delivery := messaging.Delivery{Message: msg}

		decision, err := handler.Process(ctx, delivery)
		assert.NoError(t, err)
		assert.Equal(t, messaging.Ack, decision)
	})

	t.Run("HandlerFuncNil", func(t *testing.T) {
		var handler messaging.HandlerFunc
		ctx := context.Background()
		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")
		delivery := messaging.Delivery{Message: msg}

		decision, err := handler.Process(ctx, delivery)
		assert.Error(t, err)
		assert.Equal(t, messaging.NackRequeue, decision)
		assert.Contains(t, err.Error(), "handler function is nil")
	})
}

// TestTransportRegistry tests the TransportRegistry functionality
func TestTransportRegistry(t *testing.T) {
	t.Run("NewTransportRegistry", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		assert.NotNil(t, registry)
		assert.Empty(t, registry.List())
	})

	t.Run("RegisterAndGet", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		mockFactory := &MockTransportFactory{name: "test-transport"}

		// Test successful registration
		registry.Register("test", mockFactory)
		factory, exists := registry.Get("test")
		assert.True(t, exists)
		assert.Equal(t, mockFactory, factory)
		assert.Equal(t, "test-transport", factory.Name())
	})

	t.Run("RegisterEmptyName", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		mockFactory := &MockTransportFactory{name: "test-transport"}

		// Test panic on empty name
		assert.Panics(t, func() {
			registry.Register("", mockFactory)
		})
	})

	t.Run("RegisterNilFactory", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()

		// Test panic on nil factory
		assert.Panics(t, func() {
			registry.Register("test", nil)
		})
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		factory, exists := registry.Get("non-existent")
		assert.False(t, exists)
		assert.Nil(t, factory)
	})

	t.Run("List", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		mockFactory1 := &MockTransportFactory{name: "transport1"}
		mockFactory2 := &MockTransportFactory{name: "transport2"}

		registry.Register("transport1", mockFactory1)
		registry.Register("transport2", mockFactory2)

		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "transport1")
		assert.Contains(t, names, "transport2")
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		var wg sync.WaitGroup
		const numGoroutines = 10

		// Test concurrent registration
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				factory := &MockTransportFactory{name: fmt.Sprintf("transport-%d", id)}
				registry.Register(fmt.Sprintf("transport-%d", id), factory)
			}(i)
		}

		wg.Wait()
		names := registry.List()
		assert.Len(t, names, numGoroutines)

		// Test concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				factory, exists := registry.Get(fmt.Sprintf("transport-%d", id))
				assert.True(t, exists)
				assert.NotNil(t, factory)
			}(i)
		}

		wg.Wait()
	})
}

// TestDefaultRegistry tests the default registry functionality
func TestDefaultRegistry(t *testing.T) {
	t.Run("RegisterTransport", func(t *testing.T) {
		mockFactory := &MockTransportFactory{name: "default-test"}
		messaging.RegisterTransport("default-test", mockFactory)

		factory, exists := messaging.GetTransport("default-test")
		assert.True(t, exists)
		assert.Equal(t, mockFactory, factory)
	})

	t.Run("GetTransportEmptyName", func(t *testing.T) {
		factory, exists := messaging.GetTransport("")
		assert.False(t, exists)
		assert.Nil(t, factory)
	})

	t.Run("GetTransportNonExistent", func(t *testing.T) {
		factory, exists := messaging.GetTransport("non-existent")
		assert.False(t, exists)
		assert.Nil(t, factory)
	})

	t.Run("NewPublisher", func(t *testing.T) {
		// Register a mock factory
		mockFactory := &MockTransportFactory{name: "publisher-test"}
		messaging.RegisterTransport("publisher-test", mockFactory)

		// Test successful publisher creation
		ctx := context.Background()
		config := &messaging.Config{Transport: "publisher-test"}
		publisher, err := messaging.NewPublisher(ctx, config)
		assert.NoError(t, err)
		assert.NotNil(t, publisher)

		// Test nil config
		publisher, err = messaging.NewPublisher(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Contains(t, err.Error(), "config cannot be nil")

		// Test nil context
		publisher, err = messaging.NewPublisher(nil, config)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Contains(t, err.Error(), "context cannot be nil")

		// Test unsupported transport
		config.Transport = "unsupported"
		publisher, err = messaging.NewPublisher(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, publisher)
		assert.Equal(t, messaging.ErrUnsupportedTransport, err)
	})

	t.Run("NewConsumer", func(t *testing.T) {
		// Register a mock factory
		mockFactory := &MockTransportFactory{name: "consumer-test"}
		messaging.RegisterTransport("consumer-test", mockFactory)

		// Test successful consumer creation
		ctx := context.Background()
		config := &messaging.Config{Transport: "consumer-test"}
		consumer, err := messaging.NewConsumer(ctx, config)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)

		// Test nil config
		consumer, err = messaging.NewConsumer(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "config cannot be nil")

		// Test nil context
		consumer, err = messaging.NewConsumer(nil, config)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "context cannot be nil")

		// Test unsupported transport
		config.Transport = "unsupported"
		consumer, err = messaging.NewConsumer(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Equal(t, messaging.ErrUnsupportedTransport, err)
	})
}

// TestAckDecision tests the AckDecision enum and its String method
func TestAckDecision(t *testing.T) {
	t.Run("AckDecisionValues", func(t *testing.T) {
		assert.Equal(t, messaging.AckDecision(0), messaging.Ack)
		assert.Equal(t, messaging.AckDecision(1), messaging.NackRequeue)
		assert.Equal(t, messaging.AckDecision(2), messaging.NackDLQ)
	})

	t.Run("AckDecisionString", func(t *testing.T) {
		assert.Equal(t, "ack", messaging.Ack.String())
		assert.Equal(t, "nack_requeue", messaging.NackRequeue.String())
		assert.Equal(t, "nack_dlq", messaging.NackDLQ.String())

		// Test unknown value
		unknown := messaging.AckDecision(999)
		assert.Equal(t, "unknown", unknown.String())
	})

	t.Run("AckDecisionUsage", func(t *testing.T) {
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			// Test different decision paths
			if delivery.Message.ID == "ack" {
				return messaging.Ack, nil
			} else if delivery.Message.ID == "requeue" {
				return messaging.NackRequeue, nil
			} else if delivery.Message.ID == "dlq" {
				return messaging.NackDLQ, nil
			}
			return messaging.Ack, nil
		})

		ctx := context.Background()
		factory := messaging.NewTestMessageFactory()

		// Test Ack
		msgAck := factory.CreateMessage([]byte("test"), messaging.WithID("ack"))
		deliveryAck := messaging.Delivery{Message: msgAck}
		decision, err := handler.Process(ctx, deliveryAck)
		assert.NoError(t, err)
		assert.Equal(t, messaging.Ack, decision)

		// Test NackRequeue
		msgRequeue := factory.CreateMessage([]byte("test"), messaging.WithID("requeue"))
		deliveryRequeue := messaging.Delivery{Message: msgRequeue}
		decision, err = handler.Process(ctx, deliveryRequeue)
		assert.NoError(t, err)
		assert.Equal(t, messaging.NackRequeue, decision)

		// Test NackDLQ
		msgDLQ := factory.CreateMessage([]byte("test"), messaging.WithID("dlq"))
		deliveryDLQ := messaging.Delivery{Message: msgDLQ}
		decision, err = handler.Process(ctx, deliveryDLQ)
		assert.NoError(t, err)
		assert.Equal(t, messaging.NackDLQ, decision)
	})
}

// TestPublishResult tests the PublishResult struct
func TestPublishResult(t *testing.T) {
	t.Run("PublishResultCreation", func(t *testing.T) {
		timestamp := time.Now()
		result := messaging.PublishResult{
			MessageID:   "test-msg-123",
			DeliveryTag: 456,
			Timestamp:   timestamp,
			Success:     true,
			Reason:      "published successfully",
		}

		assert.Equal(t, "test-msg-123", result.MessageID)
		assert.Equal(t, uint64(456), result.DeliveryTag)
		assert.Equal(t, timestamp, result.Timestamp)
		assert.True(t, result.Success)
		assert.Equal(t, "published successfully", result.Reason)
	})

	t.Run("PublishResultFailure", func(t *testing.T) {
		timestamp := time.Now()
		result := messaging.PublishResult{
			MessageID:   "test-msg-456",
			DeliveryTag: 0,
			Timestamp:   timestamp,
			Success:     false,
			Reason:      "connection failed",
		}

		assert.Equal(t, "test-msg-456", result.MessageID)
		assert.Equal(t, uint64(0), result.DeliveryTag)
		assert.Equal(t, timestamp, result.Timestamp)
		assert.False(t, result.Success)
		assert.Equal(t, "connection failed", result.Reason)
	})
}

// TestReceiptInterface tests the Receipt interface
func TestReceiptInterface(t *testing.T) {
	t.Run("MockReceiptInterface", func(t *testing.T) {
		config := messaging.NewTestConfig()
		receipt := messaging.NewMockReceipt("test-receipt", config)

		// Verify interface compliance
		var _ messaging.Receipt = receipt

		// Test basic properties
		assert.Equal(t, "test-receipt", receipt.ID())
		assert.NotNil(t, receipt.Context())
		assert.NotNil(t, receipt.Done())

		// Test completion
		result := messaging.PublishResult{
			MessageID: "test-msg-123",
			Success:   true,
		}
		receipt.Complete(result, nil)

		// Wait for completion
		select {
		case <-receipt.Done():
			completedResult, err := receipt.Result()
			assert.NoError(t, err)
			assert.Equal(t, result, completedResult)
		case <-time.After(2 * time.Second):
			t.Fatal("Receipt not completed within timeout")
		}
	})

	t.Run("ReceiptError", func(t *testing.T) {
		config := messaging.NewTestConfig()
		receipt := messaging.NewMockReceipt("test-receipt-error", config)

		// Complete with error
		testError := errors.New("test error")
		receipt.Complete(messaging.PublishResult{}, testError)

		// Wait for completion
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			assert.Error(t, err)
			assert.Equal(t, testError, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Receipt not completed within timeout")
		}
	})

	t.Run("ReceiptDoubleCompletion", func(t *testing.T) {
		config := messaging.NewTestConfig()
		receipt := messaging.NewMockReceipt("test-receipt-double", config)

		// Complete first time
		result1 := messaging.PublishResult{MessageID: "first"}
		receipt.Complete(result1, nil)

		// Try to complete again
		result2 := messaging.PublishResult{MessageID: "second"}
		receipt.Complete(result2, nil)

		// Wait for completion
		select {
		case <-receipt.Done():
			completedResult, err := receipt.Result()
			assert.NoError(t, err)
			// Should still have first result
			assert.Equal(t, result1, completedResult)
		case <-time.After(2 * time.Second):
			t.Fatal("Receipt not completed within timeout")
		}
	})
}

// TestTransportFactoryInterface tests the TransportFactory interface
func TestTransportFactoryInterface(t *testing.T) {
	t.Run("MockTransportFactory", func(t *testing.T) {
		factory := &MockTransportFactory{name: "test-factory"}

		// Verify interface compliance
		var _ messaging.TransportFactory = factory

		// Test factory methods
		assert.Equal(t, "test-factory", factory.Name())

		ctx := context.Background()
		config := &messaging.Config{Transport: "test-factory"}

		// Test validation
		err := factory.ValidateConfig(config)
		assert.NoError(t, err)

		// Test publisher creation
		publisher, err := factory.NewPublisher(ctx, config)
		assert.NoError(t, err)
		assert.NotNil(t, publisher)

		// Test consumer creation
		consumer, err := factory.NewConsumer(ctx, config)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
	})

	t.Run("TransportFactoryValidation", func(t *testing.T) {
		factory := &MockTransportFactory{name: "validation-test"}

		// Test nil config
		err := factory.ValidateConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")

		// Test invalid transport
		config := &messaging.Config{Transport: "wrong-transport"}
		err = factory.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transport")
	})
}

// MockTransportFactory implements TransportFactory for testing
type MockTransportFactory struct {
	name string
}

func (f *MockTransportFactory) NewPublisher(ctx context.Context, cfg *messaging.Config) (messaging.Publisher, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if cfg.Transport != f.name {
		return nil, errors.New("invalid transport")
	}

	// Return a mock publisher
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)
	return transport.NewPublisher(nil, config.Observability), nil
}

func (f *MockTransportFactory) NewConsumer(ctx context.Context, cfg *messaging.Config) (messaging.Consumer, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if cfg.Transport != f.name {
		return nil, errors.New("invalid transport")
	}

	// Return a mock consumer
	config := messaging.NewTestConfig()
	transport := messaging.NewMockTransport(config)
	return transport.NewConsumer(nil, config.Observability), nil
}

func (f *MockTransportFactory) ValidateConfig(cfg *messaging.Config) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if cfg.Transport != f.name {
		return errors.New("invalid transport")
	}
	return nil
}

func (f *MockTransportFactory) Name() string {
	return f.name
}

// TestErrorScenarios tests various error scenarios
func TestErrorScenarios(t *testing.T) {
	t.Run("ContextCancellation", func(t *testing.T) {
		config := messaging.NewTestConfig()
		transport := messaging.NewMockTransport(config)
		publisher := transport.NewPublisher(nil, config.Observability)

		// Test context cancellation during publish
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		factory := messaging.NewTestMessageFactory()
		msg := factory.CreateJSONMessage("test-data")

		receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
		assert.NoError(t, err) // Mock implementation doesn't check context immediately
		assert.NotNil(t, receipt)

		// Wait for completion
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "context cancelled")
		case <-time.After(2 * time.Second):
			t.Fatal("Receipt not completed within timeout")
		}
	})

	t.Run("TimeoutScenarios", func(t *testing.T) {
		config := messaging.NewTestConfig()
		config.Latency = 2 * time.Second // Set high latency
		transport := messaging.NewMockTransport(config)

		// Test connection timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := transport.Connect(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection timeout")
	})

	t.Run("ConcurrentRegistryAccess", func(t *testing.T) {
		registry := messaging.NewTransportRegistry()
		var wg sync.WaitGroup
		const numOperations = 100

		// Test concurrent registration and lookup
		for i := 0; i < numOperations; i++ {
			wg.Add(2)
			go func(id int) {
				defer wg.Done()
				factory := &MockTransportFactory{name: fmt.Sprintf("concurrent-%d", id)}
				registry.Register(fmt.Sprintf("concurrent-%d", id), factory)
			}(i)

			go func(id int) {
				defer wg.Done()
				registry.Get(fmt.Sprintf("concurrent-%d", id))
			}(i)
		}

		wg.Wait()
		// Should not panic and should have registered all factories
		names := registry.List()
		assert.Len(t, names, numOperations)
	})
}

// TestTransportIntegrationScenarios tests integration scenarios
func TestTransportIntegrationScenarios(t *testing.T) {
	t.Run("FullPublishConsumeCycle", func(t *testing.T) {
		// Register a transport factory
		mockFactory := &MockTransportFactory{name: "integration-test"}
		messaging.RegisterTransport("integration-test", mockFactory)

		ctx := context.Background()
		config := &messaging.Config{Transport: "integration-test"}

		// Create publisher and consumer
		publisher, err := messaging.NewPublisher(ctx, config)
		require.NoError(t, err)
		defer publisher.Close(ctx)

		consumer, err := messaging.NewConsumer(ctx, config)
		require.NoError(t, err)
		// Note: Consumer interface doesn't have Close method, only Start/Stop

		// Test the full cycle
		var receivedMessages []messaging.Message
		var mu sync.Mutex
		handler := messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
			mu.Lock()
			receivedMessages = append(receivedMessages, delivery.Message)
			mu.Unlock()
			return messaging.Ack, nil
		})

		// Start consumer
		err = consumer.Start(ctx, handler)
		require.NoError(t, err)

		// Publish messages
		factory := messaging.NewTestMessageFactory()
		messages := factory.CreateBulkMessages(5, `{"test": "message-%d"}`)

		for _, msg := range messages {
			receipt, err := publisher.PublishAsync(ctx, "test.exchange", msg)
			assert.NoError(t, err)
			assert.NotNil(t, receipt)

			// Wait for completion
			select {
			case <-receipt.Done():
				result, err := receipt.Result()
				assert.NoError(t, err)
				assert.True(t, result.Success)
			case <-time.After(2 * time.Second):
				t.Fatal("Receipt not completed within timeout")
			}
		}

		// Let the consumer run for a bit to process auto-generated messages
		time.Sleep(500 * time.Millisecond)

		// Stop consumer
		err = consumer.Stop(ctx)
		assert.NoError(t, err)

		// Verify that the consumer was able to process messages (auto-generated by mock)
		mu.Lock()
		messageCount := len(receivedMessages)
		mu.Unlock()
		assert.Greater(t, messageCount, 0, "Consumer should have processed auto-generated messages")
	})
}
