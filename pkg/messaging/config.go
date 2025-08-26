// Package messaging provides transport-agnostic interfaces for messaging systems.
//
// Thread Safety: This configuration package is designed to be read-only after initialization.
// Configuration structs should not be modified after they are created and validated.
// If you need to modify configuration at runtime, create a new instance.
package messaging

import (
	"fmt"
	"time"
)

// Config represents the main configuration structure.
type Config struct {
	// Transport specifies the messaging transport to use (e.g., "rabbitmq", "kafka")
	Transport string `yaml:"transport" env:"MSG_TRANSPORT" validate:"required,oneof=rabbitmq kafka"`

	// RabbitMQ configuration (when transport is "rabbitmq")
	// Required when Transport is "rabbitmq"
	RabbitMQ *RabbitMQConfig `yaml:"rabbitmq,omitempty" validate:"required_if=Transport rabbitmq"`

	// Telemetry configuration
	Telemetry *TelemetryConfig `yaml:"telemetry,omitempty"`
}

// RabbitMQConfig contains RabbitMQ-specific configuration.
type RabbitMQConfig struct {
	// URIs is a list of RabbitMQ connection URIs
	URIs []string `yaml:"uris" env:"MSG_RABBITMQ_URIS" validate:"required,min=1,dive,url"`

	// ConnectionPool configuration for connection management
	ConnectionPool *ConnectionPoolConfig `yaml:"connectionPool,omitempty" validate:"omitempty"`

	// ChannelPool configuration for channel management
	ChannelPool *ChannelPoolConfig `yaml:"channelPool,omitempty" validate:"omitempty"`

	// Topology configuration for exchanges, queues, and bindings
	Topology *TopologyConfig `yaml:"topology,omitempty" validate:"omitempty"`

	// Publisher configuration
	Publisher *PublisherConfig `yaml:"publisher,omitempty" validate:"omitempty"`

	// Consumer configuration
	Consumer *ConsumerConfig `yaml:"consumer,omitempty" validate:"omitempty"`

	// TLS configuration
	TLS *TLSConfig `yaml:"tls,omitempty" validate:"omitempty"`

	// Security configuration
	Security *SecurityConfig `yaml:"security,omitempty" validate:"omitempty"`

	// Advanced features configuration
	Persistence    *MessagePersistenceConfig    `yaml:"persistence,omitempty" validate:"omitempty"`
	DLQ            *DeadLetterQueueConfig       `yaml:"dlq,omitempty" validate:"omitempty"`
	Transformation *MessageTransformationConfig `yaml:"transformation,omitempty" validate:"omitempty"`
	Routing        *AdvancedRoutingConfig       `yaml:"routing,omitempty" validate:"omitempty"`
}

// ConnectionPoolConfig defines connection pool settings.
type ConnectionPoolConfig struct {
	// Min is the minimum number of connections to maintain
	Min int `yaml:"min" env:"MSG_RABBITMQ_CONNECTIONPOOL_MIN" validate:"min=1,max=100" default:"2"`

	// Max is the maximum number of connections allowed
	Max int `yaml:"max" env:"MSG_RABBITMQ_CONNECTIONPOOL_MAX" validate:"min=1,max=1000" default:"8"`

	// HealthCheckInterval is the interval between connection health checks
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval" env:"MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL" validate:"min=1s,max=300s" default:"30s"`

	// ConnectionTimeout is the timeout for establishing new connections
	ConnectionTimeout time.Duration `yaml:"connectionTimeout" env:"MSG_RABBITMQ_CONNECTIONPOOL_CONNECTIONTIMEOUT" validate:"min=1s,max=60s" default:"10s"`

	// HeartbeatInterval is the heartbeat interval for connections
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval" env:"MSG_RABBITMQ_CONNECTIONPOOL_HEARTBEATINTERVAL" validate:"min=1s,max=60s" default:"10s"`
}

