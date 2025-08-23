// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"time"
)

// Config represents the main configuration structure.
type Config struct {
	// Transport specifies the messaging transport to use (e.g., "rabbitmq", "kafka")
	Transport string `yaml:"transport" env:"MSG_TRANSPORT" validate:"required,oneof=rabbitmq kafka"`

	// RabbitMQ configuration (when transport is "rabbitmq")
	RabbitMQ *RabbitMQConfig `yaml:"rabbitmq,omitempty"`

	// Telemetry configuration
	Telemetry *TelemetryConfig `yaml:"telemetry,omitempty"`
}

// RabbitMQConfig contains RabbitMQ-specific configuration.
type RabbitMQConfig struct {
	// URIs is a list of RabbitMQ connection URIs
	URIs []string `yaml:"uris" env:"MSG_RABBITMQ_URIS" validate:"required,min=1,dive,url"`

	// ConnectionPool configuration for connection management
	ConnectionPool *ConnectionPoolConfig `yaml:"connectionPool,omitempty"`

	// ChannelPool configuration for channel management
	ChannelPool *ChannelPoolConfig `yaml:"channelPool,omitempty"`

	// Topology configuration for exchanges, queues, and bindings
	Topology *TopologyConfig `yaml:"topology,omitempty"`

	// Publisher configuration
	Publisher *PublisherConfig `yaml:"publisher,omitempty"`

	// Consumer configuration
	Consumer *ConsumerConfig `yaml:"consumer,omitempty"`

	// TLS configuration
	TLS *TLSConfig `yaml:"tls,omitempty"`

	// Security configuration
	Security *SecurityConfig `yaml:"security,omitempty"`

	// Advanced features configuration
	Persistence    *MessagePersistenceConfig    `yaml:"persistence,omitempty"`
	DLQ            *DeadLetterQueueConfig       `yaml:"dlq,omitempty"`
	Transformation *MessageTransformationConfig `yaml:"transformation,omitempty"`
	Routing        *AdvancedRoutingConfig       `yaml:"routing,omitempty"`
}

// ConnectionPoolConfig defines connection pool settings.
type ConnectionPoolConfig struct {
	// Min is the minimum number of connections to maintain
	Min int `yaml:"min" env:"MSG_RABBITMQ_CONNECTIONPOOL_MIN" validate:"min=1,max=100" default:"2"`

	// Max is the maximum number of connections allowed
	Max int `yaml:"max" env:"MSG_RABBITMQ_CONNECTIONPOOL_MAX" validate:"min=1,max=1000" default:"8"`

	// HealthCheckInterval is the interval between connection health checks
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval" env:"MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL" default:"30s"`

	// ConnectionTimeout is the timeout for establishing new connections
	ConnectionTimeout time.Duration `yaml:"connectionTimeout" env:"MSG_RABBITMQ_CONNECTIONPOOL_CONNECTIONTIMEOUT" default:"10s"`

	// HeartbeatInterval is the heartbeat interval for connections
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval" env:"MSG_RABBITMQ_CONNECTIONPOOL_HEARTBEATINTERVAL" default:"10s"`
}

// ChannelPoolConfig defines channel pool settings.
type ChannelPoolConfig struct {
	// PerConnectionMin is the minimum number of channels per connection
	PerConnectionMin int `yaml:"perConnectionMin" env:"MSG_RABBITMQ_CHANNELPOOL_PERCONNECTIONMIN" validate:"min=1,max=1000" default:"10"`

	// PerConnectionMax is the maximum number of channels per connection
	PerConnectionMax int `yaml:"perConnectionMax" env:"MSG_RABBITMQ_CHANNELPOOL_PERCONNECTIONMAX" validate:"min=1,max=10000" default:"100"`

	// BorrowTimeout is the timeout for borrowing a channel
	BorrowTimeout time.Duration `yaml:"borrowTimeout" env:"MSG_RABBITMQ_CHANNELPOOL_BORROWTIMEOUT" default:"5s"`

	// HealthCheckInterval is the interval between channel health checks
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval" env:"MSG_RABBITMQ_CHANNELPOOL_HEALTHCHECKINTERVAL" default:"30s"`
}

