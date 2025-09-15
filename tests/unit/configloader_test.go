package unit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/internal/configloader"
	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

func TestNewLoader(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
}

func TestLoadFromBytes(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	yamlData := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  consumer:
    queue: "test.queue"
    prefetch: 100
`)

	config, err := loader.LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if config.Transport != "rabbitmq" {
		t.Errorf("Expected transport 'rabbitmq', got '%s'", config.Transport)
	}

	if config.RabbitMQ == nil {
		t.Fatal("Expected RabbitMQ config to be set")
	}

	if len(config.RabbitMQ.URIs) != 1 || config.RabbitMQ.URIs[0] != "amqp://localhost:5672/" {
		t.Errorf("Expected URI 'amqp://localhost:5672/', got %v", config.RabbitMQ.URIs)
	}

	if config.RabbitMQ.Consumer == nil {
		t.Fatal("Expected Consumer config to be set")
	}

	if config.RabbitMQ.Consumer.Queue != "test.queue" {
		t.Errorf("Expected queue 'test.queue', got '%s'", config.RabbitMQ.Consumer.Queue)
	}

	if config.RabbitMQ.Consumer.Prefetch != 100 {
		t.Errorf("Expected prefetch 100, got %d", config.RabbitMQ.Consumer.Prefetch)
	}
}

func TestEnvironmentVariableOverlay(t *testing.T) {
	loader := configloader.NewLoader("", true)

	// Set environment variables using the exact names from struct tags
	os.Setenv("MSG_TRANSPORT", "rabbitmq")
	os.Setenv("MSG_RABBITMQ_CONSUMER_QUEUE", "env.queue")
	os.Setenv("MSG_RABBITMQ_CONSUMER_PREFETCH", "200")
	os.Setenv("MSG_RABBITMQ_CONNECTIONPOOL_MIN", "5")
	os.Setenv("MSG_RABBITMQ_CONNECTIONPOOL_MAX", "10")
	defer func() {
		os.Unsetenv("MSG_TRANSPORT")
		os.Unsetenv("MSG_RABBITMQ_CONSUMER_QUEUE")
		os.Unsetenv("MSG_RABBITMQ_CONSUMER_PREFETCH")
		os.Unsetenv("MSG_RABBITMQ_CONNECTIONPOOL_MIN")
		os.Unsetenv("MSG_RABBITMQ_CONNECTIONPOOL_MAX")
	}()

	yamlData := []byte(`
transport: kafka
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  consumer:
    queue: "yaml.queue"
    prefetch: 100
`)

	config, err := loader.LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Environment variables should override YAML values
	if config.Transport != "rabbitmq" {
		t.Errorf("Expected transport 'rabbitmq' (from env), got '%s'", config.Transport)
	}

	if config.RabbitMQ.Consumer.Queue != "env.queue" {
		t.Errorf("Expected queue 'env.queue' (from env), got '%s'", config.RabbitMQ.Consumer.Queue)
	}

	if config.RabbitMQ.Consumer.Prefetch != 200 {
		t.Errorf("Expected prefetch 200 (from env), got %d", config.RabbitMQ.Consumer.Prefetch)
	}

	if config.RabbitMQ.ConnectionPool.Min != 5 {
		t.Errorf("Expected connection pool min 5 (from env), got %d", config.RabbitMQ.ConnectionPool.Min)
	}

	if config.RabbitMQ.ConnectionPool.Max != 10 {
		t.Errorf("Expected connection pool max 10 (from env), got %d", config.RabbitMQ.ConnectionPool.Max)
	}
}

func TestEnvironmentVariableParsingErrors(t *testing.T) {
	loader := configloader.NewLoader("", true)

	testCases := []struct {
		name        string
		envVar      string
		envValue    string
		expectError bool
	}{
		{
			name:        "invalid integer",
			envVar:      "MSG_RABBITMQ_CONSUMER_PREFETCH",
			envValue:    "abc",
			expectError: true,
		},
		{
			name:        "invalid unsigned integer",
			envVar:      "MSG_RABBITMQ_CONNECTIONPOOL_MIN",
			envValue:    "-5",
			expectError: true,
		},
		{
			name:        "invalid float",
			envVar:      "MSG_RABBITMQ_PUBLISHER_RETRY_BACKOFFMULTIPLIER",
			envValue:    "not_a_float",
			expectError: true,
		},
		{
			name:        "invalid boolean",
			envVar:      "MSG_RABBITMQ_PUBLISHER_CONFIRMS",
			envValue:    "maybe",
			expectError: true,
		},
		{
			name:        "invalid duration",
			envVar:      "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			envValue:    "invalid_duration",
			expectError: true,
		},
		{
			name:        "duration out of bounds",
			envVar:      "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			envValue:    "25h",
			expectError: true,
		},
		{
			name:        "valid integer",
			envVar:      "MSG_RABBITMQ_CONSUMER_PREFETCH",
			envValue:    "100",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.envVar, tc.envValue)
			defer os.Unsetenv(tc.envVar)

			yamlData := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
`)

			_, err := loader.LoadFromBytes(yamlData)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s=%s, but got none", tc.envVar, tc.envValue)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s=%s: %v", tc.envVar, tc.envValue, err)
			}
		})
	}
}