// ChannelPoolConfig defines channel pool settings.
type ChannelPoolConfig struct {
	// PerConnectionMin is the minimum number of channels per connection
	PerConnectionMin int `yaml:"perConnectionMin" env:"MSG_RABBITMQ_CHANNELPOOL_PERCONNECTIONMIN" validate:"min=1,max=1000" default:"10"`

	// PerConnectionMax is the maximum number of channels per connection
	PerConnectionMax int `yaml:"perConnectionMax" env:"MSG_RABBITMQ_CHANNELPOOL_PERCONNECTIONMAX" validate:"min=1,max=10000" default:"100"`

	// BorrowTimeout is the timeout for borrowing a channel
	BorrowTimeout time.Duration `yaml:"borrowTimeout" env:"MSG_RABBITMQ_CHANNELPOOL_BORROWTIMEOUT" validate:"min=100ms,max=30s" default:"5s"`

	// HealthCheckInterval is the interval between channel health checks
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval" env:"MSG_RABBITMQ_CHANNELPOOL_HEALTHCHECKINTERVAL" validate:"min=1s,max=300s" default:"30s"`
}

// TopologyConfig defines the messaging topology.
type TopologyConfig struct {
	// Exchanges to declare
	Exchanges []ExchangeConfig `yaml:"exchanges,omitempty" validate:"omitempty,dive"`

	// Queues to declare
	Queues []QueueConfig `yaml:"queues,omitempty" validate:"omitempty,dive"`

	// Bindings between exchanges and queues
	Bindings []BindingConfig `yaml:"bindings,omitempty" validate:"omitempty,dive"`

	// DeadLetterExchange is the default dead letter exchange
	DeadLetterExchange string `yaml:"deadLetterExchange" env:"MSG_RABBITMQ_TOPOLOGY_DEADLETTEREXCHANGE" validate:"omitempty,min=1,max=255" default:"dlx"`

	// AutoCreateDeadLetter enables automatic dead letter exchange/queue creation
	AutoCreateDeadLetter bool `yaml:"autoCreateDeadLetter" env:"MSG_RABBITMQ_TOPOLOGY_AUTOCREATEDEADLETTER" default:"true"`
}

// ExchangeConfig defines an exchange configuration.
type ExchangeConfig struct {
	// Name of the exchange
	Name string `yaml:"name" validate:"required,min=1,max=255"`

	// Type of the exchange (direct, fanout, topic, headers)
	Type string `yaml:"type" validate:"required,oneof=direct fanout topic headers"`

	// Durable indicates if the exchange survives broker restart
	Durable bool `yaml:"durable" default:"true"`

	// AutoDelete indicates if the exchange is deleted when no queues are bound
	AutoDelete bool `yaml:"autoDelete" default:"false"`

	// Internal indicates if the exchange is internal (not for client use)
	Internal bool `yaml:"internal" default:"false"`

	// NoWait indicates if the declaration should not wait for confirmation
	NoWait bool `yaml:"noWait" default:"false"`

	// Arguments for the exchange
	Arguments map[string]interface{} `yaml:"args,omitempty" validate:"omitempty"`
}

// QueueConfig defines a queue configuration.
type QueueConfig struct {
	// Name of the queue
	Name string `yaml:"name" validate:"required,min=1,max=255"`

	// Durable indicates if the queue survives broker restart
	Durable bool `yaml:"durable" default:"true"`

	// AutoDelete indicates if the queue is deleted when no consumers are connected
	AutoDelete bool `yaml:"autoDelete" default:"false"`

	// Exclusive indicates if the queue is exclusive to the connection
	Exclusive bool `yaml:"exclusive" default:"false"`

	// NoWait indicates if the declaration should not wait for confirmation
	NoWait bool `yaml:"noWait" default:"false"`

	// Arguments for the queue
	Arguments map[string]interface{} `yaml:"args,omitempty" validate:"omitempty"`

	// Priority indicates if this queue supports priority messages
	Priority bool `yaml:"priority" default:"false"`

	// MaxPriority is the maximum priority level (1-255)
	MaxPriority int `yaml:"maxPriority" validate:"min=1,max=255" default:"10"`

	// DeadLetterExchange is the dead letter exchange for this queue
	DeadLetterExchange string `yaml:"deadLetterExchange,omitempty" validate:"omitempty,min=1,max=255"`

	// DeadLetterRoutingKey is the routing key for dead letter messages
	DeadLetterRoutingKey string `yaml:"deadLetterRoutingKey,omitempty" validate:"omitempty,min=1,max=255"`
}

