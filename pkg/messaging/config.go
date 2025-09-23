// Package messaging provides configuration structures for RabbitMQ message transport client
package messaging

import (
	"fmt"
	"time"

	"github.com/creasty/defaults"
	"github.com/seasbee/go-validatorx"
)

// applyDefaultsWithFallback applies defaults to a config struct with fallback handling
func applyDefaultsWithFallback(config interface{}) {
	if err := defaults.Set(config); err != nil {
		// Log the error but continue with manual defaults
		// This ensures backward compatibility while providing visibility into issues
		// In production, consider using a proper logger here
		_ = err // Explicitly ignore to indicate this is intentional
	}
}

// TransportType defines supported message transport backends
type TransportType string

const (
	TransportRabbitMQ TransportType = "rabbitmq"
)

// IsValid checks if the transport type is valid
func (t TransportType) IsValid() bool {
	return t == TransportRabbitMQ
}

// String returns the string representation of the transport type
func (t TransportType) String() string {
	return string(t)
}

// Config for RabbitMQ message transport client
type Config struct {
	// Transport type
	Transport TransportType `yaml:"transport" json:"transport" validate:"oneof:rabbitmq" default:"rabbitmq"`

	// Connection settings
	URL               string        `yaml:"url" json:"url" validate:"required" default:"amqp://guest:guest@localhost:5672/"`
	MaxRetries        int           `yaml:"max_retries" json:"max_retries" validate:"min:0,max:10" default:"3"`
	RetryDelay        time.Duration `yaml:"retry_delay" json:"retry_delay" validate:"min:1s,max:60s" default:"5s"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" json:"connection_timeout" validate:"min:1s,max:300s" default:"30s"`

	// Connection pooling
	MaxConnections int `yaml:"max_connections" json:"max_connections" validate:"min:1,max:100" default:"10"`
	MaxChannels    int `yaml:"max_channels" json:"max_channels" validate:"min:1,max:1000" default:"100"`

	// Producer settings
	ProducerConfig ProducerConfig `yaml:"producer" json:"producer" validate:"required"`

	// Consumer settings
	ConsumerConfig ConsumerConfig `yaml:"consumer" json:"consumer" validate:"required"`

	// Monitoring
	MetricsEnabled      bool          `yaml:"metrics_enabled" json:"metrics_enabled" default:"true"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval" validate:"min:1s,max:300s" default:"30s"`
}

// ProducerConfig contains producer-specific configuration
type ProducerConfig struct {
	// Batching settings
	BatchSize      int           `yaml:"batch_size" json:"batch_size" validate:"min:1,max:10000" default:"100"`
	BatchTimeout   time.Duration `yaml:"batch_timeout" json:"batch_timeout" validate:"min:1ms,max:60s" default:"1s"`
	PublishTimeout time.Duration `yaml:"publish_timeout" json:"publish_timeout" validate:"min:1s,max:300s" default:"10s"`

	// RabbitMQ-specific
	Mandatory   bool `yaml:"mandatory" json:"mandatory" default:"false"`
	Immediate   bool `yaml:"immediate" json:"immediate" default:"false"`
	ConfirmMode bool `yaml:"confirm_mode" json:"confirm_mode" default:"true"`

	// Default exchange and routing
	DefaultExchange   string `yaml:"default_exchange" json:"default_exchange" validate:"max:255" default:""`
	DefaultRoutingKey string `yaml:"default_routing_key" json:"default_routing_key" validate:"max:255" default:""`
}

