// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"context"
	"sync"
	"time"
)

// receipt implements the Receipt interface.
type receipt struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	result  PublishResult
	err     error
	mu      sync.RWMutex
	timeout time.Duration
}

// NewReceipt creates a new receipt for tracking async publish operations.
func NewReceipt(ctx context.Context, id string, timeout time.Duration) Receipt {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return &receipt{
		id:      id,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
		timeout: timeout,
	}
}

// ID returns the unique identifier for this receipt.
func (r *receipt) ID() string {
	return r.id
}

// Done returns a channel that's closed when the publish operation completes.
func (r *receipt) Done() <-chan struct{} {
	return r.done
}

// Result returns the publish result and any error.
func (r *receipt) Result() (PublishResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.result, r.err
}

// Context returns the context associated with this receipt.
func (r *receipt) Context() context.Context {
	return r.ctx
}

// complete marks the receipt as complete with the given result.
func (r *receipt) complete(result PublishResult, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-r.done:
		// Already completed
		return
	default:
		r.result = result
		r.err = err
		r.cancel()
		close(r.done)
	}
}

// CompleteSuccess marks the receipt as successfully completed.
func (r *receipt) CompleteSuccess(result PublishResult) {
	r.complete(result, nil)
}

// CompleteError marks the receipt as completed with an error.
func (r *receipt) CompleteError(err error) {
	r.complete(PublishResult{}, err)
}

// CompleteTimeout marks the receipt as completed due to timeout.
func (r *receipt) CompleteTimeout() {
	r.complete(PublishResult{}, TimeoutError("publish", ErrTimeout))
}

// ReceiptManager manages multiple receipts for async operations.
type ReceiptManager struct {
	receipts map[string]*receipt
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewReceiptManager creates a new receipt manager.
func NewReceiptManager(timeout time.Duration) *ReceiptManager {
	return &ReceiptManager{
		receipts: make(map[string]*receipt),
		timeout:  timeout,
	}
}

// CreateReceipt creates a new receipt and registers it with the manager.
func (rm *ReceiptManager) CreateReceipt(ctx context.Context, id string) Receipt {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r := NewReceipt(ctx, id, rm.timeout).(*receipt)
	rm.receipts[id] = r

	// Start a goroutine to clean up the receipt after completion
	go rm.cleanupReceipt(r)

	return r
}

// GetReceipt retrieves a receipt by ID.
func (rm *ReceiptManager) GetReceipt(id string) (Receipt, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	r, exists := rm.receipts[id]
	return r, exists
}

// CompleteReceipt marks a receipt as complete.
func (rm *ReceiptManager) CompleteReceipt(id string, result PublishResult, err error) bool {
	rm.mu.RLock()
	r, exists := rm.receipts[id]
	rm.mu.RUnlock()

	if !exists {
		return false
	}

	r.complete(result, err)
	return true
}

// cleanupReceipt removes a receipt from the manager after it's complete.
func (rm *ReceiptManager) cleanupReceipt(r *receipt) {
	<-r.Done()

	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.receipts, r.id)
}

// PendingCount returns the number of pending receipts.
func (rm *ReceiptManager) PendingCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.receipts)
}

// Close closes all pending receipts and cleans up the manager.
func (rm *ReceiptManager) Close() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, r := range rm.receipts {
		r.CompleteError(ErrPublisherClosed)
	}

	// Clear the map
	rm.receipts = make(map[string]*receipt)
}

// DrainWithTimeout waits for all pending receipts to complete or times out.
func (rm *ReceiptManager) DrainWithTimeout(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		count := rm.PendingCount()
		if count == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return TimeoutError("drain_receipts", ErrTimeout)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// AwaitAll waits for all provided receipts to complete.
func AwaitAll(ctx context.Context, receipts ...Receipt) error {
	if len(receipts) == 0 {
		return nil
	}

	// Create a channel to signal completion
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Wait for all receipts
	for _, r := range receipts {
		wg.Add(1)
		go func(receipt Receipt) {
			defer wg.Done()
			select {
			case <-receipt.Done():
			case <-ctx.Done():
			}
		}(r)
	}

	// Signal when all are done
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or context cancellation
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AwaitAny waits for any of the provided receipts to complete.
func AwaitAny(ctx context.Context, receipts ...Receipt) (Receipt, error) {
	if len(receipts) == 0 {
		return nil, ErrInvalidMessage
	}

	// Create a channel to signal first completion
	done := make(chan Receipt, len(receipts))

	// Wait for any receipt
	for _, r := range receipts {
		go func(receipt Receipt) {
			select {
			case <-receipt.Done():
				select {
				case done <- receipt:
				default:
					// Channel is full, another receipt already completed
				}
			case <-ctx.Done():
			}
		}(r)
	}

	// Wait for first completion or context cancellation
	select {
	case receipt := <-done:
		return receipt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetResults extracts results from multiple receipts.
func GetResults(receipts ...Receipt) ([]PublishResult, []error) {
	results := make([]PublishResult, len(receipts))
	errors := make([]error, len(receipts))

	for i, r := range receipts {
		results[i], errors[i] = r.Result()
	}

	return results, errors
}

// AllSuccessful checks if all receipts completed successfully.
func AllSuccessful(receipts ...Receipt) bool {
	for _, r := range receipts {
		_, err := r.Result()
		if err != nil {
			return false
		}
	}
	return true
}

// AnyFailed checks if any receipt failed.
func AnyFailed(receipts ...Receipt) bool {
	for _, r := range receipts {
		_, err := r.Result()
		if err != nil {
			return true
		}
	}
	return false
}