// BindingConfig defines a binding configuration.
type BindingConfig struct {
	// Exchange is the source exchange
	Exchange string `yaml:"exchange" validate:"required,min=1,max=255"`

	// Queue is the target queue
	Queue string `yaml:"queue" validate:"required,min=1,max=255"`

	// Key is the routing key
	Key string `yaml:"key" validate:"required,min=1,max=255"`

	// NoWait indicates if the binding should not wait for confirmation
	NoWait bool `yaml:"noWait" default:"false"`

	// Arguments for the binding
	Arguments map[string]interface{} `yaml:"args,omitempty" validate:"omitempty"`
}

// PublisherConfig defines publisher settings.
type PublisherConfig struct {
	// Confirms enables publisher confirms
	Confirms bool `yaml:"confirms" env:"MSG_RABBITMQ_PUBLISHER_CONFIRMS" default:"true"`

	// Mandatory indicates if messages must be routable
	Mandatory bool `yaml:"mandatory" env:"MSG_RABBITMQ_PUBLISHER_MANDATORY" default:"true"`

	// Immediate indicates if messages must be deliverable immediately
	Immediate bool `yaml:"immediate" env:"MSG_RABBITMQ_PUBLISHER_IMMEDIATE" default:"false"`

	// MaxInFlight is the maximum number of unconfirmed messages
	MaxInFlight int `yaml:"maxInFlight" env:"MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT" validate:"min=1,max=100000" default:"10000"`

	// DropOnOverflow indicates if messages should be dropped when queue is full
	DropOnOverflow bool `yaml:"dropOnOverflow" env:"MSG_RABBITMQ_PUBLISHER_DROPONOVERFLOW" default:"false"`

	// PublishTimeout is the timeout for publish operations
	PublishTimeout time.Duration `yaml:"publishTimeout" env:"MSG_RABBITMQ_PUBLISHER_PUBLISHTIMEOUT" validate:"min=100ms,max=60s" default:"2s"`

	// WorkerCount is the number of publisher workers
	WorkerCount int `yaml:"workerCount" env:"MSG_RABBITMQ_PUBLISHER_WORKERCOUNT" validate:"min=1,max=100" default:"4"`

	// Retry configuration
	Retry *RetryConfig `yaml:"retry,omitempty" validate:"omitempty"`

	// Serialization configuration
	Serialization *SerializationConfig `yaml:"serialization,omitempty" validate:"omitempty"`
}

// ConsumerConfig defines consumer settings.
type ConsumerConfig struct {
	// Queue is the queue to consume from
	Queue string `yaml:"queue" env:"MSG_RABBITMQ_CONSUMER_QUEUE" validate:"required,min=1,max=255"`

	// Prefetch is the number of messages to prefetch
	Prefetch int `yaml:"prefetch" env:"MSG_RABBITMQ_CONSUMER_PREFETCH" validate:"min=1,max=65535" default:"256"`

	// MaxConcurrentHandlers is the maximum number of concurrent message handlers
	MaxConcurrentHandlers int `yaml:"maxConcurrentHandlers" env:"MSG_RABBITMQ_CONSUMER_MAXCONCURRENTHANDLERS" validate:"min=1,max=10000" default:"512"`

	// RequeueOnError indicates if messages should be requeued on error
	RequeueOnError bool `yaml:"requeueOnError" env:"MSG_RABBITMQ_CONSUMER_REQUEUEONERROR" default:"true"`

	// AckOnSuccess indicates if messages should be acknowledged on success
	AckOnSuccess bool `yaml:"ackOnSuccess" env:"MSG_RABBITMQ_CONSUMER_ACKONSUCCESS" default:"true"`

	// AutoAck indicates if messages should be automatically acknowledged
	AutoAck bool `yaml:"autoAck" env:"MSG_RABBITMQ_CONSUMER_AUTOACK" default:"false"`

	// Exclusive indicates if the consumer is exclusive
	Exclusive bool `yaml:"exclusive" env:"MSG_RABBITMQ_CONSUMER_EXCLUSIVE" default:"false"`

	// NoLocal indicates if messages from the same connection should be excluded
	NoLocal bool `yaml:"noLocal" env:"MSG_RABBITMQ_CONSUMER_NOLOCAL" default:"false"`

	// NoWait indicates if the consumer should not wait for confirmation
	NoWait bool `yaml:"noWait" env:"MSG_RABBITMQ_CONSUMER_NOWAIT" default:"false"`

	// Arguments for the consumer
	Arguments map[string]interface{} `yaml:"args,omitempty" validate:"omitempty"`

	// HandlerTimeout is the timeout for message handlers
	HandlerTimeout time.Duration `yaml:"handlerTimeout" env:"MSG_RABBITMQ_CONSUMER_HANDLERTIMEOUT" validate:"min=1s,max=300s" default:"30s"`

	// PanicRecovery enables panic recovery in handlers
	PanicRecovery bool `yaml:"panicRecovery" env:"MSG_RABBITMQ_CONSUMER_PANICRECOVERY" default:"true"`

	// MaxRetries is the maximum number of retries before sending to DLQ
	MaxRetries int `yaml:"maxRetries" env:"MSG_RABBITMQ_CONSUMER_MAXRETRIES" validate:"min=0,max=100" default:"3"`
}

