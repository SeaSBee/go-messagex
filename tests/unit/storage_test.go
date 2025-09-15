package unit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a valid test message
func createTestMessage(id string) *messaging.Message {
	return &messaging.Message{
		ID:          id,
		Body:        []byte("test message body"),
		ContentType: "application/json",
		Key:         "test.key",
		Priority:    1,
		Timestamp:   time.Now(),
		Headers: map[string]string{
			"test-header": "test-value",
		},
		CorrelationID: "test-correlation",
	}
}

// Helper function to create a temporary directory for disk storage tests
func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "storage_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// Test MemoryMessageStorage
func TestMemoryMessageStorage_Store(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			message:     createTestMessage("test1"),
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			message:       createTestMessage("test2"),
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
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
			message:     createTestMessage("test3"),
			expectError: false,
		},
		{
			name:        "duplicate message ID",
			ctx:         context.Background(),
			message:     createTestMessage("test4"),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Store(tc.ctx, tc.message)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryMessageStorage_Retrieve(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	// Store a test message first
	testMessage := createTestMessage("test-retrieve")
	err := storage.Store(context.Background(), testMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			messageID:   "test-retrieve",
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			messageID:     "test-retrieve",
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:          "message not found",
			ctx:           context.Background(),
			messageID:     "non-existent",
			expectError:   true,
			errorContains: "message not found",
		},
		{
			name:        "existing message",
			ctx:         context.Background(),
			messageID:   "test-retrieve",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := storage.Retrieve(tc.ctx, tc.messageID)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, message)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, message)
				assert.Equal(t, tc.messageID, message.ID)
			}
		})
	}
}

func TestMemoryMessageStorage_Delete(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	// Store a test message first
	testMessage := createTestMessage("test-delete")
	err := storage.Store(context.Background(), testMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			messageID:   "test-delete",
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			messageID:     "test-delete",
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:        "existing message",
			ctx:         context.Background(),
			messageID:   "test-delete",
			expectError: false,
		},
		{
			name:        "non-existent message",
			ctx:         context.Background(),
			messageID:   "non-existent",
			expectError: false, // Delete doesn't error on non-existent
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Delete(tc.ctx, tc.messageID)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryMessageStorage_Cleanup(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	// Store messages with different timestamps
	oldMessage := createTestMessage("old-message")
	oldMessage.Timestamp = time.Now().Add(-2 * time.Hour)
	err := storage.Store(context.Background(), oldMessage)
	require.NoError(t, err)

	newMessage := createTestMessage("new-message")
	newMessage.Timestamp = time.Now()
	err = storage.Store(context.Background(), newMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		before        time.Time
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			before:      time.Now().Add(-1 * time.Hour),
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			before:        time.Now().Add(-1 * time.Hour),
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:        "cleanup old messages",
			ctx:         context.Background(),
			before:      time.Now().Add(-1 * time.Hour),
			expectError: false,
		},
		{
			name:        "cleanup all messages",
			ctx:         context.Background(),
			before:      time.Now().Add(1 * time.Hour),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Cleanup(tc.ctx, tc.before)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryMessageStorage_Close(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	// Store a message first
	testMessage := createTestMessage("test-close")
	err := storage.Store(context.Background(), testMessage)
	require.NoError(t, err)

	// Close the storage
	err = storage.Close(context.Background())
	assert.NoError(t, err)

	// Verify that the storage is cleared
	message, err := storage.Retrieve(context.Background(), "test-close")
	assert.Error(t, err)
	assert.Nil(t, message)
}

func TestMemoryMessageStorage_StorageLimits(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 2, // Very small limit for testing
	}
	storage := messaging.NewMemoryMessageStorage(config)

	// Store up to the limit
	err := storage.Store(context.Background(), createTestMessage("msg1"))
	assert.NoError(t, err)

	err = storage.Store(context.Background(), createTestMessage("msg2"))
	assert.NoError(t, err)

	// Try to store beyond the limit
	err = storage.Store(context.Background(), createTestMessage("msg3"))
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "storage size limit exceeded"))
}

func TestMemoryMessageStorage_ConcurrentAccess(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		MaxStorageSize: 1000,
	}
	storage := messaging.NewMemoryMessageStorage(config)

	const numGoroutines = 10
	const operationsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent store operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				messageID := fmt.Sprintf("concurrent-%d-%d", id, j)
				message := createTestMessage(messageID)
				err := storage.Store(context.Background(), message)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were stored
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < operationsPerGoroutine; j++ {
			messageID := fmt.Sprintf("concurrent-%d-%d", i, j)
			message, err := storage.Retrieve(context.Background(), messageID)
			assert.NoError(t, err)
			assert.Equal(t, messageID, message.ID)
		}
	}
}

