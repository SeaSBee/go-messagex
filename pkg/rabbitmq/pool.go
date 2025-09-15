// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"fmt" // Added for fmt.Sprintf
	"math/rand"
	"sync"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/rabbitmq/amqp091-go"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// ConnectionState represents the state of a connection
type ConnectionState int

const (
	ConnectionStateUnknown ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateDisconnected
	ConnectionStateFailed
	ConnectionStateClosed
)

// ConnectionInfo holds information about a connection
type ConnectionInfo struct {
	Connection *amqp091.Connection
	State      ConnectionState
	CreatedAt  time.Time
	LastUsed   time.Time
	ErrorCount int
	CloseChan  chan *amqp091.Error
	URI        string // Store URI for recovery

	// Connection reuse optimization fields
	UseCount    int64         // Number of times this connection has been used
	IdleTime    time.Duration // Current idle time
	HealthScore float64       // Health score based on error count and age
	IsIdle      bool          // Whether connection is currently idle
}

// ConnectionPool manages a pool of RabbitMQ connections with health monitoring and auto-recovery.
type ConnectionPool struct {
	config      *messaging.ConnectionPoolConfig
	connections map[*amqp091.Connection]*ConnectionInfo
	mu          sync.RWMutex
	closed      bool

	// Health monitoring
	healthTicker *time.Ticker
	healthCtx    context.Context
	healthCancel context.CancelFunc

	// Auto-recovery
	recoveryMu    sync.Mutex
	recoveryDelay time.Duration
	maxRetries    int

	// Metrics and logging
	logger  *logx.Logger
	metrics messaging.Metrics

	// Lifecycle management
	lifecycleChan chan ConnectionLifecycleEvent

	// Connection reuse optimization
	lastUsedIndex int           // For round-robin selection
	idleThreshold time.Duration // Threshold for considering connection idle
	maxIdleTime   time.Duration // Maximum idle time before closing
}

// ConnectionLifecycleEvent represents a connection lifecycle event
type ConnectionLifecycleEvent struct {
	Connection *amqp091.Connection
	Event      string
	State      ConnectionState
	Error      *amqp091.Error
	Timestamp  time.Time
}

// NewConnectionPool creates a new connection pool with health monitoring.
func NewConnectionPool(config *messaging.ConnectionPoolConfig, logger *logx.Logger, metrics messaging.Metrics) *ConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())

	// Set default idle thresholds
	idleThreshold := 5 * time.Minute
	maxIdleTime := 30 * time.Minute
	if config != nil {
		// Use health check interval as idle threshold
		idleThreshold = config.HealthCheckInterval * 2
		maxIdleTime = config.HealthCheckInterval * 10
	}

	pool := &ConnectionPool{
		config:        config,
		connections:   make(map[*amqp091.Connection]*ConnectionInfo),
		recoveryDelay: time.Second,
		maxRetries:    5,
		logger:        logger,
		metrics:       metrics,
		lifecycleChan: make(chan ConnectionLifecycleEvent, 100),
		healthCtx:     ctx,
		healthCancel:  cancel,
		idleThreshold: idleThreshold,
		maxIdleTime:   maxIdleTime,
	}

	// Start health monitoring
	pool.startHealthMonitoring()

	// Start lifecycle event processing
	go pool.processLifecycleEvents()

	// Warm up the connection pool with minimum connections
	go pool.warmupConnections()

	return pool
}