// TLSConfig defines TLS settings.
type TLSConfig struct {
	// Enabled indicates if TLS is enabled
	Enabled bool `yaml:"enabled" env:"MSG_RABBITMQ_TLS_ENABLED" default:"false"`

	// CAFile is the path to the CA certificate file
	CAFile string `yaml:"caFile" env:"MSG_RABBITMQ_TLS_CAFILE" validate:"omitempty,file"`

	// CertFile is the path to the client certificate file
	CertFile string `yaml:"certFile" env:"MSG_RABBITMQ_TLS_CERTFILE" validate:"required_if=Enabled true,omitempty,file"`

	// KeyFile is the path to the client private key file
	KeyFile string `yaml:"keyFile" env:"MSG_RABBITMQ_TLS_KEYFILE" validate:"required_if=Enabled true,omitempty,file"`

	// MinVersion is the minimum TLS version
	MinVersion string `yaml:"minVersion" env:"MSG_RABBITMQ_TLS_MINVERSION" validate:"omitempty,oneof=1.0 1.1 1.2 1.3" default:"1.2"`

	// InsecureSkipVerify disables certificate verification (not recommended for production)
	InsecureSkipVerify bool `yaml:"insecureSkipVerify" env:"MSG_RABBITMQ_TLS_INSECURESKIPVERIFY" default:"false"`

	// ServerName is the expected server name for certificate verification
	ServerName string `yaml:"serverName" env:"MSG_RABBITMQ_TLS_SERVERNAME" validate:"omitempty,min=1,max=255"`
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	// HMACEnabled enables HMAC message signing
	HMACEnabled bool `yaml:"hmacEnabled" env:"MSG_RABBITMQ_SECURITY_HMACENABLED" default:"false"`

	// HMACSecret is the secret key for HMAC signing
	HMACSecret string `yaml:"hmacSecret" env:"MSG_RABBITMQ_SECURITY_HMACSECRET" validate:"required_if=HMACEnabled true,omitempty,min=32"`

	// HMACAlgorithm is the HMAC algorithm to use
	HMACAlgorithm string `yaml:"hmacAlgorithm" env:"MSG_RABBITMQ_SECURITY_HMACALGORITHM" validate:"omitempty,oneof=sha1 sha256 sha512" default:"sha256"`

	// VerifyHostname enables hostname verification
	VerifyHostname bool `yaml:"verifyHostname" env:"MSG_RABBITMQ_SECURITY_VERIFYHOSTNAME" default:"true"`
}

// MessagePersistenceConfig defines message persistence settings.
type MessagePersistenceConfig struct {
	// Enabled enables message persistence
	Enabled bool `yaml:"enabled" env:"MSG_PERSISTENCE_ENABLED" default:"true"`

	// StorageType defines the storage type (memory, disk, redis)
	StorageType string `yaml:"storageType" env:"MSG_PERSISTENCE_STORAGE_TYPE" validate:"oneof=memory disk redis" default:"memory"`

	// StoragePath is the path for disk storage
	StoragePath string `yaml:"storagePath" env:"MSG_PERSISTENCE_STORAGE_PATH" validate:"omitempty,min=1,max=500" default:"./message-storage"`

	// MaxStorageSize is the maximum storage size in bytes (reduced from 1GB to 100MB)
	MaxStorageSize int64 `yaml:"maxStorageSize" env:"MSG_PERSISTENCE_MAX_STORAGE_SIZE" validate:"min=1048576,max=10737418240" default:"104857600"` // 100MB

	// CleanupInterval is the interval for cleaning up old messages
	CleanupInterval time.Duration `yaml:"cleanupInterval" env:"MSG_PERSISTENCE_CLEANUP_INTERVAL" validate:"min=1m,max=24h" default:"1h"`

	// MessageTTL is the time-to-live for persisted messages
	MessageTTL time.Duration `yaml:"messageTTL" env:"MSG_PERSISTENCE_MESSAGE_TTL" validate:"min=1m,max=168h" default:"24h"`
}

