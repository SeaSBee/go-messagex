// Package configloader provides configuration loading with YAML + ENV support.
package configloader

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/seasbee/go-messagex/pkg/messaging"
)

// Loader provides configuration loading functionality.
type Loader struct {
	// envPrefix is the prefix for environment variables
	envPrefix string

	// maskSecrets enables masking of sensitive information in logs
	maskSecrets bool
}

// NewLoader creates a new configuration loader.
func NewLoader(envPrefix string, maskSecrets bool) *Loader {
	return &Loader{
		envPrefix:   envPrefix,
		maskSecrets: maskSecrets,
	}
}

// Load loads configuration from YAML file and environment variables.
// Environment variables take precedence over YAML values.
func (l *Loader) Load(configPath string) (*messaging.Config, error) {
	// Validate loader
	if l == nil {
		return nil, fmt.Errorf("loader cannot be nil")
	}

	// Start with default configuration
	config := l.createDefaultConfig()

	// Load YAML configuration if file is provided
	if configPath != "" {
		if err := l.loadYAML(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load YAML config from %s: %w", configPath, err)
		}
	}

	// Overlay environment variables
	if err := l.overlayEnv(config); err != nil {
		return nil, fmt.Errorf("failed to overlay environment variables: %w", err)
	}

	// Validate configuration
	if err := l.validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadFromBytes loads configuration from YAML bytes and environment variables.
func (l *Loader) LoadFromBytes(yamlData []byte) (*messaging.Config, error) {
	// Validate loader
	if l == nil {
		return nil, fmt.Errorf("loader cannot be nil")
	}

	// Validate input data
	if yamlData == nil {
		return nil, fmt.Errorf("yaml data cannot be nil")
	}

	// Start with default configuration
	config := l.createDefaultConfig()

	// Load YAML configuration from bytes
	if err := l.loadYAMLBytes(config, yamlData); err != nil {
		return nil, fmt.Errorf("failed to load YAML config from bytes: %w", err)
	}

	// Overlay environment variables
	if err := l.overlayEnv(config); err != nil {
		return nil, fmt.Errorf("failed to overlay environment variables: %w", err)
	}

	// Validate configuration
	if err := l.validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// createDefaultConfig creates a configuration with default values.
func (l *Loader) createDefaultConfig() *messaging.Config {
	return &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{"amqp://localhost:5672/"},
			ConnectionPool: &messaging.ConnectionPoolConfig{
				Min:                 2,
				Max:                 8,
				HealthCheckInterval: 30 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				HeartbeatInterval:   10 * time.Second,
			},
			ChannelPool: &messaging.ChannelPoolConfig{
				PerConnectionMin:    10,
				PerConnectionMax:    100,
				BorrowTimeout:       5 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
			Publisher: &messaging.PublisherConfig{
				Confirms:       true,
				Mandatory:      true,
				Immediate:      false,
				MaxInFlight:    10000,
				DropOnOverflow: false,
				PublishTimeout: 2 * time.Second,
				WorkerCount:    4,
				Retry: &messaging.RetryConfig{
					MaxAttempts:       5,
					BaseBackoff:       100 * time.Millisecond,
					MaxBackoff:        5 * time.Second,
					BackoffMultiplier: 2.0,
					Jitter:            true,
				},
				Serialization: &messaging.SerializationConfig{
					DefaultContentType: "application/json",
					CompressionEnabled: false,
					CompressionLevel:   6,
				},
			},
			Consumer: &messaging.ConsumerConfig{
				Queue:                 "default.queue",
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
			},
			TLS: &messaging.TLSConfig{
				Enabled:            false,
				MinVersion:         "1.2",
				InsecureSkipVerify: false,
			},
			Security: &messaging.SecurityConfig{
				HMACEnabled:    false,
				HMACAlgorithm:  "sha256",
				VerifyHostname: true,
			},
		},
		Telemetry: &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "go-messagex",
		},
	}
}

// loadYAML loads configuration from YAML file.
func (l *Loader) loadYAML(config *messaging.Config, configPath string) error {
	// Validate inputs
	if l == nil {
		return fmt.Errorf("loader cannot be nil")
	}
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if configPath == "" {
		return fmt.Errorf("config path cannot be empty")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	return l.loadYAMLBytes(config, data)
}

// loadYAMLBytes loads configuration from YAML bytes.
func (l *Loader) loadYAMLBytes(config *messaging.Config, data []byte) error {
	// Validate inputs
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}

// overlayEnv overlays environment variables on the configuration.
func (l *Loader) overlayEnv(config *messaging.Config) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	configValue := reflect.ValueOf(config).Elem()
	return l.overlayStruct(configValue, "")
}

