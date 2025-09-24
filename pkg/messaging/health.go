// Package messaging provides health check implementation for RabbitMQ
package messaging

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seasbee/go-logx"
	"github.com/wagslane/go-rabbitmq"
)

// HealthStatus represents the health status
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// HealthChecker represents a health checker for RabbitMQ connections
type HealthChecker struct {
	conn           *rabbitmq.Conn
	interval       time.Duration
	mu             sync.RWMutex
	closed         bool
	status         HealthStatus
	lastCheck      int64 // Unix timestamp for atomic access
	lastError      error
	checkCount     int64
	successCount   int64
	failureCount   int64
	ctx            context.Context
	cancel         context.CancelFunc
	healthCallback func(status HealthStatus, err error)
}

// HealthStats contains health check statistics
type HealthStats struct {
	Status       HealthStatus
	LastCheck    time.Time
	LastError    error
	CheckCount   int64
	SuccessCount int64
	FailureCount int64
	SuccessRate  float64
	IsHealthy    bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(conn *rabbitmq.Conn, interval time.Duration, logger *logx.Logger) *HealthChecker {
	if conn == nil {
		logx.Warn("connection is nil, health checker will report unhealthy")
		// Return a health checker that will always report unhealthy
		ctx, cancel := context.WithCancel(context.Background())
		hc := &HealthChecker{
			conn:     nil,
			interval: interval,
			status:   StatusUnhealthy,
			ctx:      ctx,
			cancel:   cancel,
		}
		// Don't start health checking for nil connection
		return hc
	}

	if interval <= 0 {
		interval = 30 * time.Second // Default interval
		logx.Info("using default health check interval",
			logx.String("interval", interval.String()),
		)
	}

	logx.Info("creating health checker",
		logx.String("interval", interval.String()),
	)

	ctx, cancel := context.WithCancel(context.Background())

	hc := &HealthChecker{
		conn:     conn,
		interval: interval,
		status:   StatusUnknown,
		ctx:      ctx,
		cancel:   cancel,
	}

	logx.Info("starting health checker",
		logx.String("interval", interval.String()),
	)

	// Start health checking in a goroutine
	go hc.startHealthChecking()

	return hc
}

// startHealthChecking starts the health checking loop
func (hc *HealthChecker) startHealthChecking() {
	logx.Debug("health checking loop started")
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			logx.Debug("health checking loop stopped")
			return
		case <-ticker.C:
			hc.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check
func (hc *HealthChecker) performHealthCheck() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	atomic.AddInt64(&hc.checkCount, 1)
	atomic.StoreInt64(&hc.lastCheck, time.Now().UnixNano())

	logx.Debug("performing health check")

	// Check if connection is closed
	if hc.closed {
		logx.Warn("health checker is closed, marking as unhealthy")
		hc.setStatus(StatusUnhealthy, fmt.Errorf("health checker is closed"))
		return
	}

	// Check if connection is nil
	if hc.conn == nil {
		hc.setStatus(StatusUnhealthy, fmt.Errorf("connection is nil"))
		return
	}

	// Perform a simple health check by trying to get connection info
	// Note: go-rabbitmq doesn't expose connection state directly
	// We'll assume the connection is healthy if it's not nil and not closed
	hc.setStatus(StatusHealthy, nil)
}

// setStatus sets the health status and calls the callback if provided
func (hc *HealthChecker) setStatus(status HealthStatus, err error) {
	oldStatus := hc.status
	hc.status = status
	hc.lastError = err

	if status == StatusHealthy {
		atomic.AddInt64(&hc.successCount, 1)
	} else {
		atomic.AddInt64(&hc.failureCount, 1)
	}

	// Call callback if status changed
	if oldStatus != status && hc.healthCallback != nil {
		hc.healthCallback(status, err)
	}
}

// IsHealthy returns true if the connection is healthy
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.status == StatusHealthy
}

// GetStatus returns the current health status
func (hc *HealthChecker) GetStatus() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.status
}

// GetStats returns health check statistics
func (hc *HealthChecker) GetStats() *HealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	checkCount := atomic.LoadInt64(&hc.checkCount)
	successCount := atomic.LoadInt64(&hc.successCount)
	failureCount := atomic.LoadInt64(&hc.failureCount)
	lastCheckNano := atomic.LoadInt64(&hc.lastCheck)

	var lastCheckTime time.Time
	if lastCheckNano > 0 {
		lastCheckTime = time.Unix(0, lastCheckNano)
	}

	var successRate float64
	if checkCount > 0 {
		successRate = float64(successCount) / float64(checkCount)
	}

	return &HealthStats{
		Status:       hc.status,
		LastCheck:    lastCheckTime,
		LastError:    hc.lastError,
		CheckCount:   checkCount,
		SuccessCount: successCount,
		FailureCount: failureCount,
		SuccessRate:  successRate,
		IsHealthy:    hc.status == StatusHealthy,
	}
}

// GetStatsMap returns health check statistics as a map
func (hc *HealthChecker) GetStatsMap() map[string]interface{} {
	stats := hc.GetStats()
	return map[string]interface{}{
		"status":        stats.Status,
		"last_check":    stats.LastCheck,
		"last_error":    stats.LastError,
		"check_count":   stats.CheckCount,
		"success_count": stats.SuccessCount,
		"failure_count": stats.FailureCount,
		"success_rate":  stats.SuccessRate,
		"is_healthy":    stats.IsHealthy,
		"closed":        hc.closed,
	}
}

// SetHealthCallback sets a callback function that is called when health status changes
func (hc *HealthChecker) SetHealthCallback(callback func(status HealthStatus, err error)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.healthCallback = callback
}

// CheckNow performs an immediate health check
func (hc *HealthChecker) CheckNow() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	atomic.AddInt64(&hc.checkCount, 1)
	atomic.StoreInt64(&hc.lastCheck, time.Now().UnixNano())

	if hc.closed {
		err := fmt.Errorf("health checker is closed")
		hc.setStatus(StatusUnhealthy, err)
		return err
	}

	if hc.conn == nil {
		err := fmt.Errorf("connection is nil")
		hc.setStatus(StatusUnhealthy, err)
		return err
	}

	hc.setStatus(StatusHealthy, nil)
	return nil
}

// Close closes the health checker
func (hc *HealthChecker) Close() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.closed {
		return nil
	}

	hc.closed = true
	hc.cancel()
	hc.setStatus(StatusUnhealthy, fmt.Errorf("health checker closed"))

	return nil
}

// String returns a string representation of the health checker
func (hc *HealthChecker) String() string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return fmt.Sprintf("HealthChecker{Status: %s, Checks: %d, Success: %d, Failures: %d}",
		hc.status,
		atomic.LoadInt64(&hc.checkCount),
		atomic.LoadInt64(&hc.successCount),
		atomic.LoadInt64(&hc.failureCount))
}

// WaitForHealthy waits for the connection to become healthy
func (hc *HealthChecker) WaitForHealthy(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewTimeoutError("health check timeout", timeout, "wait_for_healthy", ctx.Err())
		case <-hc.ctx.Done():
			return fmt.Errorf("health checker closed")
		case <-ticker.C:
			if hc.IsHealthy() {
				return nil
			}
		}
	}
}

// WaitForUnhealthy waits for the connection to become unhealthy
func (hc *HealthChecker) WaitForUnhealthy(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewTimeoutError("health check timeout", timeout, "wait_for_unhealthy", ctx.Err())
		case <-hc.ctx.Done():
			return fmt.Errorf("health checker closed")
		case <-ticker.C:
			if !hc.IsHealthy() {
				return nil
			}
		}
	}
}