// DeadLetterQueueConfig defines dead letter queue settings.
type DeadLetterQueueConfig struct {
	// Enabled enables dead letter queue functionality
	Enabled bool `yaml:"enabled" env:"MSG_DLQ_ENABLED" default:"true"`

	// Exchange is the dead letter exchange name
	Exchange string `yaml:"exchange" env:"MSG_DLQ_EXCHANGE" validate:"omitempty,min=1,max=255" default:"dlx"`

	// Queue is the dead letter queue name
	Queue string `yaml:"queue" env:"MSG_DLQ_QUEUE" validate:"omitempty,min=1,max=255" default:"dlq"`

	// RoutingKey is the routing key for dead letter messages
	RoutingKey string `yaml:"routingKey" env:"MSG_DLQ_ROUTING_KEY" validate:"omitempty,min=1,max=255" default:"dlq"`

	// MaxRetries is the maximum number of retries before sending to DLQ
	MaxRetries int `yaml:"maxRetries" env:"MSG_DLQ_MAX_RETRIES" validate:"min=0,max=100" default:"3"`

	// RetryDelay is the delay between retries
	RetryDelay time.Duration `yaml:"retryDelay" env:"MSG_DLQ_RETRY_DELAY" validate:"min=100ms,max=300s" default:"5s"`

	// AutoCreate enables automatic creation of DLQ exchange and queue
	AutoCreate bool `yaml:"autoCreate" env:"MSG_DLQ_AUTO_CREATE" default:"true"`
}

// MessageTransformationConfig defines message transformation settings.
type MessageTransformationConfig struct {
	// Enabled enables message transformation
	Enabled bool `yaml:"enabled" env:"MSG_TRANSFORMATION_ENABLED" default:"false"`

	// CompressionEnabled enables message compression
	CompressionEnabled bool `yaml:"compressionEnabled" env:"MSG_TRANSFORMATION_COMPRESSION_ENABLED" default:"false"`

	// CompressionLevel is the compression level (1-9)
	CompressionLevel int `yaml:"compressionLevel" env:"MSG_TRANSFORMATION_COMPRESSION_LEVEL" validate:"min=1,max=9" default:"6"`

	// SerializationFormat is the serialization format (json, protobuf, avro)
	SerializationFormat string `yaml:"serializationFormat" env:"MSG_TRANSFORMATION_SERIALIZATION_FORMAT" validate:"oneof=json protobuf avro" default:"json"`

	// SchemaValidation enables schema validation
	SchemaValidation bool `yaml:"schemaValidation" env:"MSG_TRANSFORMATION_SCHEMA_VALIDATION" default:"false"`

	// SchemaRegistryURL is the URL for the schema registry
	SchemaRegistryURL string `yaml:"schemaRegistryURL" env:"MSG_TRANSFORMATION_SCHEMA_REGISTRY_URL" validate:"omitempty,url"`
}

// AdvancedRoutingConfig defines advanced routing settings.
type AdvancedRoutingConfig struct {
	// Enabled enables advanced routing
	Enabled bool `yaml:"enabled" env:"MSG_ROUTING_ENABLED" default:"false"`

	// DynamicRouting enables dynamic routing based on message content
	DynamicRouting bool `yaml:"dynamicRouting" env:"MSG_ROUTING_DYNAMIC_ENABLED" default:"false"`

	// RoutingRules defines routing rules
	RoutingRules []RoutingRule `yaml:"routingRules,omitempty" validate:"omitempty,dive"`

	// MessageFiltering enables message filtering
	MessageFiltering bool `yaml:"messageFiltering" env:"MSG_ROUTING_FILTERING_ENABLED" default:"false"`

	// FilterRules defines filter rules
	FilterRules []FilterRule `yaml:"filterRules,omitempty" validate:"omitempty,dive"`
}