// calculateHealthScore calculates a health score for a connection based on error count and age
func (cp *ConnectionPool) calculateHealthScore(info *ConnectionInfo) float64 {
	if info == nil {
		return 0.0
	}

	// Base score starts at 1.0
	score := 1.0

	// Penalize based on error count
	errorPenalty := float64(info.ErrorCount) * 0.2
	score -= errorPenalty

	// Penalize based on age (older connections get slightly lower scores)
	age := time.Since(info.CreatedAt)
	agePenalty := float64(age.Hours()) * 0.01 // Small penalty for age
	score -= agePenalty

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// warmupConnections pre-creates minimum connections for the pool
func (cp *ConnectionPool) warmupConnections() {
	// Wait a short time to ensure pool is fully initialized
	time.Sleep(100 * time.Millisecond)

	minConnections := 2 // Default min connections
	if cp.config != nil {
		minConnections = cp.config.Min
	}

	// Get a default URI for warmup (will be overridden when actual connections are needed)
	defaultURI := "amqp://localhost:5672"

	if cp.logger != nil {
		cp.logger.Info("warming up connection pool", logx.Int("min_connections", minConnections))
	}

	// Create minimum connections in background
	for i := 0; i < minConnections; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, err := cp.createConnectionWithRetry(ctx, defaultURI)
			if err != nil {
				if cp.logger != nil {
					cp.logger.Warn("failed to warm up connection", logx.String("error", err.Error()))
				}
				return
			}

			cp.mu.Lock()
			if !cp.closed && len(cp.connections) < minConnections {
				info := &ConnectionInfo{
					Connection:  conn,
					State:       ConnectionStateConnected,
					CreatedAt:   time.Now(),
					LastUsed:    time.Now(),
					CloseChan:   make(chan *amqp091.Error, 1),
					URI:         defaultURI,
					UseCount:    0,
					HealthScore: 1.0,
				}

				cp.connections[conn] = info
				go cp.monitorConnectionClose(conn, info)

				if cp.logger != nil {
					cp.logger.Debug("warmed up connection", logx.Int("pool_size", len(cp.connections)))
				}
			} else {
				// Pool is closed or we have enough connections, close this one
				conn.Close()
			}
			cp.mu.Unlock()
		}()
	}
}

// GetConnection returns a connection from the pool with auto-recovery.
func (cp *ConnectionPool) GetConnection(ctx context.Context, uri string) (*amqp091.Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "get_connection", "pool is closed")
	}

	// Check if we have available healthy connections
	// Use optimized connection selection: least used, then round-robin
	var bestConn *amqp091.Connection
	var bestInfo *ConnectionInfo
	var minUseCount int64 = 1<<63 - 1 // Max int64

	for conn, info := range cp.connections {
		if info.State == ConnectionStateConnected && !conn.IsClosed() {
			// Update idle time
			info.IdleTime = time.Since(info.LastUsed)

			// Select connection with least usage (load balancing)
			if info.UseCount < minUseCount {
				minUseCount = info.UseCount
				bestConn = conn
				bestInfo = info
			}
		}
	}

	if bestConn != nil {
		// Update connection usage
		bestInfo.LastUsed = time.Now()
		bestInfo.UseCount++
		bestInfo.IdleTime = 0
		bestInfo.IsIdle = false

		// Update health score
		bestInfo.HealthScore = cp.calculateHealthScore(bestInfo)

		if cp.logger != nil {
			cp.logger.Debug("reusing existing connection",
				logx.String("uri", uri),
				logx.Int64("use_count", bestInfo.UseCount),
				logx.String("idle_time", bestInfo.IdleTime.String()))
		}
		return bestConn, nil
	}

	// Create new connection if under limit
	maxConnections := 8 // Default max connections
	if cp.config != nil {
		maxConnections = cp.config.Max
	}
	if len(cp.connections) < maxConnections {
		conn, err := cp.createConnectionWithRetry(ctx, uri)
		if err != nil {
			return nil, err
		}

		info := &ConnectionInfo{
			Connection:  conn,
			State:       ConnectionStateConnected,
			CreatedAt:   time.Now(),
			LastUsed:    time.Now(),
			CloseChan:   make(chan *amqp091.Error, 1),
			URI:         uri, // Store the URI
			UseCount:    1,   // First use
			HealthScore: 1.0,
		}

		cp.connections[conn] = info

		// Set up connection close notification
		go cp.monitorConnectionClose(conn, info)

		if cp.logger != nil {
			cp.logger.Info("created new connection",
				logx.String("uri", uri),
				logx.Int("pool_size", len(cp.connections)))
		}

		cp.recordConnectionMetrics("created", uri)
		return conn, nil
	}

	// Wait for a connection to become available with timeout
	connectionTimeout := 10 * time.Second // Default timeout
	if cp.config != nil {
		connectionTimeout = cp.config.ConnectionTimeout
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(connectionTimeout):
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "get_connection", "timeout waiting for available connection")
	}
}