// Test DiskMessageStorage
func TestDiskMessageStorage_Store(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		message       *messaging.Message
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			message:     createTestMessage("test1"),
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			message:       createTestMessage("test2"),
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
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
			message:     createTestMessage("test3"),
			expectError: false,
		},
		{
			name:        "duplicate message ID",
			ctx:         context.Background(),
			message:     createTestMessage("test4"),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Store(tc.ctx, tc.message)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiskMessageStorage_Retrieve(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Store a test message first
	testMessage := createTestMessage("test-retrieve")
	err = storage.Store(context.Background(), testMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			messageID:   "test-retrieve",
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			messageID:     "test-retrieve",
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:          "message not found",
			ctx:           context.Background(),
			messageID:     "non-existent",
			expectError:   true,
			errorContains: "message not found",
		},
		{
			name:        "existing message",
			ctx:         context.Background(),
			messageID:   "test-retrieve",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := storage.Retrieve(tc.ctx, tc.messageID)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, message)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, message)
				assert.Equal(t, tc.messageID, message.ID)
			}
		})
	}
}

func TestDiskMessageStorage_Delete(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Store a test message first
	testMessage := createTestMessage("test-delete")
	err = storage.Store(context.Background(), testMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		messageID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			messageID:   "test-delete",
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			messageID:     "test-delete",
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:          "empty message ID",
			ctx:           context.Background(),
			messageID:     "",
			expectError:   true,
			errorContains: "message ID cannot be empty",
		},
		{
			name:        "existing message",
			ctx:         context.Background(),
			messageID:   "test-delete",
			expectError: false,
		},
		{
			name:        "non-existent message",
			ctx:         context.Background(),
			messageID:   "non-existent",
			expectError: false, // Delete doesn't error on non-existent
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Delete(tc.ctx, tc.messageID)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiskMessageStorage_Cleanup(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Store messages with different timestamps
	oldMessage := createTestMessage("old-message")
	oldMessage.Timestamp = time.Now().Add(-2 * time.Hour)
	err = storage.Store(context.Background(), oldMessage)
	require.NoError(t, err)

	newMessage := createTestMessage("new-message")
	newMessage.Timestamp = time.Now()
	err = storage.Store(context.Background(), newMessage)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		ctx           context.Context
		before        time.Time
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			before:      time.Now().Add(-1 * time.Hour),
			expectError: false, // nil context is allowed
		},
		{
			name:          "cancelled context",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			before:        time.Now().Add(-1 * time.Hour),
			expectError:   true,
			errorContains: "TIMEOUT: operation cancelled",
		},
		{
			name:        "cleanup old messages",
			ctx:         context.Background(),
			before:      time.Now().Add(-1 * time.Hour),
			expectError: false,
		},
		{
			name:        "cleanup all messages",
			ctx:         context.Background(),
			before:      time.Now().Add(1 * time.Hour),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := storage.Cleanup(tc.ctx, tc.before)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiskMessageStorage_Close(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Close should not error
	err = storage.Close(context.Background())
	assert.NoError(t, err)
}

func TestDiskMessageStorage_FileOperations(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Test file creation
	message := createTestMessage("test-file")
	err = storage.Store(context.Background(), message)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "test-file.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Test file deletion
	err = storage.Delete(context.Background(), "test-file")
	assert.NoError(t, err)

	// Verify file is deleted
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestDiskMessageStorage_JSONHandling(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	// Test JSON serialization/deserialization
	originalMessage := createTestMessage("test-json")
	originalMessage.Headers = map[string]string{
		"complex-header": "complex-value",
		"number-header":  "123",
	}

	err = storage.Store(context.Background(), originalMessage)
	assert.NoError(t, err)

	retrievedMessage, err := storage.Retrieve(context.Background(), "test-json")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedMessage)

	// Verify all fields are preserved
	assert.Equal(t, originalMessage.ID, retrievedMessage.ID)
	assert.Equal(t, originalMessage.Body, retrievedMessage.Body)
	assert.Equal(t, originalMessage.ContentType, retrievedMessage.ContentType)
	assert.Equal(t, originalMessage.Key, retrievedMessage.Key)
	assert.Equal(t, originalMessage.Priority, retrievedMessage.Priority)
	assert.Equal(t, originalMessage.Headers, retrievedMessage.Headers)
}

func TestDiskMessageStorage_ErrorScenarios(t *testing.T) {
	// Test invalid storage path
	config := &messaging.MessagePersistenceConfig{
		StoragePath: "/invalid/path/that/should/not/exist",
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	assert.Error(t, err)
	assert.Nil(t, storage)

	// Test nil config
	storage, err = messaging.NewDiskMessageStorage(nil)
	assert.Error(t, err)
	assert.Nil(t, storage)

	// Test empty storage path
	config = &messaging.MessagePersistenceConfig{
		StoragePath: "",
	}
	storage, err = messaging.NewDiskMessageStorage(config)
	assert.Error(t, err)
	assert.Nil(t, storage)
}

func TestDiskMessageStorage_ConcurrentAccess(t *testing.T) {
	tempDir := createTempDir(t)
	config := &messaging.MessagePersistenceConfig{
		StoragePath: tempDir,
	}
	storage, err := messaging.NewDiskMessageStorage(config)
	require.NoError(t, err)

	const numGoroutines = 5
	const operationsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent store operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				messageID := fmt.Sprintf("concurrent-%d-%d", id, j)
				message := createTestMessage(messageID)
				err := storage.Store(context.Background(), message)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were stored
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < operationsPerGoroutine; j++ {
			messageID := fmt.Sprintf("concurrent-%d-%d", i, j)
			message, err := storage.Retrieve(context.Background(), messageID)
			assert.NoError(t, err)
			assert.Equal(t, messageID, message.ID)
		}
	}
}

// Test RedisMessageStorage (not implemented)
func TestRedisMessageStorage_NotImplemented(t *testing.T) {
	config := &messaging.MessagePersistenceConfig{
		StoragePath: "redis://localhost:6379",
	}

	// Test creation
	storage, err := messaging.NewRedisMessageStorage(config)
	assert.Error(t, err)
	assert.Nil(t, storage)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))

	// Test all methods return "not implemented" errors
	redisStorage := &messaging.RedisMessageStorage{}

	// Store
	err = redisStorage.Store(context.Background(), createTestMessage("test"))
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))

	// Retrieve
	message, err := redisStorage.Retrieve(context.Background(), "test")
	assert.Error(t, err)
	assert.Nil(t, message)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))

	// Delete
	err = redisStorage.Delete(context.Background(), "test")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))

	// Cleanup
	err = redisStorage.Cleanup(context.Background(), time.Now())
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))

	// Close
	err = redisStorage.Close(context.Background())
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not yet implemented"))
}