// RoutingRule defines a routing rule.
type RoutingRule struct {
	// Name is the rule name
	Name string `yaml:"name" validate:"required,min=1,max=255"`

	// Condition is the routing condition (JSONPath expression)
	Condition string `yaml:"condition" validate:"required,min=1,max=1000"`

	// TargetExchange is the target exchange
	TargetExchange string `yaml:"targetExchange" validate:"required,min=1,max=255"`

	// TargetRoutingKey is the target routing key
	TargetRoutingKey string `yaml:"targetRoutingKey" validate:"required,min=1,max=255"`

	// Priority is the rule priority (higher numbers = higher priority)
	Priority int `yaml:"priority" validate:"min=0,max=1000" default:"0"`

	// Enabled enables this rule
	Enabled bool `yaml:"enabled" default:"true"`
}

// FilterRule defines a filter rule.
type FilterRule struct {
	// Name is the rule name
	Name string `yaml:"name" validate:"required,min=1,max=255"`

	// Condition is the filter condition (JSONPath expression)
	Condition string `yaml:"condition" validate:"required,min=1,max=1000"`

	// Action is the filter action (accept, reject, modify)
	Action string `yaml:"action" validate:"oneof=accept reject modify" default:"accept"`

	// Priority is the rule priority (higher numbers = higher priority)
	Priority int `yaml:"priority" validate:"min=0,max=1000" default:"0"`

	// Enabled enables this rule
	Enabled bool `yaml:"enabled" default:"true"`
}

// RetryConfig defines retry settings.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `yaml:"maxAttempts" env:"MSG_RABBITMQ_PUBLISHER_RETRY_MAXATTEMPTS" validate:"min=1,max=100" default:"5"`

	// BaseBackoff is the base backoff duration
	BaseBackoff time.Duration `yaml:"baseBackoff" env:"MSG_RABBITMQ_PUBLISHER_RETRY_BASEBACKOFF" validate:"min=10ms,max=10s" default:"100ms"`

	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration `yaml:"maxBackoff" env:"MSG_RABBITMQ_PUBLISHER_RETRY_MAXBACKOFF" validate:"min=100ms,max=60s" default:"5s"`

	// BackoffMultiplier is the backoff multiplier
	BackoffMultiplier float64 `yaml:"backoffMultiplier" env:"MSG_RABBITMQ_PUBLISHER_RETRY_BACKOFFMULTIPLIER" validate:"min=1.0,max=10.0" default:"2.0"`

	// Jitter enables jitter in backoff calculations
	Jitter bool `yaml:"jitter" env:"MSG_RABBITMQ_PUBLISHER_RETRY_JITTER" default:"true"`
}

// SerializationConfig defines serialization settings.
type SerializationConfig struct {
	// DefaultContentType is the default content type for messages
	DefaultContentType string `yaml:"defaultContentType" env:"MSG_RABBITMQ_PUBLISHER_SERIALIZATION_DEFAULTCONTENTTYPE" validate:"omitempty,min=1,max=100" default:"application/json"`

	// CompressionEnabled enables message compression
	CompressionEnabled bool `yaml:"compressionEnabled" env:"MSG_RABBITMQ_PUBLISHER_SERIALIZATION_COMPRESSIONENABLED" default:"false"`

	// CompressionLevel is the compression level (1-9)
	CompressionLevel int `yaml:"compressionLevel" env:"MSG_RABBITMQ_PUBLISHER_SERIALIZATION_COMPRESSIONLEVEL" validate:"min=1,max=9" default:"6"`
}

// TelemetryConfig defines telemetry settings.
type TelemetryConfig struct {
	// MetricsEnabled enables metrics collection
	MetricsEnabled bool `yaml:"metricsEnabled" env:"MSG_TELEMETRY_METRICSENABLED" default:"true"`

	// TracingEnabled enables distributed tracing
	TracingEnabled bool `yaml:"tracingEnabled" env:"MSG_TELEMETRY_TRACINGENABLED" default:"true"`

	// OTLPEndpoint is the OpenTelemetry endpoint
	OTLPEndpoint string `yaml:"otlpEndpoint" env:"MSG_TELEMETRY_OTLPENDPOINT" validate:"omitempty,url"`

	// ServiceName is the service name for telemetry
	ServiceName string `yaml:"serviceName" env:"MSG_TELEMETRY_SERVICENAME" validate:"omitempty,min=1,max=255" default:"go-messagex"`

	// ServiceVersion is the service version for telemetry
	ServiceVersion string `yaml:"serviceVersion" env:"MSG_TELEMETRY_SERVICEVERSION" validate:"omitempty,min=1,max=50"`
}

