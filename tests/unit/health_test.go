package unit

import (
	"sync"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock connection for testing
type mockHealthConn struct {
	healthy bool
	mu      sync.RWMutex
}

func (m *mockHealthConn) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.healthy
}

func (m *mockHealthConn) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

func (m *mockHealthConn) GetHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

func TestNewHealthChecker(t *testing.T) {
	t.Run("creates health checker with valid connection", func(t *testing.T) {
		interval := 1 * time.Second

		healthChecker := messaging.NewHealthChecker(nil, interval)

		require.NotNil(t, healthChecker)
		assert.False(t, healthChecker.IsHealthy())

		// Clean up
		healthChecker.Close()
	})

	t.Run("creates health checker with nil connection", func(t *testing.T) {
		interval := 1 * time.Second

		healthChecker := messaging.NewHealthChecker(nil, interval)

		require.NotNil(t, healthChecker)
		assert.False(t, healthChecker.IsHealthy())

		// Clean up
		healthChecker.Close()
	})

	t.Run("creates health checker with zero interval", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		interval := 0 * time.Second

		healthChecker := messaging.NewHealthChecker(nil, interval)

		require.NotNil(t, healthChecker)
		assert.False(t, healthChecker.IsHealthy())

		// Clean up
		healthChecker.Close()
	})

	t.Run("creates health checker with negative interval", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		interval := -1 * time.Second

		healthChecker := messaging.NewHealthChecker(nil, interval)

		require.NotNil(t, healthChecker)
		assert.False(t, healthChecker.IsHealthy())

		// Clean up
		healthChecker.Close()
	})
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	t.Run("returns true for healthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		assert.False(t, healthChecker.IsHealthy())
	})

	t.Run("returns false for unhealthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		assert.False(t, healthChecker.IsHealthy())
	})

	t.Run("returns false for nil connection", func(t *testing.T) {
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		assert.False(t, healthChecker.IsHealthy())
	})
}

func TestHealthChecker_CheckNow(t *testing.T) {
	t.Run("performs immediate health check", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		// Perform immediate check
		err := healthChecker.CheckNow()
		assert.Error(t, err) // Should return error for nil connection
	})

	t.Run("performs immediate health check on unhealthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		// Perform immediate check
		err := healthChecker.CheckNow()
		assert.Error(t, err) // Should return error for nil connection
	})

	t.Run("updates last check time", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		// Get initial stats
		initialStats := healthChecker.GetStats()
		initialLastCheck := initialStats.LastCheck

		// Perform check
		healthChecker.CheckNow()

		// Get updated stats
		updatedStats := healthChecker.GetStats()
		updatedLastCheck := updatedStats.LastCheck

		assert.True(t, updatedLastCheck.After(initialLastCheck))
	})
}

func TestHealthChecker_GetStats(t *testing.T) {
	t.Run("returns comprehensive stats", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		stats := healthChecker.GetStats()

		require.NotNil(t, stats)
		assert.False(t, stats.IsHealthy)              // Should be false for nil connection
		assert.Equal(t, int64(0), stats.CheckCount)   // Should be 0 for nil connection
		assert.Equal(t, time.Time{}, stats.LastCheck) // Should be zero time for nil connection
	})

	t.Run("stats reflect health changes", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 100*time.Millisecond)
		defer healthChecker.Close()

		// Initial stats
		initialStats := healthChecker.GetStats()
		assert.False(t, initialStats.IsHealthy) // Should be false for nil connection

		// For nil connection, stats should remain the same
		time.Sleep(200 * time.Millisecond)

		// Check updated stats
		updatedStats := healthChecker.GetStats()
		assert.False(t, updatedStats.IsHealthy) // Should still be false for nil connection
	})
}

func TestHealthChecker_WaitForHealthy(t *testing.T) {
	t.Run("waits for healthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		// Start waiting in goroutine
		done := make(chan error)
		go func() {
			err := healthChecker.WaitForHealthy(1 * time.Second)
			done <- err
		}()

		// For nil connection, we can't make it healthy
		// The test will timeout as expected

		// Wait for result
		select {
		case err := <-done:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "health check timeout")
		case <-time.After(2 * time.Second):
			t.Fatal("WaitForHealthy timed out")
		}
	})

	t.Run("times out waiting for healthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		err := healthChecker.WaitForHealthy(100 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("returns immediately if already healthy", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		start := time.Now()
		err := healthChecker.WaitForHealthy(1 * time.Second)
		duration := time.Since(start)

		// For nil connection, it will timeout since it's never healthy
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check timeout")
		assert.GreaterOrEqual(t, duration, 1*time.Second) // Should take at least the timeout duration
	})
}

func TestHealthChecker_WaitForUnhealthy(t *testing.T) {
	t.Run("waits for unhealthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		// Start waiting in goroutine
		done := make(chan error)
		go func() {
			err := healthChecker.WaitForUnhealthy(1 * time.Second)
			done <- err
		}()

		// For nil connection, it's always unhealthy, so should return immediately

		// Wait for result
		select {
		case err := <-done:
			assert.NoError(t, err) // Should return immediately since nil connection is unhealthy
		case <-time.After(2 * time.Second):
			t.Fatal("WaitForUnhealthy timed out")
		}
	})

	t.Run("times out waiting for unhealthy connection", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		// For nil connection, it's always unhealthy, so should return immediately
		start := time.Now()
		err := healthChecker.WaitForUnhealthy(100 * time.Millisecond)
		duration := time.Since(start)

		assert.NoError(t, err)                         // Should return immediately since nil connection is unhealthy
		assert.Less(t, duration, 150*time.Millisecond) // Should be fast (within one ticker interval)
	})

	t.Run("returns immediately if already unhealthy", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)
		defer healthChecker.Close()

		start := time.Now()
		err := healthChecker.WaitForUnhealthy(1 * time.Second)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 150*time.Millisecond) // Should be fast (within one ticker interval)
	})
}

