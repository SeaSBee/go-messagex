package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/internal/configloader"
	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/seasbee/go-messagex/pkg/rabbitmq"
)

type ConsumerCLI struct {
	configFile     string
	queue          string
	prefetch       int
	concurrency    int
	timeout        time.Duration
	failureRate    float64
	interactive    bool
	verbose        bool
	help           bool
	processedCount int64
}

func (cli *ConsumerCLI) parseFlags() {
	flag.StringVar(&cli.configFile, "config", "", "Configuration file path (YAML)")
	flag.StringVar(&cli.queue, "queue", "demo.queue", "Queue name")
	flag.IntVar(&cli.prefetch, "prefetch", 256, "Prefetch count")
	flag.IntVar(&cli.concurrency, "concurrency", 512, "Max concurrent handlers")
	flag.DurationVar(&cli.timeout, "timeout", 30*time.Second, "Handler timeout")
	flag.Float64Var(&cli.failureRate, "failure-rate", 0.0, "Simulated failure rate (0.0 to 1.0)")
	flag.BoolVar(&cli.interactive, "interactive", false, "Interactive mode")
	flag.BoolVar(&cli.verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&cli.help, "help", false, "Show help")
	flag.Parse()

	// Validate input parameters
	if err := cli.validateParams(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid parameters: %v\n", err)
		os.Exit(1)
	}

	if cli.help {
		cli.showHelp()
		os.Exit(0)
	}
}

func (cli *ConsumerCLI) validateParams() error {
	if cli.queue == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	if cli.prefetch <= 0 {
		return fmt.Errorf("prefetch count must be positive")
	}
	if cli.concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	if cli.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if cli.failureRate < 0.0 || cli.failureRate > 1.0 {
		return fmt.Errorf("failure rate must be between 0.0 and 1.0")
	}
	return nil
}

func (cli *ConsumerCLI) showHelp() {
	fmt.Println("🚀 go-messagex Consumer CLI")
	fmt.Println("============================")
	fmt.Println()
	fmt.Println("Usage: go run cmd/consumer/main.go [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Start consumer with default settings")
	fmt.Println("  go run cmd/consumer/main.go")
	fmt.Println()
	fmt.Println("  # Use custom queue and configuration")
	fmt.Println("  go run cmd/consumer/main.go -queue my.queue -config config.yaml")
	fmt.Println()
	fmt.Println("  # Simulate failures for testing")
	fmt.Println("  go run cmd/consumer/main.go -failure-rate 0.1")
	fmt.Println()
	fmt.Println("  # Interactive mode")
	fmt.Println("  go run cmd/consumer/main.go -interactive")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  MSG_RABBITMQ_URIS                        RabbitMQ connection URIs")
	fmt.Println("  MSG_RABBITMQ_CONSUMER_QUEUE              Queue name")
	fmt.Println("  MSG_RABBITMQ_CONSUMER_PREFETCH           Prefetch count")
	fmt.Println("  MSG_RABBITMQ_CONSUMER_MAXCONCURRENTHANDLERS Max concurrent handlers")
}