// IsValid returns true if the configuration is valid for the specified transport.
// This method performs additional validation beyond struct tags.
func (c *Config) IsValid() error {
	if c.Transport == "rabbitmq" && c.RabbitMQ == nil {
		return fmt.Errorf("RabbitMQ configuration is required when transport is 'rabbitmq'")
	}

	if c.RabbitMQ != nil {
		if err := c.RabbitMQ.IsValid(); err != nil {
			return fmt.Errorf("RabbitMQ configuration error: %w", err)
		}
	}

	return nil
}

// IsValid returns true if the RabbitMQ configuration is valid.
func (r *RabbitMQConfig) IsValid() error {
	if len(r.URIs) == 0 {
		return fmt.Errorf("at least one URI is required")
	}

	// Validate TLS configuration
	if r.TLS != nil && r.TLS.Enabled {
		if r.TLS.CertFile == "" || r.TLS.KeyFile == "" {
			return fmt.Errorf("both CertFile and KeyFile are required when TLS is enabled")
		}
	}

	// Validate security configuration
	if r.Security != nil && r.Security.HMACEnabled {
		if r.Security.HMACSecret == "" {
			return fmt.Errorf("HMACSecret is required when HMAC is enabled")
		}
		if len(r.Security.HMACSecret) < 32 {
			return fmt.Errorf("HMACSecret must be at least 32 characters long")
		}
	}

	return nil
}

// GetConnectionPool returns the connection pool configuration with defaults applied.
func (r *RabbitMQConfig) GetConnectionPool() *ConnectionPoolConfig {
	if r.ConnectionPool == nil {
		return &ConnectionPoolConfig{
			Min:                 2,
			Max:                 8,
			HealthCheckInterval: 30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			HeartbeatInterval:   10 * time.Second,
		}
	}
	return r.ConnectionPool
}

// GetChannelPool returns the channel pool configuration with defaults applied.
func (r *RabbitMQConfig) GetChannelPool() *ChannelPoolConfig {
	if r.ChannelPool == nil {
		return &ChannelPoolConfig{
			PerConnectionMin:    10,
			PerConnectionMax:    100,
			BorrowTimeout:       5 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}
	}
	return r.ChannelPool
}

// GetPublisher returns the publisher configuration with defaults applied.
func (r *RabbitMQConfig) GetPublisher() *PublisherConfig {
	if r.Publisher == nil {
		return &PublisherConfig{
			Confirms:       true,
			Mandatory:      true,
			Immediate:      false,
			MaxInFlight:    10000,
			DropOnOverflow: false,
			PublishTimeout: 2 * time.Second,
			WorkerCount:    4,
		}
	}
	return r.Publisher
}

// GetConsumer returns the consumer configuration with defaults applied.
func (r *RabbitMQConfig) GetConsumer() *ConsumerConfig {
	if r.Consumer == nil {
		return &ConsumerConfig{
			Queue:                 "",
			Prefetch:              256,
			MaxConcurrentHandlers: 512,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			Exclusive:             false,
			NoLocal:               false,
			NoWait:                false,
			HandlerTimeout:        30 * time.Second,
			PanicRecovery:         true,
			MaxRetries:            3,
		}
	}
	return r.Consumer
}

// GetTLS returns the TLS configuration with defaults applied.
func (r *RabbitMQConfig) GetTLS() *TLSConfig {
	if r.TLS == nil {
		return &TLSConfig{
			Enabled:            false,
			MinVersion:         "1.2",
			InsecureSkipVerify: false,
		}
	}
	return r.TLS
}

// GetSecurity returns the security configuration with defaults applied.
func (r *RabbitMQConfig) GetSecurity() *SecurityConfig {
	if r.Security == nil {
		return &SecurityConfig{
			HMACEnabled:    false,
			HMACAlgorithm:  "sha256",
			VerifyHostname: true,
		}
	}
	return r.Security
}