// TopologyConfig defines the messaging topology.
type TopologyConfig struct {
	// Exchanges to declare
	Exchanges []ExchangeConfig `yaml:"exchanges,omitempty"`

	// Queues to declare
	Queues []QueueConfig `yaml:"queues,omitempty"`

	// Bindings between exchanges and queues
	Bindings []BindingConfig `yaml:"bindings,omitempty"`

	// DeadLetterExchange is the default dead letter exchange
	DeadLetterExchange string `yaml:"deadLetterExchange" env:"MSG_RABBITMQ_TOPOLOGY_DEADLETTEREXCHANGE" default:"dlx"`

	// AutoCreateDeadLetter enables automatic dead letter exchange/queue creation
	AutoCreateDeadLetter bool `yaml:"autoCreateDeadLetter" env:"MSG_RABBITMQ_TOPOLOGY_AUTOCREATEDEADLETTER" default:"true"`
}

// ExchangeConfig defines an exchange configuration.
type ExchangeConfig struct {
	// Name of the exchange
	Name string `yaml:"name" validate:"required"`

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
	Arguments map[string]interface{} `yaml:"args,omitempty"`
}

// QueueConfig defines a queue configuration.
type QueueConfig struct {
	// Name of the queue
	Name string `yaml:"name" validate:"required"`

	// Durable indicates if the queue survives broker restart
	Durable bool `yaml:"durable" default:"true"`

	// AutoDelete indicates if the queue is deleted when no consumers are connected
	AutoDelete bool `yaml:"autoDelete" default:"false"`

	// Exclusive indicates if the queue is exclusive to the connection
	Exclusive bool `yaml:"exclusive" default:"false"`

	// NoWait indicates if the declaration should not wait for confirmation
	NoWait bool `yaml:"noWait" default:"false"`

	// Arguments for the queue
	Arguments map[string]interface{} `yaml:"args,omitempty"`

	// Priority indicates if this queue supports priority messages
	Priority bool `yaml:"priority" default:"false"`

	// MaxPriority is the maximum priority level (1-255)
	MaxPriority int `yaml:"maxPriority" validate:"min=1,max=255" default:"10"`

	// DeadLetterExchange is the dead letter exchange for this queue
	DeadLetterExchange string `yaml:"deadLetterExchange,omitempty"`

	// DeadLetterRoutingKey is the routing key for dead letter messages
	DeadLetterRoutingKey string `yaml:"deadLetterRoutingKey,omitempty"`
}

// BindingConfig defines a binding configuration.
type BindingConfig struct {
	// Exchange is the source exchange
	Exchange string `yaml:"exchange" validate:"required"`

	// Queue is the target queue
	Queue string `yaml:"queue" validate:"required"`

	// Key is the routing key
	Key string `yaml:"key" validate:"required"`

	// NoWait indicates if the binding should not wait for confirmation
	NoWait bool `yaml:"noWait" default:"false"`

	// Arguments for the binding
	Arguments map[string]interface{} `yaml:"args,omitempty"`
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
	PublishTimeout time.Duration `yaml:"publishTimeout" env:"MSG_RABBITMQ_PUBLISHER_PUBLISHTIMEOUT" default:"2s"`

	// WorkerCount is the number of publisher workers
	WorkerCount int `yaml:"workerCount" env:"MSG_RABBITMQ_PUBLISHER_WORKERCOUNT" validate:"min=1,max=100" default:"4"`

	// Retry configuration
	Retry *RetryConfig `yaml:"retry,omitempty"`

	// Serialization configuration
	Serialization *SerializationConfig `yaml:"serialization,omitempty"`
}