// overlayStruct recursively overlays environment variables on a struct.
func (l *Loader) overlayStruct(v reflect.Value, prefix string) error {
	// Validate input
	if !v.IsValid() {
		return fmt.Errorf("invalid reflect value")
	}
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %v", v.Kind())
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get environment variable name from tag
		envTag := fieldType.Tag.Get("env")

		// If env tag is provided, use it directly (it's already complete)
		// If no env tag, build the name from field name and prefix
		if envTag == "" {
			envTag = strings.ToUpper(fieldType.Name)
			// Apply environment prefix first, then struct prefix
			if l.envPrefix != "" {
				envTag = l.envPrefix + "_" + envTag
			}
			if prefix != "" {
				envTag = prefix + "_" + envTag
			}
		}

		// Validate environment variable name
		if envTag == "" {
			continue // Skip fields with empty environment variable names
		}

		// Handle different field types
		switch field.Kind() {
		case reflect.String:
			if envValue := os.Getenv(envTag); envValue != "" {
				field.SetString(envValue)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// Check if this is actually a time.Duration field
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if envValue := os.Getenv(envTag); envValue != "" {
					val, err := time.ParseDuration(envValue)
					if err != nil {
						return fmt.Errorf("failed to parse duration environment variable %s=%s: %w", envTag, envValue, err)
					}
					// Validate duration bounds (allow 0 for some cases, but generally expect positive values)
					if val < 0 {
						return fmt.Errorf("duration value for %s cannot be negative, got: %v", envTag, val)
					}
					// Allow up to 7 days for configuration flexibility
					if val > 7*24*time.Hour {
						return fmt.Errorf("duration value for %s cannot exceed 7 days, got: %v", envTag, val)
					}
					field.Set(reflect.ValueOf(val))
				}
			} else {
				if envValue := os.Getenv(envTag); envValue != "" {
					val, err := strconv.ParseInt(envValue, 10, 64)
					if err != nil {
						return fmt.Errorf("failed to parse integer environment variable %s=%s: %w", envTag, envValue, err)
					}
					field.SetInt(val)
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if envValue := os.Getenv(envTag); envValue != "" {
				val, err := strconv.ParseUint(envValue, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse unsigned integer environment variable %s=%s: %w", envTag, envValue, err)
				}
				field.SetUint(val)
			}
		case reflect.Float32, reflect.Float64:
			if envValue := os.Getenv(envTag); envValue != "" {
				val, err := strconv.ParseFloat(envValue, 64)
				if err != nil {
					return fmt.Errorf("failed to parse float environment variable %s=%s: %w", envTag, envValue, err)
				}
				field.SetFloat(val)
			}
		case reflect.Bool:
			if envValue := os.Getenv(envTag); envValue != "" {
				val, err := strconv.ParseBool(envValue)
				if err != nil {
					return fmt.Errorf("failed to parse boolean environment variable %s=%s: %w", envTag, envValue, err)
				}
				field.SetBool(val)
			}
		case reflect.Slice:
			if envValue := os.Getenv(envTag); envValue != "" {
				if err := l.setSliceFromEnv(field, envValue, envTag); err != nil {
					return err
				}
			}
		case reflect.Struct:
			// Handle time.Duration specially
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if envValue := os.Getenv(envTag); envValue != "" {
					val, err := time.ParseDuration(envValue)
					if err != nil {
						return fmt.Errorf("failed to parse duration environment variable %s=%s: %w", envTag, envValue, err)
					}
					// Validate duration bounds (allow 0 for some cases, but generally expect positive values)
					if val < 0 {
						return fmt.Errorf("duration value for %s cannot be negative, got: %v", envTag, val)
					}
					// Allow up to 7 days for configuration flexibility
					if val > 7*24*time.Hour {
						return fmt.Errorf("duration value for %s cannot exceed 7 days, got: %v", envTag, val)
					}
					field.Set(reflect.ValueOf(val))
				}
			} else {
				// Recursively overlay nested structs
				if err := l.overlayStruct(field, envTag); err != nil {
					return err
				}
			}
		case reflect.Ptr:
			if field.IsNil() {
				// Create new instance for pointer fields
				newVal := reflect.New(field.Type().Elem())
				if !newVal.IsValid() {
					return fmt.Errorf("failed to create new instance for pointer field %s", envTag)
				}
				field.Set(newVal)
			}
			// Add type checking to prevent panic
			if field.Elem().IsValid() && field.Elem().Kind() == reflect.Struct {
				if err := l.overlayStruct(field.Elem(), envTag); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// setSliceFromEnv sets a slice field from environment variable.
func (l *Loader) setSliceFromEnv(field reflect.Value, envValue string, envTag string) error {
	// Validate inputs
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("invalid or unsettable field for environment variable %s", envTag)
	}
	if envValue == "" {
		return fmt.Errorf("empty environment variable value for %s", envTag)
	}

	// Split by comma for list values
	values := strings.Split(envValue, ",")

	// Add bounds checking to prevent memory issues
	const maxSliceSize = 10000 // Reasonable limit for configuration values
	if len(values) > maxSliceSize {
		return fmt.Errorf("environment variable %s contains too many values (%d), maximum allowed is %d", envTag, len(values), maxSliceSize)
	}

	// Create new slice
	sliceType := field.Type()
	slice := reflect.MakeSlice(sliceType, len(values), len(values))

	// Parse each value
	for i, value := range values {
		value = strings.TrimSpace(value)
		elemType := sliceType.Elem()

		switch elemType.Kind() {
		case reflect.String:
			slice.Index(i).SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			val, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse int value '%s' at index %d in environment variable %s: %w", value, i, envTag, err)
			}
			slice.Index(i).SetInt(val)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			val, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse uint value '%s' at index %d in environment variable %s: %w", value, i, envTag, err)
			}
			slice.Index(i).SetUint(val)
		case reflect.Float32, reflect.Float64:
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("failed to parse float value '%s' at index %d in environment variable %s: %w", value, i, envTag, err)
			}
			slice.Index(i).SetFloat(val)
		case reflect.Bool:
			val, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("failed to parse bool value '%s' at index %d in environment variable %s: %w", value, i, envTag, err)
			}
			slice.Index(i).SetBool(val)
		default:
			return fmt.Errorf("unsupported slice element type for environment variable %s: %v", envTag, elemType.Kind())
		}
	}

	field.Set(slice)
	return nil
}

