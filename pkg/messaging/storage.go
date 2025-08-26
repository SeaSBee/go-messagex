// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/seasbee/go-logx"
)

// validateMessageID checks if a message ID is valid.
func validateMessageID(messageID string) error {
	if messageID == "" {
		return NewError(ErrorCodeValidation, "validate", "message ID cannot be empty")
	}
	if len(messageID) > MaxMessageIDLength {
		return NewError(ErrorCodeValidation, "validate", "message ID exceeds maximum length")
	}
	return nil
}

// MemoryMessageStorage provides in-memory message storage.
type MemoryMessageStorage struct {
	config   *MessagePersistenceConfig
	messages map[string]*Message
	mu       sync.RWMutex
	logger   *logx.Logger
}

// NewMemoryMessageStorage creates a new memory message storage.
func NewMemoryMessageStorage(config *MessagePersistenceConfig) *MemoryMessageStorage {
	return NewMemoryMessageStorageWithLogger(config, NoOpLogger())
}

// NewMemoryMessageStorageWithLogger creates a new memory message storage with a custom logger.
func NewMemoryMessageStorageWithLogger(config *MessagePersistenceConfig, logger *logx.Logger) *MemoryMessageStorage {
	if config == nil {
		config = &MessagePersistenceConfig{
			MaxStorageSize: 1000, // Default value
		}
	}
	if logger == nil {
		logger = NoOpLogger()
	}
	return &MemoryMessageStorage{
		config:   config,
		messages: make(map[string]*Message),
		logger:   logger,
	}
}

// Store stores a message in memory.
func (m *MemoryMessageStorage) Store(ctx context.Context, message *Message) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "store", "operation cancelled")
		default:
		}
	}

	if err := validateMessage(message); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check storage size limit
	if int64(len(m.messages)) >= m.config.MaxStorageSize {
		return NewError(ErrorCodePersistence, "store", "storage size limit exceeded")
	}

	m.messages[message.ID] = message
	m.logger.Debug("Message stored in memory",
		logx.String("message_id", message.ID),
		logx.Int("storage_size", len(m.messages)))
	return nil
}

// Retrieve retrieves a message from memory.
func (m *MemoryMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, NewError(ErrorCodeTimeout, "retrieve", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	message, exists := m.messages[messageID]
	if !exists {
		return nil, NewError(ErrorCodePersistence, "retrieve", "message not found: "+messageID)
	}

	return message, nil
}

// Delete deletes a message from memory.
func (m *MemoryMessageStorage) Delete(ctx context.Context, messageID string) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "delete", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.messages, messageID)
	return nil
}

// Cleanup cleans up old messages from memory.
func (m *MemoryMessageStorage) Cleanup(ctx context.Context, before time.Time) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "cleanup", "operation cancelled")
		default:
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	deletedCount := 0
	for id, message := range m.messages {
		if message != nil && message.Timestamp.Before(before) {
			delete(m.messages, id)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		m.logger.Info("Memory cleanup completed",
			logx.Int("deleted_count", deletedCount),
			logx.Int("remaining_count", len(m.messages)),
			logx.String("cutoff_time", before.Format(time.RFC3339)))
	}

	return nil
}

// Close closes the memory storage.
func (m *MemoryMessageStorage) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = nil
	return nil
}

// DiskMessageStorage provides disk-based message storage.
type DiskMessageStorage struct {
	config   *MessagePersistenceConfig
	basePath string
	mu       sync.RWMutex
	logger   *logx.Logger
}

// NewDiskMessageStorage creates a new disk message storage.
func NewDiskMessageStorage(config *MessagePersistenceConfig) (*DiskMessageStorage, error) {
	return NewDiskMessageStorageWithLogger(config, NoOpLogger())
}

// NewDiskMessageStorageWithLogger creates a new disk message storage with a custom logger.
func NewDiskMessageStorageWithLogger(config *MessagePersistenceConfig, logger *logx.Logger) (*DiskMessageStorage, error) {
	if config == nil {
		return nil, NewError(ErrorCodeConfiguration, "new_disk_storage", "config cannot be nil")
	}

	if config.StoragePath == "" {
		return nil, NewError(ErrorCodeConfiguration, "new_disk_storage", "storage path cannot be empty")
	}

	if logger == nil {
		logger = NoOpLogger()
	}

	// Create storage directory if it doesn't exist
	// Use 0o750 for better security: owner read/write/execute, group read/execute, others no access
	if err := os.MkdirAll(config.StoragePath, 0o750); err != nil {
		return nil, WrapError(ErrorCodePersistence, "new_disk_storage", "failed to create storage directory", err)
	}

	return &DiskMessageStorage{
		config:   config,
		basePath: config.StoragePath,
		logger:   logger,
	}, nil
}

// Store stores a message on disk.
func (d *DiskMessageStorage) Store(ctx context.Context, message *Message) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "store", "operation cancelled")
		default:
		}
	}

	if err := validateMessage(message); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create message file path
	messagePath := filepath.Join(d.basePath, message.ID+".json")

	// Serialize message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return WrapError(ErrorCodeSerialization, "store", "failed to serialize message", err)
	}

	// Write to file
	// Use 0o640 for better security: owner read/write, group read, others no access
	if err := os.WriteFile(messagePath, data, 0o640); err != nil {
		return WrapError(ErrorCodePersistence, "store", "failed to write message file", err)
	}

	d.logger.Debug("Message stored on disk",
		logx.String("message_id", message.ID),
		logx.String("file_path", messagePath))
	return nil
}

