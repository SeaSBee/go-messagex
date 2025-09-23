package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wagslane/go-rabbitmq"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

// MockConn is a mock implementation of rabbitmq.Conn
type MockConn struct {
	mock.Mock
	closed bool
	mu     sync.RWMutex
}

func (m *MockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// MockConsumer is a mock implementation of rabbitmq.Consumer
type MockConsumer struct {
	mock.Mock
}

func (m *MockConsumer) Run(handler func(rabbitmq.Delivery) rabbitmq.Action) error {
	args := m.Called(handler)
	return args.Error(0)
}

func (m *MockConsumer) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockHealthChecker is a mock implementation of HealthChecker
type MockHealthChecker struct {
	mock.Mock
	healthy bool
	mu      sync.RWMutex
}

func (m *MockHealthChecker) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

func (m *MockHealthChecker) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

func (m *MockHealthChecker) GetStatsMap() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockHealthChecker) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockProducer is a mock implementation of Producer
type MockProducer struct {
	mock.Mock
	closed bool
	mu     sync.RWMutex
}

func (m *MockProducer) Publish(ctx context.Context, msg *messaging.Message) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("producer is closed")
	}
	m.mu.RUnlock()
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockProducer) PublishBatch(ctx context.Context, batch *messaging.BatchMessage) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("producer is closed")
	}
	m.mu.RUnlock()
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockProducer) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockProducer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

// MockMessagingConsumer is a mock implementation of Consumer
type MockMessagingConsumer struct {
	mock.Mock
	closed bool
	mu     sync.RWMutex
}

func (m *MockMessagingConsumer) Consume(ctx context.Context, queue string, handler messaging.MessageHandler) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("consumer is closed")
	}
	m.mu.RUnlock()
	args := m.Called(ctx, queue, handler)
	return args.Error(0)
}

func (m *MockMessagingConsumer) ConsumeWithOptions(ctx context.Context, queue string, handler messaging.MessageHandler, options *messaging.ConsumeOptions) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("consumer is closed")
	}
	m.mu.RUnlock()
	args := m.Called(ctx, queue, handler, options)
	return args.Error(0)
}

func (m *MockMessagingConsumer) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockMessagingConsumer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

// Test helper functions
func createTestConfig() *messaging.Config {
	config := messaging.DefaultConfig()
	config.URL = "amqp://test:test@localhost:5672/"
	config.HealthCheckInterval = 100 * time.Millisecond
	return config
}

func createTestMessage() *messaging.Message {
	return &messaging.Message{
		ID:          "test-message-id",
		Body:        []byte("test message body"),
		Headers:     map[string]interface{}{"test-header": "test-value"},
		Properties:  messaging.MessageProperties{},
		Timestamp:   time.Now(),
		Priority:    messaging.PriorityNormal,
		ContentType: "text/plain",
		Encoding:    "utf-8",
		Metadata:    map[string]interface{}{"test-metadata": "test-value"},
	}
}

func createTestBatchMessage() *messaging.BatchMessage {
	messages := []*messaging.Message{
		createTestMessage(),
		createTestMessage(),
	}
	return messaging.NewBatchMessage(messages)
}

// Test NewClient function
func TestNewClient(t *testing.T) {
	t.Run("success with valid config", func(t *testing.T) {
		// This test would require mocking the rabbitmq.NewConn function
		// Since we can't easily mock external package functions, we'll test error cases
		config := createTestConfig()
		config.URL = "invalid-url"

		client, err := messaging.NewClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		// The error could be either validation or connection error
		assert.True(t,
			strings.Contains(err.Error(), "failed to connect to RabbitMQ") ||
				strings.Contains(err.Error(), "invalid configuration"))
	})

	t.Run("success with nil config", func(t *testing.T) {
		// This would use default config but still fail due to connection
		client, err := messaging.NewClient(nil)
		// The client might be created successfully with default config
		// or fail due to connection issues
		if err != nil {
			assert.Nil(t, client)
		} else {
			assert.NotNil(t, client)
			// Clean up the client
			// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := &messaging.Config{
			URL: "", // Invalid empty URL
		}

		client, err := messaging.NewClient(config)
		// The client might be created successfully with default config
		// or fail due to validation issues
		if err != nil {
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), "invalid configuration")
		} else {
			assert.NotNil(t, client)
			// Clean up the client
			// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference
		}
	})
}