// Test Storage Interface Validation
func TestStorageInterface_Validation(t *testing.T) {
	// Test message validation
	invalidMessages := []*messaging.Message{
		nil,
		{ID: "", Body: []byte("test")},
		{ID: "test", Body: nil},
		{ID: strings.Repeat("a", messaging.MaxMessageIDLength+1), Body: []byte("test")},
	}

	for i, message := range invalidMessages {
		t.Run(fmt.Sprintf("invalid_message_%d", i), func(t *testing.T) {
			config := &messaging.MessagePersistenceConfig{
				MaxStorageSize: 1000,
			}
			storage := messaging.NewMemoryMessageStorage(config)

			err := storage.Store(context.Background(), message)
			assert.Error(t, err)
		})
	}

	// Test message ID validation
	invalidMessageIDs := []string{
		"",
		strings.Repeat("a", messaging.MaxMessageIDLength+1),
	}

	for i, messageID := range invalidMessageIDs {
		t.Run(fmt.Sprintf("invalid_message_id_%d", i), func(t *testing.T) {
			config := &messaging.MessagePersistenceConfig{
				MaxStorageSize: 1000,
			}
			storage := messaging.NewMemoryMessageStorage(config)

			_, err := storage.Retrieve(context.Background(), messageID)
			assert.Error(t, err)

			err = storage.Delete(context.Background(), messageID)
			assert.Error(t, err)
		})
	}
}