// ConsumerConfig defines consumer settings.
type ConsumerConfig struct {
	// Queue is the queue to consume from
	Queue string `yaml:"queue" env:"MSG_RABBITMQ_CONSUMER_QUEUE" validate:"required"`

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
	Arguments map[string]interface{} `yaml:"args,omitempty"`

	// HandlerTimeout is the timeout for message handlers
	HandlerTimeout time.Duration `yaml:"handlerTimeout" env:"MSG_RABBITMQ_CONSUMER_HANDLERTIMEOUT" default:"30s"`

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
	CAFile string `yaml:"caFile" env:"MSG_RABBITMQ_TLS_CAFILE"`

	// CertFile is the path to the client certificate file
	CertFile string `yaml:"certFile" env:"MSG_RABBITMQ_TLS_CERTFILE"`

	// KeyFile is the path to the client private key file
	KeyFile string `yaml:"keyFile" env:"MSG_RABBITMQ_TLS_KEYFILE"`

	// MinVersion is the minimum TLS version
	MinVersion string `yaml:"minVersion" env:"MSG_RABBITMQ_TLS_MINVERSION" default:"1.2"`

	// InsecureSkipVerify disables certificate verification (not recommended for production)
	InsecureSkipVerify bool `yaml:"insecureSkipVerify" env:"MSG_RABBITMQ_TLS_INSECURESKIPVERIFY" default:"false"`

	// ServerName is the expected server name for certificate verification
	ServerName string `yaml:"serverName" env:"MSG_RABBITMQ_TLS_SERVERNAME"`
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	// HMACEnabled enables HMAC message signing
	HMACEnabled bool `yaml:"hmacEnabled" env:"MSG_RABBITMQ_SECURITY_HMACENABLED" default:"false"`

	// HMACSecret is the secret key for HMAC signing
	HMACSecret string `yaml:"hmacSecret" env:"MSG_RABBITMQ_SECURITY_HMACSECRET"`

	// HMACAlgorithm is the HMAC algorithm to use
	HMACAlgorithm string `yaml:"hmacAlgorithm" env:"MSG_RABBITMQ_SECURITY_HMACALGORITHM" default:"sha256"`

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
	StoragePath string `yaml:"storagePath" env:"MSG_PERSISTENCE_STORAGE_PATH" default:"./message-storage"`

	// MaxStorageSize is the maximum storage size in bytes
	MaxStorageSize int64 `yaml:"maxStorageSize" env:"MSG_PERSISTENCE_MAX_STORAGE_SIZE" default:"1073741824"` // 1GB

	// CleanupInterval is the interval for cleaning up old messages
	CleanupInterval time.Duration `yaml:"cleanupInterval" env:"MSG_PERSISTENCE_CLEANUP_INTERVAL" default:"1h"`

	// MessageTTL is the time-to-live for persisted messages
	MessageTTL time.Duration `yaml:"messageTTL" env:"MSG_PERSISTENCE_MESSAGE_TTL" default:"24h"`
}

// DeadLetterQueueConfig defines dead letter queue settings.
type DeadLetterQueueConfig struct {
	// Enabled enables dead letter queue functionality
	Enabled bool `yaml:"enabled" env:"MSG_DLQ_ENABLED" default:"true"`

	// Exchange is the dead letter exchange name
	Exchange string `yaml:"exchange" env:"MSG_DLQ_EXCHANGE" default:"dlx"`

	// Queue is the dead letter queue name
	Queue string `yaml:"queue" env:"MSG_DLQ_QUEUE" default:"dlq"`

	// RoutingKey is the routing key for dead letter messages
	RoutingKey string `yaml:"routingKey" env:"MSG_DLQ_ROUTING_KEY" default:"dlq"`

	// MaxRetries is the maximum number of retries before sending to DLQ
	MaxRetries int `yaml:"maxRetries" env:"MSG_DLQ_MAX_RETRIES" validate:"min=0,max=100" default:"3"`

	// RetryDelay is the delay between retries
	RetryDelay time.Duration `yaml:"retryDelay" env:"MSG_DLQ_RETRY_DELAY" default:"5s"`

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
	SchemaRegistryURL string `yaml:"schemaRegistryURL" env:"MSG_TRANSFORMATION_SCHEMA_REGISTRY_URL"`
}