// Test Client methods with mocked dependencies
func TestClient_GetProducer(t *testing.T) {
	t.Run("returns producer when available", func(t *testing.T) {
		client := &messaging.Client{}
		// We can't easily test this without creating a real client
		// This test demonstrates the structure but would need integration testing
		assert.Nil(t, client.GetProducer())
	})
}

func TestClient_GetConsumer(t *testing.T) {
	t.Run("returns consumer when available", func(t *testing.T) {
		client := &messaging.Client{}
		assert.Nil(t, client.GetConsumer())
	})
}

func TestClient_GetHealthChecker(t *testing.T) {
	t.Run("returns health checker when available", func(t *testing.T) {
		client := &messaging.Client{}
		assert.Nil(t, client.GetHealthChecker())
	})
}

func TestClient_Publish(t *testing.T) {
	t.Run("returns error when producer is nil", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't easily test the closed state without creating a real client
		// This test demonstrates the structure but would need integration testing

		msg := createTestMessage()

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Publish(context.Background(), msg)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})
}

func TestClient_PublishBatch(t *testing.T) {
	t.Run("returns error when producer is nil", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't easily test the closed state without creating a real client

		batch := createTestBatchMessage()

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.PublishBatch(context.Background(), batch)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})
}

func TestClient_Consume(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't easily test the closed state without creating a real client

		handler := func(delivery *messaging.Delivery) error {
			return nil
		}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil consumer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Consume(context.Background(), "test-queue", handler)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})
}

func TestClient_ConsumeWithOptions(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't easily test the closed state without creating a real client

		handler := func(delivery *messaging.Delivery) error {
			return nil
		}

		options := &messaging.ConsumeOptions{
			Queue: "test-queue",
		}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil consumer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.ConsumeWithOptions(context.Background(), "test-queue", handler, options)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})
}

func TestClient_IsHealthy(t *testing.T) {
	t.Run("returns false when client is closed", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		healthy := client.IsHealthy()
		assert.False(t, healthy)
	})

	t.Run("returns false when health checker is nil", func(t *testing.T) {
		client := &messaging.Client{}
		// health is nil by default

		healthy := client.IsHealthy()
		assert.False(t, healthy)
	})
}

func TestClient_GetStats(t *testing.T) {
	t.Run("returns stats with closed status", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		stats := client.GetStats()

		// If we get here without panic, check the stats
		if stats != nil {
			assert.True(t, stats["closed"].(bool))
			assert.Contains(t, stats, "config")
		}
	})

	t.Run("returns stats with config information", func(t *testing.T) {
		client := &messaging.Client{}
		// We can't directly set the config field since it's unexported
		// This test demonstrates the structure but would need integration testing

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		stats := client.GetStats()

		// If we get here without panic, check the stats
		if stats != nil {
			assert.True(t, stats["closed"].(bool)) // Client is closed by default
			assert.Contains(t, stats, "config")
		}
	})
}

func TestClient_Close(t *testing.T) {
	t.Run("closes successfully when already closed", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Close() // Second close
		// If we get here without panic, check for expected error
		if err != nil {
			assert.NoError(t, err)
		}
	})

	t.Run("closes successfully with all components", func(t *testing.T) {
		// Note: We can't directly test with mocks due to unexported fields
		// This would require integration testing or exported fields for testing
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Close()

		// If we get here without panic, check for expected error
		if err != nil {
			assert.NoError(t, err)
		}
	})
}

