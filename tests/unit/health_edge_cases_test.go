package unit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

// TestHealthManagerEdgeCases tests edge cases for health manager
func TestHealthManagerEdgeCases(t *testing.T) {
	t.Run("NewHealthManagerEdgeCases", func(t *testing.T) {
		// Test zero timeout
		manager := messaging.NewHealthManager(0)
		assert.NotNil(t, manager)
		manager.RegisterFunc("test", func(ctx context.Context) messaging.HealthCheck {
			return messaging.HealthCheck{Status: messaging.HealthStatusHealthy}
		})
		results := manager.Check(context.Background())
		assert.Len(t, results, 1)

		// Test negative timeout
		manager = messaging.NewHealthManager(-5 * time.Second)
		assert.NotNil(t, manager)
		manager.RegisterFunc("test", func(ctx context.Context) messaging.HealthCheck {
			return messaging.HealthCheck{Status: messaging.HealthStatusHealthy}
		})
		results = manager.Check(context.Background())
		assert.Len(t, results, 1)
	})

	t.Run("RegistrationEdgeCases", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)

		// Test empty name registration
		assert.Panics(t, func() {
			manager.Register("", &customHealthChecker{name: "test", status: messaging.HealthStatusHealthy})
		})

		// Test nil checker registration
		assert.Panics(t, func() {
			manager.Register("test", nil)
		})

		// Test empty name function registration
		assert.Panics(t, func() {
			manager.RegisterFunc("", func(ctx context.Context) messaging.HealthCheck {
				return messaging.HealthCheck{}
			})
		})

		// Test nil function registration
		assert.Panics(t, func() {
			manager.RegisterFunc("test", nil)
		})

		// Test duplicate registration (should overwrite)
		checker1 := &customHealthChecker{name: "test", status: messaging.HealthStatusHealthy}
		checker2 := &customHealthChecker{name: "test", status: messaging.HealthStatusUnhealthy}
		manager.Register("test", checker1)
		manager.Register("test", checker2)
		results := manager.Check(context.Background())
		assert.Len(t, results, 1)
	})

	t.Run("UnregistrationEdgeCases", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)
		checker := &customHealthChecker{name: "test", status: messaging.HealthStatusHealthy}

		// Test unregistering non-existent checker
		manager.Unregister("non_existent")
		assert.Len(t, manager.Check(context.Background()), 0)

		// Test unregistering with empty name
		manager.Register("test", checker)
		manager.Unregister("")
		assert.Len(t, manager.Check(context.Background()), 1) // Should still have "test"
	})

	t.Run("CheckEdgeCases", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)

		// Test empty manager
		results := manager.Check(context.Background())
		assert.Empty(t, results)

		// Test nil context
		manager.RegisterFunc("test", func(ctx context.Context) messaging.HealthCheck {
			return messaging.HealthCheck{Status: messaging.HealthStatusHealthy}
		})
		results = manager.Check(nil)
		assert.Len(t, results, 1)
	})
}

// TestHealthManagerPanicRecovery tests panic recovery in health checks
func TestHealthManagerPanicRecovery(t *testing.T) {
	t.Run("PanicInHealthCheck", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)
		manager.RegisterFunc("panic_check", func(ctx context.Context) messaging.HealthCheck {
			panic("test panic")
		})

		results := manager.Check(context.Background())
		assert.Len(t, results, 1)
		assert.Contains(t, results, "panic_check")

		result := results["panic_check"]
		assert.Equal(t, messaging.HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "Health check panicked")
		assert.NotNil(t, result.Error)
		assert.Contains(t, result.Error.Error(), "panic")
	})

	t.Run("PanicWithNilContext", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)
		manager.RegisterFunc("panic_nil_ctx", func(ctx context.Context) messaging.HealthCheck {
			if ctx == nil {
				panic("nil context")
			}
			return messaging.HealthCheck{Status: messaging.HealthStatusHealthy}
		})

		results := manager.Check(nil) // Should not panic
		assert.Len(t, results, 1)
		assert.Contains(t, results, "panic_nil_ctx")
	})
}

