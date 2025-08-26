// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import "time"

// Validation constants for message and configuration limits
const (
	// Message size limits
	// MaxMessageSize defines the maximum allowed message size (10MB)
	// This limit prevents memory exhaustion and ensures system stability
	MaxMessageSize = 10 * 1024 * 1024 // 10MB maximum message size
	// MinMessageSize defines the minimum allowed message size
	// Messages smaller than this may indicate corruption or incomplete data
	MinMessageSize = 1 // 1 byte minimum message size
	// MaxHeaderSize defines the maximum allowed header size (1MB)
	// Headers larger than this could impact performance and memory usage
	MaxHeaderSize = 1024 * 1024 // 1MB maximum header size

	// String length limits
	// These limits prevent buffer overflow attacks and ensure database compatibility
	MaxTopicLength          = 255 // Maximum topic name length
	MaxRoutingKeyLength     = 255 // Maximum routing key length
	MaxMessageIDLength      = 255 // Maximum message ID length
	MaxCorrelationIDLength  = 255 // Maximum correlation ID length
	MaxReplyToLength        = 255 // Maximum reply-to address length
	MaxIdempotencyKeyLength = 255 // Maximum idempotency key length

	// Priority limits (8-bit unsigned integer range)
	// Standardized priority range for both message and queue priorities
	MaxPriority = uint8(255) // Maximum priority value (highest priority)
	MinPriority = uint8(0)   // Minimum priority value (lowest priority)

	// Timeout limits
	// MaxTimeout defines the maximum allowed timeout (1 hour)
	// Reduced from 24 hours to prevent resource exhaustion
	MaxTimeout = 60 * 60 * time.Second // 1 hour maximum timeout
	// MinTimeout defines the minimum allowed timeout
	// Prevents excessive polling and ensures reasonable responsiveness
	MinTimeout = 1 * time.Millisecond // 1ms minimum timeout

	// Connection pool limits
	// These limits prevent resource exhaustion while allowing scalability
	MaxConnectionPoolSize = 1000  // Maximum connections in pool
	MinConnectionPoolSize = 1     // Minimum connections in pool
	MaxChannelPoolSize    = 10000 // Maximum channels in pool
	MinChannelPoolSize    = 1     // Minimum channels in pool

	// Publisher limits
	// These limits ensure system stability and prevent resource exhaustion
	MaxPublisherWorkers = 100    // Maximum concurrent publisher workers
	MinPublisherWorkers = 1      // Minimum publisher workers
	MaxInFlightMessages = 100000 // Maximum unacknowledged messages
	MinInFlightMessages = 1      // Minimum in-flight messages

	// Consumer limits
	// These limits prevent overwhelming the system with too many handlers
	MaxConsumerHandlers = 10000 // Maximum concurrent consumer handlers
	MinConsumerHandlers = 1     // Minimum consumer handlers
	MaxPrefetchCount    = 65535 // Maximum prefetch count (16-bit unsigned)
	MinPrefetchCount    = 1     // Minimum prefetch count

	// Retry limits
	// These limits prevent infinite retry loops and ensure eventual failure
	MaxRetryAttempts     = 100  // Maximum retry attempts
	MinRetryAttempts     = 1    // Minimum retry attempts
	MaxBackoffMultiplier = 10.0 // Maximum exponential backoff multiplier
	MinBackoffMultiplier = 1.0  // Minimum backoff multiplier

	// Compression limits
	// These limits define the valid range for compression levels
	MaxCompressionLevel = 9 // Maximum compression level (best compression)
	MinCompressionLevel = 1 // Minimum compression level (fastest)

	// Queue limits
	// Standardized priority range matching message priorities
	MaxQueuePriority = 255 // Maximum queue priority (highest priority)
	MinQueuePriority = 0   // Minimum queue priority (lowest priority)

	// Benchmark limits
	// These limits prevent benchmark runs from consuming excessive resources
	MaxBenchmarkDuration    = 60 * 60 * time.Second // 1 hour maximum benchmark duration
	MinBenchmarkDuration    = 1 * time.Second       // 1 second minimum benchmark duration
	MaxBenchmarkPublishers  = 100                   // Maximum benchmark publishers
	MinBenchmarkPublishers  = 1                     // Minimum benchmark publishers
	MaxBenchmarkConsumers   = 100                   // Maximum benchmark consumers
	MinBenchmarkConsumers   = 1                     // Minimum benchmark consumers
	MaxBenchmarkMessageSize = 1048576               // 1MB maximum benchmark message size
	MinBenchmarkMessageSize = 1                     // Minimum benchmark message size
	MaxBenchmarkBatchSize   = 10000                 // Maximum benchmark batch size
	MinBenchmarkBatchSize   = 1                     // Minimum benchmark batch size
)