func TestSliceEnvironmentVariableParsing(t *testing.T) {
	loader := configloader.NewLoader("", true)

	testCases := []struct {
		name        string
		envVar      string
		envValue    string
		expectError bool
		expected    []string
	}{
		{
			name:        "valid string slice",
			envVar:      "MSG_RABBITMQ_URIS",
			envValue:    "amqp://host1:5672/,amqp://host2:5672/",
			expectError: false,
			expected:    []string{"amqp://host1:5672/", "amqp://host2:5672/"},
		},
		{
			name:        "invalid int in slice",
			envVar:      "MSG_RABBITMQ_CONSUMER_PREFETCH",
			envValue:    "100,abc,200",
			expectError: true,
		},
		{
			name:        "empty slice",
			envVar:      "MSG_RABBITMQ_URIS",
			envValue:    "",
			expectError: false,
			expected:    []string{"amqp://localhost:5672/"}, // Default config provides this
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.envVar, tc.envValue)
			defer os.Unsetenv(tc.envVar)

			yamlData := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
`)

			config, err := loader.LoadFromBytes(yamlData)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s=%s, but got none", tc.envVar, tc.envValue)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s=%s: %v", tc.envVar, tc.envValue, err)
			}
			if !tc.expectError && tc.expected != nil {
				if len(config.RabbitMQ.URIs) != len(tc.expected) {
					t.Errorf("Expected %d URIs, got %d", len(tc.expected), len(config.RabbitMQ.URIs))
				}
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	loader := configloader.NewLoader("", true)

	testCases := []struct {
		name          string
		yamlData      string
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty transport",
			yamlData:      `transport: ""`,
			expectError:   true,
			errorContains: "transport is required",
		},
		{
			name:          "unsupported transport",
			yamlData:      `transport: unsupported`,
			expectError:   true,
			errorContains: "unsupported transport",
		},
		{
			name: "rabbitmq transport without config",
			yamlData: `transport: rabbitmq
rabbitmq: null`,
			expectError:   true,
			errorContains: "at least one RabbitMQ URI is required",
		},
		{
			name: "empty URIs",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: []`,
			expectError:   true,
			errorContains: "at least one RabbitMQ URI is required",
		},
		{
			name: "invalid URI",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["invalid://uri:port"]`,
			expectError:   true,
			errorContains: "invalid RabbitMQ URI",
		},
		{
			name: "connection pool min too low",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  connectionPool:
    min: 0`,
			expectError:   true,
			errorContains: "connection pool min must be between 1 and 100",
		},
		{
			name: "connection pool max less than min",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  connectionPool:
    min: 10
    max: 5`,
			expectError:   true,
			errorContains: "connection pool max (5) must be greater than or equal to min (10)",
		},
		{
			name: "missing HMAC secret when enabled",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  security:
    hmacEnabled: true`,
			expectError:   true,
			errorContains: "HMAC secret is required when HMAC is enabled",
		},
		{
			name: "invalid HMAC algorithm",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  security:
    hmacAlgorithm: "md5"`,
			expectError:   true,
			errorContains: "HMAC algorithm must be one of",
		},
		{
			name: "invalid TLS version",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  tls:
    minVersion: "0.9"`,
			expectError:   true,
			errorContains: "TLS min version must be one of",
		},
		{
			name: "empty service name",
			yamlData: `transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
telemetry:
  serviceName: ""`,
			expectError:   true,
			errorContains: "service name cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loader.LoadFromBytes([]byte(tc.yamlData))
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestMaskSecrets(t *testing.T) {
	loader := configloader.NewLoader("", true)

	config := &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{
				"amqp://user:password@localhost:5672/",
				"amqp://another:secret@host2:5672/vhost",
			},
			Security: &messaging.SecurityConfig{
				HMACSecret: "super-secret-key",
			},
		},
	}

	masked := loader.MaskSecrets(config)

	// Check that original config is not modified
	if config.RabbitMQ.URIs[0] != "amqp://user:password@localhost:5672/" {
		t.Error("Original config was modified")
	}

	if config.RabbitMQ.Security.HMACSecret != "super-secret-key" {
		t.Error("Original config was modified")
	}

	// Check that masked config has secrets masked
	if !contains(masked.RabbitMQ.URIs[0], "***") && !contains(masked.RabbitMQ.URIs[0], "%2A%2A%2A") {
		t.Errorf("Expected URI to be masked, got: %s", masked.RabbitMQ.URIs[0])
	}

	if !contains(masked.RabbitMQ.URIs[1], "***") && !contains(masked.RabbitMQ.URIs[1], "%2A%2A%2A") {
		t.Errorf("Expected URI to be masked, got: %s", masked.RabbitMQ.URIs[1])
	}

	if masked.RabbitMQ.Security.HMACSecret != "***" {
		t.Errorf("Expected HMAC secret to be masked, got: %s", masked.RabbitMQ.Security.HMACSecret)
	}
}