// TestHealthManagerConcurrencyEdgeCases tests concurrent operations edge cases
func TestHealthManagerConcurrencyEdgeCases(t *testing.T) {
	t.Run("ConcurrentRegistrationUnregistration", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)
		var wg sync.WaitGroup
		const numGoroutines = 10

		// Concurrent registration
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent_%d", id)
				manager.RegisterFunc(name, func(ctx context.Context) messaging.HealthCheck {
					return messaging.HealthCheck{
						Name:    name,
						Status:  messaging.HealthStatusHealthy,
						Message: fmt.Sprintf("concurrent check %d", id),
					}
				})
			}(i)
		}

		wg.Wait()
		results := manager.Check(context.Background())
		assert.Len(t, results, numGoroutines)

		// Concurrent unregistration
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent_%d", id)
				manager.Unregister(name)
			}(i)
		}

		wg.Wait()
		results = manager.Check(context.Background())
		assert.Len(t, results, 0)
	})

	t.Run("ConcurrentChecksWithSharedManager", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)
		manager.RegisterFunc("shared_check", func(ctx context.Context) messaging.HealthCheck {
			time.Sleep(1 * time.Millisecond)
			return messaging.HealthCheck{
				Name:    "shared_check",
				Status:  messaging.HealthStatusHealthy,
				Message: "shared check",
			}
		})

		var wg sync.WaitGroup
		const numChecks = 10
		results := make([]map[string]messaging.HealthCheck, numChecks)

		for i := 0; i < numChecks; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				results[id] = manager.Check(context.Background())
			}(i)
		}

		wg.Wait()

		// All results should have the same structure but may differ in timing
		for i := 1; i < numChecks; i++ {
			// Check that all results have the same keys
			assert.Equal(t, len(results[0]), len(results[i]))
			for key := range results[0] {
				assert.Contains(t, results[i], key)
				// Check that the health check has the same basic properties
				expected := results[0][key]
				actual := results[i][key]
				assert.Equal(t, expected.Name, actual.Name)
				assert.Equal(t, expected.Status, actual.Status)
				assert.Equal(t, expected.Message, actual.Message)
				// Don't compare timestamps and durations as they will differ
			}
		}
	})
}

// TestConnectionHealthCheckerEdgeCases tests connection health checker edge cases
func TestConnectionHealthCheckerEdgeCases(t *testing.T) {
	t.Run("ConnectionHealthCheckerInvalidParams", func(t *testing.T) {
		// Test empty transport name
		assert.Panics(t, func() {
			messaging.NewConnectionHealthChecker("", func(ctx context.Context) (bool, error) {
				return true, nil
			})
		})

		// Test nil check function
		assert.Panics(t, func() {
			messaging.NewConnectionHealthChecker("test", nil)
		})
	})

	t.Run("ConnectionHealthCheckerNilContext", func(t *testing.T) {
		checker := messaging.NewConnectionHealthChecker("test", func(ctx context.Context) (bool, error) {
			return true, nil
		})

		result := checker.Check(nil) // Should not panic
		assert.Equal(t, "test_connection", result.Name)
		assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
	})

	t.Run("ConnectionHealthCheckerAllScenarios", func(t *testing.T) {
		// Healthy connection
		checker := messaging.NewConnectionHealthChecker("test", func(ctx context.Context) (bool, error) {
			return true, nil
		})

		result := checker.Check(context.Background())
		assert.Equal(t, "test_connection", result.Name)
		assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
		assert.Contains(t, result.Message, "healthy")
		assert.Nil(t, result.Error)
		assert.Contains(t, result.Details, "transport")
		assert.Equal(t, "test", result.Details["transport"])

		// Degraded connection
		checker = messaging.NewConnectionHealthChecker("test", func(ctx context.Context) (bool, error) {
			return false, nil
		})

		result = checker.Check(context.Background())
		assert.Equal(t, "test_connection", result.Name)
		assert.Equal(t, messaging.HealthStatusDegraded, result.Status)
		assert.Contains(t, result.Message, "degraded")
		assert.Nil(t, result.Error)

		// Unhealthy connection
		testError := errors.New("connection failed")
		checker = messaging.NewConnectionHealthChecker("test", func(ctx context.Context) (bool, error) {
			return false, testError
		})

		result = checker.Check(context.Background())
		assert.Equal(t, "test_connection", result.Name)
		assert.Equal(t, messaging.HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "error")
		assert.Equal(t, testError, result.Error)
	})
}

