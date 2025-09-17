package unit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seasbee/go-messagex/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

func TestReceipt(t *testing.T) {
	t.Run("NewReceipt", func(t *testing.T) {
		// Test valid receipt creation
		ctx := context.Background()
		timeout := 5 * time.Second
		receipt := messaging.NewReceipt(ctx, "test-id", timeout)

		assert.NotNil(t, receipt)
		assert.Equal(t, "test-id", receipt.ID())
		assert.NotNil(t, receipt.Context())
		assert.NotNil(t, receipt.Done())

		// Test with cancelled context
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		receipt = messaging.NewReceipt(cancelledCtx, "test-id-2", timeout)
		assert.NotNil(t, receipt)

		// Wait for receipt to complete due to cancelled context
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			assert.Error(t, err)
			assert.Equal(t, context.Canceled, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately with cancelled context")
		}
	})

	t.Run("ReceiptID", func(t *testing.T) {
		receipt := messaging.NewReceipt(context.Background(), "unique-id", 5*time.Second)
		assert.Equal(t, "unique-id", receipt.ID())
	})

	t.Run("ReceiptContext", func(t *testing.T) {
		ctx := context.Background()
		receipt := messaging.NewReceipt(ctx, "test-id", 5*time.Second)

		receiptCtx := receipt.Context()
		assert.NotNil(t, receiptCtx)
		assert.NotEqual(t, ctx, receiptCtx) // Should be a child context with timeout
	})

	t.Run("ReceiptDone", func(t *testing.T) {
		receipt := messaging.NewReceipt(context.Background(), "test-id", 5*time.Second)

		done := receipt.Done()
		assert.NotNil(t, done)

		// Initially should not be closed
		select {
		case <-done:
			t.Fatal("Done channel should not be closed initially")
		default:
			// Expected
		}
	})

	t.Run("ReceiptResult", func(t *testing.T) {
		receipt := messaging.NewReceipt(context.Background(), "test-id", 5*time.Second)

		// Initially should return zero values
		result, err := receipt.Result()
		assert.Empty(t, result.MessageID)
		assert.NoError(t, err)
	})

	t.Run("CompleteSuccess", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		receipt := manager.CreateReceipt(context.Background(), "test-id")

		expectedResult := messaging.PublishResult{
			MessageID:   "test-id",
			DeliveryTag: 123,
			Timestamp:   time.Now(),
			Success:     true,
			Reason:      "success",
		}

		// Complete the receipt through manager
		completed := manager.CompleteReceipt("test-id", expectedResult, nil)
		assert.True(t, completed)

		// Wait for completion
		select {
		case <-receipt.Done():
			result, err := receipt.Result()
			assert.NoError(t, err)
			assert.Equal(t, expectedResult.MessageID, result.MessageID)
			assert.Equal(t, expectedResult.DeliveryTag, result.DeliveryTag)
			assert.Equal(t, expectedResult.Success, result.Success)
			assert.Equal(t, expectedResult.Reason, result.Reason)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately")
		}
	})

	t.Run("CompleteError", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		receipt := manager.CreateReceipt(context.Background(), "test-id")

		expectedError := errors.New("test error")

		// Complete the receipt with error through manager
		completed := manager.CompleteReceipt("test-id", messaging.PublishResult{}, expectedError)
		assert.True(t, completed)

		// Wait for completion
		select {
		case <-receipt.Done():
			result, err := receipt.Result()
			assert.Error(t, err)
			assert.Equal(t, expectedError, err)
			assert.Empty(t, result.MessageID)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately")
		}
	})

	t.Run("CompleteTimeout", func(t *testing.T) {
		// Test timeout completion by creating a receipt with very short timeout
		receipt := messaging.NewReceipt(context.Background(), "test-id", 10*time.Millisecond)

		// Wait for context timeout
		select {
		case <-receipt.Context().Done():
			// Context timed out, but receipt is not automatically completed
			// The receipt should still be pending
			select {
			case <-receipt.Done():
				t.Fatal("Receipt should not be completed automatically on context timeout")
			default:
				// Expected - receipt is still pending
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Context should timeout")
		}

		// Verify that the receipt is still pending
		result, err := receipt.Result()
		assert.Empty(t, result.MessageID)
		assert.NoError(t, err)
	})

	t.Run("DoubleCompletion", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		receipt := manager.CreateReceipt(context.Background(), "test-id")

		// Complete the receipt first time
		completed := manager.CompleteReceipt("test-id", messaging.PublishResult{
			MessageID: "test-id",
			Success:   true,
		}, nil)
		assert.True(t, completed)

		// Wait for first completion
		<-receipt.Done()

		// Try to complete again (should be ignored)
		completed = manager.CompleteReceipt("test-id", messaging.PublishResult{}, errors.New("second error"))
		assert.True(t, completed) // Manager allows re-completion

		// Should still have the first result
		result, err := receipt.Result()
		assert.NoError(t, err)
		assert.Equal(t, "test-id", result.MessageID)
		assert.True(t, result.Success)
	})

	t.Run("ConcurrentCompletion", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		receipt := manager.CreateReceipt(context.Background(), "test-id")

		var wg sync.WaitGroup
		wg.Add(2)

		// Try to complete concurrently
		go func() {
			defer wg.Done()
			manager.CompleteReceipt("test-id", messaging.PublishResult{
				MessageID: "test-id",
				Success:   true,
			}, nil)
		}()

		go func() {
			defer wg.Done()
			manager.CompleteReceipt("test-id", messaging.PublishResult{}, errors.New("concurrent error"))
		}()

		wg.Wait()

		// Wait for completion
		<-receipt.Done()

		// Should have completed (one of the completions)
		_, err := receipt.Result()
		// Either success or error, but should not panic
		assert.NotNil(t, err)
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		// Create a receipt with a very short timeout
		receipt := messaging.NewReceipt(context.Background(), "test-id", 10*time.Millisecond)

		// Wait for context timeout
		select {
		case <-receipt.Context().Done():
			// Context timed out, but receipt is not automatically completed
			// The receipt should still be pending
			select {
			case <-receipt.Done():
				t.Fatal("Receipt should not be completed automatically on context timeout")
			default:
				// Expected - receipt is still pending
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Context should timeout")
		}

		// Verify that the receipt is still pending
		result, err := receipt.Result()
		assert.Empty(t, result.MessageID)
		assert.NoError(t, err)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		receipt := messaging.NewReceipt(ctx, "test-id", 5*time.Second)

		// Cancel the context
		cancel()

		// Wait for context cancellation
		select {
		case <-receipt.Context().Done():
			// Context is cancelled, but receipt is not automatically completed
			// The receipt should still be pending
			select {
			case <-receipt.Done():
				t.Fatal("Receipt should not be completed automatically on context cancellation")
			default:
				// Expected - receipt is still pending
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context should be cancelled")
		}

		// Verify that the receipt is still pending
		result, err := receipt.Result()
		assert.Empty(t, result.MessageID)
		assert.NoError(t, err)
	})
}