// Retrieve retrieves a message from disk.
func (d *DiskMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, NewError(ErrorCodeTimeout, "retrieve", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Create message file path
	messagePath := filepath.Join(d.basePath, messageID+".json")

	// Read file
	data, err := os.ReadFile(messagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewError(ErrorCodePersistence, "retrieve", "message not found: "+messageID)
		}
		return nil, WrapError(ErrorCodePersistence, "retrieve", "failed to read message file", err)
	}

	// Deserialize message from JSON
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, WrapError(ErrorCodeSerialization, "retrieve", "failed to deserialize message", err)
	}

	return &message, nil
}

// Delete deletes a message from disk.
func (d *DiskMessageStorage) Delete(ctx context.Context, messageID string) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "delete", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create message file path
	messagePath := filepath.Join(d.basePath, messageID+".json")

	// Remove file
	if err := os.Remove(messagePath); err != nil && !os.IsNotExist(err) {
		return WrapError(ErrorCodePersistence, "delete", "failed to delete message file", err)
	}

	return nil
}

// Cleanup cleans up old messages from disk.
func (d *DiskMessageStorage) Cleanup(ctx context.Context, before time.Time) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "cleanup", "operation cancelled")
		default:
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Read directory
	entries, err := os.ReadDir(d.basePath)
	if err != nil {
		return WrapError(ErrorCodePersistence, "cleanup", "failed to read storage directory", err)
	}

	// Check each file
	deletedCount := 0
	for _, entry := range entries {
		// Check context cancellation periodically during cleanup
		if ctx != nil {
			select {
			case <-ctx.Done():
				return NewError(ErrorCodeTimeout, "cleanup", "operation cancelled")
			default:
			}
		}

		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(d.basePath, entry.Name())

		// Use os.Stat instead of entry.Info() to avoid potential race conditions
		info, err := os.Stat(filePath)
		if err != nil {
			// Skip files that can't be stat'd (might have been deleted)
			continue
		}

		// Check if file is older than the cutoff time
		if info.ModTime().Before(before) {
			if err := os.Remove(filePath); err != nil {
				// Log error but continue with other files
				d.logger.Error("Failed to delete old message file",
					logx.String("file_path", filePath),
					logx.ErrorField(err))
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		d.logger.Info("Disk cleanup completed",
			logx.Int("deleted_count", deletedCount),
			logx.String("cutoff_time", before.Format(time.RFC3339)))
	}

	return nil
}

// Close closes the disk storage.
func (d *DiskMessageStorage) Close(ctx context.Context) error {
	// Nothing to do for disk storage
	return nil
}

// RedisMessageStorage provides Redis-based message storage.
type RedisMessageStorage struct {
	// TODO: Add Redis client when Redis dependency is added
}

// NewRedisMessageStorage creates a new Redis message storage.
func NewRedisMessageStorage(config *MessagePersistenceConfig) (*RedisMessageStorage, error) {
	if config == nil {
		return nil, NewError(ErrorCodeConfiguration, "new_redis_storage", "config cannot be nil")
	}

	// TODO: Implement Redis client initialization
	// For now, return an error indicating Redis is not yet supported
	return nil, NewError(ErrorCodeConfiguration, "new_redis_storage", "Redis storage is not yet implemented")
}

// Store stores a message in Redis.
func (r *RedisMessageStorage) Store(ctx context.Context, message *Message) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "store", "operation cancelled")
		default:
		}
	}

	if err := validateMessage(message); err != nil {
		return err
	}

	// TODO: Implement Redis storage
	return NewError(ErrorCodePersistence, "store", "Redis storage is not yet implemented")
}

// Retrieve retrieves a message from Redis.
func (r *RedisMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, NewError(ErrorCodeTimeout, "retrieve", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return nil, err
	}

	// TODO: Implement Redis retrieval
	return nil, NewError(ErrorCodePersistence, "retrieve", "Redis storage is not yet implemented")
}

// Delete deletes a message from Redis.
func (r *RedisMessageStorage) Delete(ctx context.Context, messageID string) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "delete", "operation cancelled")
		default:
		}
	}

	if err := validateMessageID(messageID); err != nil {
		return err
	}

	// TODO: Implement Redis deletion
	return NewError(ErrorCodePersistence, "delete", "Redis storage is not yet implemented")
}

// Cleanup cleans up old messages from Redis.
func (r *RedisMessageStorage) Cleanup(ctx context.Context, before time.Time) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "cleanup", "operation cancelled")
		default:
		}
	}

	// TODO: Implement Redis cleanup
	return NewError(ErrorCodePersistence, "cleanup", "Redis storage is not yet implemented")
}

// Close closes the Redis storage.
func (r *RedisMessageStorage) Close(ctx context.Context) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return NewError(ErrorCodeTimeout, "close", "operation cancelled")
		default:
		}
	}

	// TODO: Implement Redis close
	return NewError(ErrorCodePersistence, "close", "Redis storage is not yet implemented")
}
