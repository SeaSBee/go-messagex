// Package configloader provides configuration loading with YAML + ENV support.
package configloader

import (
	"fmt"
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
	// Start with default configuration
	config := l.createDefaultConfig()

	// Load YAML configuration if file is provided
	if configPath != "" {
		if err := l.loadYAML(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load YAML config: %w", err)
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
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	return l.loadYAMLBytes(config, data)
}

// loadYAMLBytes loads configuration from YAML bytes.
func (l *Loader) loadYAMLBytes(config *messaging.Config, data []byte) error {
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}

// overlayEnv overlays environment variables on the configuration.
func (l *Loader) overlayEnv(config *messaging.Config) error {
	configValue := reflect.ValueOf(config).Elem()
	return l.overlayStruct(configValue, "")
}

// overlayStruct recursively overlays environment variables on a struct.
func (l *Loader) overlayStruct(v reflect.Value, prefix string) error {
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
		if envTag == "" {
			if prefix == "" {
				envTag = strings.ToUpper(fieldType.Name)
			} else {
				envTag = prefix + "_" + strings.ToUpper(fieldType.Name)
			}
		} else if prefix != "" {
			envTag = prefix + "_" + envTag
		}

		// Handle different field types
		switch field.Kind() {
		case reflect.String:
			if envValue := os.Getenv(envTag); envValue != "" {
				field.SetString(envValue)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if envValue := os.Getenv(envTag); envValue != "" {
				if val, err := strconv.ParseInt(envValue, 10, 64); err == nil {
					field.SetInt(val)
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if envValue := os.Getenv(envTag); envValue != "" {
				if val, err := strconv.ParseUint(envValue, 10, 64); err == nil {
					field.SetUint(val)
				}
			}
		case reflect.Float32, reflect.Float64:
			if envValue := os.Getenv(envTag); envValue != "" {
				if val, err := strconv.ParseFloat(envValue, 64); err == nil {
					field.SetFloat(val)
				}
			}
		case reflect.Bool:
			if envValue := os.Getenv(envTag); envValue != "" {
				if val, err := strconv.ParseBool(envValue); err == nil {
					field.SetBool(val)
				}
			}
		case reflect.Slice:
			if envValue := os.Getenv(envTag); envValue != "" {
				if err := l.setSliceFromEnv(field, envValue); err != nil {
					return fmt.Errorf("failed to set slice from env %s: %w", envTag, err)
				}
			}
		case reflect.Struct:
			// Handle time.Duration specially
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if envValue := os.Getenv(envTag); envValue != "" {
					if val, err := time.ParseDuration(envValue); err == nil {
						field.Set(reflect.ValueOf(val))
					}
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
				field.Set(reflect.New(field.Type().Elem()))
			}
			if field.Elem().Kind() == reflect.Struct {
				if err := l.overlayStruct(field.Elem(), envTag); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// setSliceFromEnv sets a slice field from environment variable.
func (l *Loader) setSliceFromEnv(field reflect.Value, envValue string) error {
	// Split by comma for list values
	values := strings.Split(envValue, ",")

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
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				slice.Index(i).SetInt(val)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if val, err := strconv.ParseUint(value, 10, 64); err == nil {
				slice.Index(i).SetUint(val)
			}
		case reflect.Float32, reflect.Float64:
			if val, err := strconv.ParseFloat(value, 64); err == nil {
				slice.Index(i).SetFloat(val)
			}
		case reflect.Bool:
			if val, err := strconv.ParseBool(value); err == nil {
				slice.Index(i).SetBool(val)
			}
		default:
			return fmt.Errorf("unsupported slice element type: %v", elemType.Kind())
		}
	}

	field.Set(slice)
	return nil
}

// validate validates the configuration.
func (l *Loader) validate(config *messaging.Config) error {
	// Validate transport
	if config.Transport == "" {
		return fmt.Errorf("transport is required")
	}
	if config.Transport != "rabbitmq" && config.Transport != "kafka" {
		return fmt.Errorf("unsupported transport: %s", config.Transport)
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

	return nil
}

// validateRabbitMQConfig validates RabbitMQ configuration.
func (l *Loader) validateRabbitMQConfig(config *messaging.RabbitMQConfig) error {
	// Validate URIs
	if len(config.URIs) == 0 {
		return fmt.Errorf("at least one RabbitMQ URI is required")
	}

	// Validate connection pool
	if config.ConnectionPool != nil {
		if config.ConnectionPool.Min < 1 || config.ConnectionPool.Min > 100 {
			return fmt.Errorf("connection pool min must be between 1 and 100")
		}
		if config.ConnectionPool.Max < config.ConnectionPool.Min {
			return fmt.Errorf("connection pool max must be greater than or equal to min")
		}
		if config.ConnectionPool.Max > 1000 {
			return fmt.Errorf("connection pool max must be less than or equal to 1000")
		}
	}

	// Validate channel pool
	if config.ChannelPool != nil {
		if config.ChannelPool.PerConnectionMin < 1 || config.ChannelPool.PerConnectionMin > 1000 {
			return fmt.Errorf("channel pool per connection min must be between 1 and 1000")
		}
		if config.ChannelPool.PerConnectionMax < config.ChannelPool.PerConnectionMin {
			return fmt.Errorf("channel pool per connection max must be greater than or equal to min")
		}
		if config.ChannelPool.PerConnectionMax > 10000 {
			return fmt.Errorf("channel pool per connection max must be less than or equal to 10000")
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

	return nil
}

// validatePublisherConfig validates publisher configuration.
func (l *Loader) validatePublisherConfig(config *messaging.PublisherConfig) error {
	if config.MaxInFlight < 1 || config.MaxInFlight > 100000 {
		return fmt.Errorf("max in flight must be between 1 and 100000")
	}

	if config.WorkerCount < 1 || config.WorkerCount > 100 {
		return fmt.Errorf("worker count must be between 1 and 100")
	}

	if config.Retry != nil {
		if config.Retry.MaxAttempts < 1 || config.Retry.MaxAttempts > 100 {
			return fmt.Errorf("retry max attempts must be between 1 and 100")
		}
		if config.Retry.BackoffMultiplier < 1.0 || config.Retry.BackoffMultiplier > 10.0 {
			return fmt.Errorf("retry backoff multiplier must be between 1.0 and 10.0")
		}
	}

	if config.Serialization != nil {
		if config.Serialization.CompressionLevel < 1 || config.Serialization.CompressionLevel > 9 {
			return fmt.Errorf("compression level must be between 1 and 9")
		}
	}

	return nil
}

// validateConsumerConfig validates consumer configuration.
func (l *Loader) validateConsumerConfig(config *messaging.ConsumerConfig) error {
	if config.Queue == "" {
		return fmt.Errorf("consumer queue is required")
	}

	if config.Prefetch < 1 || config.Prefetch > 65535 {
		return fmt.Errorf("prefetch must be between 1 and 65535")
	}

	if config.MaxConcurrentHandlers < 1 || config.MaxConcurrentHandlers > 10000 {
		return fmt.Errorf("max concurrent handlers must be between 1 and 10000")
	}

	return nil
}

// MaskSecrets masks sensitive information in configuration for logging.
func (l *Loader) MaskSecrets(config *messaging.Config) *messaging.Config {
	if !l.maskSecrets {
		return config
	}

	// Create a copy to avoid modifying the original
	masked := *config

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

	return &masked
}

// maskURI masks sensitive information in URIs.
func (l *Loader) maskURI(uri string) string {
	// Simple masking - replace password with ***
	// This is a basic implementation; in production, you might want more sophisticated masking
	if strings.Contains(uri, "@") {
		parts := strings.Split(uri, "@")
		if len(parts) == 2 {
			authPart := parts[0]
			if strings.Contains(authPart, ":") {
				authParts := strings.Split(authPart, ":")
				if len(authParts) >= 3 {
					// amqp://user:pass@host format
					return authParts[0] + ":***@" + parts[1]
				}
			}
		}
	}
	return uri
}
