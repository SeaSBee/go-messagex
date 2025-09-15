package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

func TestObservabilityProvider(t *testing.T) {
	cfg := &messaging.TelemetryConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
	}

	provider, err := messaging.NewObservabilityProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test that we get no-op implementations
	assert.IsType(t, messaging.NoOpMetrics{}, provider.Metrics())
	assert.IsType(t, messaging.NoOpTracer{}, provider.Tracer())
	assert.NotNil(t, provider.Logger())
}

func TestObservabilityContext(t *testing.T) {
	cfg := &messaging.TelemetryConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
	}

	provider, err := messaging.NewObservabilityProvider(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	obsCtx := messaging.NewObservabilityContext(ctx, provider)

	// Test basic functionality
	assert.Equal(t, ctx, obsCtx.Context())
	assert.Equal(t, provider.Metrics(), obsCtx.Metrics())
	assert.Equal(t, provider.Tracer(), obsCtx.Tracer())
	assert.Equal(t, provider.Logger(), obsCtx.Logger())

	// Test WithSpan
	newObsCtx, span := obsCtx.WithSpan("test-span")
	assert.NotNil(t, newObsCtx)
	assert.NotNil(t, span)
	defer span.End()

	// Test WithLogger
	loggerCtx := obsCtx.WithLogger(logx.String("key", "value"))
	assert.NotNil(t, loggerCtx)

	// Test metrics recording
	obsCtx.RecordPublishMetrics("rabbitmq", "test.exchange", 100*time.Millisecond, true, "")
	obsCtx.RecordConsumeMetrics("rabbitmq", "test.queue", 50*time.Millisecond, true, "")
	obsCtx.RecordConnectionMetrics("rabbitmq", 5, 10, 25)
	obsCtx.RecordSystemMetrics("rabbitmq", "backpressure", "queue_full")
}

func TestHealthManager(t *testing.T) {
	manager := messaging.NewHealthManager(5 * time.Second)

	// Test registration
	checker := messaging.NewConnectionHealthChecker("rabbitmq", func(ctx context.Context) (bool, error) {
		return true, nil
	})

	manager.Register("test_connection", checker)
	assert.Len(t, manager.Check(context.Background()), 1)

	// Test function registration
	manager.RegisterFunc("test_func", func(ctx context.Context) messaging.HealthCheck {
		// Add a small delay to ensure measurable duration
		time.Sleep(1 * time.Millisecond)
		return messaging.HealthCheck{
			Name:   "test_func",
			Status: messaging.HealthStatusHealthy,
		}
	})

	// Test health checks
	results := manager.Check(context.Background())
	assert.Len(t, results, 2)

	// Verify results
	for name, result := range results {
		assert.NotEmpty(t, name)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.Duration)
	}

	// Test overall status
	status := manager.OverallStatus(context.Background())
	assert.Equal(t, messaging.HealthStatusHealthy, status)

	// Test unregister
	manager.Unregister("test_connection")
	results = manager.Check(context.Background())
	assert.Len(t, results, 1)
}

func TestHealthReport(t *testing.T) {
	manager := messaging.NewHealthManager(5 * time.Second)

	// Add some test health checks
	manager.RegisterFunc("healthy", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Name:   "healthy",
			Status: messaging.HealthStatusHealthy,
		}
	})

	manager.RegisterFunc("degraded", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Name:   "degraded",
			Status: messaging.HealthStatusDegraded,
		}
	})

	manager.RegisterFunc("unhealthy", func(ctx context.Context) messaging.HealthCheck {
		return messaging.HealthCheck{
			Name:   "unhealthy",
			Status: messaging.HealthStatusUnhealthy,
		}
	})

	// Generate report
	report := manager.GenerateReport(context.Background())

	// Verify report structure
	assert.NotZero(t, report.Timestamp)
	assert.NotZero(t, report.Duration)
	assert.Len(t, report.Checks, 3)

	// Verify summary
	summary := report.Summary
	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 1, summary.Healthy)
	assert.Equal(t, 1, summary.Degraded)
	assert.Equal(t, 1, summary.Unhealthy)
	assert.Equal(t, 0, summary.Unknown)

	// Overall status should be unhealthy due to one unhealthy check
	assert.Equal(t, messaging.HealthStatusUnhealthy, report.Status)
}