// Content types supported by the messaging system
// These are the MIME types that the system can handle
var supportedContentTypes = []string{
	"application/json",
	"application/xml",
	"text/plain",
	"text/html",
	"application/octet-stream",
	"application/protobuf",
	"application/avro",
}

// Transport types supported by the messaging system
// These are the underlying transport mechanisms
var supportedTransports = []string{
	"rabbitmq",
	"kafka",
}

// Exchange types supported by RabbitMQ
// These define the routing behavior for RabbitMQ exchanges
var supportedExchangeTypes = []string{
	"direct",
	"fanout",
	"topic",
	"headers",
}

// Filter actions supported by the routing system
// These define what actions can be taken on filtered messages
var supportedFilterActions = []string{
	"accept",
	"reject",
	"modify",
}

// Serialization formats supported by the transformation system
// These are the data formats the system can serialize/deserialize
var supportedSerializationFormats = []string{
	"json",
	"protobuf",
	"avro",
}

// Storage types supported by the persistence system
// These define where messages can be persisted
var supportedStorageTypes = []string{
	"memory",
	"disk",
	"redis",
}

// HMAC algorithms supported by the security system
// These are the hashing algorithms used for message authentication
var supportedHMACAlgorithms = []string{
	"sha256",
	"sha512",
	"sha1",
}

// TLS versions supported by the security system
// These are the TLS protocol versions for secure connections
var supportedTLSVersions = []string{
	"1.2",
	"1.3",
}

// Getter functions to provide immutable access to supported values

// SupportedContentTypes returns a copy of supported content types
func SupportedContentTypes() []string {
	result := make([]string, len(supportedContentTypes))
	copy(result, supportedContentTypes)
	return result
}

// SupportedTransports returns a copy of supported transport types
func SupportedTransports() []string {
	result := make([]string, len(supportedTransports))
	copy(result, supportedTransports)
	return result
}

// SupportedExchangeTypes returns a copy of supported exchange types
func SupportedExchangeTypes() []string {
	result := make([]string, len(supportedExchangeTypes))
	copy(result, supportedExchangeTypes)
	return result
}

// SupportedFilterActions returns a copy of supported filter actions
func SupportedFilterActions() []string {
	result := make([]string, len(supportedFilterActions))
	copy(result, supportedFilterActions)
	return result
}

// SupportedSerializationFormats returns a copy of supported serialization formats
func SupportedSerializationFormats() []string {
	result := make([]string, len(supportedSerializationFormats))
	copy(result, supportedSerializationFormats)
	return result
}

// SupportedStorageTypes returns a copy of supported storage types
func SupportedStorageTypes() []string {
	result := make([]string, len(supportedStorageTypes))
	copy(result, supportedStorageTypes)
	return result
}

// SupportedHMACAlgorithms returns a copy of supported HMAC algorithms
func SupportedHMACAlgorithms() []string {
	result := make([]string, len(supportedHMACAlgorithms))
	copy(result, supportedHMACAlgorithms)
	return result
}

// SupportedTLSVersions returns a copy of supported TLS versions
func SupportedTLSVersions() []string {
	result := make([]string, len(supportedTLSVersions))
	copy(result, supportedTLSVersions)
	return result
}

// Validation helper functions

// IsValidMessageSize checks if a message size is within valid bounds
func IsValidMessageSize(size int) bool {
	return size >= MinMessageSize && size <= MaxMessageSize
}

// IsValidHeaderSize checks if a header size is within valid bounds
func IsValidHeaderSize(size int) bool {
	return size >= 0 && size <= MaxHeaderSize
}

// IsValidPriority checks if a priority value is within valid bounds
func IsValidPriority(priority uint8) bool {
	return priority >= MinPriority && priority <= MaxPriority
}

// IsValidTimeout checks if a timeout duration is within valid bounds
func IsValidTimeout(timeout time.Duration) bool {
	return timeout >= MinTimeout && timeout <= MaxTimeout
}

// IsValidConnectionPoolSize checks if a connection pool size is within valid bounds
func IsValidConnectionPoolSize(size int) bool {
	return size >= MinConnectionPoolSize && size <= MaxConnectionPoolSize
}

// IsValidChannelPoolSize checks if a channel pool size is within valid bounds
func IsValidChannelPoolSize(size int) bool {
	return size >= MinChannelPoolSize && size <= MaxChannelPoolSize
}

// IsValidPublisherWorkers checks if the number of publisher workers is within valid bounds
func IsValidPublisherWorkers(workers int) bool {
	return workers >= MinPublisherWorkers && workers <= MaxPublisherWorkers
}