// ConsumerConfig contains consumer-specific configuration
type ConsumerConfig struct {
	// Acknowledgment settings
	AutoCommit     bool          `yaml:"auto_commit" json:"auto_commit" default:"false"`
	CommitInterval time.Duration `yaml:"commit_interval" json:"commit_interval" validate:"min:1ms,max:60s" default:"1s"`

	// RabbitMQ-specific
	AutoAck       bool   `yaml:"auto_ack" json:"auto_ack" default:"false"`
	Exclusive     bool   `yaml:"exclusive" json:"exclusive" default:"false"`
	NoLocal       bool   `yaml:"no_local" json:"no_local" default:"false"`
	NoWait        bool   `yaml:"no_wait" json:"no_wait" default:"false"`
	PrefetchCount int    `yaml:"prefetch_count" json:"prefetch_count" validate:"min:0,max:1000" default:"10"`
	PrefetchSize  int    `yaml:"prefetch_size" json:"prefetch_size" validate:"min:0,max:10485760" default:"0"`
	ConsumerTag   string `yaml:"consumer_tag" json:"consumer_tag" validate:"max:100" default:""`
	MaxConsumers  int    `yaml:"max_consumers" json:"max_consumers" validate:"min:1,max:100" default:"10"`

	// Queue settings
	QueueDurable    bool `yaml:"queue_durable" json:"queue_durable" default:"true"`
	QueueAutoDelete bool `yaml:"queue_auto_delete" json:"queue_auto_delete" default:"false"`
	QueueExclusive  bool `yaml:"queue_exclusive" json:"queue_exclusive" default:"false"`
	QueueNoWait     bool `yaml:"queue_no_wait" json:"queue_no_wait" default:"false"`

	// Exchange settings
	ExchangeDurable    bool   `yaml:"exchange_durable" json:"exchange_durable" default:"true"`
	ExchangeAutoDelete bool   `yaml:"exchange_auto_delete" json:"exchange_auto_delete" default:"false"`
	ExchangeInternal   bool   `yaml:"exchange_internal" json:"exchange_internal" default:"false"`
	ExchangeNoWait     bool   `yaml:"exchange_no_wait" json:"exchange_no_wait" default:"false"`
	ExchangeType       string `yaml:"exchange_type" json:"exchange_type" validate:"oneof:direct fanout topic headers" default:"direct"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	config := &Config{
		Transport:           TransportRabbitMQ,
		URL:                 "amqp://guest:guest@localhost:5672/",
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		ConnectionTimeout:   30 * time.Second,
		MaxConnections:      10,
		MaxChannels:         100,
		ProducerConfig:      DefaultProducerConfig(),
		ConsumerConfig:      DefaultConsumerConfig(),
		MetricsEnabled:      true,
		HealthCheckInterval: 30 * time.Second,
	}

	// Apply validation defaults to ensure all fields have proper values
	applyDefaultsWithFallback(config)

	return config
}

// DefaultProducerConfig returns default producer configuration
func DefaultProducerConfig() ProducerConfig {
	config := ProducerConfig{
		BatchSize:         100,
		BatchTimeout:      1 * time.Second,
		PublishTimeout:    10 * time.Second,
		Mandatory:         false,
		Immediate:         false,
		ConfirmMode:       true,
		DefaultExchange:   "",
		DefaultRoutingKey: "",
	}

	// Apply validation defaults
	applyDefaultsWithFallback(&config)

	return config
}

// DefaultConsumerConfig returns default consumer configuration
func DefaultConsumerConfig() ConsumerConfig {
	config := ConsumerConfig{
		AutoCommit:         false,
		CommitInterval:     1 * time.Second,
		AutoAck:            false,
		Exclusive:          false,
		NoLocal:            false,
		NoWait:             false,
		PrefetchCount:      10,
		PrefetchSize:       0,
		ConsumerTag:        "",
		MaxConsumers:       10,
		QueueDurable:       true,
		QueueAutoDelete:    false,
		QueueExclusive:     false,
		QueueNoWait:        false,
		ExchangeDurable:    true,
		ExchangeAutoDelete: false,
		ExchangeInternal:   false,
		ExchangeNoWait:     false,
		ExchangeType:       "direct",
	}

	// Apply validation defaults
	applyDefaultsWithFallback(&config)

	return config
}

// Validate validates the Config using go-validatorx
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate the main config struct
	result := validatorx.ValidateStruct(c)
	if !result.Valid {
		if len(result.Errors) == 0 {
			return fmt.Errorf("config validation failed: unknown error")
		}
		return fmt.Errorf("config validation failed: %s", result.Errors[0].Message)
	}

	// Validate nested ProducerConfig
	producerResult := validatorx.ValidateStruct(c.ProducerConfig)
	if !producerResult.Valid {
		if len(producerResult.Errors) == 0 {
			return fmt.Errorf("producer config validation failed: unknown error")
		}
		return fmt.Errorf("producer config validation failed: %s", producerResult.Errors[0].Message)
	}

	// Validate nested ConsumerConfig
	consumerResult := validatorx.ValidateStruct(c.ConsumerConfig)
	if !consumerResult.Valid {
		if len(consumerResult.Errors) == 0 {
			return fmt.Errorf("consumer config validation failed: unknown error")
		}
		return fmt.Errorf("consumer config validation failed: %s", consumerResult.Errors[0].Message)
	}

	return nil
}

// SetDefaults sets default values for the Config using go-validatorx
func (c *Config) SetDefaults() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	return defaults.Set(c)
}

// ValidateAndSetDefaults validates the Config and sets default values using go-validatorx
func (c *Config) ValidateAndSetDefaults() error {
	if err := c.SetDefaults(); err != nil {
		return fmt.Errorf("failed to set defaults: %w", err)
	}
	return c.Validate()
}