func TestConnectionHealthChecker(t *testing.T) {
	// Test healthy connection
	checker := messaging.NewConnectionHealthChecker("rabbitmq", func(ctx context.Context) (bool, error) {
		return true, nil
	})

	result := checker.Check(context.Background())
	assert.Equal(t, "rabbitmq_connection", result.Name)
	assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.Nil(t, result.Error)

	// Test degraded connection
	checker = messaging.NewConnectionHealthChecker("rabbitmq", func(ctx context.Context) (bool, error) {
		return false, nil
	})

	result = checker.Check(context.Background())
	assert.Equal(t, messaging.HealthStatusDegraded, result.Status)
	assert.Contains(t, result.Message, "degraded")

	// Test unhealthy connection
	checker = messaging.NewConnectionHealthChecker("rabbitmq", func(ctx context.Context) (bool, error) {
		return false, assert.AnError
	})

	result = checker.Check(context.Background())
	assert.Equal(t, messaging.HealthStatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "error")
	assert.Equal(t, assert.AnError, result.Error)
}

func TestBuiltInHealthCheckers(t *testing.T) {
	// Test memory health checker
	memoryChecker := messaging.NewMemoryHealthChecker(80.0)
	result := memoryChecker.Check(context.Background())
	assert.Equal(t, "memory", result.Name)
	assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
	assert.Contains(t, result.Details, "max_usage_percent")

	// Test goroutine health checker
	goroutineChecker := messaging.NewGoroutineHealthChecker(1000)
	result = goroutineChecker.Check(context.Background())
	assert.Equal(t, "goroutines", result.Name)
	assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
	assert.Contains(t, result.Details, "max_goroutines")
}

func TestHealthStatusString(t *testing.T) {
	// Test health status constants
	assert.Equal(t, "healthy", string(messaging.HealthStatusHealthy))
	assert.Equal(t, "unhealthy", string(messaging.HealthStatusUnhealthy))
	assert.Equal(t, "degraded", string(messaging.HealthStatusDegraded))
	assert.Equal(t, "unknown", string(messaging.HealthStatusUnknown))
}

func TestHealthManagerTimeout(t *testing.T) {
	// Test with a very short timeout
	manager := messaging.NewHealthManager(1 * time.Millisecond)

	// Add a slow health check
	manager.RegisterFunc("slow", func(ctx context.Context) messaging.HealthCheck {
		time.Sleep(10 * time.Millisecond) // This should timeout
		return messaging.HealthCheck{
			Name:   "slow",
			Status: messaging.HealthStatusHealthy,
		}
	})

	// The check should timeout
	results := manager.Check(context.Background())
	assert.Len(t, results, 1)

	result := results["slow"]
	assert.NotZero(t, result.Duration)
	// Note: The actual timeout behavior depends on the context cancellation
}

func TestHealthManagerConcurrency(t *testing.T) {
	manager := messaging.NewHealthManager(5 * time.Second)

	// Add multiple health checks
	for i := 0; i < 10; i++ {
		manager.RegisterFunc(fmt.Sprintf("check_%d", i), func(ctx context.Context) messaging.HealthCheck {
			return messaging.HealthCheck{
				Name:   fmt.Sprintf("check_%d", i),
				Status: messaging.HealthStatusHealthy,
			}
		})
	}

	// Run health checks concurrently
	results := manager.Check(context.Background())
	assert.Len(t, results, 10)

	// Verify all results are present
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("check_%d", i)
		result, exists := results[name]
		assert.True(t, exists)
		assert.Equal(t, name, result.Name)
		assert.Equal(t, messaging.HealthStatusHealthy, result.Status)
	}
}