// validate validates the configuration.
func (l *Loader) validate(config *messaging.Config) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate transport
	if config.Transport == "" {
		return fmt.Errorf("transport is required")
	}
	if config.Transport != "rabbitmq" && config.Transport != "kafka" {
		return fmt.Errorf("unsupported transport: %s (supported: rabbitmq, kafka)", config.Transport)
	}

	// Validate RabbitMQ configuration if transport is rabbitmq
	if config.Transport == "rabbitmq" {
		if config.RabbitMQ == nil {
			return fmt.Errorf("rabbitmq configuration is required when transport is rabbitmq")
		}

		if err := l.validateRabbitMQConfig(config.RabbitMQ); err != nil {
			return fmt.Errorf("rabbitmq configuration validation failed: %w", err)
		}
	}

	// Validate telemetry configuration
	if config.Telemetry != nil {
		if err := l.validateTelemetryConfig(config.Telemetry); err != nil {
			return fmt.Errorf("telemetry configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateRabbitMQConfig validates RabbitMQ configuration.
func (l *Loader) validateRabbitMQConfig(config *messaging.RabbitMQConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("rabbitmq config cannot be nil")
	}

	// Validate URIs
	if len(config.URIs) == 0 {
		return fmt.Errorf("at least one RabbitMQ URI is required")
	}

	// Validate each URI format
	for i, uri := range config.URIs {
		if uri == "" {
			return fmt.Errorf("RabbitMQ URI at index %d cannot be empty", i)
		}
		if _, err := url.Parse(uri); err != nil {
			return fmt.Errorf("invalid RabbitMQ URI at index %d (%s): %w", i, uri, err)
		}
	}

	// Validate connection pool
	if config.ConnectionPool != nil {
		if err := l.validateConnectionPoolConfig(config.ConnectionPool); err != nil {
			return fmt.Errorf("connection pool validation failed: %w", err)
		}
	}

	// Validate channel pool
	if config.ChannelPool != nil {
		if err := l.validateChannelPoolConfig(config.ChannelPool); err != nil {
			return fmt.Errorf("channel pool validation failed: %w", err)
		}
	}

	// Validate publisher configuration
	if config.Publisher != nil {
		if err := l.validatePublisherConfig(config.Publisher); err != nil {
			return fmt.Errorf("publisher configuration validation failed: %w", err)
		}
	}

	// Validate consumer configuration
	if config.Consumer != nil {
		if err := l.validateConsumerConfig(config.Consumer); err != nil {
			return fmt.Errorf("consumer configuration validation failed: %w", err)
		}
	}

	// Validate TLS configuration
	if config.TLS != nil {
		if err := l.validateTLSConfig(config.TLS); err != nil {
			return fmt.Errorf("TLS configuration validation failed: %w", err)
		}
	}

	// Validate security configuration
	if config.Security != nil {
		if err := l.validateSecurityConfig(config.Security); err != nil {
			return fmt.Errorf("security configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateConnectionPoolConfig validates connection pool configuration.
func (l *Loader) validateConnectionPoolConfig(config *messaging.ConnectionPoolConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("connection pool config cannot be nil")
	}

	if config.Min < 1 || config.Min > 100 {
		return fmt.Errorf("connection pool min must be between 1 and 100, got: %d", config.Min)
	}
	if config.Max < config.Min {
		return fmt.Errorf("connection pool max (%d) must be greater than or equal to min (%d)", config.Max, config.Min)
	}
	if config.Max > 1000 {
		return fmt.Errorf("connection pool max must be less than or equal to 1000, got: %d", config.Max)
	}
	if config.HealthCheckInterval < time.Second || config.HealthCheckInterval > 5*time.Minute {
		return fmt.Errorf("health check interval must be between 1s and 5m, got: %v", config.HealthCheckInterval)
	}
	if config.ConnectionTimeout < time.Second || config.ConnectionTimeout > 30*time.Second {
		return fmt.Errorf("connection timeout must be between 1s and 30s, got: %v", config.ConnectionTimeout)
	}
	if config.HeartbeatInterval < time.Second || config.HeartbeatInterval > 60*time.Second {
		return fmt.Errorf("heartbeat interval must be between 1s and 60s, got: %v", config.HeartbeatInterval)
	}
	return nil
}

// validateChannelPoolConfig validates channel pool configuration.
func (l *Loader) validateChannelPoolConfig(config *messaging.ChannelPoolConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("channel pool config cannot be nil")
	}

	if config.PerConnectionMin < 1 || config.PerConnectionMin > 1000 {
		return fmt.Errorf("channel pool per connection min must be between 1 and 1000, got: %d", config.PerConnectionMin)
	}
	if config.PerConnectionMax < config.PerConnectionMin {
		return fmt.Errorf("channel pool per connection max (%d) must be greater than or equal to min (%d)", config.PerConnectionMax, config.PerConnectionMin)
	}
	if config.PerConnectionMax > 10000 {
		return fmt.Errorf("channel pool per connection max must be less than or equal to 10000, got: %d", config.PerConnectionMax)
	}
	if config.BorrowTimeout < time.Millisecond || config.BorrowTimeout > 30*time.Second {
		return fmt.Errorf("borrow timeout must be between 1ms and 30s, got: %v", config.BorrowTimeout)
	}
	if config.HealthCheckInterval < time.Second || config.HealthCheckInterval > 5*time.Minute {
		return fmt.Errorf("health check interval must be between 1s and 5m, got: %v", config.HealthCheckInterval)
	}
	return nil
}

// validatePublisherConfig validates publisher configuration.
func (l *Loader) validatePublisherConfig(config *messaging.PublisherConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("publisher config cannot be nil")
	}

	if config.MaxInFlight < 1 || config.MaxInFlight > 100000 {
		return fmt.Errorf("max in flight must be between 1 and 100000, got: %d", config.MaxInFlight)
	}

	if config.WorkerCount < 1 || config.WorkerCount > 100 {
		return fmt.Errorf("worker count must be between 1 and 100, got: %d", config.WorkerCount)
	}

	if config.PublishTimeout < time.Millisecond || config.PublishTimeout > 30*time.Second {
		return fmt.Errorf("publish timeout must be between 1ms and 30s, got: %v", config.PublishTimeout)
	}

	if config.Retry != nil {
		if err := l.validateRetryConfig(config.Retry); err != nil {
			return fmt.Errorf("retry configuration validation failed: %w", err)
		}
	}

	if config.Serialization != nil {
		if err := l.validateSerializationConfig(config.Serialization); err != nil {
			return fmt.Errorf("serialization configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateRetryConfig validates retry configuration.
func (l *Loader) validateRetryConfig(config *messaging.RetryConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("retry config cannot be nil")
	}

	if config.MaxAttempts < 1 || config.MaxAttempts > 100 {
		return fmt.Errorf("retry max attempts must be between 1 and 100, got: %d", config.MaxAttempts)
	}
	if config.BaseBackoff < time.Millisecond || config.BaseBackoff > 10*time.Second {
		return fmt.Errorf("base backoff must be between 1ms and 10s, got: %v", config.BaseBackoff)
	}
	if config.MaxBackoff < config.BaseBackoff {
		return fmt.Errorf("max backoff (%v) must be greater than or equal to base backoff (%v)", config.MaxBackoff, config.BaseBackoff)
	}
	if config.MaxBackoff > 5*time.Minute {
		return fmt.Errorf("max backoff must be less than or equal to 5m, got: %v", config.MaxBackoff)
	}
	if config.BackoffMultiplier < 1.0 || config.BackoffMultiplier > 10.0 {
		return fmt.Errorf("retry backoff multiplier must be between 1.0 and 10.0, got: %f", config.BackoffMultiplier)
	}
	return nil
}

// validateSerializationConfig validates serialization configuration.
func (l *Loader) validateSerializationConfig(config *messaging.SerializationConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("serialization config cannot be nil")
	}

	if config.DefaultContentType == "" {
		return fmt.Errorf("default content type cannot be empty")
	}
	if config.CompressionLevel < 1 || config.CompressionLevel > 9 {
		return fmt.Errorf("compression level must be between 1 and 9, got: %d", config.CompressionLevel)
	}
	return nil
}

// validateConsumerConfig validates consumer configuration.
func (l *Loader) validateConsumerConfig(config *messaging.ConsumerConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("consumer config cannot be nil")
	}

	if config.Queue == "" {
		return fmt.Errorf("consumer queue is required")
	}

	if config.Prefetch < 1 || config.Prefetch > 65535 {
		return fmt.Errorf("prefetch must be between 1 and 65535, got: %d", config.Prefetch)
	}

	if config.MaxConcurrentHandlers < 1 || config.MaxConcurrentHandlers > 10000 {
		return fmt.Errorf("max concurrent handlers must be between 1 and 10000, got: %d", config.MaxConcurrentHandlers)
	}

	if config.HandlerTimeout < time.Second || config.HandlerTimeout > 10*time.Minute {
		return fmt.Errorf("handler timeout must be between 1s and 10m, got: %v", config.HandlerTimeout)
	}

	return nil
}

// validateTLSConfig validates TLS configuration.
func (l *Loader) validateTLSConfig(config *messaging.TLSConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("TLS config cannot be nil")
	}

	if config.MinVersion != "" {
		validVersions := []string{"1.0", "1.1", "1.2", "1.3"}
		valid := false
		for _, version := range validVersions {
			if config.MinVersion == version {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("TLS min version must be one of: %v, got: %s", validVersions, config.MinVersion)
		}
	}
	return nil
}

// validateSecurityConfig validates security configuration.
func (l *Loader) validateSecurityConfig(config *messaging.SecurityConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("security config cannot be nil")
	}

	if config.HMACEnabled && config.HMACSecret == "" {
		return fmt.Errorf("HMAC secret is required when HMAC is enabled")
	}
	if config.HMACAlgorithm != "" {
		validAlgorithms := []string{"sha1", "sha256", "sha512"}
		valid := false
		for _, algo := range validAlgorithms {
			if config.HMACAlgorithm == algo {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("HMAC algorithm must be one of: %v, got: %s", validAlgorithms, config.HMACAlgorithm)
		}
	}
	return nil
}

// validateTelemetryConfig validates telemetry configuration.
func (l *Loader) validateTelemetryConfig(config *messaging.TelemetryConfig) error {
	// Validate input
	if config == nil {
		return fmt.Errorf("telemetry config cannot be nil")
	}

	if config.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	return nil
}

// MaskSecrets masks sensitive information in configuration for logging.
func (l *Loader) MaskSecrets(config *messaging.Config) *messaging.Config {
	// Validate inputs
	if l == nil {
		return config // Return original if loader is nil
	}
	if config == nil {
		return nil
	}

	if !l.maskSecrets {
		return config
	}

	// Create a deep copy to avoid modifying the original
	masked := l.deepCopyConfig(config)

	// Mask RabbitMQ URIs
	if masked.RabbitMQ != nil {
		maskedURIs := make([]string, len(masked.RabbitMQ.URIs))
		for i, uri := range masked.RabbitMQ.URIs {
			maskedURIs[i] = l.maskURI(uri)
		}
		masked.RabbitMQ.URIs = maskedURIs

		// Mask security secrets
		if masked.RabbitMQ.Security != nil {
			if masked.RabbitMQ.Security.HMACSecret != "" {
				masked.RabbitMQ.Security.HMACSecret = "***"
			}
		}
	}

	return masked
}

// deepCopyConfig creates a deep copy of the configuration to avoid modifying the original
func (l *Loader) deepCopyConfig(config *messaging.Config) *messaging.Config {
	if config == nil {
		return nil
	}

	copied := *config

	// Deep copy RabbitMQ config
	if config.RabbitMQ != nil {
		rabbitCopy := *config.RabbitMQ

		// Deep copy slices
		if config.RabbitMQ.URIs != nil {
			rabbitCopy.URIs = make([]string, len(config.RabbitMQ.URIs))
			copy(rabbitCopy.URIs, config.RabbitMQ.URIs)
		}

		// Deep copy nested structs with nil checks
		if config.RabbitMQ.ConnectionPool != nil {
			poolCopy := *config.RabbitMQ.ConnectionPool
			rabbitCopy.ConnectionPool = &poolCopy
		}

		if config.RabbitMQ.ChannelPool != nil {
			poolCopy := *config.RabbitMQ.ChannelPool
			rabbitCopy.ChannelPool = &poolCopy
		}

		if config.RabbitMQ.Publisher != nil {
			pubCopy := *config.RabbitMQ.Publisher
			if config.RabbitMQ.Publisher.Retry != nil {
				retryCopy := *config.RabbitMQ.Publisher.Retry
				pubCopy.Retry = &retryCopy
			}
			if config.RabbitMQ.Publisher.Serialization != nil {
				serialCopy := *config.RabbitMQ.Publisher.Serialization
				pubCopy.Serialization = &serialCopy
			}
			rabbitCopy.Publisher = &pubCopy
		}

		if config.RabbitMQ.Consumer != nil {
			consumerCopy := *config.RabbitMQ.Consumer
			rabbitCopy.Consumer = &consumerCopy
		}

		if config.RabbitMQ.TLS != nil {
			tlsCopy := *config.RabbitMQ.TLS
			rabbitCopy.TLS = &tlsCopy
		}

		if config.RabbitMQ.Security != nil {
			securityCopy := *config.RabbitMQ.Security
			rabbitCopy.Security = &securityCopy
		}

		copied.RabbitMQ = &rabbitCopy
	}

	// Deep copy Telemetry config
	if config.Telemetry != nil {
		telemetryCopy := *config.Telemetry
		copied.Telemetry = &telemetryCopy
	}

	return &copied
}

// maskURI masks sensitive information in URIs.
func (l *Loader) maskURI(uri string) string {
	// Validate input
	if uri == "" {
		return uri
	}

	parsedURL, err := url.Parse(uri)
	if err != nil {
		// If parsing fails, fall back to simple masking
		return l.maskURISimple(uri)
	}

	// If there's a password in the URL, mask it
	if parsedURL.User != nil {
		username := parsedURL.User.Username()
		password, hasPassword := parsedURL.User.Password()
		if hasPassword && password != "" {
			// Reconstruct URL with masked password
			parsedURL.User = url.UserPassword(username, "***")
			return parsedURL.String()
		}
	}

	return uri
}

// maskURISimple provides a fallback simple masking for malformed URIs.
func (l *Loader) maskURISimple(uri string) string {
	// Validate input
	if uri == "" {
		return uri
	}

	// Simple masking - replace password with ***
	// This handles cases where url.Parse fails
	if strings.Contains(uri, "@") {
		parts := strings.Split(uri, "@")
		if len(parts) == 2 {
			authPart := parts[0]
			if strings.Contains(authPart, ":") {
				authParts := strings.Split(authPart, ":")
				if len(authParts) >= 3 {
					// amqp://user:pass@host format
					return authParts[0] + ":" + authParts[1] + ":***@" + parts[1]
				} else if len(authParts) == 2 {
					// user:pass format (no protocol)
					return authParts[0] + ":***@" + parts[1]
				}
			}
		}
	}
	return uri
}