// AdvancedRoutingConfig defines advanced routing settings.
type AdvancedRoutingConfig struct {
	// Enabled enables advanced routing
	Enabled bool `yaml:"enabled" env:"MSG_ROUTING_ENABLED" default:"false"`

	// DynamicRouting enables dynamic routing based on message content
	DynamicRouting bool `yaml:"dynamicRouting" env:"MSG_ROUTING_DYNAMIC_ENABLED" default:"false"`

	// RoutingRules defines routing rules
	RoutingRules []RoutingRule `yaml:"routingRules,omitempty"`

	// MessageFiltering enables message filtering
	MessageFiltering bool `yaml:"messageFiltering" env:"MSG_ROUTING_FILTERING_ENABLED" default:"false"`

	// FilterRules defines filter rules
	FilterRules []FilterRule `yaml:"filterRules,omitempty"`
}

// RoutingRule defines a routing rule.
type RoutingRule struct {
	// Name is the rule name
	Name string `yaml:"name" validate:"required"`

	// Condition is the routing condition (JSONPath expression)
	Condition string `yaml:"condition" validate:"required"`

	// TargetExchange is the target exchange
	TargetExchange string `yaml:"targetExchange" validate:"required"`

	// TargetRoutingKey is the target routing key
	TargetRoutingKey string `yaml:"targetRoutingKey" validate:"required"`

	// Priority is the rule priority (higher numbers = higher priority)
	Priority int `yaml:"priority" default:"0"`

	// Enabled enables this rule
	Enabled bool `yaml:"enabled" default:"true"`
}

// FilterRule defines a filter rule.
type FilterRule struct {
	// Name is the rule name
	Name string `yaml:"name" validate:"required"`

	// Condition is the filter condition (JSONPath expression)
	Condition string `yaml:"condition" validate:"required"`

	// Action is the filter action (accept, reject, modify)
	Action string `yaml:"action" validate:"oneof=accept reject modify" default:"accept"`

	// Priority is the rule priority (higher numbers = higher priority)
	Priority int `yaml:"priority" default:"0"`

	// Enabled enables this rule
	Enabled bool `yaml:"enabled" default:"true"`
}

// RetryConfig defines retry settings.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `yaml:"maxAttempts" env:"MSG_RABBITMQ_PUBLISHER_RETRY_MAXATTEMPTS" validate:"min=1,max=100" default:"5"`

	// BaseBackoff is the base backoff duration
	BaseBackoff time.Duration `yaml:"baseBackoff" env:"MSG_RABBITMQ_PUBLISHER_RETRY_BASEBACKOFF" default:"100ms"`

	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration `yaml:"maxBackoff" env:"MSG_RABBITMQ_PUBLISHER_RETRY_MAXBACKOFF" default:"5s"`

	// BackoffMultiplier is the backoff multiplier
	BackoffMultiplier float64 `yaml:"backoffMultiplier" env:"MSG_RABBITMQ_PUBLISHER_RETRY_BACKOFFMULTIPLIER" validate:"min=1.0,max=10.0" default:"2.0"`

	// Jitter enables jitter in backoff calculations
	Jitter bool `yaml:"jitter" env:"MSG_RABBITMQ_PUBLISHER_RETRY_JITTER" default:"true"`
}

// SerializationConfig defines serialization settings.
type SerializationConfig struct {
	// DefaultContentType is the default content type for messages
	DefaultContentType string `yaml:"defaultContentType" env:"MSG_RABBITMQ_PUBLISHER_SERIALIZATION_DEFAULTCONTENTTYPE" default:"application/json"`

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
	OTLPEndpoint string `yaml:"otlpEndpoint" env:"MSG_TELEMETRY_OTLPENDPOINT"`

	// ServiceName is the service name for telemetry
	ServiceName string `yaml:"serviceName" env:"MSG_TELEMETRY_SERVICENAME" default:"go-messagex"`

	// ServiceVersion is the service version for telemetry
	ServiceVersion string `yaml:"serviceVersion" env:"MSG_TELEMETRY_SERVICEVERSION"`
}