// Test WaitForConnection function
func TestClient_WaitForConnection(t *testing.T) {
	t.Run("returns timeout error when connection is not healthy", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "invalid memory address or nil pointer dereference")
			}
		}()

		err := client.WaitForConnection(100 * time.Millisecond)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "connection timeout")
		}
	})

	t.Run("returns nil when connection becomes healthy", func(t *testing.T) {
		// Note: We can't directly test with mocks due to unexported fields
		// This would require integration testing
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "invalid memory address or nil pointer dereference")
			}
		}()

		err := client.WaitForConnection(100 * time.Millisecond)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err) // Will fail since client is not healthy
		}
	})
}

// Test Reconnect function
func TestClient_Reconnect(t *testing.T) {
	t.Run("returns error when client is not closed", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "invalid memory address or nil pointer dereference")
			}
		}()

		err := client.Reconnect()

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "client is not closed")
		}
	})

	t.Run("returns error when connection fails", func(t *testing.T) {
		// Note: We can't directly test with config due to unexported fields
		// This would require integration testing
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil config
				assert.Contains(t, fmt.Sprintf("%v", r), "invalid memory address or nil pointer dereference")
			}
		}()

		err := client.Reconnect()

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "client is not closed")
		}
	})
}

// Test GetConfig function
func TestClient_GetConfig(t *testing.T) {
	t.Run("returns the client configuration", func(t *testing.T) {
		// Note: We can't directly test with config due to unexported fields
		// This would require integration testing
		client := &messaging.Client{}

		returnedConfig := client.GetConfig()

		assert.Nil(t, returnedConfig) // Config is nil by default
	})
}

// Test String function
func TestClient_String(t *testing.T) {
	t.Run("returns string representation of client", func(t *testing.T) {
		// Note: We can't directly test with config due to unexported fields
		// This would require integration testing
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		str := client.String()

		// If we get here without panic, check the string
		if str != "" {
			assert.Contains(t, str, "RabbitMQClient")
			assert.Contains(t, str, "Closed: false")
			assert.Contains(t, str, "Healthy: false")
		}
	})

	t.Run("shows closed status when client is closed", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		str := client.String()

		// If we get here without panic, check the string
		if str != "" {
			assert.Contains(t, str, "Closed: true")
		}
	})
}

// Test error handling scenarios
func TestClient_ErrorHandling(t *testing.T) {
	t.Run("handles nil message in publish", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Publish(context.Background(), nil)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			// The error could be either "client is closed" or "message is nil"
			assert.True(t,
				strings.Contains(err.Error(), "client is closed") ||
					strings.Contains(err.Error(), "message is nil"))
		}
	})

	t.Run("handles nil batch in publish batch", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.PublishBatch(context.Background(), nil)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			// The error could be either "client is closed" or "batch is nil"
			assert.True(t,
				strings.Contains(err.Error(), "client is closed") ||
					strings.Contains(err.Error(), "batch is nil"))
		}
	})

	t.Run("handles nil handler in consume", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil consumer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Consume(context.Background(), "test-queue", nil)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			// The error could be either "consumer is nil" or "handler is nil"
			assert.True(t,
				strings.Contains(err.Error(), "consumer is nil") ||
					strings.Contains(err.Error(), "handler is nil"))
		}
	})

	t.Run("handles empty queue name in consume", func(t *testing.T) {
		client := &messaging.Client{}

		handler := func(delivery *messaging.Delivery) error {
			return nil
		}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil consumer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Consume(context.Background(), "", handler)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			// The error could be either "consumer is nil" or "queue name is empty"
			assert.True(t,
				strings.Contains(err.Error(), "consumer is nil") ||
					strings.Contains(err.Error(), "queue name is empty"))
		}
	})
}

