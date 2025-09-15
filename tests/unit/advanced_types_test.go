package unit

import (
	"context"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// Mock storage implementation for testing
type mockStorage struct {
	storeFunc    func(ctx context.Context, message *messaging.Message) error
	retrieveFunc func(ctx context.Context, messageID string) (*messaging.Message, error)
	deleteFunc   func(ctx context.Context, messageID string) error
	cleanupFunc  func(ctx context.Context, before time.Time) error
	closeFunc    func(ctx context.Context) error
}

func (m *mockStorage) Store(ctx context.Context, message *messaging.Message) error {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, message)
	}
	return nil
}

func (m *mockStorage) Retrieve(ctx context.Context, messageID string) (*messaging.Message, error) {
	if m.retrieveFunc != nil {
		return m.retrieveFunc(ctx, messageID)
	}
	return &messaging.Message{ID: messageID}, nil
}

func (m *mockStorage) Delete(ctx context.Context, messageID string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, messageID)
	}
	return nil
}

func (m *mockStorage) Cleanup(ctx context.Context, before time.Time) error {
	if m.cleanupFunc != nil {
		return m.cleanupFunc(ctx, before)
	}
	return nil
}

func (m *mockStorage) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

// Mock transport for testing
type mockTransport struct {
	channel interface{}
}

func (m *mockTransport) GetChannel() interface{} {
	return m.channel
}

func TestNewMessagePersistence(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	testCases := []struct {
		name          string
		config        *messaging.MessagePersistenceConfig
		observability *messaging.ObservabilityContext
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil config",
			config:        nil,
			observability: observability,
			expectError:   true,
			errorContains: "config cannot be nil",
		},
		{
			name: "nil observability",
			config: &messaging.MessagePersistenceConfig{
				Enabled: true,
			},
			observability: nil,
			expectError:   true,
			errorContains: "observability cannot be nil",
		},
		{
			name: "disabled persistence",
			config: &messaging.MessagePersistenceConfig{
				Enabled: false,
			},
			observability: observability,
			expectError:   false,
		},
		{
			name: "invalid cleanup interval",
			config: &messaging.MessagePersistenceConfig{
				Enabled:         true,
				CleanupInterval: 0,
			},
			observability: observability,
			expectError:   true,
			errorContains: "cleanup interval must be positive",
		},
		{
			name: "invalid message TTL",
			config: &messaging.MessagePersistenceConfig{
				Enabled:         true,
				CleanupInterval: time.Hour,
				MessageTTL:      0,
			},
			observability: observability,
			expectError:   true,
			errorContains: "message TTL must be positive",
		},
		{
			name: "unsupported storage type",
			config: &messaging.MessagePersistenceConfig{
				Enabled:         true,
				CleanupInterval: time.Hour,
				MessageTTL:      24 * time.Hour,
				StorageType:     "unsupported",
			},
			observability: observability,
			expectError:   true,
			errorContains: "unsupported storage type",
		},
		{
			name: "valid memory storage",
			config: &messaging.MessagePersistenceConfig{
				Enabled:         true,
				StorageType:     "memory",
				CleanupInterval: time.Hour,
				MessageTTL:      24 * time.Hour,
				MaxStorageSize:  1000,
			},
			observability: observability,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := messaging.NewMessagePersistence(tc.config, tc.observability)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && mp == nil {
				t.Errorf("Expected MessagePersistence instance for test case '%s', but got nil", tc.name)
			}

			// Clean up MessagePersistence if it was created successfully
			if mp != nil {
				mp.Close(context.Background())
			}
		})
	}
}