func TestReceiptManager(t *testing.T) {
	t.Run("NewReceiptManager", func(t *testing.T) {
		timeout := 30 * time.Second
		manager := messaging.NewReceiptManager(timeout)

		assert.NotNil(t, manager)
		assert.Equal(t, 0, manager.PendingCount())
	})

	t.Run("CreateReceipt", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt
		receipt := manager.CreateReceipt(ctx, "test-id")
		assert.NotNil(t, receipt)
		assert.Equal(t, "test-id", receipt.ID())
		assert.Equal(t, 1, manager.PendingCount())

		// Create another receipt
		receipt2 := manager.CreateReceipt(ctx, "test-id-2")
		assert.NotNil(t, receipt2)
		assert.Equal(t, "test-id-2", receipt2.ID())
		assert.Equal(t, 2, manager.PendingCount())
	})

	t.Run("GetReceipt", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt
		createdReceipt := manager.CreateReceipt(ctx, "test-id")

		// Get the receipt
		receipt, exists := manager.GetReceipt("test-id")
		assert.True(t, exists)
		assert.Equal(t, createdReceipt, receipt)

		// Get non-existent receipt
		receipt, exists = manager.GetReceipt("non-existent")
		assert.False(t, exists)
		assert.Nil(t, receipt)
	})

	t.Run("CompleteReceipt", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt
		receipt := manager.CreateReceipt(ctx, "test-id")

		// Complete the receipt
		result := messaging.PublishResult{
			MessageID: "test-id",
			Success:   true,
		}
		completed := manager.CompleteReceipt("test-id", result, nil)
		assert.True(t, completed)

		// Wait for completion
		select {
		case <-receipt.Done():
			receivedResult, err := receipt.Result()
			assert.NoError(t, err)
			assert.Equal(t, "test-id", receivedResult.MessageID)
			assert.True(t, receivedResult.Success)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately")
		}

		// Try to complete non-existent receipt
		completed = manager.CompleteReceipt("non-existent", result, nil)
		assert.False(t, completed)
	})

	t.Run("CompleteReceiptWithError", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt
		receipt := manager.CreateReceipt(ctx, "test-id")

		// Complete the receipt with error
		expectedError := errors.New("test error")
		completed := manager.CompleteReceipt("test-id", messaging.PublishResult{}, expectedError)
		assert.True(t, completed)

		// Wait for completion
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			assert.Error(t, err)
			assert.Equal(t, expectedError, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately")
		}
	})

	t.Run("PendingCount", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Initially should be 0
		assert.Equal(t, 0, manager.PendingCount())

		// Create receipts
		manager.CreateReceipt(ctx, "test-id-1")
		assert.Equal(t, 1, manager.PendingCount())

		manager.CreateReceipt(ctx, "test-id-2")
		assert.Equal(t, 2, manager.PendingCount())

		manager.CreateReceipt(ctx, "test-id-3")
		assert.Equal(t, 3, manager.PendingCount())

		// Complete a receipt
		manager.CompleteReceipt("test-id-1", messaging.PublishResult{}, nil)

		// Wait for cleanup
		time.Sleep(50 * time.Millisecond)

		// Count should decrease after cleanup
		assert.LessOrEqual(t, manager.PendingCount(), 2)
	})

	t.Run("Close", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create some receipts
		receipt1 := manager.CreateReceipt(ctx, "test-id-1")
		receipt2 := manager.CreateReceipt(ctx, "test-id-2")

		assert.Equal(t, 2, manager.PendingCount())

		// Close the manager
		manager.Close()

		// All receipts should be completed with error
		select {
		case <-receipt1.Done():
			_, err := receipt1.Result()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "publisher is closed")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete due to manager closure")
		}

		select {
		case <-receipt2.Done():
			_, err := receipt2.Result()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "publisher is closed")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete due to manager closure")
		}

		// Pending count should be 0
		assert.Equal(t, 0, manager.PendingCount())

		// Double close should not panic
		manager.Close()
	})

	t.Run("CreateReceiptAfterClose", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Close the manager
		manager.Close()

		// Try to create a receipt after closure
		receipt := manager.CreateReceipt(ctx, "test-id")
		assert.NotNil(t, receipt)

		// Receipt should be completed with error immediately
		select {
		case <-receipt.Done():
			_, err := receipt.Result()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "publisher is closed")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete immediately with error")
		}
	})

	t.Run("DrainWithTimeout", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt
		receipt := manager.CreateReceipt(ctx, "test-id")

		// Complete the receipt
		manager.CompleteReceipt("test-id", messaging.PublishResult{}, nil)

		// Wait for completion
		<-receipt.Done()

		// Wait for cleanup
		time.Sleep(500 * time.Millisecond)

		// Drain with timeout - should succeed since receipt is completed
		err := manager.DrainWithTimeout(1 * time.Second)
		assert.NoError(t, err)
	})

	t.Run("DrainWithTimeoutExpired", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt but don't complete it
		manager.CreateReceipt(ctx, "test-id")

		// Try to drain with very short timeout
		err := manager.DrainWithTimeout(10 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TIMEOUT")
	})

	t.Run("DrainWithTimeoutAfterClose", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create a receipt but don't complete it
		_ = manager.CreateReceipt(ctx, "test-id")

		// Close the manager
		manager.Close()

		// Try to drain - should return nil because all receipts are completed when manager is closed
		err := manager.DrainWithTimeout(1 * time.Second)
		assert.NoError(t, err)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		const numGoroutines = 10
		const operationsPerGoroutine = 10

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					receiptID := fmt.Sprintf("receipt-%d-%d", id, j)
					receipt := manager.CreateReceipt(ctx, receiptID)
					assert.NotNil(t, receipt)

					// Complete the receipt
					result := messaging.PublishResult{
						MessageID: receiptID,
						Success:   true,
					}
					manager.CompleteReceipt(receiptID, result, nil)
				}
			}(i)
		}

		wg.Wait()

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		// Drain remaining receipts
		err := manager.DrainWithTimeout(1 * time.Second)
		assert.NoError(t, err)
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("AwaitAll", func(t *testing.T) {
		// Test with no receipts
		err := messaging.AwaitAll(context.Background())
		assert.NoError(t, err)

		// Test with nil receipts
		err = messaging.AwaitAll(context.Background(), nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid message")

		// Test with valid receipts using managers
		manager1 := messaging.NewReceiptManager(5 * time.Second)
		manager2 := messaging.NewReceiptManager(5 * time.Second)
		receipt1 := manager1.CreateReceipt(context.Background(), "test-1")
		receipt2 := manager2.CreateReceipt(context.Background(), "test-2")

		go func() {
			time.Sleep(10 * time.Millisecond)
			manager1.CompleteReceipt("test-1", messaging.PublishResult{MessageID: "test-1"}, nil)
		}()

		go func() {
			time.Sleep(20 * time.Millisecond)
			manager2.CompleteReceipt("test-2", messaging.PublishResult{MessageID: "test-2"}, nil)
		}()

		// Await all
		err = messaging.AwaitAll(context.Background(), receipt1, receipt2)
		assert.NoError(t, err)
	})

	t.Run("AwaitAllWithContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		receipt := messaging.NewReceipt(context.Background(), "test", 5*time.Second)

		// Cancel context immediately
		cancel()

		// Await all
		err := messaging.AwaitAll(ctx, receipt)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("AwaitAny", func(t *testing.T) {
		// Test with no receipts
		receipt, err := messaging.AwaitAny(context.Background())
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "invalid message")

		// Test with nil receipts
		receipt, err = messaging.AwaitAny(context.Background(), nil, nil)
		assert.Error(t, err)
		assert.Nil(t, receipt)
		assert.Contains(t, err.Error(), "invalid message")

		// Test with valid receipts using managers
		manager1 := messaging.NewReceiptManager(5 * time.Second)
		manager2 := messaging.NewReceiptManager(5 * time.Second)
		receipt1 := manager1.CreateReceipt(context.Background(), "test-1")
		receipt2 := manager2.CreateReceipt(context.Background(), "test-2")

		// Complete one receipt using manager
		go func() {
			time.Sleep(10 * time.Millisecond)
			manager1.CompleteReceipt("test-1", messaging.PublishResult{MessageID: "test-1"}, nil)
		}()

		// Await any
		completedReceipt, err := messaging.AwaitAny(context.Background(), receipt1, receipt2)
		assert.NoError(t, err)
		assert.Equal(t, receipt1, completedReceipt)
	})

	t.Run("AwaitAnyWithContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		receipt := messaging.NewReceipt(context.Background(), "test", 5*time.Second)

		// Cancel context immediately
		cancel()

		// Await any
		completedReceipt, err := messaging.AwaitAny(ctx, receipt)
		assert.Error(t, err)
		assert.Nil(t, completedReceipt)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("GetResults", func(t *testing.T) {
		// Test with no receipts
		results, errs := messaging.GetResults()
		assert.Empty(t, results)
		assert.Empty(t, errs)

		// Test with nil receipts
		results, failures := messaging.GetResults(nil, nil)
		assert.Len(t, results, 2)
		assert.Len(t, failures, 2)
		assert.Error(t, failures[0])
		assert.Error(t, failures[1])

		// Test with valid receipts using managers
		manager1 := messaging.NewReceiptManager(5 * time.Second)
		manager2 := messaging.NewReceiptManager(5 * time.Second)
		receipt1 := manager1.CreateReceipt(context.Background(), "test-1")
		receipt2 := manager2.CreateReceipt(context.Background(), "test-2")

		// Complete receipts
		manager1.CompleteReceipt("test-1", messaging.PublishResult{MessageID: "test-1", Success: true}, nil)
		manager2.CompleteReceipt("test-2", messaging.PublishResult{}, errors.New("test error"))

		// Get results
		results, failures = messaging.GetResults(receipt1, receipt2)
		assert.Len(t, results, 2)
		assert.Len(t, failures, 2)

		assert.Equal(t, "test-1", results[0].MessageID)
		assert.True(t, results[0].Success)
		assert.NoError(t, failures[0])

		assert.Empty(t, results[1].MessageID)
		assert.Error(t, failures[1])
		assert.Contains(t, failures[1].Error(), "test error")
	})

	t.Run("AllSuccessful", func(t *testing.T) {
		// Test with no receipts
		successful := messaging.AllSuccessful()
		assert.True(t, successful)

		// Test with nil receipts
		successful = messaging.AllSuccessful(nil, nil)
		assert.False(t, successful)

		// Test with all successful receipts using managers
		manager1 := messaging.NewReceiptManager(5 * time.Second)
		manager2 := messaging.NewReceiptManager(5 * time.Second)
		manager3 := messaging.NewReceiptManager(5 * time.Second)
		receipt1 := manager1.CreateReceipt(context.Background(), "test-1")
		receipt2 := manager2.CreateReceipt(context.Background(), "test-2")
		receipt3 := manager3.CreateReceipt(context.Background(), "test-3")

		manager1.CompleteReceipt("test-1", messaging.PublishResult{MessageID: "test-1", Success: true}, nil)
		manager2.CompleteReceipt("test-2", messaging.PublishResult{MessageID: "test-2", Success: true}, nil)

		successful = messaging.AllSuccessful(receipt1, receipt2)
		assert.True(t, successful)

		// Test with one failed receipt
		manager3.CompleteReceipt("test-3", messaging.PublishResult{}, errors.New("test error"))

		successful = messaging.AllSuccessful(receipt1, receipt2, receipt3)
		assert.False(t, successful)
	})

	t.Run("AnyFailed", func(t *testing.T) {
		// Test with no receipts
		failed := messaging.AnyFailed()
		assert.False(t, failed)

		// Test with nil receipts
		failed = messaging.AnyFailed(nil, nil)
		assert.True(t, failed)

		// Test with all successful receipts using managers
		manager1 := messaging.NewReceiptManager(5 * time.Second)
		manager2 := messaging.NewReceiptManager(5 * time.Second)
		manager3 := messaging.NewReceiptManager(5 * time.Second)
		receipt1 := manager1.CreateReceipt(context.Background(), "test-1")
		receipt2 := manager2.CreateReceipt(context.Background(), "test-2")
		receipt3 := manager3.CreateReceipt(context.Background(), "test-3")

		manager1.CompleteReceipt("test-1", messaging.PublishResult{MessageID: "test-1", Success: true}, nil)
		manager2.CompleteReceipt("test-2", messaging.PublishResult{MessageID: "test-2", Success: true}, nil)

		failed = messaging.AnyFailed(receipt1, receipt2)
		assert.False(t, failed)

		// Test with one failed receipt
		manager3.CompleteReceipt("test-3", messaging.PublishResult{}, errors.New("test error"))

		failed = messaging.AnyFailed(receipt1, receipt2, receipt3)
		assert.True(t, failed)
	})
}

func TestReceiptIntegration(t *testing.T) {
	t.Run("ReceiptLifecycle", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create receipt
		receipt := manager.CreateReceipt(ctx, "test-id")
		assert.Equal(t, 1, manager.PendingCount())

		// Complete receipt
		result := messaging.PublishResult{
			MessageID: "test-id",
			Success:   true,
		}
		manager.CompleteReceipt("test-id", result, nil)

		// Wait for completion
		select {
		case <-receipt.Done():
			receivedResult, err := receipt.Result()
			assert.NoError(t, err)
			assert.Equal(t, "test-id", receivedResult.MessageID)
			assert.True(t, receivedResult.Success)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Receipt should complete")
		}

		// Wait for cleanup
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 0, manager.PendingCount())

		// Close manager
		manager.Close()
	})

	t.Run("MultipleReceipts", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create multiple receipts
		receipts := make([]messaging.Receipt, 5)
		for i := 0; i < 5; i++ {
			receipts[i] = manager.CreateReceipt(ctx, fmt.Sprintf("test-id-%d", i))
		}

		assert.Equal(t, 5, manager.PendingCount())

		// Complete some receipts
		for i := 0; i < 3; i++ {
			result := messaging.PublishResult{
				MessageID: fmt.Sprintf("test-id-%d", i),
				Success:   true,
			}
			manager.CompleteReceipt(fmt.Sprintf("test-id-%d", i), result, nil)
		}

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		// Check pending count
		assert.LessOrEqual(t, manager.PendingCount(), 2)

		// Complete remaining receipts
		for i := 3; i < 5; i++ {
			result := messaging.PublishResult{
				MessageID: fmt.Sprintf("test-id-%d", i),
				Success:   true,
			}
			manager.CompleteReceipt(fmt.Sprintf("test-id-%d", i), result, nil)
		}

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		// Drain remaining receipts
		err := manager.DrainWithTimeout(1 * time.Second)
		assert.NoError(t, err)

		// Close manager
		manager.Close()
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		// Create receipts
		receipt1 := manager.CreateReceipt(ctx, "success-id")
		receipt2 := manager.CreateReceipt(ctx, "error-id")

		// Complete with success and error
		manager.CompleteReceipt("success-id", messaging.PublishResult{MessageID: "success-id", Success: true}, nil)
		manager.CompleteReceipt("error-id", messaging.PublishResult{}, errors.New("test error"))

		// Wait for completions
		<-receipt1.Done()
		<-receipt2.Done()

		// Check results
		_, err1 := receipt1.Result()
		assert.NoError(t, err1)

		_, err2 := receipt2.Result()
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "test error")

		// Test utility functions
		successful := messaging.AllSuccessful(receipt1, receipt2)
		assert.False(t, successful)

		failed := messaging.AnyFailed(receipt1, receipt2)
		assert.True(t, failed)

		// Close manager
		manager.Close()
	})

	t.Run("StressTest", func(t *testing.T) {
		manager := messaging.NewReceiptManager(5 * time.Second)
		ctx := context.Background()

		const numReceipts = 100
		receipts := make([]messaging.Receipt, numReceipts)

		// Create receipts concurrently
		var wg sync.WaitGroup
		wg.Add(numReceipts)

		for i := 0; i < numReceipts; i++ {
			go func(id int) {
				defer wg.Done()
				receipts[id] = manager.CreateReceipt(ctx, fmt.Sprintf("stress-test-%d", id))
			}(i)
		}

		wg.Wait()

		assert.Equal(t, numReceipts, manager.PendingCount())

		// Complete receipts concurrently
		wg.Add(numReceipts)

		for i := 0; i < numReceipts; i++ {
			go func(id int) {
				defer wg.Done()
				result := messaging.PublishResult{
					MessageID: fmt.Sprintf("stress-test-%d", id),
					Success:   id%2 == 0, // Alternate success/failure
				}
				var err error
				if id%2 != 0 {
					err = errors.New("stress test error")
				}
				manager.CompleteReceipt(fmt.Sprintf("stress-test-%d", id), result, err)
			}(i)
		}

		wg.Wait()

		// Wait for cleanup
		time.Sleep(200 * time.Millisecond)

		// Drain remaining receipts
		err := manager.DrainWithTimeout(2 * time.Second)
		assert.NoError(t, err)

		// Close manager
		manager.Close()
	})
}