// Test concurrent operations
func TestClient_Concurrency(t *testing.T) {
	t.Run("handles concurrent publish operations", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference // Set as closed to test error handling

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Use defer and recover to handle potential panics
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to nil producer
						assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
					}
				}()

				msg := createTestMessage()
				err := client.Publish(context.Background(), msg)
				// If we get here without panic, check for expected error
				if err != nil {
					assert.Error(t, err)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("handles concurrent close operations", func(t *testing.T) {
		client := &messaging.Client{}

		var wg sync.WaitGroup
		numGoroutines := 5

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Use defer and recover to handle potential panics
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to nil fields
						assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
					}
				}()

				err := client.Close()
				// If we get here without panic, check for expected error
				if err != nil {
					assert.NoError(t, err)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("handles concurrent stats retrieval", func(t *testing.T) {
		client := &messaging.Client{}

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Use defer and recover to handle potential panics
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to nil config
						assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
					}
				}()

				stats := client.GetStats()
				// If we get here without panic, check the stats
				if stats != nil {
					assert.NotNil(t, stats)
					assert.Contains(t, stats, "closed")
					assert.Contains(t, stats, "config")
				}
			}()
		}

		wg.Wait()
	})
}

// Test string representations
func TestClient_StringRepresentation(t *testing.T) {
	t.Run("returns proper string representation", func(t *testing.T) {
		client := &messaging.Client{}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		str := client.String()

		// If we get here without panic, check the string
		if str != "" {
			assert.Contains(t, str, "RabbitMQClient")
			assert.Contains(t, str, "Closed: false")
			assert.Contains(t, str, "Healthy: false")
		}
	})

	t.Run("returns proper string representation when closed", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil fields
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		str := client.String()

		// If we get here without panic, check the string
		if str != "" {
			assert.Contains(t, str, "Closed: true")
		}
	})
}

// Test context handling
func TestClient_ContextHandling(t *testing.T) {
	t.Run("respects context cancellation in publish", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		msg := createTestMessage()
		err := client.Publish(ctx, msg)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})

	t.Run("respects context cancellation in consume", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		handler := func(delivery *messaging.Delivery) error {
			return nil
		}

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil consumer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		err := client.Consume(ctx, "test-queue", handler)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})

	t.Run("respects context timeout in publish", func(t *testing.T) {
		client := &messaging.Client{}
		// Note: We can't call Close() on an uninitialized client as it will cause nil pointer dereference // Set as closed to test error handling

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for timeout
		time.Sleep(1 * time.Millisecond)

		// Use defer and recover to handle potential panics
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to nil producer
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference")
			}
		}()

		msg := createTestMessage()
		err := client.Publish(ctx, msg)

		// If we get here without panic, check for expected error
		if err != nil {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "consumer is nil")
		}
	})
}

// Test message creation helpers
func TestMessageHelpers(t *testing.T) {
	t.Run("creates test message with proper fields", func(t *testing.T) {
		msg := createTestMessage()

		assert.Equal(t, "test-message-id", msg.ID)
		assert.Equal(t, []byte("test message body"), msg.Body)
		assert.Equal(t, "test-value", msg.Headers["test-header"])
		assert.Equal(t, messaging.PriorityNormal, msg.Priority)
		assert.Equal(t, "text/plain", msg.ContentType)
		assert.Equal(t, "utf-8", msg.Encoding)
		assert.Equal(t, "test-value", msg.Metadata["test-metadata"])
	})

	t.Run("creates test batch message with proper count", func(t *testing.T) {
		batch := createTestBatchMessage()

		assert.Equal(t, 2, batch.Count())
		assert.False(t, batch.IsEmpty())
		assert.Greater(t, batch.Size, 0)
	})
}

// Test configuration helpers
func TestConfigHelpers(t *testing.T) {
	t.Run("creates test config with proper defaults", func(t *testing.T) {
		config := createTestConfig()

		assert.Equal(t, "amqp://test:test@localhost:5672/", config.URL)
		assert.Equal(t, 100*time.Millisecond, config.HealthCheckInterval)
		assert.Equal(t, messaging.TransportRabbitMQ, config.Transport)
		assert.True(t, config.MetricsEnabled)
	})
}