func TestMaskSecretsDisabled(t *testing.T) {
	loader := configloader.NewLoader("", false)

	config := &messaging.Config{
		Transport: "rabbitmq",
		RabbitMQ: &messaging.RabbitMQConfig{
			URIs: []string{"amqp://user:password@localhost:5672/"},
			Security: &messaging.SecurityConfig{
				HMACSecret: "super-secret-key",
			},
		},
	}

	masked := loader.MaskSecrets(config)

	// When masking is disabled, should return the original config
	if masked != config {
		t.Error("Expected original config when masking is disabled")
	}
}

func TestDefaultConfig(t *testing.T) {
	loader := configloader.NewLoader("", true)

	// Test default config by loading empty YAML
	config, err := loader.LoadFromBytes([]byte(""))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if config.Transport != "rabbitmq" {
		t.Errorf("Expected default transport 'rabbitmq', got '%s'", config.Transport)
	}

	if config.RabbitMQ == nil {
		t.Fatal("Expected default RabbitMQ config")
	}

	if len(config.RabbitMQ.URIs) != 1 || config.RabbitMQ.URIs[0] != "amqp://localhost:5672/" {
		t.Errorf("Expected default URI 'amqp://localhost:5672/', got %v", config.RabbitMQ.URIs)
	}

	if config.RabbitMQ.ConnectionPool == nil {
		t.Fatal("Expected default connection pool config")
	}

	if config.RabbitMQ.ConnectionPool.Min != 2 {
		t.Errorf("Expected default connection pool min 2, got %d", config.RabbitMQ.ConnectionPool.Min)
	}

	if config.RabbitMQ.ConnectionPool.Max != 8 {
		t.Errorf("Expected default connection pool max 8, got %d", config.RabbitMQ.ConnectionPool.Max)
	}

	if config.RabbitMQ.ConnectionPool.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected default health check interval 30s, got %v", config.RabbitMQ.ConnectionPool.HealthCheckInterval)
	}
}

