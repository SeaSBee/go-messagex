package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/seasbee/go-messagex/internal/configloader"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

type PublisherCLI struct {
	configFile   string
	exchange     string
	routingKey   string
	messageBody  string
	messageCount int
	priority     int
	idempotency  bool
	interactive  bool
	verbose      bool
	help         bool
}

func (cli *PublisherCLI) parseFlags() {
	flag.StringVar(&cli.configFile, "config", "", "Configuration file path (YAML)")
	flag.StringVar(&cli.exchange, "exchange", "demo.exchange", "Exchange name")
	flag.StringVar(&cli.routingKey, "key", "demo.key", "Routing key")
	flag.StringVar(&cli.messageBody, "message", `{"hello": "world"}`, "Message body (JSON)")
	flag.IntVar(&cli.messageCount, "count", 1, "Number of messages to publish")
	flag.IntVar(&cli.priority, "priority", 0, "Message priority (0-255)")
	flag.BoolVar(&cli.idempotency, "idempotent", false, "Enable idempotency")
	flag.BoolVar(&cli.interactive, "interactive", false, "Interactive mode")
	flag.BoolVar(&cli.verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&cli.help, "help", false, "Show help")
	flag.Parse()

	if cli.help {
		cli.showHelp()
		os.Exit(0)
	}
}

func (cli *PublisherCLI) showHelp() {
	fmt.Println("🚀 go-messagex Publisher CLI")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("Usage: go run cmd/publisher/main.go [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Publish a single message")
	fmt.Println("  go run cmd/publisher/main.go -message '{\"test\": \"data\"}'")
	fmt.Println()
	fmt.Println("  # Publish multiple messages with priority")
	fmt.Println("  go run cmd/publisher/main.go -count 10 -priority 5")
	fmt.Println()
	fmt.Println("  # Use custom configuration file")
	fmt.Println("  go run cmd/publisher/main.go -config config.yaml -exchange my.exchange")
	fmt.Println()
	fmt.Println("  # Interactive mode")
	fmt.Println("  go run cmd/publisher/main.go -interactive")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  MSG_RABBITMQ_URIS              RabbitMQ connection URIs")
	fmt.Println("  MSG_RABBITMQ_PUBLISHER_CONFIRMS Enable publisher confirms")
	fmt.Println("  MSG_RABBITMQ_PUBLISHER_MAXINFLIGHT Max in-flight messages")
	fmt.Println("  MSG_RABBITMQ_PUBLISHER_WORKERCOUNT Worker count")
}

func (cli *PublisherCLI) loadConfig() (*messaging.Config, error) {
	// Create loader
	loader := configloader.NewLoader("MSG_", true)

	var config *messaging.Config
	var err error

	if cli.configFile != "" {
		// Load from file
		config, err = loader.Load(cli.configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		// Load from environment only
		config, err = loader.Load("")
		if err != nil {
			return nil, fmt.Errorf("failed to load config from environment: %w", err)
		}
	}

	// Set defaults if not configured
	if config.Transport == "" {
		config.Transport = "rabbitmq"
	}
	if config.RabbitMQ == nil {
		config.RabbitMQ = &messaging.RabbitMQConfig{}
	}
	if len(config.RabbitMQ.URIs) == 0 {
		config.RabbitMQ.URIs = []string{"amqp://localhost:5672"}
	}
	if config.RabbitMQ.Publisher == nil {
		config.RabbitMQ.Publisher = &messaging.PublisherConfig{
			Confirms:       true,
			MaxInFlight:    1000,
			WorkerCount:    4,
			PublishTimeout: 30 * time.Second,
		}
	}

	// Configure telemetry for verbose logging
	if cli.verbose && config.Telemetry == nil {
		config.Telemetry = &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "publisher-cli",
		}
	}

	return config, nil
}

func (cli *PublisherCLI) validateMessageBody() error {
	if cli.messageBody == "" {
		return fmt.Errorf("message body cannot be empty")
	}

	// Validate JSON if it looks like JSON
	if strings.TrimSpace(cli.messageBody)[0] == '{' {
		var js json.RawMessage
		if err := json.Unmarshal([]byte(cli.messageBody), &js); err != nil {
			return fmt.Errorf("invalid JSON message body: %w", err)
		}
	}

	return nil
}

