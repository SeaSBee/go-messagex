// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"fmt"
	"runtime"
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
	if timeout <= 0 {
		timeout = 30 * time.Second // Default timeout
	}
	return &HealthManager{
		checkers: make(map[string]HealthChecker),
		timeout:  timeout,
	}

}

// Register registers a health checker.
func (hm *HealthManager) Register(name string, checker HealthChecker) {
	if name == "" {
		panic("health checker name cannot be empty")
	}
	if checker == nil {
		panic("health checker cannot be nil")
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checkers[name] = checker
}

// RegisterFunc registers a health check function.
func (hm *HealthManager) RegisterFunc(name string, fn func(ctx context.Context) HealthCheck) {
	if name == "" {
		panic("health checker name cannot be empty")
	}
	if fn == nil {
		panic("health check function cannot be nil")
	}

	hm.Register(name, HealthCheckerFunc(fn))
}

// Unregister removes a health checker.
func (hm *HealthManager) Unregister(name string) {
	if name == "" {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.checkers, name)
}

// Check performs all registered health checks.
func (hm *HealthManager) Check(ctx context.Context) map[string]HealthCheck {
	if ctx == nil {
		ctx = context.Background()
	}

	// Create a timeout context for the entire operation
	checkCtx, cancel := context.WithTimeout(ctx, hm.timeout)
	defer cancel()

	hm.mu.RLock()
	checkers := make(map[string]HealthChecker, len(hm.checkers))
	for k, v := range hm.checkers {
		checkers[k] = v
	}
	hm.mu.RUnlock()

	if len(checkers) == 0 {
		return make(map[string]HealthCheck)
	}

	results := make(map[string]HealthCheck)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[name] = HealthCheck{
						Name:      name,
						Status:    HealthStatusUnhealthy,
						Message:   fmt.Sprintf("Health check panicked: %v", r),
						Timestamp: time.Now(),
						Error:     fmt.Errorf("panic: %v", r),
					}
					mu.Unlock()
				}
			}()

			// Create a timeout context for this individual check
			individualCtx, cancel := context.WithTimeout(checkCtx, hm.timeout)
			defer cancel()

			start := time.Now()
			result := checker.Check(individualCtx)
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

	// Determine overall status based on the checks we already have
	status := HealthStatusUnknown
	if len(checks) > 0 {
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
			status = HealthStatusUnhealthy
		} else if hasDegraded {
			status = HealthStatusDegraded
		} else {
			status = HealthStatusHealthy
		}
	}

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
	if transport == "" {
		panic("transport name cannot be empty")
	}
	if checkFn == nil {
		panic("check function cannot be nil")
	}

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
	if ctx == nil {
		ctx = context.Background()
	}

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
	if maxUsagePercent <= 0 || maxUsagePercent > 100 {
		panic("maxUsagePercent must be between 0 and 100")
	}

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
	if ctx == nil {
		ctx = context.Background()
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate memory usage percentage
	totalMemory := memStats.Sys
	usedMemory := memStats.Alloc
	usagePercent := float64(usedMemory) / float64(totalMemory) * 100

	status := HealthStatusHealthy
	message := fmt.Sprintf("Memory usage: %.2f%%", usagePercent)

	if usagePercent > m.maxUsagePercent {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("Memory usage %.2f%% exceeds limit %.2f%%", usagePercent, m.maxUsagePercent)
	} else if usagePercent > m.maxUsagePercent*0.8 {
		status = HealthStatusDegraded
		message = fmt.Sprintf("Memory usage %.2f%% is approaching limit %.2f%%", usagePercent, m.maxUsagePercent)
	}

	return HealthCheck{
		Name:      m.Name(),
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"max_usage_percent":     m.maxUsagePercent,
			"current_usage_percent": usagePercent,
			"total_memory_bytes":    totalMemory,
			"used_memory_bytes":     usedMemory,
		},
	}
}

// GoroutineHealthChecker checks goroutine count.
type GoroutineHealthChecker struct {
	maxGoroutines int
}

// NewGoroutineHealthChecker creates a new goroutine health checker.
func NewGoroutineHealthChecker(maxGoroutines int) *GoroutineHealthChecker {
	if maxGoroutines <= 0 {
		panic("maxGoroutines must be positive")
	}

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
	if ctx == nil {
		ctx = context.Background()
	}

	currentGoroutines := runtime.NumGoroutine()

	status := HealthStatusHealthy
	message := fmt.Sprintf("Goroutine count: %d", currentGoroutines)

	if currentGoroutines > g.maxGoroutines {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("Goroutine count %d exceeds limit %d", currentGoroutines, g.maxGoroutines)
	} else if currentGoroutines > int(float64(g.maxGoroutines)*0.8) {
		status = HealthStatusDegraded
		message = fmt.Sprintf("Goroutine count %d is approaching limit %d", currentGoroutines, g.maxGoroutines)
	}

	return HealthCheck{
		Name:      g.Name(),
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"max_goroutines":     g.maxGoroutines,
			"current_goroutines": currentGoroutines,
		},
	}
}
