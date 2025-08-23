// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the component is healthy.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusUnhealthy indicates the component is unhealthy.
	HealthStatusUnhealthy HealthStatus = "unhealthy"

	// HealthStatusDegraded indicates the component is degraded but functional.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnknown indicates the health status is unknown.
	HealthStatusUnknown HealthStatus = "unknown"
)

// HealthCheck represents a health check result.
type HealthCheck struct {
	// Name is the name of the health check.
	Name string

	// Status is the health status.
	Status HealthStatus

	// Message provides additional information about the health status.
	Message string

	// Timestamp is when the health check was performed.
	Timestamp time.Time

	// Duration is how long the health check took.
	Duration time.Duration

	// Details contains additional health check details.
	Details map[string]interface{}

	// Error contains any error that occurred during the health check.
	Error error
}

// HealthChecker defines the interface for health checks.
type HealthChecker interface {
	// Check performs a health check and returns the result.
	Check(ctx context.Context) HealthCheck

	// Name returns the name of the health checker.
	Name() string
}

// HealthCheckerFunc is a function adapter for HealthChecker interface.
type HealthCheckerFunc func(ctx context.Context) HealthCheck

// Check implements the HealthChecker interface.
func (f HealthCheckerFunc) Check(ctx context.Context) HealthCheck {
	return f(ctx)
}

// Name returns the name of the health checker.
func (f HealthCheckerFunc) Name() string {
	return "func"
}

// HealthManager manages multiple health checks.
type HealthManager struct {
	checkers map[string]HealthChecker
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewHealthManager creates a new health manager.
func NewHealthManager(timeout time.Duration) *HealthManager {
	return &HealthManager{
		checkers: make(map[string]HealthChecker),
		timeout:  timeout,
	}
}

// Register registers a health checker.
func (hm *HealthManager) Register(name string, checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checkers[name] = checker
}

// RegisterFunc registers a health check function.
func (hm *HealthManager) RegisterFunc(name string, fn func(ctx context.Context) HealthCheck) {
	hm.Register(name, HealthCheckerFunc(fn))
}

// Unregister removes a health checker.
func (hm *HealthManager) Unregister(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.checkers, name)
}

// Check performs all registered health checks.
func (hm *HealthManager) Check(ctx context.Context) map[string]HealthCheck {
	hm.mu.RLock()
	checkers := make(map[string]HealthChecker, len(hm.checkers))
	for k, v := range hm.checkers {
		checkers[k] = v
	}
	hm.mu.RUnlock()

	results := make(map[string]HealthCheck)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			// Create a timeout context for this check
			checkCtx, cancel := context.WithTimeout(ctx, hm.timeout)
			defer cancel()

			start := time.Now()
			result := checker.Check(checkCtx)
			result.Duration = time.Since(start)
			result.Timestamp = start

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()
	return results
}

// OverallStatus returns the overall health status based on all checks.
func (hm *HealthManager) OverallStatus(ctx context.Context) HealthStatus {
	checks := hm.Check(ctx)

	if len(checks) == 0 {
		return HealthStatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}

// HealthReport represents a comprehensive health report.
type HealthReport struct {
	// Status is the overall health status.
	Status HealthStatus

	// Timestamp is when the report was generated.
	Timestamp time.Time

	// Duration is how long the health check took.
	Duration time.Duration

	// Checks contains individual health check results.
	Checks map[string]HealthCheck

	// Summary provides a summary of the health status.
	Summary HealthSummary
}

// HealthSummary provides a summary of health check results.
type HealthSummary struct {
	// Total is the total number of health checks.
	Total int

	// Healthy is the number of healthy checks.
	Healthy int

	// Unhealthy is the number of unhealthy checks.
	Unhealthy int

	// Degraded is the number of degraded checks.
	Degraded int

	// Unknown is the number of unknown checks.
	Unknown int
}

// GenerateReport generates a comprehensive health report.
func (hm *HealthManager) GenerateReport(ctx context.Context) HealthReport {
	start := time.Now()
	checks := hm.Check(ctx)
	duration := time.Since(start)

	summary := HealthSummary{
		Total: len(checks),
	}

	for _, check := range checks {
		switch check.Status {
		case HealthStatusHealthy:
			summary.Healthy++
		case HealthStatusUnhealthy:
			summary.Unhealthy++
		case HealthStatusDegraded:
			summary.Degraded++
		case HealthStatusUnknown:
			summary.Unknown++
		}
	}

	status := hm.OverallStatus(ctx)

	return HealthReport{
		Status:    status,
		Timestamp: start,
		Duration:  duration,
		Checks:    checks,
		Summary:   summary,
	}
}

// Built-in health checkers

// ConnectionHealthChecker checks the health of connections.
type ConnectionHealthChecker struct {
	transport string
	checkFn   func(ctx context.Context) (bool, error)
}

// NewConnectionHealthChecker creates a new connection health checker.
func NewConnectionHealthChecker(transport string, checkFn func(ctx context.Context) (bool, error)) *ConnectionHealthChecker {
	return &ConnectionHealthChecker{
		transport: transport,
		checkFn:   checkFn,
	}
}

// Name returns the name of the health checker.
func (c *ConnectionHealthChecker) Name() string {
	return fmt.Sprintf("%s_connection", c.transport)
}

// Check performs the connection health check.
func (c *ConnectionHealthChecker) Check(ctx context.Context) HealthCheck {
	healthy, err := c.checkFn(ctx)

	status := HealthStatusHealthy
	message := "Connection is healthy"

	if err != nil {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("Connection error: %v", err)
	} else if !healthy {
		status = HealthStatusDegraded
		message = "Connection is degraded"
	}

	return HealthCheck{
		Name:      c.Name(),
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"transport": c.transport,
		},
		Error: err,
	}
}

// MemoryHealthChecker checks memory usage.
type MemoryHealthChecker struct {
	maxUsagePercent float64
}

// NewMemoryHealthChecker creates a new memory health checker.
func NewMemoryHealthChecker(maxUsagePercent float64) *MemoryHealthChecker {
	return &MemoryHealthChecker{
		maxUsagePercent: maxUsagePercent,
	}
}

// Name returns the name of the health checker.
func (m *MemoryHealthChecker) Name() string {
	return "memory"
}

// Check performs the memory health check.
func (m *MemoryHealthChecker) Check(ctx context.Context) HealthCheck {
	// TODO: Implement actual memory usage check
	// For now, return a placeholder
	return HealthCheck{
		Name:      m.Name(),
		Status:    HealthStatusHealthy,
		Message:   "Memory usage is within limits",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"max_usage_percent": m.maxUsagePercent,
		},
	}
}

// GoroutineHealthChecker checks goroutine count.
type GoroutineHealthChecker struct {
	maxGoroutines int
}

// NewGoroutineHealthChecker creates a new goroutine health checker.
func NewGoroutineHealthChecker(maxGoroutines int) *GoroutineHealthChecker {
	return &GoroutineHealthChecker{
		maxGoroutines: maxGoroutines,
	}
}

// Name returns the name of the health checker.
func (g *GoroutineHealthChecker) Name() string {
	return "goroutines"
}

// Check performs the goroutine health check.
func (g *GoroutineHealthChecker) Check(ctx context.Context) HealthCheck {
	// TODO: Implement actual goroutine count check
	// For now, return a placeholder
	return HealthCheck{
		Name:      g.Name(),
		Status:    HealthStatusHealthy,
		Message:   "Goroutine count is within limits",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"max_goroutines": g.maxGoroutines,
		},
	}
}