// TestBuiltInHealthCheckersEdgeCases tests built-in health checkers edge cases
func TestBuiltInHealthCheckersEdgeCases(t *testing.T) {
	t.Run("MemoryHealthCheckerEdgeCases", func(t *testing.T) {
		// Test invalid parameters
		assert.Panics(t, func() {
			messaging.NewMemoryHealthChecker(0)
		})

		assert.Panics(t, func() {
			messaging.NewMemoryHealthChecker(-10)
		})

		assert.Panics(t, func() {
			messaging.NewMemoryHealthChecker(150)
		})

		// Test nil context
		checker := messaging.NewMemoryHealthChecker(80.0)
		result := checker.Check(nil) // Should not panic
		assert.Equal(t, "memory", result.Name)
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Details, "max_usage_percent")
		assert.Contains(t, result.Details, "current_usage_percent")
		assert.Contains(t, result.Details, "total_memory_bytes")
		assert.Contains(t, result.Details, "used_memory_bytes")
	})

	t.Run("GoroutineHealthCheckerEdgeCases", func(t *testing.T) {
		// Test invalid parameters
		assert.Panics(t, func() {
			messaging.NewGoroutineHealthChecker(0)
		})

		assert.Panics(t, func() {
			messaging.NewGoroutineHealthChecker(-10)
		})

		// Test nil context
		checker := messaging.NewGoroutineHealthChecker(1000)
		result := checker.Check(nil) // Should not panic
		assert.Equal(t, "goroutines", result.Name)
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Details, "max_goroutines")
		assert.Contains(t, result.Details, "current_goroutines")
	})
}

// TestHealthCheckStructureEdgeCases tests health check structure edge cases
func TestHealthCheckStructureEdgeCases(t *testing.T) {
	t.Run("HealthCheckWithAllFields", func(t *testing.T) {
		now := time.Now()
		testError := errors.New("test error")
		details := map[string]interface{}{
			"cpu_usage":    75.5,
			"memory_usage": "80%",
			"active":       true,
			"count":        42,
		}

		check := messaging.HealthCheck{
			Name:      "comprehensive_check",
			Status:    messaging.HealthStatusDegraded,
			Message:   "comprehensive test message",
			Timestamp: now,
			Duration:  150 * time.Millisecond,
			Details:   details,
			Error:     testError,
		}

		assert.Equal(t, "comprehensive_check", check.Name)
		assert.Equal(t, messaging.HealthStatusDegraded, check.Status)
		assert.Equal(t, "comprehensive test message", check.Message)
		assert.Equal(t, now, check.Timestamp)
		assert.Equal(t, 150*time.Millisecond, check.Duration)
		assert.Len(t, check.Details, 4)
		assert.Equal(t, testError, check.Error)
		assert.Equal(t, 75.5, check.Details["cpu_usage"])
		assert.Equal(t, "80%", check.Details["memory_usage"])
		assert.Equal(t, true, check.Details["active"])
		assert.Equal(t, 42, check.Details["count"])
	})

	t.Run("HealthCheckWithEmptyFields", func(t *testing.T) {
		check := messaging.HealthCheck{
			Name:    "",
			Status:  messaging.HealthStatusUnknown,
			Message: "",
		}

		assert.Empty(t, check.Name)
		assert.Equal(t, messaging.HealthStatusUnknown, check.Status)
		assert.Empty(t, check.Message)
		assert.Nil(t, check.Details)
		assert.Nil(t, check.Error)
		assert.Zero(t, check.Timestamp)
		assert.Zero(t, check.Duration)
	})
}