// createConnectionWithRetry creates a connection with exponential backoff and jitter
func (cp *ConnectionPool) createConnectionWithRetry(ctx context.Context, uri string) (*amqp091.Connection, error) {
	cp.recoveryMu.Lock()
	defer cp.recoveryMu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= cp.maxRetries; attempt++ {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}

		// Add jitter to backoff delay
		jitter := time.Duration(rand.Int63n(int64(cp.recoveryDelay / 2)))
		delay := cp.recoveryDelay + jitter

		if attempt > 0 && cp.logger != nil {
			cp.logger.Info("retrying connection creation",
				logx.String("uri", uri),
				logx.Int("attempt", attempt),
				logx.String("delay", delay.String()))

			if ctx != nil {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			} else {
				time.Sleep(delay)
			}
		}

		// Create connection with timeout
		heartbeatInterval := 10 * time.Second // Default heartbeat interval
		if cp.config != nil {
			heartbeatInterval = cp.config.HeartbeatInterval
		}
		conn, err := amqp091.DialConfig(uri, amqp091.Config{
			Heartbeat: heartbeatInterval,
		})

		if err == nil {
			cp.recoveryDelay = time.Second // Reset delay on success
			cp.recordConnectionMetrics("connected", uri)
			return conn, nil
		}

		lastErr = err
		cp.recordConnectionMetrics("failed", uri)

		// Exponential backoff
		cp.recoveryDelay = time.Duration(float64(cp.recoveryDelay) * 1.5)
		if cp.recoveryDelay > 30*time.Second {
			cp.recoveryDelay = 30 * time.Second
		}
	}

	return nil, messaging.WrapError(messaging.ErrorCodeConnection, "create_connection", "failed to create connection after retries", lastErr)
}

// monitorConnectionClose monitors connection close events
func (cp *ConnectionPool) monitorConnectionClose(conn *amqp091.Connection, info *ConnectionInfo) {
	defer func() {
		// Clean up the notification channel
		if info.CloseChan != nil {
			close(info.CloseChan)
		}
	}()

	closeChan := conn.NotifyClose(make(chan *amqp091.Error, 1))

	select {
	case err := <-closeChan:
		cp.handleConnectionClose(conn, info, err)
	case <-cp.healthCtx.Done():
		return
	}
}

// handleConnectionClose handles connection close events
func (cp *ConnectionPool) handleConnectionClose(conn *amqp091.Connection, info *ConnectionInfo, err *amqp091.Error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	info.State = ConnectionStateDisconnected
	if err != nil {
		info.State = ConnectionStateFailed
		info.ErrorCount++
	}

	if cp.logger != nil {
		if err != nil {
			cp.logger.Warn("connection closed",
				logx.String("error", err.Error()),
				logx.Int("error_count", info.ErrorCount))
		} else {
			cp.logger.Warn("connection closed",
				logx.Int("error_count", info.ErrorCount))
		}
	}

	cp.recordConnectionMetrics("closed", "")

	// Emit lifecycle event only if pool is not closed
	// Use a non-blocking check to avoid deadlock
	cp.mu.RLock()
	poolClosed := cp.closed
	cp.mu.RUnlock()

	if !poolClosed {
		select {
		case cp.lifecycleChan <- ConnectionLifecycleEvent{
			Connection: conn,
			Event:      "closed",
			State:      info.State,
			Error:      err,
			Timestamp:  time.Now(),
		}:
		default:
			// Channel full, skip event
		}
	}

	// Attempt auto-recovery if not at max retries
	if info.ErrorCount < cp.maxRetries {
		go cp.attemptRecovery(conn, info)
	} else {
		// Remove failed connection
		delete(cp.connections, conn)
		if cp.logger != nil {
			cp.logger.Error("removing failed connection after max retries",
				logx.Int("error_count", info.ErrorCount))
		}
	}
}