func TestPointerFieldHandling(t *testing.T) {
	loader := configloader.NewLoader("", true)

	// Test that pointer fields are properly initialized
	yamlData := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
`)

	config, err := loader.LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// All pointer fields should be initialized
	if config.RabbitMQ.ConnectionPool == nil {
		t.Error("Expected ConnectionPool to be initialized")
	}

	if config.RabbitMQ.ChannelPool == nil {
		t.Error("Expected ChannelPool to be initialized")
	}

	if config.RabbitMQ.Publisher == nil {
		t.Error("Expected Publisher to be initialized")
	}

	if config.RabbitMQ.Consumer == nil {
		t.Error("Expected Consumer to be initialized")
	}

	if config.RabbitMQ.TLS == nil {
		t.Error("Expected TLS to be initialized")
	}

	if config.RabbitMQ.Security == nil {
		t.Error("Expected Security to be initialized")
	}

	if config.Telemetry == nil {
		t.Error("Expected Telemetry to be initialized")
	}
}

func TestDurationValidation(t *testing.T) {
	loader := configloader.NewLoader("", true)

	testCases := []struct {
		name        string
		field       string
		value       string
		expectError bool
	}{
		{
			name:        "valid duration",
			field:       "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			value:       "30s",
			expectError: false,
		},
		{
			name:        "negative duration",
			field:       "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			value:       "-30s",
			expectError: true,
		},
		{
			name:        "duration too long",
			field:       "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			value:       "25h",
			expectError: true,
		},
		{
			name:        "zero duration",
			field:       "MSG_RABBITMQ_CONNECTIONPOOL_HEALTHCHECKINTERVAL",
			value:       "0s",
			expectError: true, // 0s is not valid for health check interval
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.field, tc.value)
			defer os.Unsetenv(tc.field)

			yamlData := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
`)

			_, err := loader.LoadFromBytes(yamlData)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s=%s, but got none", tc.field, tc.value)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s=%s: %v", tc.field, tc.value, err)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	// Create a temporary YAML file
	yamlContent := `
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  consumer:
    queue: "file.queue"
    prefetch: 150
`

	// Create temporary directory and file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_config.yaml")

	err := os.WriteFile(tempFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	// Test loading from file
	config, err := loader.Load(tempFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if config.Transport != "rabbitmq" {
		t.Errorf("Expected transport 'rabbitmq', got '%s'", config.Transport)
	}

	if config.RabbitMQ.Consumer.Queue != "file.queue" {
		t.Errorf("Expected queue 'file.queue', got '%s'", config.RabbitMQ.Consumer.Queue)
	}

	if config.RabbitMQ.Consumer.Prefetch != 150 {
		t.Errorf("Expected prefetch 150, got %d", config.RabbitMQ.Consumer.Prefetch)
	}
}

func TestLoadFromFileErrors(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	testCases := []struct {
		name          string
		configPath    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "non-existent file",
			configPath:    "/non/existent/path/config.yaml",
			expectError:   true,
			errorContains: "failed to read config file",
		},
		{
			name:        "empty path",
			configPath:  "",
			expectError: false, // Should load defaults
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loader.Load(tc.configPath)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}
			if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Error message '%s' does not contain expected text '%s'", err.Error(), tc.errorContains)
				}
			}
		})
	}
}

func TestKafkaTransportConfiguration(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	yamlData := []byte(`
transport: kafka
kafka:
  brokers: ["localhost:9092", "localhost:9093"]
  consumer:
    groupId: "test-group"
    autoOffsetReset: "earliest"
  producer:
    acks: "all"
    retries: 3
`)

	config, err := loader.LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if config.Transport != "kafka" {
		t.Errorf("Expected transport 'kafka', got '%s'", config.Transport)
	}

	// Note: The current implementation doesn't have Kafka config structs defined,
	// so we're testing that the transport validation accepts "kafka" as valid
	// and that the YAML parsing doesn't fail with kafka configuration
}

