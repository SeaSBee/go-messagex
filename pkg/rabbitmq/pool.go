// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/pkg/messaging"
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
	}

	// Start health monitoring
	pool.startHealthMonitoring()

	// Start lifecycle event processing
	go pool.processLifecycleEvents()

	return pool
}

// GetConnection returns a connection from the pool with auto-recovery.
func (cp *ConnectionPool) GetConnection(ctx context.Context, uri string) (*amqp091.Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "get_connection", "pool is closed")
	}

	// Check if we have available healthy connections
	for conn, info := range cp.connections {
		if info.State == ConnectionStateConnected && !conn.IsClosed() {
			info.LastUsed = time.Now()
			cp.logger.Debug("reusing existing connection", logx.String("uri", uri))
			return conn, nil
		}
	}

	// Create new connection if under limit
	if len(cp.connections) < cp.config.Max {
		conn, err := cp.createConnectionWithRetry(ctx, uri)
		if err != nil {
			return nil, err
		}

		info := &ConnectionInfo{
			Connection: conn,
			State:      ConnectionStateConnected,
			CreatedAt:  time.Now(),
			LastUsed:   time.Now(),
			CloseChan:  make(chan *amqp091.Error, 1),
		}

		cp.connections[conn] = info

		// Set up connection close notification
		go cp.monitorConnectionClose(conn, info)

		cp.logger.Info("created new connection",
			logx.String("uri", uri),
			logx.Int("pool_size", len(cp.connections)))

		cp.recordConnectionMetrics("created", uri)
		return conn, nil
	}

	// Wait for a connection to become available with timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(cp.config.ConnectionTimeout):
		return nil, messaging.NewError(messaging.ErrorCodeConnection, "get_connection", "timeout waiting for available connection")
	}
}