func (cli *ConsumerCLI) loadConfig() (*messaging.Config, error) {
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

	// Ensure config is not nil
	if config == nil {
		config = &messaging.Config{}
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
	if config.RabbitMQ.Consumer == nil {
		config.RabbitMQ.Consumer = &messaging.ConsumerConfig{
			Queue:                 cli.queue,
			Prefetch:              cli.prefetch,
			MaxConcurrentHandlers: cli.concurrency,
			HandlerTimeout:        cli.timeout,
			RequeueOnError:        true,
			AckOnSuccess:          true,
			AutoAck:               false,
			PanicRecovery:         true,
			MaxRetries:            3,
		}
	} else {
		// Override with CLI values
		config.RabbitMQ.Consumer.Queue = cli.queue
		config.RabbitMQ.Consumer.Prefetch = cli.prefetch
		config.RabbitMQ.Consumer.MaxConcurrentHandlers = cli.concurrency
		config.RabbitMQ.Consumer.HandlerTimeout = cli.timeout
	}

	// Configure telemetry for verbose logging
	if cli.verbose && config.Telemetry == nil {
		config.Telemetry = &messaging.TelemetryConfig{
			MetricsEnabled: true,
			TracingEnabled: true,
			ServiceName:    "consumer-cli",
		}
	}

	return config, nil
}

func (cli *ConsumerCLI) createMessageHandler() messaging.Handler {
	return messaging.HandlerFunc(func(ctx context.Context, delivery messaging.Delivery) (messaging.AckDecision, error) {
		// Increment processed count atomically
		processedCount := atomic.AddInt64(&cli.processedCount, 1)

		// Log message details
		logx.Info("Processing message",
			logx.String("message_id", delivery.Message.ID),
			logx.String("queue", cli.queue),
			logx.Int("processed_count", int(processedCount)),
			logx.String("routing_key", delivery.Message.Key),
			logx.Int("priority", int(delivery.Message.Priority)),
		)

		// Calculate processing time with nil check for message ID
		var processingTime time.Duration
		if delivery.Message.ID != "" {
			processingTime = time.Duration(10+delivery.Message.ID[0]%50) * time.Millisecond
		} else {
			processingTime = time.Duration(10+processedCount%50) * time.Millisecond
		}
		time.Sleep(processingTime)

		// Simulate failures based on failure rate
		if cli.failureRate > 0 && float64(processedCount%100) < cli.failureRate*100 {
			logx.Error("Simulated failure",
				logx.String("message_id", delivery.Message.ID),
				logx.String("error", fmt.Sprintf("simulated failure for message %s", delivery.Message.ID)),
			)
			return messaging.NackRequeue, fmt.Errorf("simulated failure for message %s", delivery.Message.ID)
		}

		// Simulate panic occasionally
		if processedCount%1000 == 0 && cli.failureRate > 0.1 {
			logx.Error("Simulated panic",
				logx.String("message_id", delivery.Message.ID),
				logx.String("error", "simulated panic"),
			)
			panic(fmt.Sprintf("simulated panic for message %s", delivery.Message.ID))
		}

		// Echo message details
		fmt.Printf("✅ Processed message %s (count: %d, time: %v)\n",
			delivery.Message.ID, processedCount, processingTime)

		// Log message body if it's JSON with proper error handling
		if strings.HasPrefix(delivery.Message.ContentType, "application/json") {
			var data map[string]interface{}
			if err := json.Unmarshal(delivery.Message.Body, &data); err != nil {
				logx.Warn("Failed to unmarshal JSON message body",
					logx.String("message_id", delivery.Message.ID),
					logx.String("error", err.Error()),
				)
			} else {
				logx.Debug("Message body",
					logx.String("message_id", delivery.Message.ID),
					logx.Any("body", data),
				)
			}
		}

		// Log correlation ID if present
		if delivery.Message.CorrelationID != "" {
			logx.Debug("Correlation ID",
				logx.String("message_id", delivery.Message.ID),
				logx.String("correlation_id", delivery.Message.CorrelationID),
			)
		}

		// Log idempotency key if present
		if delivery.Message.IdempotencyKey != "" {
			logx.Debug("Idempotency Key",
				logx.String("message_id", delivery.Message.ID),
				logx.String("idempotency_key", delivery.Message.IdempotencyKey),
			)
		}

		// Log headers if present
		if len(delivery.Message.Headers) > 0 {
			logx.Debug("Message headers",
				logx.String("message_id", delivery.Message.ID),
				logx.Any("headers", delivery.Message.Headers),
			)
		}

		return messaging.Ack, nil
	})
}

func (cli *ConsumerCLI) runInteractive() error {
	fmt.Println("🚀 Interactive Consumer Mode")
	fmt.Println("============================")
	fmt.Println("Type 'help' for commands, 'quit' to exit")
	fmt.Println()

	// Load config
	config, err := cli.loadConfig()
	if err != nil {
		return err
	}

	// Logging is already initialized in main()

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create cancellable context for the consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create consumer with proper context
	consumer, err := factory.NewConsumer(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	defer func() {
		// Use a timeout context for stopping the consumer
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		consumer.Stop(stopCtx)
	}()

	// Create message handler
	handler := cli.createMessageHandler()

	fmt.Printf("Connected to RabbitMQ at: %s\n", strings.Join(config.RabbitMQ.URIs, ", "))
	fmt.Printf("Queue: %s\n", cli.queue)
	fmt.Printf("Prefetch: %d, Concurrency: %d\n", cli.prefetch, cli.concurrency)
	fmt.Printf("Failure Rate: %.2f\n", cli.failureRate)
	fmt.Println()

	// Start consumer
	err = consumer.Start(ctx, handler)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	fmt.Println("Consumer started. Type 'help' for commands.")

	// Set up graceful shutdown for interactive mode
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel for user input
	inputChan := make(chan string, 1)
	go func() {
		for {
			fmt.Print("consumer> ")
			var input string
			if _, err := fmt.Scanln(&input); err != nil {
				// Handle EOF or other input errors
				inputChan <- "quit"
				return
			}
			inputChan <- input
		}
	}()

	for {
		select {
		case input := <-inputChan:
			parts := strings.Fields(input)
			if len(parts) == 0 {
				continue
			}

			command := parts[0]

			switch command {
			case "help":
				cli.showInteractiveHelp()
			case "quit", "exit":
				fmt.Println("Stopping consumer...")
				return nil
			case "stats":
				cli.showStats(consumer)
			case "config":
				cli.showConfig(config)
			case "clear":
				fmt.Print("\033[H\033[2J") // Clear screen
			default:
				fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
			}
		case <-sigChan:
			fmt.Println("\n🛑 Shutting down...")
			return nil
		case <-ctx.Done():
			fmt.Println("Context cancelled, shutting down...")
			return nil
		}
	}
}

func (cli *ConsumerCLI) showInteractiveHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  stats                                           - Show consumer statistics")
	fmt.Println("  config                                          - Show current configuration")
	fmt.Println("  clear                                           - Clear screen")
	fmt.Println("  help                                            - Show this help")
	fmt.Println("  quit                                            - Exit")
	fmt.Println()
	fmt.Println("Consumer is running and processing messages automatically.")
}

func (cli *ConsumerCLI) showStats(consumer messaging.Consumer) {
	if concurrentConsumer, ok := consumer.(*rabbitmq.ConcurrentConsumer); ok {
		stats := concurrentConsumer.GetStats()
		fmt.Println("Consumer Statistics:")
		fmt.Printf("  Messages Processed: %d\n", stats.MessagesProcessed)
		fmt.Printf("  Messages Failed: %d\n", stats.MessagesFailed)
		fmt.Printf("  Messages Requeued: %d\n", stats.MessagesRequeued)
		fmt.Printf("  Messages Sent to DLQ: %d\n", stats.MessagesSentToDLQ)
		fmt.Printf("  Active Workers: %d\n", stats.ActiveWorkers)
		fmt.Printf("  Queued Tasks: %d\n", stats.QueuedTasks)
	} else {
		fmt.Println("Statistics not available for this consumer type")
	}
	fmt.Println()
}

func (cli *ConsumerCLI) showConfig(config *messaging.Config) {
	fmt.Println("Current Configuration:")
	fmt.Printf("  Transport: %s\n", config.Transport)
	fmt.Printf("  RabbitMQ URIs: %s\n", strings.Join(config.RabbitMQ.URIs, ", "))
	if config.RabbitMQ.Consumer != nil {
		fmt.Printf("  Queue: %s\n", config.RabbitMQ.Consumer.Queue)
		fmt.Printf("  Prefetch: %d\n", config.RabbitMQ.Consumer.Prefetch)
		fmt.Printf("  Max Concurrent Handlers: %d\n", config.RabbitMQ.Consumer.MaxConcurrentHandlers)
		fmt.Printf("  Handler Timeout: %v\n", config.RabbitMQ.Consumer.HandlerTimeout)
		fmt.Printf("  Requeue On Error: %t\n", config.RabbitMQ.Consumer.RequeueOnError)
		fmt.Printf("  Panic Recovery: %t\n", config.RabbitMQ.Consumer.PanicRecovery)
		fmt.Printf("  Max Retries: %d\n", config.RabbitMQ.Consumer.MaxRetries)
	}
	fmt.Println()
}

func (cli *ConsumerCLI) runBatch() error {
	fmt.Println("🚀 Batch Consumer Mode")
	fmt.Println("======================")

	// Load configuration
	config, err := cli.loadConfig()
	if err != nil {
		return err
	}

	// Logging is already initialized in main()

	// Create transport factory
	factory := &rabbitmq.TransportFactory{}

	// Create cancellable context for the consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create consumer with proper context
	consumer, err := factory.NewConsumer(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	defer func() {
		// Use a timeout context for stopping the consumer
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		consumer.Stop(stopCtx)
	}()

	// Create message handler
	handler := cli.createMessageHandler()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		cancel()
	}()

	fmt.Printf("Starting consumer for queue '%s'\n", cli.queue)
	fmt.Printf("Prefetch: %d, Concurrency: %d, Timeout: %v\n",
		cli.prefetch, cli.concurrency, cli.timeout)
	fmt.Printf("Failure Rate: %.2f\n", cli.failureRate)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start consumer
	err = consumer.Start(ctx, handler)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	// Start statistics reporting
	go cli.reportStats(consumer)

	// Wait for shutdown signal
	<-ctx.Done()

	fmt.Printf("\n📊 Final Statistics:\n")
	fmt.Printf("  Total Messages Processed: %d\n", atomic.LoadInt64(&cli.processedCount))

	// Show consumer stats if available
	if concurrentConsumer, ok := consumer.(*rabbitmq.ConcurrentConsumer); ok {
		stats := concurrentConsumer.GetStats()
		fmt.Printf("  Messages Failed: %d\n", stats.MessagesFailed)
		fmt.Printf("  Active Workers: %d\n", stats.ActiveWorkers)
	}

	return nil
}

func (cli *ConsumerCLI) reportStats(consumer messaging.Consumer) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if concurrentConsumer, ok := consumer.(*rabbitmq.ConcurrentConsumer); ok {
			stats := concurrentConsumer.GetStats()
			fmt.Printf("📊 Stats: Processed=%d, Failed=%d, Requeued=%d, DLQ=%d, Active=%d, Queued=%d\n",
				stats.MessagesProcessed, stats.MessagesFailed, stats.MessagesRequeued,
				stats.MessagesSentToDLQ, stats.ActiveWorkers, stats.QueuedTasks)
		} else {
			fmt.Printf("📊 Stats: Processed=%d\n", atomic.LoadInt64(&cli.processedCount))
		}
	}
}

func main() {
	cli := &ConsumerCLI{}
	cli.parseFlags()

	// Initialize logging at the start
	if err := logx.InitDefault(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logx.Sync()

	if cli.interactive {
		if err := cli.runInteractive(); err != nil {
			logx.Fatal("Interactive mode failed",
				logx.String("error", err.Error()),
			)
		}
	} else {
		if err := cli.runBatch(); err != nil {
			logx.Fatal("Batch mode failed",
				logx.String("error", err.Error()),
			)
		}
	}
}
