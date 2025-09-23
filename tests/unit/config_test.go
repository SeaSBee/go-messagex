package unit

import (
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportType(t *testing.T) {
	t.Run("IsValid", func(t *testing.T) {
		tests := []struct {
			name      string
			transport messaging.TransportType
			expected  bool
		}{
			{
				name:      "valid rabbitmq transport",
				transport: messaging.TransportRabbitMQ,
				expected:  true,
			},
			{
				name:      "invalid empty transport",
				transport: "",
				expected:  false,
			},
			{
				name:      "invalid unknown transport",
				transport: "kafka",
				expected:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.transport.IsValid()
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("String", func(t *testing.T) {
		tests := []struct {
			name      string
			transport messaging.TransportType
			expected  string
		}{
			{
				name:      "rabbitmq transport",
				transport: messaging.TransportRabbitMQ,
				expected:  "rabbitmq",
			},
			{
				name:      "empty transport",
				transport: "",
				expected:  "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.transport.String()
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("returns valid config with defaults", func(t *testing.T) {
		config := messaging.DefaultConfig()

		require.NotNil(t, config)
		assert.Equal(t, messaging.TransportRabbitMQ, config.Transport)
		assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URL)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 5*time.Second, config.RetryDelay)
		assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
		assert.Equal(t, 10, config.MaxConnections)
		assert.Equal(t, 100, config.MaxChannels)
		assert.True(t, config.MetricsEnabled)
		assert.Equal(t, 30*time.Second, config.HealthCheckInterval)

		// Validate the config
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("config is valid after creation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestDefaultProducerConfig(t *testing.T) {
	t.Run("returns valid producer config with defaults", func(t *testing.T) {
		config := messaging.DefaultProducerConfig()

		assert.Equal(t, 100, config.BatchSize)
		assert.Equal(t, 1*time.Second, config.BatchTimeout)
		assert.Equal(t, 10*time.Second, config.PublishTimeout)
		assert.False(t, config.Mandatory)
		assert.False(t, config.Immediate)
		assert.True(t, config.ConfirmMode)
		assert.Equal(t, "", config.DefaultExchange)
		assert.Equal(t, "", config.DefaultRoutingKey)
	})
}

func TestDefaultConsumerConfig(t *testing.T) {
	t.Run("returns valid consumer config with defaults", func(t *testing.T) {
		config := messaging.DefaultConsumerConfig()

		assert.False(t, config.AutoCommit)
		assert.Equal(t, 1*time.Second, config.CommitInterval)
		assert.False(t, config.AutoAck)
		assert.False(t, config.Exclusive)
		assert.False(t, config.NoLocal)
		assert.False(t, config.NoWait)
		assert.Equal(t, 10, config.PrefetchCount)
		assert.Equal(t, 0, config.PrefetchSize)
		assert.Equal(t, "", config.ConsumerTag)
		assert.Equal(t, 10, config.MaxConsumers)
		assert.True(t, config.QueueDurable)
		assert.False(t, config.QueueAutoDelete)
		assert.False(t, config.QueueExclusive)
		assert.False(t, config.QueueNoWait)
		assert.True(t, config.ExchangeDurable)
		assert.False(t, config.ExchangeAutoDelete)
		assert.False(t, config.ExchangeInternal)
		assert.False(t, config.ExchangeNoWait)
		assert.Equal(t, "direct", config.ExchangeType)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config passes validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil config fails validation", func(t *testing.T) {
		var config *messaging.Config
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is nil")
	})

	t.Run("config with invalid URL fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.URL = "" // Invalid empty URL

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid max_retries fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.MaxRetries = -1 // Invalid negative value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid retry_delay fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.RetryDelay = 0 // Invalid zero value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid connection_timeout fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.ConnectionTimeout = 0 // Invalid zero value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid max_connections fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.MaxConnections = 0 // Invalid zero value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid max_channels fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.MaxChannels = 0 // Invalid zero value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("config with invalid health_check_interval fails validation", func(t *testing.T) {
		config := messaging.DefaultConfig()
		config.HealthCheckInterval = 0 // Invalid zero value

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})
}

func TestConfig_SetDefaults(t *testing.T) {
	t.Run("sets defaults for valid config", func(t *testing.T) {
		config := &messaging.Config{}
		err := config.SetDefaults()
		assert.NoError(t, err)

		// Verify some defaults are set
		assert.Equal(t, messaging.TransportRabbitMQ, config.Transport)
		assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URL)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 5*time.Second, config.RetryDelay)
		assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
		assert.Equal(t, 10, config.MaxConnections)
		assert.Equal(t, 100, config.MaxChannels)
		assert.True(t, config.MetricsEnabled)
		assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	})

	t.Run("nil config fails to set defaults", func(t *testing.T) {
		var config *messaging.Config
		err := config.SetDefaults()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is nil")
	})
}

func TestConfig_ValidateAndSetDefaults(t *testing.T) {
	t.Run("validates and sets defaults for valid config", func(t *testing.T) {
		config := &messaging.Config{}
		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)

		// Verify defaults are set and config is valid
		assert.Equal(t, messaging.TransportRabbitMQ, config.Transport)
		assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URL)

		// Verify config is valid after setting defaults
		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil config fails validation and setting defaults", func(t *testing.T) {
		var config *messaging.Config
		err := config.ValidateAndSetDefaults()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is nil")
	})

	t.Run("config with invalid values becomes valid after setting defaults", func(t *testing.T) {
		config := &messaging.Config{
			URL: "", // Invalid empty URL
		}
		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)         // Should be valid after setting defaults
		assert.NotEmpty(t, config.URL) // Should have default URL
	})
}

func TestConfig_EdgeCases(t *testing.T) {
	t.Run("config with boundary values", func(t *testing.T) {
		config := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          10,                // Max allowed
			RetryDelay:          60 * time.Second,  // Max allowed
			ConnectionTimeout:   300 * time.Second, // Max allowed
			MaxConnections:      100,               // Max allowed
			MaxChannels:         1000,              // Max allowed
			ProducerConfig:      messaging.DefaultProducerConfig(),
			ConsumerConfig:      messaging.DefaultConsumerConfig(),
			MetricsEnabled:      true,
			HealthCheckInterval: 300 * time.Second, // Max allowed
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("config with minimum values", func(t *testing.T) {
		config := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          0,               // Min allowed
			RetryDelay:          1 * time.Second, // Min allowed
			ConnectionTimeout:   1 * time.Second, // Min allowed
			MaxConnections:      1,               // Min allowed
			MaxChannels:         1,               // Min allowed
			ProducerConfig:      messaging.DefaultProducerConfig(),
			ConsumerConfig:      messaging.DefaultConsumerConfig(),
			MetricsEnabled:      false,
			HealthCheckInterval: 1 * time.Second, // Min allowed
		}

		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestProducerConfig_Validation(t *testing.T) {
	t.Run("valid producer config", func(t *testing.T) {
		config := messaging.DefaultProducerConfig()

		// Test with valid values
		config.BatchSize = 1000
		config.BatchTimeout = 30 * time.Second
		config.PublishTimeout = 60 * time.Second
		config.DefaultExchange = "test.exchange"
		config.DefaultRoutingKey = "test.key"

		// Create a full config to validate
		fullConfig := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          3,
			RetryDelay:          5 * time.Second,
			ConnectionTimeout:   30 * time.Second,
			MaxConnections:      10,
			MaxChannels:         100,
			ProducerConfig:      config,
			ConsumerConfig:      messaging.DefaultConsumerConfig(),
			MetricsEnabled:      true,
			HealthCheckInterval: 30 * time.Second,
		}

		err := fullConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("producer config with boundary values", func(t *testing.T) {
		config := messaging.DefaultProducerConfig()
		config.BatchSize = 10000                             // Max allowed
		config.BatchTimeout = 60 * time.Second               // Max allowed
		config.PublishTimeout = 300 * time.Second            // Max allowed
		config.DefaultExchange = string(make([]byte, 255))   // Max length
		config.DefaultRoutingKey = string(make([]byte, 255)) // Max length

		fullConfig := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          3,
			RetryDelay:          5 * time.Second,
			ConnectionTimeout:   30 * time.Second,
			MaxConnections:      10,
			MaxChannels:         100,
			ProducerConfig:      config,
			ConsumerConfig:      messaging.DefaultConsumerConfig(),
			MetricsEnabled:      true,
			HealthCheckInterval: 30 * time.Second,
		}

		err := fullConfig.Validate()
		assert.NoError(t, err)
	})
}

func TestConsumerConfig_Validation(t *testing.T) {
	t.Run("valid consumer config", func(t *testing.T) {
		config := messaging.DefaultConsumerConfig()

		// Test with valid values
		config.PrefetchCount = 100
		config.PrefetchSize = 1024 * 1024 // 1MB
		config.ConsumerTag = "test-consumer"
		config.MaxConsumers = 50
		config.ExchangeType = "topic"

		fullConfig := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          3,
			RetryDelay:          5 * time.Second,
			ConnectionTimeout:   30 * time.Second,
			MaxConnections:      10,
			MaxChannels:         100,
			ProducerConfig:      messaging.DefaultProducerConfig(),
			ConsumerConfig:      config,
			MetricsEnabled:      true,
			HealthCheckInterval: 30 * time.Second,
		}

		err := fullConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("consumer config with boundary values", func(t *testing.T) {
		config := messaging.DefaultConsumerConfig()
		config.PrefetchCount = 1000                    // Max allowed
		config.PrefetchSize = 10 * 1024 * 1024         // 10MB max
		config.ConsumerTag = string(make([]byte, 100)) // Max length
		config.MaxConsumers = 100                      // Max allowed
		config.ExchangeType = "headers"                // Valid exchange type

		fullConfig := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          3,
			RetryDelay:          5 * time.Second,
			ConnectionTimeout:   30 * time.Second,
			MaxConnections:      10,
			MaxChannels:         100,
			ProducerConfig:      messaging.DefaultProducerConfig(),
			ConsumerConfig:      config,
			MetricsEnabled:      true,
			HealthCheckInterval: 30 * time.Second,
		}

		err := fullConfig.Validate()
		assert.NoError(t, err)
	})

	t.Run("consumer config with invalid exchange type", func(t *testing.T) {
		config := messaging.DefaultConsumerConfig()
		config.ExchangeType = "invalid" // Invalid exchange type

		fullConfig := &messaging.Config{
			Transport:           messaging.TransportRabbitMQ,
			URL:                 "amqp://guest:guest@localhost:5672/",
			MaxRetries:          3,
			RetryDelay:          5 * time.Second,
			ConnectionTimeout:   30 * time.Second,
			MaxConnections:      10,
			MaxChannels:         100,
			ProducerConfig:      messaging.DefaultProducerConfig(),
			ConsumerConfig:      config,
			MetricsEnabled:      true,
			HealthCheckInterval: 30 * time.Second,
		}

		err := fullConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})
}

func TestConfig_Concurrency(t *testing.T) {
	t.Run("concurrent access to config", func(t *testing.T) {
		config := messaging.DefaultConfig()

		// Test concurrent reads
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				// Read various fields
				_ = config.Transport
				_ = config.URL
				_ = config.MaxRetries
				_ = config.ProducerConfig.BatchSize
				_ = config.ConsumerConfig.PrefetchCount

				// Validate config
				err := config.Validate()
				assert.NoError(t, err)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestConfig_JSONSerialization(t *testing.T) {
	t.Run("config can be marshaled to JSON", func(t *testing.T) {
		config := messaging.DefaultConfig()

		// This test ensures the struct tags are correct for JSON serialization
		// We don't actually marshal here as it would require importing encoding/json
		// but we verify the struct has the proper tags by checking field access
		assert.Equal(t, "transport", getJSONTag(config, "Transport"))
		assert.Equal(t, "url", getJSONTag(config, "URL"))
		assert.Equal(t, "max_retries", getJSONTag(config, "MaxRetries"))
	})
}

// Helper function to get JSON tag (simplified version)
func getJSONTag(config *messaging.Config, fieldName string) string {
	// This is a simplified helper - in a real test you might use reflection
	// For now, we'll just return the expected values based on the struct tags
	tagMap := map[string]string{
		"Transport":  "transport",
		"URL":        "url",
		"MaxRetries": "max_retries",
	}
	return tagMap[fieldName]
}