// attemptRecovery attempts to recover a failed connection
func (cp *ConnectionPool) attemptRecovery(conn *amqp091.Connection, info *ConnectionInfo) {
	cp.mu.Lock()
	info.State = ConnectionStateConnecting
	cp.mu.Unlock()

	if cp.logger != nil {
		cp.logger.Info("attempting connection recovery",
			logx.Int("error_count", info.ErrorCount))
	}

	// Wait before attempting recovery
	time.Sleep(cp.recoveryDelay)

	// Create a timeout context for recovery
	recoveryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to create new connection
	newConn, err := cp.createConnectionWithRetry(recoveryCtx, info.URI)
	if err != nil {
		if cp.logger != nil {
			cp.logger.Error("connection recovery failed", logx.String("error", err.Error()))
		}
		return
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Replace old connection with new one
	delete(cp.connections, conn)

	newInfo := &ConnectionInfo{
		Connection:  newConn,
		State:       ConnectionStateConnected,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		ErrorCount:  0,
		CloseChan:   make(chan *amqp091.Error, 1),
		URI:         info.URI, // Use the stored URI for recovery
		UseCount:    0,        // Reset use count for recovered connection
		HealthScore: 1.0,      // Reset health score
	}

	cp.connections[newConn] = newInfo

	// Set up monitoring for new connection
	go cp.monitorConnectionClose(newConn, newInfo)

	if cp.logger != nil {
		cp.logger.Info("connection recovery successful")
	}
	cp.recordConnectionMetrics("recovered", "")
}

// startHealthMonitoring starts periodic health checks
func (cp *ConnectionPool) startHealthMonitoring() {
	// Use default health check interval if config is nil
	healthCheckInterval := 30 * time.Second
	if cp.config != nil {
		healthCheckInterval = cp.config.HealthCheckInterval
	}

	cp.healthTicker = time.NewTicker(healthCheckInterval)

	go func() {
		defer cp.healthTicker.Stop()
		for {
			select {
			case <-cp.healthTicker.C:
				cp.performHealthCheck()
			case <-cp.healthCtx.Done():
				return
			}
		}
	}()
}

// performHealthCheck performs health checks on all connections
func (cp *ConnectionPool) performHealthCheck() {
	cp.mu.RLock()
	connections := make(map[*amqp091.Connection]*ConnectionInfo)
	for conn, info := range cp.connections {
		connections[conn] = info
	}
	cp.mu.RUnlock()

	// Collect connections that need to be closed
	var connectionsToClose []*amqp091.Connection
	var idleConnections []*amqp091.Connection

	for conn, info := range connections {
		if conn.IsClosed() {
			connectionsToClose = append(connectionsToClose, conn)
			continue
		}

		// Update idle time and health score
		info.IdleTime = time.Since(info.LastUsed)
		info.HealthScore = cp.calculateHealthScore(info)

		// Mark as idle if threshold exceeded
		if info.IdleTime > cp.idleThreshold {
			info.IsIdle = true
			idleConnections = append(idleConnections, conn)
		} else {
			info.IsIdle = false
		}

		// Check if connection should be closed due to excessive idle time
		if info.IdleTime > cp.maxIdleTime {
			connectionsToClose = append(connectionsToClose, conn)
			if cp.logger != nil {
				cp.logger.Info("closing idle connection",
					logx.String("idle_time", info.IdleTime.String()),
					logx.String("max_idle_time", cp.maxIdleTime.String()))
			}
			continue
		}

		// Log health metrics
		if cp.logger != nil {
			cp.logger.Debug("connection health check",
				logx.String("age", time.Since(info.CreatedAt).String()),
				logx.String("idle_time", info.IdleTime.String()),
				logx.Int("error_count", info.ErrorCount),
				logx.Float64("health_score", info.HealthScore),
				logx.Int64("use_count", info.UseCount))
		}

		// Record health metrics
		cp.recordConnectionHealthMetrics(conn, info)
	}

	// Handle connections that need to be closed (outside of read lock)
	for _, conn := range connectionsToClose {
		if info, exists := connections[conn]; exists {
			cp.handleConnectionClose(conn, info, nil)
		}
	}

	// Log idle connection count
	if len(idleConnections) > 0 && cp.logger != nil {
		cp.logger.Debug("idle connections detected",
			logx.Int("idle_count", len(idleConnections)),
			logx.Int("total_connections", len(connections)))
	}
}

// processLifecycleEvents processes connection lifecycle events
func (cp *ConnectionPool) processLifecycleEvents() {
	for {
		select {
		case event, ok := <-cp.lifecycleChan:
			if !ok {
				// Channel closed, exit
				return
			}
			if cp.logger != nil {
				if event.Error != nil {
					cp.logger.Info("connection lifecycle event",
						logx.String("event", event.Event),
						logx.String("state", event.State.String()),
						logx.String("error", event.Error.Error()))
				} else {
					cp.logger.Info("connection lifecycle event",
						logx.String("event", event.Event),
						logx.String("state", event.State.String()))
				}
			}
		case <-cp.healthCtx.Done():
			return
		}
	}
}

// recordConnectionMetrics records connection-related metrics
func (cp *ConnectionPool) recordConnectionMetrics(action, uri string) {
	if cp.metrics != nil && cp.connections != nil {
		// Record connection pool metrics
		cp.metrics.ConnectionsActive("rabbitmq", len(cp.connections))
	}
}

// recordConnectionHealthMetrics records connection health metrics
func (cp *ConnectionPool) recordConnectionHealthMetrics(conn *amqp091.Connection, info *ConnectionInfo) {
	if cp.metrics != nil && cp.connections != nil {
		// Record health metrics
		cp.metrics.ConnectionsActive("rabbitmq", len(cp.connections))
	}
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	maxConnections := 8 // Default max connections
	minConnections := 2 // Default min connections
	if cp.config != nil {
		maxConnections = cp.config.Max
		minConnections = cp.config.Min
	}

	// Calculate enhanced statistics
	var totalUseCount int64
	var totalIdleTime time.Duration
	var idleConnections int
	var avgHealthScore float64
	var maxUseCount int64
	var minUseCount int64 = 1<<63 - 1

	for _, info := range cp.connections {
		totalUseCount += info.UseCount
		totalIdleTime += info.IdleTime

		if info.IsIdle {
			idleConnections++
		}

		if info.UseCount > maxUseCount {
			maxUseCount = info.UseCount
		}
		if info.UseCount < minUseCount {
			minUseCount = info.UseCount
		}

		avgHealthScore += info.HealthScore
	}

	connectionCount := len(cp.connections)
	if connectionCount > 0 {
		avgHealthScore /= float64(connectionCount)
		if minUseCount == 1<<63-1 {
			minUseCount = 0
		}
	} else {
		// Handle empty pool case
		avgHealthScore = 0
		minUseCount = 0
	}

	stats := map[string]interface{}{
		"total_connections":  connectionCount,
		"max_connections":    maxConnections,
		"min_connections":    minConnections,
		"closed":             cp.closed,
		"idle_connections":   idleConnections,
		"active_connections": connectionCount - idleConnections,
		"total_use_count":    totalUseCount,
		"avg_use_count": func() float64 {
			if connectionCount > 0 {
				return float64(totalUseCount) / float64(connectionCount)
			}
			return 0
		}(),
		"max_use_count": maxUseCount,
		"min_use_count": minUseCount,
		"avg_idle_time": func() time.Duration {
			if connectionCount > 0 {
				return totalIdleTime / time.Duration(connectionCount)
			}
			return 0
		}(),
		"avg_health_score": avgHealthScore,
		"idle_threshold":   cp.idleThreshold,
		"max_idle_time":    cp.maxIdleTime,
	}

	// Count connections by state
	stateCounts := make(map[ConnectionState]int)
	for _, info := range cp.connections {
		stateCounts[info.State]++
	}

	stats["state_counts"] = stateCounts
	return stats
}

// Close closes all connections in the pool.
func (cp *ConnectionPool) Close() error {
	// Try to acquire lock with timeout to avoid deadlock
	done := make(chan struct{})

	go func() {
		cp.mu.Lock()
		close(done)
	}()

	select {
	case <-done:
		// Lock acquired successfully
	case <-time.After(5 * time.Second):
		// Timeout - return error
		return messaging.NewError(messaging.ErrorCodeConnection, "close_pool", "timeout acquiring lock for pool close")
	}

	if cp.closed {
		cp.mu.Unlock()
		return nil
	}

	cp.closed = true

	// Stop health monitoring
	if cp.healthTicker != nil {
		cp.healthTicker.Stop()
	}
	cp.healthCancel()

	// Close lifecycle channel to signal goroutines to stop
	close(cp.lifecycleChan)

	// Close all connections
	var errs []error
	for conn, info := range cp.connections {
		info.State = ConnectionStateClosed
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	cp.mu.Unlock()

	// Wait a short time for goroutines to finish
	// This prevents the test from hanging indefinitely
	time.Sleep(200 * time.Millisecond)

	if len(errs) > 0 {
		// Create a comprehensive error message
		errMsg := "failed to close some connections"
		if len(errs) == 1 {
			return messaging.WrapError(messaging.ErrorCodeConnection, "close_pool", errMsg, errs[0])
		}
		// For multiple errors, wrap the first one but include count
		return messaging.WrapError(messaging.ErrorCodeConnection, "close_pool",
			errMsg+fmt.Sprintf(" (%d errors)", len(errs)), errs[0])
	}

	return nil
}

// String returns string representation of ConnectionState
func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateUnknown:
		return "unknown"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateFailed:
		return "failed"
	case ConnectionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ChannelPool manages a pool of AMQP channels.
type ChannelPool struct {
	config   *messaging.ChannelPoolConfig
	channels chan *amqp091.Channel
	conn     *amqp091.Connection
	mu       sync.RWMutex
	closed   bool
	// Track all created channels for proper cleanup
	allChannels map[*amqp091.Channel]bool
}

// NewChannelPool creates a new channel pool.
func NewChannelPool(config *messaging.ChannelPoolConfig, conn *amqp091.Connection) *ChannelPool {
	return &ChannelPool{
		config:      config,
		channels:    make(chan *amqp091.Channel, config.PerConnectionMax),
		conn:        conn,
		allChannels: make(map[*amqp091.Channel]bool),
	}
}

// Initialize initializes the channel pool.
func (cp *ChannelPool) Initialize() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Create initial channels
	perConnectionMin := 10 // Default min channels per connection
	if cp.config != nil {
		perConnectionMin = cp.config.PerConnectionMin
	}
	for i := 0; i < perConnectionMin; i++ {
		channel, err := cp.createChannel()
		if err != nil {
			return err
		}
		cp.channels <- channel
	}

	return nil
}

// Borrow borrows a channel from the pool.
func (cp *ChannelPool) Borrow(ctx context.Context) (*amqp091.Channel, error) {
	borrowTimeout := 5 * time.Second // Default borrow timeout
	if cp.config != nil {
		borrowTimeout = cp.config.BorrowTimeout
	}

	select {
	case channel := <-cp.channels:
		if channel.IsClosed() {
			// Create new channel if borrowed one is closed
			cp.mu.Lock()
			newChannel, err := cp.createChannel()
			cp.mu.Unlock()
			return newChannel, err
		}
		return channel, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(borrowTimeout):
		return nil, messaging.NewError(messaging.ErrorCodeChannel, "borrow_channel", "timeout waiting for channel")
	}
}

// Return returns a channel to the pool.
func (cp *ChannelPool) Return(channel *amqp091.Channel) {
	if channel == nil || channel.IsClosed() || cp.closed || cp.channels == nil {
		return
	}

	select {
	case cp.channels <- channel:
		// Successfully returned
	default:
		// Pool is full, close the channel
		channel.Close()
	}
}

// createChannel creates a new AMQP channel.
func (cp *ChannelPool) createChannel() (*amqp091.Channel, error) {
	channel, err := cp.conn.Channel()
	if err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeChannel, "create_channel", "failed to create channel", err)
	}

	// Configure channel
	if err := cp.configureChannel(channel); err != nil {
		channel.Close()
		return nil, err
	}

	// Track the channel with proper synchronization
	cp.mu.Lock()
	cp.allChannels[channel] = true
	cp.mu.Unlock()

	return channel, nil
}