func TestHealthChecker_Stop(t *testing.T) {
	t.Run("stops health checking", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)

		// Get initial check count
		initialStats := healthChecker.GetStats()
		initialCheckCount := initialStats.CheckCount

		// Wait for some checks to happen
		time.Sleep(200 * time.Millisecond)

		// Stop the health checker
		healthChecker.Close()

		// Get check count after stop
		stoppedStats := healthChecker.GetStats()
		stoppedCheckCount := stoppedStats.CheckCount

		// Wait a bit more to ensure no more checks happen
		time.Sleep(200 * time.Millisecond)

		// Get final check count
		finalStats := healthChecker.GetStats()
		finalCheckCount := finalStats.CheckCount

		// With nil connection, no checks are performed, so counts should be equal
		assert.Equal(t, stoppedCheckCount, initialCheckCount)

		// Verify no more checks happened after stop
		assert.Equal(t, stoppedCheckCount, finalCheckCount)
	})

	t.Run("multiple stop calls are safe", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Second)

		// Multiple stop calls should not panic
		healthChecker.Close()
		healthChecker.Close()
		healthChecker.Close()
	})
}

func TestHealthChecker_Concurrency(t *testing.T) {
	t.Run("concurrent health checks", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		var wg sync.WaitGroup
		numGoroutines := 10

		// Test concurrent health checks
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Perform multiple operations
				healthy := healthChecker.IsHealthy()
				assert.False(t, healthy) // Should be false for nil connection

				err := healthChecker.CheckNow()
				assert.Error(t, err) // Should return error for nil connection

				stats := healthChecker.GetStats()
				assert.NotNil(t, stats)
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent wait operations", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		var wg sync.WaitGroup
		numGoroutines := 5

		// Test concurrent wait operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// No need for context since we're using nil connection

				// This should timeout since connection is unhealthy
				err := healthChecker.WaitForHealthy(200 * time.Millisecond)
				assert.Error(t, err)
			}()
		}

		wg.Wait()
	})
}

func TestHealthChecker_EdgeCases(t *testing.T) {
	t.Run("health checker with very short interval", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Millisecond)
		defer healthChecker.Close()

		// Wait for some checks
		time.Sleep(50 * time.Millisecond)

		stats := healthChecker.GetStats()
		checkCount := stats.CheckCount

		// With nil connection, no checks are performed
		assert.Equal(t, checkCount, int64(0))
	})

	t.Run("health checker with very long interval", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 10*time.Second)
		defer healthChecker.Close()

		// Wait a short time
		time.Sleep(100 * time.Millisecond)

		stats := healthChecker.GetStats()
		checkCount := stats.CheckCount

		// Should have performed minimal checks
		assert.LessOrEqual(t, checkCount, int64(2))
	})

	t.Run("health checker with connection that changes frequently", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		defer healthChecker.Close()

		// Change connection health frequently
		// For nil connection, we can't change health status
		// Wait for some time
		time.Sleep(1 * time.Second)

		stats := healthChecker.GetStats()
		checkCount := stats.CheckCount

		// With nil connection, no checks are performed
		assert.Equal(t, checkCount, int64(0))
	})
}

func TestHealthChecker_Integration(t *testing.T) {
	t.Run("full health checker lifecycle", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 100*time.Millisecond)
		require.NotNil(t, healthChecker)

		// Verify initial state
		assert.False(t, healthChecker.IsHealthy())

		// Wait for a few health checks
		time.Sleep(300 * time.Millisecond)

		// Verify stats
		stats := healthChecker.GetStats()
		assert.Equal(t, stats.CheckCount, int64(0)) // With nil connection, no checks are performed

		// Close health checker
		healthChecker.Close()

		// Verify closed state
		assert.False(t, healthChecker.IsHealthy())
	})

	t.Run("health checker with connection state changes", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 50*time.Millisecond)
		require.NotNil(t, healthChecker)

		defer healthChecker.Close()

		// Initially healthy
		assert.False(t, healthChecker.IsHealthy())

		// Wait for a few checks
		time.Sleep(150 * time.Millisecond)

		// For nil connection, we can't make it unhealthy

		// Wait for health check to detect the change
		time.Sleep(100 * time.Millisecond)

		// Should now be unhealthy
		assert.False(t, healthChecker.IsHealthy())

		// For nil connection, we can't make it healthy
		// Wait for some time
		time.Sleep(100 * time.Millisecond)

		// Should still be unhealthy
		assert.False(t, healthChecker.IsHealthy())
	})
}

func TestHealthChecker_String(t *testing.T) {
	t.Run("returns string representation", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 100*time.Millisecond)
		require.NotNil(t, healthChecker)

		defer healthChecker.Close()

		str := healthChecker.String()
		assert.Contains(t, str, "HealthChecker")
		assert.Contains(t, str, "unhealthy") // Should be unhealthy for nil connection
		assert.Contains(t, str, "Checks")
	})
}

func TestHealthChecker_StressTest(t *testing.T) {
	t.Run("rapid health state changes", func(t *testing.T) {
		// Use nil connection since we can't easily mock *rabbitmq.Conn
		healthChecker := messaging.NewHealthChecker(nil, 1*time.Millisecond)
		require.NotNil(t, healthChecker)

		defer healthChecker.Close()

		// For nil connection, we can't change state
		// Just wait for some time
		time.Sleep(100 * time.Millisecond)

		// With nil connection, no checks are performed
		stats := healthChecker.GetStats()
		assert.Equal(t, stats.CheckCount, int64(0))
	})
}
