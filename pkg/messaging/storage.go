// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryMessageStorage provides in-memory message storage.
type MemoryMessageStorage struct {
	config   *MessagePersistenceConfig
	messages map[string]*Message
	mu       sync.RWMutex
}

// NewMemoryMessageStorage creates a new memory message storage.
func NewMemoryMessageStorage(config *MessagePersistenceConfig) *MemoryMessageStorage {
	return &MemoryMessageStorage{
		config:   config,
		messages: make(map[string]*Message),
	}
}

// Store stores a message in memory.
func (m *MemoryMessageStorage) Store(ctx context.Context, message *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check storage size limit
	if int64(len(m.messages)) >= m.config.MaxStorageSize {
		return NewError(ErrorCodePersistence, "store", "storage size limit exceeded")
	}

	m.messages[message.ID] = message
	return nil
}

// Retrieve retrieves a message from memory.
func (m *MemoryMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
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
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.messages, messageID)
	return nil
}

// Cleanup cleans up old messages from memory.
func (m *MemoryMessageStorage) Cleanup(ctx context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, message := range m.messages {
		if message.Timestamp.Before(before) {
			delete(m.messages, id)
		}
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
}

// NewDiskMessageStorage creates a new disk message storage.
func NewDiskMessageStorage(config *MessagePersistenceConfig) (*DiskMessageStorage, error) {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(config.StoragePath, 0o755); err != nil {
		return nil, WrapError(ErrorCodePersistence, "new_disk_storage", "failed to create storage directory", err)
	}

	return &DiskMessageStorage{
		config:   config,
		basePath: config.StoragePath,
	}, nil
}

// Store stores a message on disk.
func (d *DiskMessageStorage) Store(ctx context.Context, message *Message) error {
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
	if err := os.WriteFile(messagePath, data, 0o644); err != nil {
		return WrapError(ErrorCodePersistence, "store", "failed to write message file", err)
	}

	return nil
}

// Retrieve retrieves a message from disk.
func (d *DiskMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
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
	d.mu.Lock()
	defer d.mu.Unlock()

	// Read directory
	entries, err := os.ReadDir(d.basePath)
	if err != nil {
		return WrapError(ErrorCodePersistence, "cleanup", "failed to read storage directory", err)
	}

	// Check each file
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(d.basePath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if file is older than the cutoff time
		if info.ModTime().Before(before) {
			if err := os.Remove(filePath); err != nil {
				// Log error but continue with other files
				fmt.Printf("Failed to delete old message file %s: %v\n", filePath, err)
			}
		}
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
	// TODO: Implement Redis client initialization
	// For now, return an error indicating Redis is not yet supported
	return nil, NewError(ErrorCodeConfiguration, "new_redis_storage", "Redis storage is not yet implemented")
}

// Store stores a message in Redis.
func (r *RedisMessageStorage) Store(ctx context.Context, message *Message) error {
	// TODO: Implement Redis storage
	return NewError(ErrorCodePersistence, "store", "Redis storage is not yet implemented")
}

// Retrieve retrieves a message from Redis.
func (r *RedisMessageStorage) Retrieve(ctx context.Context, messageID string) (*Message, error) {
	// TODO: Implement Redis retrieval
	return nil, NewError(ErrorCodePersistence, "retrieve", "Redis storage is not yet implemented")
}

// Delete deletes a message from Redis.
func (r *RedisMessageStorage) Delete(ctx context.Context, messageID string) error {
	// TODO: Implement Redis deletion
	return NewError(ErrorCodePersistence, "delete", "Redis storage is not yet implemented")
}

// Cleanup cleans up old messages from Redis.
func (r *RedisMessageStorage) Cleanup(ctx context.Context, before time.Time) error {
	// TODO: Implement Redis cleanup
	return NewError(ErrorCodePersistence, "cleanup", "Redis storage is not yet implemented")
}

// Close closes the Redis storage.
func (r *RedisMessageStorage) Close(ctx context.Context) error {
	// TODO: Implement Redis close
	return NewError(ErrorCodePersistence, "close", "Redis storage is not yet implemented")
}