func (cli *PublisherCLI) createMessage(index int) messaging.Message {
	// Create unique message body for multiple messages
	messageBody := cli.messageBody
	if cli.messageCount > 1 {
		// Try to parse as JSON and add index
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(cli.messageBody), &data); err == nil {
			data["index"] = index
			data["timestamp"] = time.Now().Unix()
			if newBody, err := json.Marshal(data); err == nil {
				messageBody = string(newBody)
			}
		}
	}

	msg := messaging.NewMessage(
		[]byte(messageBody),
		messaging.WithID(fmt.Sprintf("msg-%d-%d", time.Now().Unix(), index)),
		messaging.WithContentType("application/json"),
		messaging.WithKey(cli.routingKey),
		messaging.WithPriority(uint8(cli.priority)),
		messaging.WithTimestamp(time.Now()),
	)

	if cli.idempotency {
		msg.IdempotencyKey = fmt.Sprintf("idemp-%d-%d", time.Now().Unix(), index)
	}

	return msg
}

func (cli *PublisherCLI) runInteractive() error {
	fmt.Println("🚀 Interactive Publisher Mode")
	fmt.Println("=============================")
	fmt.Println("Type 'help' for commands, 'quit' to exit")
	fmt.Println()

	// Load config
	config, err := cli.loadConfig()
	if err != nil {
		return err
	}

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create publisher
	publisher, err := factory.NewPublisher(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create publisher: %w", err)
	}
	defer publisher.Close(context.Background())

	// Create observability context
	obsProvider, err := messaging.NewObservabilityProvider(config.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to create observability provider: %w", err)
	}
	obsCtx := messaging.NewObservabilityContext(context.Background(), obsProvider)

	fmt.Printf("Connected to RabbitMQ at: %s\n", strings.Join(config.RabbitMQ.URIs, ", "))
	fmt.Printf("Default exchange: %s\n", cli.exchange)
	fmt.Printf("Default routing key: %s\n", cli.routingKey)
	fmt.Println()

	for {
		fmt.Print("publisher> ")
		var input string
		fmt.Scanln(&input)

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch command {
		case "help":
			cli.showInteractiveHelp()
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return nil
		case "publish", "send":
			cli.handlePublishCommand(publisher, obsCtx, args)
		case "config":
			cli.showConfig(config)
		case "stats":
			cli.showStats(publisher)
		case "clear":
			fmt.Print("\033[H\033[2J") // Clear screen
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
		}
	}
}

func (cli *PublisherCLI) showInteractiveHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  publish <message> [exchange] [key] [priority] - Publish a message")
	fmt.Println("  config                                          - Show current configuration")
	fmt.Println("  stats                                           - Show publisher statistics")
	fmt.Println("  clear                                           - Clear screen")
	fmt.Println("  help                                            - Show this help")
	fmt.Println("  quit                                            - Exit")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  publish '{\"hello\": \"world\"}'")
	fmt.Println("  publish '{\"data\": \"test\"}' my.exchange my.key 5")
}