func TestMessagePersistenceStore(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessagePersistenceConfig{
		Enabled:         true,
		StorageType:     "memory",
		CleanupInterval: time.Hour,
		MessageTTL:      24 * time.Hour,
		MaxStorageSize:  1000,
	}

	mp, err := messaging.NewMessagePersistence(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessagePersistence: %v", err)
	}
	defer mp.Close(context.Background())

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "nil message",
			ctx:           context.Background(),
			message:       nil,
			expectError:   true,
			errorContains: "message cannot be nil",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			message:       &messaging.Message{ID: "", Body: []byte("test")},
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:          "nil message body",
			ctx:           context.Background(),
			message:       &messaging.Message{ID: "test", Body: nil},
			expectError:   true,
			errorContains: "message body cannot be nil",
		},
		{
			name:        "valid message",
			ctx:         context.Background(),
			message:     &messaging.Message{ID: "test", Key: "test.key", ContentType: "application/json", Body: []byte("test")},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mp.Store(tc.ctx, tc.message)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestMessagePersistenceRetrieve(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessagePersistenceConfig{
		Enabled:         true,
		StorageType:     "memory",
		CleanupInterval: time.Hour,
		MessageTTL:      24 * time.Hour,
		MaxStorageSize:  1000,
	}

	mp, err := messaging.NewMessagePersistence(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessagePersistence: %v", err)
	}
	defer mp.Close(context.Background())

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			messageID:     "test",
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:          "valid message ID",
			ctx:           context.Background(),
			messageID:     "test",
			expectError:   true,
			errorContains: "message not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mp.Retrieve(tc.ctx, tc.messageID)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}

		})
	}
}