// Test Storage Performance (basic benchmarks)
func TestStorageInterface_Performance(t *testing.T) {
	// Memory storage performance test
	t.Run("MemoryStorage_Performance", func(t *testing.T) {
		config := &messaging.MessagePersistenceConfig{
			MaxStorageSize: 10000,
		}
		storage := messaging.NewMemoryMessageStorage(config)

		start := time.Now()
		for i := 0; i < 1000; i++ {
			message := createTestMessage(fmt.Sprintf("perf-test-%d", i))
			err := storage.Store(context.Background(), message)
			assert.NoError(t, err)
		}
		storeDuration := time.Since(start)

		start = time.Now()
		for i := 0; i < 1000; i++ {
			_, err := storage.Retrieve(context.Background(), fmt.Sprintf("perf-test-%d", i))
			assert.NoError(t, err)
		}
		retrieveDuration := time.Since(start)

		t.Logf("Memory Storage - Store: %v, Retrieve: %v", storeDuration, retrieveDuration)
	})

	// Disk storage performance test
	t.Run("DiskStorage_Performance", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := &messaging.MessagePersistenceConfig{
			StoragePath: tempDir,
		}
		storage, err := messaging.NewDiskMessageStorage(config)
		require.NoError(t, err)

		start := time.Now()
		for i := 0; i < 100; i++ { // Fewer iterations for disk test
			message := createTestMessage(fmt.Sprintf("perf-test-%d", i))
			err := storage.Store(context.Background(), message)
			assert.NoError(t, err)
		}
		storeDuration := time.Since(start)

		start = time.Now()
		for i := 0; i < 100; i++ {
			_, err := storage.Retrieve(context.Background(), fmt.Sprintf("perf-test-%d", i))
			assert.NoError(t, err)
		}
		retrieveDuration := time.Since(start)

		t.Logf("Disk Storage - Store: %v, Retrieve: %v", storeDuration, retrieveDuration)
	})
}

// Test Storage Error Handling
func TestStorageInterface_ErrorHandling(t *testing.T) {
	// Test context cancellation
	t.Run("ContextCancellation", func(t *testing.T) {
		config := &messaging.MessagePersistenceConfig{
			MaxStorageSize: 1000,
		}
		storage := messaging.NewMemoryMessageStorage(config)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		message := createTestMessage("test")
		err := storage.Store(ctx, message)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))

		_, err = storage.Retrieve(ctx, "test")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))

		err = storage.Delete(ctx, "test")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))

		err = storage.Cleanup(ctx, time.Now())
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))
	})

	// Test storage limits
	t.Run("StorageLimits", func(t *testing.T) {
		config := &messaging.MessagePersistenceConfig{
			MaxStorageSize: 1, // Very small limit
		}
		storage := messaging.NewMemoryMessageStorage(config)

		// First message should succeed
		err := storage.Store(context.Background(), createTestMessage("msg1"))
		assert.NoError(t, err)

		// Second message should fail
		err = storage.Store(context.Background(), createTestMessage("msg2"))
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "storage size limit exceeded"))
	})
}
