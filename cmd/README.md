# go-messagex CLI Applications

This directory contains command-line interface (CLI) applications for the go-messagex messaging library.

## Applications

### Publisher CLI (`cmd/publisher/main.go`)

A command-line publisher application that demonstrates message publishing capabilities.

#### Features

- **Config file and ENV support**: Load configuration from YAML files or environment variables
- **Sample JSON payload publishing**: Publish JSON messages with customizable content
- **Idempotency demonstration**: Enable idempotency keys for message deduplication
- **Priority demonstration**: Set message priorities (0-255)
- **Receipt result handling**: Track message delivery confirmations
- **Interactive mode**: Interactive shell for testing and development
- **Batch mode**: Publish multiple messages with statistics

#### Usage

```bash
# Show help
go run cmd/publisher/main.go -help

# Publish a single message
go run cmd/publisher/main.go -message '{"hello": "world"}'

# Publish multiple messages with priority
go run cmd/publisher/main.go -count 10 -priority 5

# Use custom configuration file
go run cmd/publisher/main.go -config configs/cli.example.yaml -exchange my.exchange

# Interactive mode
go run cmd/publisher/main.go -interactive

# Enable idempotency
go run cmd/publisher/main.go -idempotent -message '{"idempotent": "message"}'
```

#### Options

- `-config`: Configuration file path (YAML)
- `-exchange`: Exchange name (default: "demo.exchange")
- `-key`: Routing key (default: "demo.key")
- `-message`: Message body (JSON) (default: `{"hello": "world"}`)
- `-count`: Number of messages to publish (default: 1)
- `-priority`: Message priority (0-255) (default: 0)
- `-idempotent`: Enable idempotency
- `-interactive`: Interactive mode
- `-verbose`: Verbose logging

#### Interactive Commands

When running in interactive mode, you can use these commands:

- `publish <message> [exchange] [key] [priority]`: Publish a message
- `config`: Show current configuration
- `stats`: Show publisher statistics
- `clear`: Clear screen
- `help`: Show help
- `quit`: Exit

### Consumer CLI (`cmd/consumer/main.go`)

A command-line consumer application that demonstrates message consumption capabilities.

#### Features

- **Simple message echo handler**: Process and display received messages
- **Ack/nack decision demonstration**: Handle message acknowledgment decisions
- **Structured logging examples**: Comprehensive logging with structured fields
- **Failure simulation**: Simulate processing failures for testing
- **Interactive mode**: Interactive shell for monitoring and control
- **Statistics reporting**: Real-time statistics and monitoring

#### Usage

```bash
# Show help
go run cmd/consumer/main.go -help

# Start consumer with default settings
go run cmd/consumer/main.go

# Use custom queue and configuration
go run cmd/consumer/main.go -queue my.queue -config configs/cli.example.yaml

# Simulate failures for testing
go run cmd/consumer/main.go -failure-rate 0.1

# Interactive mode
go run cmd/consumer/main.go -interactive

# Custom concurrency and prefetch
go run cmd/consumer/main.go -concurrency 100 -prefetch 50
```

#### Options

- `-config`: Configuration file path (YAML)
- `-queue`: Queue name (default: "demo.queue")
- `-prefetch`: Prefetch count (default: 256)
- `-concurrency`: Max concurrent handlers (default: 512)
- `-timeout`: Handler timeout (default: 30s)
- `-failure-rate`: Simulated failure rate (0.0 to 1.0) (default: 0.0)
- `-interactive`: Interactive mode
- `-verbose`: Verbose logging

#### Interactive Commands

When running in interactive mode, you can use these commands:

- `stats`: Show consumer statistics
- `config`: Show current configuration
- `clear`: Clear screen
- `help`: Show help
- `quit`: Exit

## Configuration

Both CLI applications support configuration through:

1. **YAML Configuration Files**: Use the `-config` flag to specify a configuration file
2. **Environment Variables**: Set environment variables with the `MSG_` prefix
3. **Command-line Flags**: Override specific settings with command-line options

### Configuration Precedence

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

### Example Configuration

See `configs/cli.example.yaml` for a complete configuration example.

### Environment Variables

#### Publisher Environment Variables

- `MSG_RABBITMQ_URIS`: RabbitMQ connection URIs
- `MSG_RABBITMQ_PUBLISHER_CONFIRMS`: Enable publisher confirms
- `MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT`: Max in-flight messages
- `MSG_RABBITMQ_PUBLISHER_WORKERCOUNT`: Worker count

#### Consumer Environment Variables

- `MSG_RABBITMQ_URIS`: RabbitMQ connection URIs
- `MSG_RABBITMQ_CONSUMER_QUEUE`: Queue name
- `MSG_RABBITMQ_CONSUMER_PREFETCH`: Prefetch count
- `MSG_RABBITMQ_CONSUMER_MAXCONCURRENTHANDLERS`: Max concurrent handlers

## Examples

### Basic Publisher-Consumer Workflow

1. **Start the consumer**:
   ```bash
   go run cmd/consumer/main.go -queue test.queue
   ```

2. **Publish messages**:
   ```bash
   go run cmd/publisher/main.go -exchange test.exchange -key test.key -message '{"test": "data"}'
   ```

### Testing with Failure Simulation

1. **Start consumer with failure simulation**:
   ```bash
   go run cmd/consumer/main.go -failure-rate 0.2
   ```

2. **Publish messages to test error handling**:
   ```bash
   go run cmd/publisher/main.go -count 100 -message '{"test": "failure"}'
   ```

### Interactive Development

1. **Start interactive publisher**:
   ```bash
   go run cmd/publisher/main.go -interactive
   ```

2. **Start interactive consumer**:
   ```bash
   go run cmd/consumer/main.go -interactive
   ```

3. **Use interactive commands to test and monitor**:
   ```
   publisher> publish '{"hello": "world"}'
   publisher> stats
   consumer> stats
   ```

## Integration with Makefile

The CLI applications are integrated with the project's Makefile:

```bash
# Run publisher example
make run-publisher

# Run consumer example
make run-consumer
```

## Troubleshooting

### Common Issues

1. **Connection Errors**: Ensure RabbitMQ is running and accessible
2. **Configuration Errors**: Check YAML syntax and environment variables
3. **Permission Errors**: Ensure proper permissions for configuration files

### Debug Mode

Use the `-verbose` flag to enable detailed logging:

```bash
go run cmd/publisher/main.go -verbose -message '{"debug": "test"}'
go run cmd/consumer/main.go -verbose
```

### Configuration Validation

The CLI applications validate configuration on startup and provide helpful error messages for common issues.

## Development

### Adding New Features

1. **New CLI Options**: Add new flag variables in the `parseFlags()` method
2. **New Commands**: Add new command handlers in the interactive mode
3. **New Configuration**: Extend the configuration structs in the messaging package

### Testing

Test the CLI applications with different configurations and scenarios:

```bash
# Test with custom configuration
go run cmd/publisher/main.go -config test-config.yaml

# Test error handling
go run cmd/consumer/main.go -failure-rate 0.5

# Test performance
go run cmd/publisher/main.go -count 1000
go run cmd/consumer/main.go -concurrency 1000
```