// IsValidInFlightMessages checks if the number of in-flight messages is within valid bounds
func IsValidInFlightMessages(count int) bool {
	return count >= MinInFlightMessages && count <= MaxInFlightMessages
}

// IsValidConsumerHandlers checks if the number of consumer handlers is within valid bounds
func IsValidConsumerHandlers(handlers int) bool {
	return handlers >= MinConsumerHandlers && handlers <= MaxConsumerHandlers
}

// IsValidPrefetchCount checks if a prefetch count is within valid bounds
func IsValidPrefetchCount(count int) bool {
	return count >= MinPrefetchCount && count <= MaxPrefetchCount
}

// IsValidRetryAttempts checks if the number of retry attempts is within valid bounds
func IsValidRetryAttempts(attempts int) bool {
	return attempts >= MinRetryAttempts && attempts <= MaxRetryAttempts
}

// IsValidBackoffMultiplier checks if a backoff multiplier is within valid bounds
func IsValidBackoffMultiplier(multiplier float64) bool {
	return multiplier >= MinBackoffMultiplier && multiplier <= MaxBackoffMultiplier
}

// IsValidCompressionLevel checks if a compression level is within valid bounds
func IsValidCompressionLevel(level int) bool {
	return level >= MinCompressionLevel && level <= MaxCompressionLevel
}

// IsValidQueuePriority checks if a queue priority is within valid bounds
func IsValidQueuePriority(priority int) bool {
	return priority >= MinQueuePriority && priority <= MaxQueuePriority
}

// IsValidBenchmarkDuration checks if a benchmark duration is within valid bounds
func IsValidBenchmarkDuration(duration time.Duration) bool {
	return duration >= MinBenchmarkDuration && duration <= MaxBenchmarkDuration
}

// IsValidBenchmarkPublishers checks if the number of benchmark publishers is within valid bounds
func IsValidBenchmarkPublishers(count int) bool {
	return count >= MinBenchmarkPublishers && count <= MaxBenchmarkPublishers
}

// IsValidBenchmarkConsumers checks if the number of benchmark consumers is within valid bounds
func IsValidBenchmarkConsumers(count int) bool {
	return count >= MinBenchmarkConsumers && count <= MaxBenchmarkConsumers
}

// IsValidBenchmarkMessageSize checks if a benchmark message size is within valid bounds
func IsValidBenchmarkMessageSize(size int) bool {
	return size >= MinBenchmarkMessageSize && size <= MaxBenchmarkMessageSize
}

// IsValidBenchmarkBatchSize checks if a benchmark batch size is within valid bounds
func IsValidBenchmarkBatchSize(size int) bool {
	return size >= MinBenchmarkBatchSize && size <= MaxBenchmarkBatchSize
}

// IsValidStringLength checks if a string length is within the specified maximum
func IsValidStringLength(s string, maxLength int) bool {
	return len(s) <= maxLength
}

// IsValidContentType checks if a content type is supported
func IsValidContentType(contentType string) bool {
	for _, supported := range supportedContentTypes {
		if supported == contentType {
			return true
		}
	}
	return false
}

// IsValidTransport checks if a transport type is supported
func IsValidTransport(transport string) bool {
	for _, supported := range supportedTransports {
		if supported == transport {
			return true
		}
	}
	return false
}

// IsValidExchangeType checks if an exchange type is supported
func IsValidExchangeType(exchangeType string) bool {
	for _, supported := range supportedExchangeTypes {
		if supported == exchangeType {
			return true
		}
	}
	return false
}

// IsValidFilterAction checks if a filter action is supported
func IsValidFilterAction(action string) bool {
	for _, supported := range supportedFilterActions {
		if supported == action {
			return true
		}
	}
	return false
}

// IsValidSerializationFormat checks if a serialization format is supported
func IsValidSerializationFormat(format string) bool {
	for _, supported := range supportedSerializationFormats {
		if supported == format {
			return true
		}
	}
	return false
}

// IsValidStorageType checks if a storage type is supported
func IsValidStorageType(storageType string) bool {
	for _, supported := range supportedStorageTypes {
		if supported == storageType {
			return true
		}
	}
	return false
}

// IsValidHMACAlgorithm checks if an HMAC algorithm is supported
func IsValidHMACAlgorithm(algorithm string) bool {
	for _, supported := range supportedHMACAlgorithms {
		if supported == algorithm {
			return true
		}
	}
	return false
}

// IsValidTLSVersion checks if a TLS version is supported
func IsValidTLSVersion(version string) bool {
	for _, supported := range supportedTLSVersions {
		if supported == version {
			return true
		}
	}
	return false
}