// TestHealthCheckerInterfaceEdgeCases tests health checker interface edge cases
func TestHealthCheckerInterfaceEdgeCases(t *testing.T) {
	t.Run("CustomHealthCheckerWithError", func(t *testing.T) {
		checker := &customHealthCheckerWithError{
			name:   "error_checker",
			status: messaging.HealthStatusUnhealthy,
			err:    errors.New("custom error"),
		}

		result := checker.Check(context.Background())
		assert.Equal(t, "error_checker", result.Name)
		assert.Equal(t, messaging.HealthStatusUnhealthy, result.Status)
		assert.Equal(t, "custom error", result.Error.Error())
	})

	t.Run("HealthCheckerFuncWithContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		checker := messaging.HealthCheckerFunc(func(ctx context.Context) messaging.HealthCheck {
			select {
			case <-ctx.Done():
				return messaging.HealthCheck{
					Name:    "timeout_check",
					Status:  messaging.HealthStatusUnhealthy,
					Message: "context timeout",
				}
			case <-time.After(10 * time.Millisecond):
				return messaging.HealthCheck{
					Name:    "timeout_check",
					Status:  messaging.HealthStatusHealthy,
					Message: "completed",
				}
			}
		})

		result := checker.Check(ctx)
		assert.Equal(t, "timeout_check", result.Name)
		assert.Equal(t, messaging.HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "timeout")
	})
}

// TestHealthManagerIntegrationEdgeCases tests integration edge cases
func TestHealthManagerIntegrationEdgeCases(t *testing.T) {
	t.Run("MixedHealthCheckers", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)

		// Add function-based checker
		manager.RegisterFunc("func_check", func(ctx context.Context) messaging.HealthCheck {
			return messaging.HealthCheck{
				Name:    "func_check",
				Status:  messaging.HealthStatusHealthy,
				Message: "function check",
			}
		})

		// Add custom checker
		customChecker := &customHealthChecker{
			name:   "custom_check",
			status: messaging.HealthStatusDegraded,
		}
		manager.Register("custom_check", customChecker)

		// Add built-in checkers
		memoryChecker := messaging.NewMemoryHealthChecker(80.0)
		manager.Register("memory", memoryChecker)

		goroutineChecker := messaging.NewGoroutineHealthChecker(1000)
		manager.Register("goroutines", goroutineChecker)

		// Generate report
		report := manager.GenerateReport(context.Background())

		assert.NotZero(t, report.Timestamp)
		assert.NotZero(t, report.Duration)
		assert.Len(t, report.Checks, 4)
		assert.Equal(t, 4, report.Summary.Total)
		assert.GreaterOrEqual(t, report.Summary.Healthy, 0)
		assert.GreaterOrEqual(t, report.Summary.Degraded, 0)

		// Verify all checkers are present
		assert.Contains(t, report.Checks, "func_check")
		assert.Contains(t, report.Checks, "custom_check")
		assert.Contains(t, report.Checks, "memory")
		assert.Contains(t, report.Checks, "goroutines")
	})

	t.Run("ErrorRecoveryScenario", func(t *testing.T) {
		manager := messaging.NewHealthManager(5 * time.Second)

		// Add a health check that sometimes fails
		failCount := 0
		manager.RegisterFunc("unreliable", func(ctx context.Context) messaging.HealthCheck {
			failCount++
			if failCount%3 == 0 {
				return messaging.HealthCheck{
					Name:    "unreliable",
					Status:  messaging.HealthStatusUnhealthy,
					Message: "Temporary failure",
					Error:   errors.New("temporary error"),
				}
			}
			return messaging.HealthCheck{
				Name:    "unreliable",
				Status:  messaging.HealthStatusHealthy,
				Message: "Working normally",
			}
		})

		// Run multiple checks
		for i := 0; i < 5; i++ {
			report := manager.GenerateReport(context.Background())
			assert.Len(t, report.Checks, 1)
			assert.Contains(t, report.Checks, "unreliable")
		}
	})
}

// Helper structs for testing
type customHealthChecker struct {
	name   string
	status messaging.HealthStatus
}

func (c *customHealthChecker) Check(ctx context.Context) messaging.HealthCheck {
	return messaging.HealthCheck{
		Name:    c.name,
		Status:  c.status,
		Message: fmt.Sprintf("Custom check: %s", c.name),
	}
}

func (c *customHealthChecker) Name() string {
	return c.name
}

type customHealthCheckerWithError struct {
	name   string
	status messaging.HealthStatus
	err    error
}

func (c *customHealthCheckerWithError) Check(ctx context.Context) messaging.HealthCheck {
	return messaging.HealthCheck{
		Name:    c.name,
		Status:  c.status,
		Message: fmt.Sprintf("Custom check with error: %s", c.name),
		Error:   c.err,
	}
}

func (c *customHealthCheckerWithError) Name() string {
	return c.name
}