func TestKafkaEnvironmentVariableOverlay(t *testing.T) {
	loader := configloader.NewLoader("", true)

	// Set environment variables for Kafka configuration
	os.Setenv("MSG_TRANSPORT", "kafka")
	os.Setenv("MSG_KAFKA_BROKERS", "kafka1:9092,kafka2:9092")
	os.Setenv("MSG_KAFKA_CONSUMER_GROUPID", "env-group")
	defer func() {
		os.Unsetenv("MSG_TRANSPORT")
		os.Unsetenv("MSG_KAFKA_BROKERS")
		os.Unsetenv("MSG_KAFKA_CONSUMER_GROUPID")
	}()

	yamlData := []byte(`
transport: rabbitmq
kafka:
  brokers: ["localhost:9092"]
  consumer:
    groupId: "yaml-group"
`)

	config, err := loader.LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Environment variables should override YAML values
	if config.Transport != "kafka" {
		t.Errorf("Expected transport 'kafka' (from env), got '%s'", config.Transport)
	}

	// Note: Since Kafka config structs aren't implemented yet, we're testing
	// that the environment variable overlay system works for kafka transport
	// and that validation accepts kafka as a valid transport
}

func TestNilLoaderHandling(t *testing.T) {
	// Test LoadFromBytes with nil loader
	var loader *configloader.Loader
	_, err := loader.LoadFromBytes([]byte(`transport: rabbitmq`))
	if err == nil {
		t.Error("Expected error when loader is nil")
	}
	if !contains(err.Error(), "loader cannot be nil") {
		t.Errorf("Expected 'loader cannot be nil' error, got: %v", err)
	}

	// Test Load with nil loader
	_, err = loader.Load("")
	if err == nil {
		t.Error("Expected error when loader is nil")
	}
	if !contains(err.Error(), "loader cannot be nil") {
		t.Errorf("Expected 'loader cannot be nil' error, got: %v", err)
	}

	// Test MaskSecrets with nil loader
	config := &messaging.Config{Transport: "rabbitmq"}
	result := loader.MaskSecrets(config)
	if result != config {
		t.Error("Expected original config when loader is nil")
	}
}

func TestNilInputHandling(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	// Test LoadFromBytes with nil YAML data
	_, err := loader.LoadFromBytes(nil)
	if err == nil {
		t.Error("Expected error when YAML data is nil")
	}
	if !contains(err.Error(), "yaml data cannot be nil") {
		t.Errorf("Expected 'yaml data cannot be nil' error, got: %v", err)
	}

	// Test MaskSecrets with nil config
	result := loader.MaskSecrets(nil)
	if result != nil {
		t.Error("Expected nil result when config is nil")
	}
}

func TestInvalidYAMLHandling(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	invalidYAML := []byte(`
transport: rabbitmq
rabbitmq:
  uris: ["amqp://localhost:5672/"]
  consumer:
    prefetch: "not_a_number"
`)

	_, err := loader.LoadFromBytes(invalidYAML)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
	if !contains(err.Error(), "failed to unmarshal YAML") {
		t.Errorf("Expected YAML unmarshal error, got: %v", err)
	}
}

func TestEmptyYAMLHandling(t *testing.T) {
	loader := configloader.NewLoader("TEST", true)

	// Test with completely empty YAML
	config, err := loader.LoadFromBytes([]byte(""))
	if err != nil {
		t.Fatalf("LoadFromBytes failed with empty YAML: %v", err)
	}

	// Should load default configuration
	if config.Transport != "rabbitmq" {
		t.Errorf("Expected default transport 'rabbitmq', got '%s'", config.Transport)
	}

	// Test with valid but minimal YAML
	config, err = loader.LoadFromBytes([]byte("# Empty config\n"))
	if err != nil {
		t.Fatalf("LoadFromBytes failed with comment-only YAML: %v", err)
	}

	if config.Transport != "rabbitmq" {
		t.Errorf("Expected default transport 'rabbitmq', got '%s'", config.Transport)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