// configureChannel configures a channel with QoS settings.
func (cp *ChannelPool) configureChannel(channel *amqp091.Channel) error {
	// Set QoS
	if err := channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		return messaging.WrapError(messaging.ErrorCodeChannel, "configure_channel", "failed to set QoS", err)
	}

	return nil
}

// Close closes the channel pool.
func (cp *ChannelPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.closed = true
	close(cp.channels)

	// Close all channels in the pool
	for channel := range cp.channels {
		channel.Close()
	}

	// Close all tracked channels
	for channel := range cp.allChannels {
		channel.Close()
	}

	return nil
}

// PooledTransport extends Transport with connection and channel pooling.
type PooledTransport struct {
	*Transport
	connPool  *ConnectionPool
	chanPools map[*amqp091.Connection]*ChannelPool
	// Track which connection each channel belongs to
	channelToConn map[*amqp091.Channel]*amqp091.Connection
	mu            sync.RWMutex
}

// NewPooledTransport creates a new pooled transport.
func NewPooledTransport(config *messaging.RabbitMQConfig, observability *messaging.ObservabilityContext) *PooledTransport {
	return &PooledTransport{
		Transport:     NewTransport(config, observability),
		connPool:      NewConnectionPool(config.ConnectionPool, observability.Logger(), observability.Metrics()),
		chanPools:     make(map[*amqp091.Connection]*ChannelPool),
		channelToConn: make(map[*amqp091.Channel]*amqp091.Connection),
	}
}