func (cli *PublisherCLI) handlePublishCommand(publisher messaging.Publisher, obsCtx *messaging.ObservabilityContext, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Message body required")
		return
	}

	messageBody := args[0]
	exchange := cli.exchange
	routingKey := cli.routingKey
	priority := cli.priority

	if len(args) > 1 {
		exchange = args[1]
	}
	if len(args) > 2 {
		routingKey = args[2]
	}
	if len(args) > 3 {
		if p, err := strconv.Atoi(args[3]); err == nil {
			priority = p
		}
	}

	msg := messaging.NewMessage(
		[]byte(messageBody),
		messaging.WithID(fmt.Sprintf("msg-%d", time.Now().UnixNano())),
		messaging.WithContentType("application/json"),
		messaging.WithKey(routingKey),
		messaging.WithPriority(uint8(priority)),
		messaging.WithTimestamp(time.Now()),
	)

	if cli.idempotency {
		msg.IdempotencyKey = fmt.Sprintf("idemp-%d", time.Now().UnixNano())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	receipt, err := publisher.PublishAsync(ctx, exchange, msg)
	if err != nil {
		fmt.Printf("❌ Failed to publish: %v\n", err)
		return
	}

	<-receipt.Done()
	result, err := receipt.Result()
	if err != nil {
		fmt.Printf("❌ Publish failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Message published successfully\n")
	fmt.Printf("   Message ID: %s\n", msg.ID)
	fmt.Printf("   Exchange: %s\n", exchange)
	fmt.Printf("   Routing Key: %s\n", routingKey)
	fmt.Printf("   Priority: %d\n", priority)
	fmt.Printf("   Delivery Tag: %d\n", result.DeliveryTag)
	if cli.idempotency {
		fmt.Printf("   Idempotency Key: %s\n", msg.IdempotencyKey)
	}
}

func (cli *PublisherCLI) showConfig(config *messaging.Config) {
	fmt.Println("Current Configuration:")
	fmt.Printf("  Transport: %s\n", config.Transport)
	fmt.Printf("  RabbitMQ URIs: %s\n", strings.Join(config.RabbitMQ.URIs, ", "))
	if config.RabbitMQ.Publisher != nil {
		fmt.Printf("  Publisher Confirms: %t\n", config.RabbitMQ.Publisher.Confirms)
		fmt.Printf("  Max In Flight: %d\n", config.RabbitMQ.Publisher.MaxInFlight)
		fmt.Printf("  Worker Count: %d\n", config.RabbitMQ.Publisher.WorkerCount)
	}
	fmt.Println()
}

func (cli *PublisherCLI) showStats(publisher messaging.Publisher) {
	if asyncPublisher, ok := publisher.(*rabbitmq.AsyncPublisher); ok {
		stats := asyncPublisher.GetStats()
		fmt.Println("Publisher Statistics:")
		fmt.Printf("  Tasks Queued: %d\n", stats.TasksQueued)
		fmt.Printf("  Tasks Processed: %d\n", stats.TasksProcessed)
		fmt.Printf("  Tasks Failed: %d\n", stats.TasksFailed)
		fmt.Printf("  Tasks Dropped: %d\n", stats.TasksDropped)
		fmt.Printf("  Queue Full Count: %d\n", stats.QueueFullCount)
	} else {
		fmt.Println("Statistics not available for this publisher type")
	}
	fmt.Println()
}

func (cli *PublisherCLI) runBatch() error {
	fmt.Println("🚀 Batch Publisher Mode")
	fmt.Println("=======================")

	// Validate inputs
	if err := cli.validateMessageBody(); err != nil {
		return err
	}

	// Load configuration
	config, err := cli.loadConfig()
	if err != nil {
		return err
	}

	// Initialize logging
	if err := logx.InitDefault(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logx.Sync()

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create publisher
	publisher, err := factory.NewPublisher(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create publisher: %w", err)
	}
	defer publisher.Close(context.Background())

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		cancel()
	}()

	fmt.Printf("Publishing %d messages to exchange '%s' with routing key '%s'\n",
		cli.messageCount, cli.exchange, cli.routingKey)
	fmt.Printf("Priority: %d, Idempotency: %t\n", cli.priority, cli.idempotency)
	fmt.Println()

	// Publish messages
	receipts := make([]messaging.Receipt, cli.messageCount)
	startTime := time.Now()

	for i := 0; i < cli.messageCount; i++ {
		msg := cli.createMessage(i)

		receipt, err := publisher.PublishAsync(ctx, cli.exchange, msg)
		if err != nil {
			fmt.Printf("❌ Failed to publish message %d: %v\n", i+1, err)
			continue
		}

		receipts[i] = receipt
		fmt.Printf("📤 Queued message %d/%d (ID: %s)\n", i+1, cli.messageCount, msg.ID)
	}

	// Wait for all confirmations
	fmt.Println("\n⏳ Waiting for confirmations...")
	successCount := 0
	failureCount := 0

	for i, receipt := range receipts {
		if receipt == nil {
			failureCount++
			continue
		}

		<-receipt.Done()
		result, err := receipt.Result()
		if err != nil {
			fmt.Printf("❌ Message %d failed: %v\n", i+1, err)
			failureCount++
		} else {
			fmt.Printf("✅ Message %d confirmed (Delivery Tag: %d)\n", i+1, result.DeliveryTag)
			successCount++
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Messages: %d\n", cli.messageCount)
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", failureCount)
	fmt.Printf("  Duration: %v\n", duration)
	if duration > 0 {
		fmt.Printf("  Rate: %.2f msg/sec\n", float64(successCount)/duration.Seconds())
	}

	return nil
}

func main() {
	cli := &PublisherCLI{}
	cli.parseFlags()

	if cli.interactive {
		if err := cli.runInteractive(); err != nil {
			log.Fatalf("Interactive mode failed: %v", err)
		}
	} else {
		if err := cli.runBatch(); err != nil {
			log.Fatalf("Batch mode failed: %v", err)
		}
	}
}