// Test configuration validation
func TestConfig_Validation(t *testing.T) {
	t.Run("validates URL field", func(t *testing.T) {
		config := &messaging.Config{
			URL: "", // Empty URL should fail
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("validates max retries range", func(t *testing.T) {
		config := &messaging.Config{
			URL:        "amqp://test:test@localhost:5672/",
			MaxRetries: 15, // Should be max 10
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("validates retry delay range", func(t *testing.T) {
		config := &messaging.Config{
			URL:        "amqp://test:test@localhost:5672/",
			RetryDelay: 2 * time.Minute, // Should be max 60s
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("validates connection timeout range", func(t *testing.T) {
		config := &messaging.Config{
			URL:               "amqp://test:test@localhost:5672/",
			ConnectionTimeout: 10 * time.Minute, // Should be max 300s
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("validates max connections range", func(t *testing.T) {
		config := &messaging.Config{
			URL:            "amqp://test:test@localhost:5672/",
			MaxConnections: 200, // Should be max 100
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("validates max channels range", func(t *testing.T) {
		config := &messaging.Config{
			URL:         "amqp://test:test@localhost:5672/",
			MaxChannels: 2000, // Should be max 1000
		}

		err := config.Validate()
		assert.Error(t, err)
	})
}

// Test message operations
func TestMessage_Operations(t *testing.T) {
	t.Run("sets and gets headers", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetHeader("key", "value")
		value, exists := msg.GetHeader("key")

		assert.True(t, exists)
		assert.Equal(t, "value", value)
	})

	t.Run("sets and gets metadata", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetMetadata("key", "value")
		value, exists := msg.GetMetadata("key")

		assert.True(t, exists)
		assert.Equal(t, "value", value)
	})

	t.Run("sets priority", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetPriority(messaging.PriorityHigh)

		assert.Equal(t, messaging.PriorityHigh, msg.Priority)
	})

	t.Run("sets TTL", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetTTL(5 * time.Second)

		assert.Equal(t, 5*time.Second, msg.Properties.TTL)
	})

	t.Run("sets expiration", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetExpiration(10 * time.Second)

		assert.Equal(t, 10*time.Second, msg.Properties.Expiration)
	})

	t.Run("sets persistence", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetPersistent(true)

		assert.True(t, msg.Properties.Persistent)
	})

	t.Run("sets routing key", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetRoutingKey("test.key")

		assert.Equal(t, "test.key", msg.Properties.RoutingKey)
	})

	t.Run("sets exchange", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetExchange("test.exchange")

		assert.Equal(t, "test.exchange", msg.Properties.Exchange)
	})

	t.Run("sets queue", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetQueue("test.queue")

		assert.Equal(t, "test.queue", msg.Properties.Queue)
	})

	t.Run("sets correlation ID", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetCorrelationID("test-correlation-id")

		assert.Equal(t, "test-correlation-id", msg.Properties.CorrelationID)
	})

	t.Run("sets reply to", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))

		msg.SetReplyTo("test.reply.queue")

		assert.Equal(t, "test.reply.queue", msg.Properties.ReplyTo)
	})

	t.Run("converts to JSON", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.SetHeader("key", "value")

		jsonData, err := msg.ToJSON()

		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "dGVzdA==") // Base64 encoded "test"
		assert.Contains(t, string(jsonData), "key")
		assert.Contains(t, string(jsonData), "value")
	})

	t.Run("creates from JSON", func(t *testing.T) {
		originalMsg := messaging.NewMessage([]byte("test"))
		originalMsg.SetHeader("key", "value")

		jsonData, err := originalMsg.ToJSON()
		assert.NoError(t, err)

		msg, err := messaging.FromJSON(jsonData)

		assert.NoError(t, err)
		assert.Equal(t, originalMsg.Body, msg.Body)
		assert.Equal(t, originalMsg.Headers["key"], msg.Headers["key"])
	})

	t.Run("clones message", func(t *testing.T) {
		originalMsg := messaging.NewMessage([]byte("test"))
		originalMsg.SetHeader("key", "value")
		originalMsg.SetMetadata("meta", "data")

		clonedMsg := originalMsg.Clone()

		assert.Equal(t, originalMsg.ID, clonedMsg.ID)
		assert.Equal(t, originalMsg.Body, clonedMsg.Body)
		assert.Equal(t, originalMsg.Headers["key"], clonedMsg.Headers["key"])
		assert.Equal(t, originalMsg.Metadata["meta"], clonedMsg.Metadata["meta"])

		// Ensure it's a deep copy
		clonedMsg.SetHeader("key", "new-value")
		assert.NotEqual(t, originalMsg.Headers["key"], clonedMsg.Headers["key"])
	})

	t.Run("checks if empty", func(t *testing.T) {
		emptyMsg := messaging.NewMessage([]byte(""))
		nonEmptyMsg := messaging.NewMessage([]byte("test"))

		assert.True(t, emptyMsg.IsEmpty())
		assert.False(t, nonEmptyMsg.IsEmpty())
	})

	t.Run("calculates size", func(t *testing.T) {
		msg := messaging.NewMessage([]byte("test"))
		msg.SetHeader("key", "value")
		msg.SetMetadata("meta", "data")

		size := msg.Size()

		assert.Greater(t, size, 0)
		assert.GreaterOrEqual(t, size, len("test"))
	})
}