// createConnectionWithRetry creates a connection with exponential backoff and jitter
func (cp *ConnectionPool) createConnectionWithRetry(ctx context.Context, uri string) (*amqp091.Connection, error) {
	cp.recoveryMu.Lock()
	defer cp.recoveryMu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= cp.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Add jitter to backoff delay
		jitter := time.Duration(rand.Int63n(int64(cp.recoveryDelay / 2)))
		delay := cp.recoveryDelay + jitter

		if attempt > 0 {
			cp.logger.Info("retrying connection creation",
				logx.String("uri", uri),
				logx.Int("attempt", attempt),
				logx.String("delay", delay.String()))

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create connection with timeout
		conn, err := amqp091.DialConfig(uri, amqp091.Config{
			Heartbeat: cp.config.HeartbeatInterval,
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

	if err != nil {
		cp.logger.Warn("connection closed",
			logx.String("error", err.Error()),
			logx.Int("error_count", info.ErrorCount))
	} else {
		cp.logger.Warn("connection closed",
			logx.Int("error_count", info.ErrorCount))
	}

	cp.recordConnectionMetrics("closed", "")

	// Emit lifecycle event
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

	// Attempt auto-recovery if not at max retries
	if info.ErrorCount < cp.maxRetries {
		go cp.attemptRecovery(conn, info)
	} else {
		// Remove failed connection
		delete(cp.connections, conn)
		cp.logger.Error("removing failed connection after max retries",
			logx.Int("error_count", info.ErrorCount))
	}
}

// attemptRecovery attempts to recover a failed connection
func (cp *ConnectionPool) attemptRecovery(conn *amqp091.Connection, info *ConnectionInfo) {
	cp.mu.Lock()
	info.State = ConnectionStateConnecting
	cp.mu.Unlock()

	cp.logger.Info("attempting connection recovery",
		logx.Int("error_count", info.ErrorCount))

	// Wait before attempting recovery
	time.Sleep(cp.recoveryDelay)

	// Try to create new connection
	newConn, err := cp.createConnectionWithRetry(context.Background(), "")
	if err != nil {
		cp.logger.Error("connection recovery failed", logx.String("error", err.Error()))
		return
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Replace old connection with new one
	delete(cp.connections, conn)

	newInfo := &ConnectionInfo{
		Connection: newConn,
		State:      ConnectionStateConnected,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		ErrorCount: 0,
		CloseChan:  make(chan *amqp091.Error, 1),
	}

	cp.connections[newConn] = newInfo

	// Set up monitoring for new connection
	go cp.monitorConnectionClose(newConn, newInfo)

	cp.logger.Info("connection recovery successful")
	cp.recordConnectionMetrics("recovered", "")
}

// startHealthMonitoring starts periodic health checks
func (cp *ConnectionPool) startHealthMonitoring() {
	cp.healthTicker = time.NewTicker(cp.config.HealthCheckInterval)

	go func() {
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

	for conn, info := range connections {
		if conn.IsClosed() {
			cp.handleConnectionClose(conn, info, nil)
			continue
		}

		// Check connection age and usage
		age := time.Since(info.CreatedAt)
		idleTime := time.Since(info.LastUsed)

		// Log health metrics
		cp.logger.Debug("connection health check",
			logx.String("age", age.String()),
			logx.String("idle_time", idleTime.String()),
			logx.Int("error_count", info.ErrorCount))

		// Record health metrics
		cp.recordConnectionHealthMetrics(conn, info)
	}
}

// processLifecycleEvents processes connection lifecycle events
func (cp *ConnectionPool) processLifecycleEvents() {
	for {
		select {
		case event := <-cp.lifecycleChan:
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
		case <-cp.healthCtx.Done():
			return
		}
	}
}

// recordConnectionMetrics records connection-related metrics
func (cp *ConnectionPool) recordConnectionMetrics(action, uri string) {
	if cp.metrics != nil {
		// Record connection pool metrics
		cp.metrics.ConnectionsActive("rabbitmq", len(cp.connections))
	}
}

// recordConnectionHealthMetrics records connection health metrics
func (cp *ConnectionPool) recordConnectionHealthMetrics(conn *amqp091.Connection, info *ConnectionInfo) {
	if cp.metrics != nil {
		// Record health metrics
		cp.metrics.ConnectionsActive("rabbitmq", len(cp.connections))
	}
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := map[string]interface{}{
		"total_connections": len(cp.connections),
		"max_connections":   cp.config.Max,
		"min_connections":   cp.config.Min,
		"closed":            cp.closed,
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
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil
	}

	cp.closed = true

	// Stop health monitoring
	if cp.healthTicker != nil {
		cp.healthTicker.Stop()
	}
	cp.healthCancel()

	// Close lifecycle channel
	close(cp.lifecycleChan)

	var errs []error
	for conn, info := range cp.connections {
		info.State = ConnectionStateClosed
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return messaging.WrapError(messaging.ErrorCodeConnection, "close_pool", "failed to close some connections", errs[0])
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
}

// NewChannelPool creates a new channel pool.
func NewChannelPool(config *messaging.ChannelPoolConfig, conn *amqp091.Connection) *ChannelPool {
	return &ChannelPool{
		config:   config,
		channels: make(chan *amqp091.Channel, config.PerConnectionMax),
		conn:     conn,
	}
}

// Initialize initializes the channel pool.
func (cp *ChannelPool) Initialize() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Create initial channels
	for i := 0; i < cp.config.PerConnectionMin; i++ {
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
	select {
	case channel := <-cp.channels:
		if channel.IsClosed() {
			// Create new channel if borrowed one is closed
			return cp.createChannel()
		}
		return channel, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(cp.config.BorrowTimeout):
		return nil, messaging.NewError(messaging.ErrorCodeChannel, "borrow_channel", "timeout waiting for channel")
	}
}

// Return returns a channel to the pool.
func (cp *ChannelPool) Return(channel *amqp091.Channel) {
	if channel == nil || channel.IsClosed() {
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

	// Close all channels
	for channel := range cp.channels {
		channel.Close()
	}

	return nil
}

// PooledTransport extends Transport with connection and channel pooling.
type PooledTransport struct {
	*Transport
	connPool  *ConnectionPool
	chanPools map[*amqp091.Connection]*ChannelPool
	mu        sync.RWMutex
}

// NewPooledTransport creates a new pooled transport.
func NewPooledTransport(config *messaging.RabbitMQConfig, observability *messaging.ObservabilityContext) *PooledTransport {
	return &PooledTransport{
		Transport: NewTransport(config, observability),
		connPool:  NewConnectionPool(config.ConnectionPool, observability.Logger(), observability.Metrics()),
		chanPools: make(map[*amqp091.Connection]*ChannelPool),
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
	return chanPool.Borrow(ctx)
}

// ReturnChannel returns a channel to the pool.
func (pt *PooledTransport) ReturnChannel(channel *amqp091.Channel) {
	// Find the channel pool for this channel's connection
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	for _, chanPool := range pt.chanPools {
		chanPool.Return(channel)
		break // Return to first pool for now
	}
}

// Close closes the pooled transport.
func (pt *PooledTransport) Close(ctx context.Context) error {
	// Close channel pools
	pt.mu.Lock()
	for _, chanPool := range pt.chanPools {
		chanPool.Close()
	}
	pt.mu.Unlock()

	// Close connection pool
	if err := pt.connPool.Close(); err != nil {
		return err
	}

	// Close base transport
	return pt.Transport.Disconnect(ctx)
}