func TestMessagePersistenceDelete(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessagePersistenceConfig{
		Enabled:         true,
		StorageType:     "memory",
		CleanupInterval: time.Hour,
		MessageTTL:      24 * time.Hour,
		MaxStorageSize:  1000,
	}

	mp, err := messaging.NewMessagePersistence(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessagePersistence: %v", err)
	}
	defer mp.Close(context.Background())

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			messageID:     "test",
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:        "valid message ID",
			ctx:         context.Background(),
			messageID:   "test",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mp.Delete(tc.ctx, tc.messageID)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestMessagePersistenceClose(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessagePersistenceConfig{
		Enabled:         true,
		StorageType:     "memory",
		CleanupInterval: time.Hour,
		MessageTTL:      24 * time.Hour,
		MaxStorageSize:  1000,
	}

	mp, err := messaging.NewMessagePersistence(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessagePersistence: %v", err)
	}
	defer mp.Close(context.Background())

	testCases := []struct {
		name          string
		ctx           context.Context
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:        "valid context",
			ctx:         context.Background(),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mp.Close(tc.ctx)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestMessagePersistenceDisabled(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessagePersistenceConfig{
		Enabled: false, // Disabled
	}

	mp, err := messaging.NewMessagePersistence(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessagePersistence: %v", err)
	}
	defer mp.Close(context.Background())

	// Test that operations return appropriate errors when disabled
	message := &messaging.Message{ID: "test", Key: "test.key", Body: []byte("test")}

	err = mp.Store(context.Background(), message)
	if err == nil {
		t.Error("Expected error when persistence is disabled")
	}
	if !contains(err.Error(), "persistence is disabled") {
		t.Errorf("Expected 'persistence is disabled' error, got: %v", err)
	}

	_, err = mp.Retrieve(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when persistence is disabled")
	}
	if !contains(err.Error(), "persistence is disabled") {
		t.Errorf("Expected 'persistence is disabled' error, got: %v", err)
	}

	err = mp.Delete(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when persistence is disabled")
	}
	if !contains(err.Error(), "persistence is disabled") {
		t.Errorf("Expected 'persistence is disabled' error, got: %v", err)
	}
}

func TestNewDeadLetterQueue(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	transport := &mockTransport{}

	testCases := []struct {
		name          string
		config        *messaging.DeadLetterQueueConfig
		transport     interface{ GetChannel() interface{} }
		observability *messaging.ObservabilityContext
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil config (should use defaults)",
			config:        nil,
			transport:     transport,
			observability: observability,
			expectError:   false,
		},
		{
			name:          "nil transport (should use no-op)",
			config:        &messaging.DeadLetterQueueConfig{},
			transport:     nil,
			observability: observability,
			expectError:   false,
		},
		{
			name:          "nil observability (should create default)",
			config:        &messaging.DeadLetterQueueConfig{},
			transport:     transport,
			observability: nil,
			expectError:   false,
		},
		{
			name: "valid configuration",
			config: &messaging.DeadLetterQueueConfig{
				Enabled:    true,
				Exchange:   "dlx",
				Queue:      "dlq",
				RoutingKey: "dlq",
			},
			transport:     transport,
			observability: observability,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dlq, err := messaging.NewDeadLetterQueue(tc.config, tc.transport, tc.observability)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && dlq == nil {
				t.Errorf("Expected DeadLetterQueue instance for test case '%s', but got nil", tc.name)
			}
		})
	}
}

func TestDeadLetterQueueSendToDLQ(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.DeadLetterQueueConfig{
		Enabled:    true,
		Exchange:   "dlx",
		Queue:      "dlq",
		RoutingKey: "dlq",
	}

	transport := &mockTransport{}
	dlq, err := messaging.NewDeadLetterQueue(config, transport, observability)
	if err != nil {
		t.Fatalf("Failed to create DeadLetterQueue: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		reason        string
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			reason:        "test reason",
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "nil message",
			ctx:           context.Background(),
			message:       nil,
			reason:        "test reason",
			expectError:   true,
			errorContains: "message cannot be nil",
		},
		{
			name:          "empty reason",
			ctx:           context.Background(),
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			reason:        "",
			expectError:   true,
			errorContains: "reason cannot be empty",
		},
		{
			name:          "valid message and reason",
			ctx:           context.Background(),
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			reason:        "test reason",
			expectError:   true, // Currently returns "not yet implemented"
			errorContains: "not yet implemented",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := dlq.SendToDLQ(tc.ctx, tc.message, tc.reason)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestDeadLetterQueueReplayFromDLQ(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.DeadLetterQueueConfig{
		Enabled:    true,
		Exchange:   "dlx",
		Queue:      "dlq",
		RoutingKey: "dlq",
	}

	transport := &mockTransport{}
	dlq, err := messaging.NewDeadLetterQueue(config, transport, observability)
	if err != nil {
		t.Fatalf("Failed to create DeadLetterQueue: %v", err)
	}

	testCases := []struct {
		name             string
		ctx              context.Context
		targetExchange   string
		targetRoutingKey string
		limit            int
		expectError      bool
		errorContains    string
	}{
		{
			name:             "nil context",
			ctx:              nil,
			targetExchange:   "target",
			targetRoutingKey: "target",
			limit:            10,
			expectError:      true,
			errorContains:    "context cannot be nil",
		},
		{
			name:             "empty target exchange",
			ctx:              context.Background(),
			targetExchange:   "",
			targetRoutingKey: "target",
			limit:            10,
			expectError:      true,
			errorContains:    "target exchange cannot be empty",
		},
		{
			name:             "empty target routing key",
			ctx:              context.Background(),
			targetExchange:   "target",
			targetRoutingKey: "",
			limit:            10,
			expectError:      true,
			errorContains:    "target routing key cannot be empty",
		},
		{
			name:             "invalid limit",
			ctx:              context.Background(),
			targetExchange:   "target",
			targetRoutingKey: "target",
			limit:            0,
			expectError:      true,
			errorContains:    "limit must be positive",
		},
		{
			name:             "valid parameters",
			ctx:              context.Background(),
			targetExchange:   "target",
			targetRoutingKey: "target",
			limit:            10,
			expectError:      true, // Currently returns "not yet implemented"
			errorContains:    "not yet implemented",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count, err := dlq.ReplayFromDLQ(tc.ctx, tc.targetExchange, tc.targetRoutingKey, tc.limit)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && count < 0 {
				t.Errorf("Expected non-negative count for test case '%s', but got %d", tc.name, count)
			}
		})
	}
}

func TestDeadLetterQueueClose(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.DeadLetterQueueConfig{
		Enabled:    true,
		Exchange:   "dlx",
		Queue:      "dlq",
		RoutingKey: "dlq",
	}

	transport := &mockTransport{}
	dlq, err := messaging.NewDeadLetterQueue(config, transport, observability)
	if err != nil {
		t.Fatalf("Failed to create DeadLetterQueue: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:        "valid context",
			ctx:         context.Background(),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := dlq.Close(tc.ctx)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestDeadLetterQueueDisabled(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.DeadLetterQueueConfig{
		Enabled: false, // Disabled
	}

	transport := &mockTransport{}
	dlq, err := messaging.NewDeadLetterQueue(config, transport, observability)
	if err != nil {
		t.Fatalf("Failed to create DeadLetterQueue: %v", err)
	}

	// Test that operations return appropriate errors when disabled
	message := &messaging.Message{ID: "test", Body: []byte("test")}

	err = dlq.SendToDLQ(context.Background(), message, "test reason")
	if err == nil {
		t.Error("Expected error when DLQ is disabled")
	}
	if !contains(err.Error(), "DLQ is disabled") {
		t.Errorf("Expected 'DLQ is disabled' error, got: %v", err)
	}

	_, err = dlq.ReplayFromDLQ(context.Background(), "target", "target", 10)
	if err == nil {
		t.Error("Expected error when DLQ is disabled")
	}
	if !contains(err.Error(), "DLQ is disabled") {
		t.Errorf("Expected 'DLQ is disabled' error, got: %v", err)
	}
}

func TestNewMessageTransformation(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	testCases := []struct {
		name          string
		config        *messaging.MessageTransformationConfig
		observability *messaging.ObservabilityContext
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil config (should use defaults)",
			config:        nil,
			observability: observability,
			expectError:   false,
		},
		{
			name:          "nil observability (should create default)",
			config:        &messaging.MessageTransformationConfig{},
			observability: nil,
			expectError:   false,
		},
		{
			name: "valid configuration",
			config: &messaging.MessageTransformationConfig{
				Enabled:             true,
				CompressionEnabled:  true,
				CompressionLevel:    6,
				SerializationFormat: "json",
				SchemaValidation:    true,
			},
			observability: observability,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mt, err := messaging.NewMessageTransformation(tc.config, tc.observability)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && mt == nil {
				t.Errorf("Expected MessageTransformation instance for test case '%s', but got nil", tc.name)
			}
		})
	}
}

func TestMessageTransformationTransform(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessageTransformationConfig{
		Enabled:             true,
		CompressionEnabled:  false,
		SerializationFormat: "json",
		SchemaValidation:    false,
	}

	mt, err := messaging.NewMessageTransformation(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessageTransformation: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "nil message",
			ctx:           context.Background(),
			message:       nil,
			expectError:   true,
			errorContains: "message cannot be nil",
		},
		{
			name:        "valid message",
			ctx:         context.Background(),
			message:     &messaging.Message{ID: "test", Key: "test.key", Body: []byte("test")},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transformed, err := mt.Transform(tc.ctx, tc.message)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && transformed == nil {
				t.Errorf("Expected transformed message for test case '%s', but got nil", tc.name)
			}
		})
	}
}

func TestMessageTransformationWithCompression(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessageTransformationConfig{
		Enabled:             true,
		CompressionEnabled:  true, // Enable compression
		SerializationFormat: "json",
		SchemaValidation:    false,
	}

	mt, err := messaging.NewMessageTransformation(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessageTransformation: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that compression returns "not yet implemented" error
	_, err = mt.Transform(context.Background(), message)
	if err == nil {
		t.Error("Expected error when compression is enabled")
	}
	if !contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestMessageTransformationWithSchemaValidation(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessageTransformationConfig{
		Enabled:             true,
		CompressionEnabled:  false,
		SerializationFormat: "json",
		SchemaValidation:    true, // Enable schema validation
	}

	mt, err := messaging.NewMessageTransformation(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessageTransformation: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that schema validation returns "not yet implemented" error
	_, err = mt.Transform(context.Background(), message)
	if err == nil {
		t.Error("Expected error when schema validation is enabled")
	}
	if !contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestMessageTransformationClose(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessageTransformationConfig{
		Enabled: true,
	}

	mt, err := messaging.NewMessageTransformation(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessageTransformation: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:        "valid context",
			ctx:         context.Background(),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mt.Close(tc.ctx)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestMessageTransformationDisabled(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.MessageTransformationConfig{
		Enabled: false, // Disabled
	}

	mt, err := messaging.NewMessageTransformation(config, observability)
	if err != nil {
		t.Fatalf("Failed to create MessageTransformation: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that transform returns original message when disabled
	transformed, err := mt.Transform(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when transformation is disabled: %v", err)
	}
	if transformed != message {
		t.Error("Expected original message when transformation is disabled")
	}
}

func TestNewAdvancedRouting(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	testCases := []struct {
		name          string
		config        *messaging.AdvancedRoutingConfig
		observability *messaging.ObservabilityContext
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil config (should use defaults)",
			config:        nil,
			observability: observability,
			expectError:   false,
		},
		{
			name:          "nil observability (should create default)",
			config:        &messaging.AdvancedRoutingConfig{},
			observability: nil,
			expectError:   false,
		},
		{
			name: "valid configuration with routing rules",
			config: &messaging.AdvancedRoutingConfig{
				Enabled: true,
				RoutingRules: []messaging.RoutingRule{
					{
						Name:             "test-rule",
						Condition:        "$.type == 'test'",
						TargetExchange:   "test-exchange",
						TargetRoutingKey: "test-key",
						Priority:         1,
						Enabled:          true,
					},
				},
			},
			observability: observability,
			expectError:   false,
		},
		{
			name: "valid configuration with filter rules",
			config: &messaging.AdvancedRoutingConfig{
				Enabled:          true,
				MessageFiltering: true,
				FilterRules: []messaging.FilterRule{
					{
						Name:      "test-filter",
						Condition: "$.priority > 5",
						Action:    "accept",
						Priority:  1,
						Enabled:   true,
					},
				},
			},
			observability: observability,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ar, err := messaging.NewAdvancedRouting(tc.config, tc.observability)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && ar == nil {
				t.Errorf("Expected AdvancedRouting instance for test case '%s', but got nil", tc.name)
			}
		})
	}
}

func TestAdvancedRoutingRoute(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled: true,
		RoutingRules: []messaging.RoutingRule{
			{
				Name:             "test-rule",
				Condition:        "$.type == 'test'",
				TargetExchange:   "test-exchange",
				TargetRoutingKey: "test-key",
				Priority:         1,
				Enabled:          true,
			},
		},
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "nil message",
			ctx:           context.Background(),
			message:       nil,
			expectError:   true,
			errorContains: "message cannot be nil",
		},
		{
			name:        "valid message",
			ctx:         context.Background(),
			message:     &messaging.Message{ID: "test", Key: "test.key", Body: []byte("test")},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exchange, routingKey, err := ar.Route(tc.ctx, tc.message)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError {
				// Currently returns empty strings due to "not yet implemented" condition evaluation
				if exchange != "" || routingKey != "" {
					t.Errorf("Expected empty exchange and routing key for test case '%s', got: %s, %s", tc.name, exchange, routingKey)
				}
			}
		})
	}
}

func TestAdvancedRoutingFilter(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled:          true,
		MessageFiltering: true,
		FilterRules: []messaging.FilterRule{
			{
				Name:      "test-filter",
				Condition: "$.priority > 5",
				Action:    "accept",
				Priority:  1,
				Enabled:   true,
			},
		},
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			message:       &messaging.Message{ID: "test", Body: []byte("test")},
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:          "nil message",
			ctx:           context.Background(),
			message:       nil,
			expectError:   true,
			errorContains: "message cannot be nil",
		},
		{
			name:          "valid message",
			ctx:           context.Background(),
			message:       &messaging.Message{ID: "test", Key: "test.key", Body: []byte("test")},
			expectError:   true, // Currently returns error due to "not yet implemented" condition evaluation
			errorContains: "not yet implemented",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accepted, err := ar.Filter(tc.ctx, tc.message)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
			if !tc.expectError && !accepted {
				t.Errorf("Expected message to be accepted for test case '%s'", tc.name)
			}
		})
	}
}

func TestAdvancedRoutingClose(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled: true,
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	testCases := []struct {
		name          string
		ctx           context.Context
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil context",
			ctx:           nil,
			expectError:   true,
			errorContains: "context cannot be nil",
		},
		{
			name:        "valid context",
			ctx:         context.Background(),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ar.Close(tc.ctx)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestAdvancedRoutingDisabled(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled: false, // Disabled
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that route returns empty values when disabled
	exchange, routingKey, err := ar.Route(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when routing is disabled: %v", err)
	}
	if exchange != "" || routingKey != "" {
		t.Errorf("Expected empty exchange and routing key when routing is disabled, got: %s, %s", exchange, routingKey)
	}

	// Test that filter returns true when disabled
	accepted, err := ar.Filter(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when filtering is disabled: %v", err)
	}
	if !accepted {
		t.Error("Expected message to be accepted when filtering is disabled")
	}
}

func TestAdvancedRoutingNilConfig(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled: true,
		// No routing rules or filter rules
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that route returns empty values when no rules
	exchange, routingKey, err := ar.Route(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when no routing rules: %v", err)
	}
	if exchange != "" || routingKey != "" {
		t.Errorf("Expected empty exchange and routing key when no routing rules, got: %s, %s", exchange, routingKey)
	}

	// Test that filter returns true when no rules
	accepted, err := ar.Filter(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when no filter rules: %v", err)
	}
	if !accepted {
		t.Error("Expected message to be accepted when no filter rules")
	}
}

func TestAdvancedRoutingValidation(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	// Test with invalid routing rule (empty target exchange)
	config := &messaging.AdvancedRoutingConfig{
		Enabled: true,
		RoutingRules: []messaging.RoutingRule{
			{
				Name:             "invalid-rule",
				Condition:        "$.type == 'test'",
				TargetExchange:   "", // Invalid: empty target exchange
				TargetRoutingKey: "test-key",
				Priority:         1,
				Enabled:          true,
			},
		},
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test that route returns empty values when rule is invalid
	exchange, routingKey, err := ar.Route(context.Background(), message)
	if err != nil {
		t.Errorf("Unexpected error when routing rule is invalid: %v", err)
	}
	if exchange != "" || routingKey != "" {
		t.Errorf("Expected empty exchange and routing key when routing rule is invalid, got: %s, %s", exchange, routingKey)
	}
}

func TestAdvancedRoutingConcurrency(t *testing.T) {
	// Create observability context
	provider, err := messaging.NewObservabilityProvider(&messaging.TelemetryConfig{
		MetricsEnabled: false,
		TracingEnabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create observability provider: %v", err)
	}
	observability := messaging.NewObservabilityContext(context.Background(), provider)

	config := &messaging.AdvancedRoutingConfig{
		Enabled: true,
	}

	ar, err := messaging.NewAdvancedRouting(config, observability)
	if err != nil {
		t.Fatalf("Failed to create AdvancedRouting: %v", err)
	}

	message := &messaging.Message{ID: "test", Body: []byte("test")}

	// Test concurrent access to route method
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _, err := ar.Route(context.Background(), message)
			if err != nil {
				t.Errorf("Unexpected error in concurrent route: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent access to filter method
	for i := 0; i < 10; i++ {
		go func() {
			_, err := ar.Filter(context.Background(), message)
			if err != nil {
				t.Errorf("Unexpected error in concurrent filter: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