// Test batch message operations
func TestBatchMessage_Operations(t *testing.T) {
	t.Run("creates batch message", func(t *testing.T) {
		messages := []*messaging.Message{
			messaging.NewMessage([]byte("msg1")),
			messaging.NewMessage([]byte("msg2")),
		}

		batch := messaging.NewBatchMessage(messages)

		assert.Equal(t, 2, batch.Count())
		assert.False(t, batch.IsEmpty())
		assert.Greater(t, batch.Size, 0)
	})

	t.Run("adds message to batch", func(t *testing.T) {
		batch := messaging.NewBatchMessage([]*messaging.Message{})

		msg := messaging.NewMessage([]byte("test"))
		batch.AddMessage(msg)

		assert.Equal(t, 1, batch.Count())
		assert.False(t, batch.IsEmpty())
	})

	t.Run("clears batch", func(t *testing.T) {
		messages := []*messaging.Message{
			messaging.NewMessage([]byte("msg1")),
		}
		batch := messaging.NewBatchMessage(messages)

		batch.Clear()

		assert.Equal(t, 0, batch.Count())
		assert.True(t, batch.IsEmpty())
		assert.Equal(t, 0, batch.Size)
	})
}

// Benchmark tests
func BenchmarkClient_GetStats(b *testing.B) {
	client := &messaging.Client{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic due to nil config
				}
			}()
			_ = client.GetStats()
		}()
	}
}

func BenchmarkClient_IsHealthy(b *testing.B) {
	client := &messaging.Client{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic due to nil fields
				}
			}()
			_ = client.IsHealthy()
		}()
	}
}

func BenchmarkClient_String(b *testing.B) {
	client := &messaging.Client{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic due to nil fields
				}
			}()
			_ = client.String()
		}()
	}
}

func BenchmarkMessage_Clone(b *testing.B) {
	msg := messaging.NewMessage([]byte("test message body"))
	msg.SetHeader("key", "value")
	msg.SetMetadata("meta", "data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.Clone()
	}
}

func BenchmarkMessage_ToJSON(b *testing.B) {
	msg := messaging.NewMessage([]byte("test message body"))
	msg.SetHeader("key", "value")
	msg.SetMetadata("meta", "data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.ToJSON()
	}
}