// GetChannel returns a channel from the pool.
func (pt *PooledTransport) GetChannel(ctx context.Context) (*amqp091.Channel, error) {
	// Get connection from pool
	uri, err := pt.selectURI()
	if err != nil {
		return nil, err
	}

	conn, err := pt.connPool.GetConnection(ctx, uri)
	if err != nil {
		return nil, err
	}

	// Get or create channel pool for this connection
	pt.mu.Lock()
	chanPool, exists := pt.chanPools[conn]
	if !exists {
		chanPool = NewChannelPool(pt.config.ChannelPool, conn)
		if err := chanPool.Initialize(); err != nil {
			pt.mu.Unlock()
			return nil, err
		}
		pt.chanPools[conn] = chanPool
	}
	pt.mu.Unlock()

	// Borrow channel from pool
	channel, err := chanPool.Borrow(ctx)
	if err != nil {
		return nil, err
	}

	// Track the channel's connection
	pt.mu.Lock()
	pt.channelToConn[channel] = conn
	pt.mu.Unlock()

	return channel, nil
}

// ReturnChannel returns a channel to the pool.
func (pt *PooledTransport) ReturnChannel(channel *amqp091.Channel) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	conn, ok := pt.channelToConn[channel]
	if !ok {
		// Channel not found in tracking map, skip return
		return
	}

	// Find the channel pool for this channel's connection
	chanPool, exists := pt.chanPools[conn]
	if !exists {
		// Channel pool not found, skip return
		return
	}

	chanPool.Return(channel)

	// Remove the channel from the map after returning
	delete(pt.channelToConn, channel)
}

// Close closes the pooled transport.
func (pt *PooledTransport) Close(ctx context.Context) error {
	// Close channel pools
	pt.mu.Lock()
	for _, chanPool := range pt.chanPools {
		chanPool.Close()
	}
	// Clear the channel tracking map
	pt.channelToConn = make(map[*amqp091.Channel]*amqp091.Connection)
	pt.mu.Unlock()

	// Close connection pool
	if err := pt.connPool.Close(); err != nil {
		return err
	}

	// Close base transport
	return pt.Transport.Disconnect(ctx)
}
