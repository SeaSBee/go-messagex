// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"sync"
	"time"

	"github.com/SeaSBee/go-logx"
	"github.com/rabbitmq/amqp091-go"

	"github.com/SeaSBee/go-messagex/pkg/messaging"
)

// ConfirmTracker manages delivery confirmations and returns for a RabbitMQ channel.
type ConfirmTracker struct {
	channel       *amqp091.Channel
	receipts      map[uint64]*messaging.ReceiptManager
	deliveryTags  map[string]uint64
	publishTimes  map[uint64]time.Time // Track publish times for duration metrics
	nextTag       uint64
	mu            sync.RWMutex
	confirmChan   chan amqp091.Confirmation
	returnChan    chan amqp091.Return
	observability *messaging.ObservabilityContext
	closed        bool
	wg            sync.WaitGroup
}

// ConfirmTrackerConfig holds configuration for the confirm tracker.
type ConfirmTrackerConfig struct {
	ConfirmBufferSize int
	ReturnBufferSize  int
}

// DefaultConfirmTrackerConfig returns default configuration.
func DefaultConfirmTrackerConfig() *ConfirmTrackerConfig {
	return &ConfirmTrackerConfig{
		ConfirmBufferSize: 1000,
		ReturnBufferSize:  100,
	}
}

// NewConfirmTracker creates a new confirm tracker for a channel.
func NewConfirmTracker(channel *amqp091.Channel, observability *messaging.ObservabilityContext) (*ConfirmTracker, error) {
	return NewConfirmTrackerWithConfig(channel, observability, DefaultConfirmTrackerConfig())
}

// NewConfirmTrackerWithConfig creates a new confirm tracker with custom configuration.
func NewConfirmTrackerWithConfig(channel *amqp091.Channel, observability *messaging.ObservabilityContext, config *ConfirmTrackerConfig) (*ConfirmTracker, error) {
	// Validate inputs
	if channel == nil {
		return nil, messaging.NewError(messaging.ErrorCodeChannel, "nil_channel", "channel cannot be nil")
	}
	if config == nil {
		config = DefaultConfirmTrackerConfig()
	}

	// Enable confirm mode on the channel
	if err := channel.Confirm(false); err != nil {
		return nil, messaging.WrapError(messaging.ErrorCodeChannel, "enable_confirms", "failed to enable confirm mode", err)
	}

	ct := &ConfirmTracker{
		channel:       channel,
		receipts:      make(map[uint64]*messaging.ReceiptManager),
		deliveryTags:  make(map[string]uint64),
		publishTimes:  make(map[uint64]time.Time),
		nextTag:       1,
		observability: observability,
		confirmChan:   make(chan amqp091.Confirmation, config.ConfirmBufferSize),
		returnChan:    make(chan amqp091.Return, config.ReturnBufferSize),
	}

	// Set up notification channels
	ct.channel.NotifyPublish(ct.confirmChan)
	ct.channel.NotifyReturn(ct.returnChan)

	// Start goroutines to handle confirmations and returns
	ct.wg.Add(2)
	go ct.handleConfirmations()
	go ct.handleReturns()

	return ct, nil
}

// TrackMessage tracks a message for confirmation and returns the delivery tag.
func (ct *ConfirmTracker) TrackMessage(messageID string, receiptMgr *messaging.ReceiptManager) uint64 {
	// Validate inputs
	if receiptMgr == nil {
		if ct.observability != nil {
			ct.observability.Logger().Error("cannot track message with nil receipt manager",
				logx.String("message_id", messageID))
		}
		return 0
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if ct.closed {
		return 0
	}

	deliveryTag := ct.nextTag
	ct.nextTag++

	ct.receipts[deliveryTag] = receiptMgr
	ct.deliveryTags[messageID] = deliveryTag
	ct.publishTimes[deliveryTag] = time.Now()

	return deliveryTag
}

// handleConfirmations processes delivery confirmations from RabbitMQ.
func (ct *ConfirmTracker) handleConfirmations() {
	defer ct.wg.Done()

	for confirm := range ct.confirmChan {
		ct.processConfirmation(confirm)
	}
}

// processConfirmation processes a single confirmation.
func (ct *ConfirmTracker) processConfirmation(confirm amqp091.Confirmation) {
	ct.mu.Lock()
	receiptMgr, exists := ct.receipts[confirm.DeliveryTag]
	publishTime, timeExists := ct.publishTimes[confirm.DeliveryTag]
	if exists {
		delete(ct.receipts, confirm.DeliveryTag)
	}
	if timeExists {
		delete(ct.publishTimes, confirm.DeliveryTag)
	}
	ct.mu.Unlock()

	if !exists {
		if ct.observability != nil {
			ct.observability.Logger().Warn("received confirmation for unknown delivery tag",
				logx.Int64("delivery_tag", int64(confirm.DeliveryTag)))
		}
		return
	}

	// Calculate confirmation duration
	var confirmDuration time.Duration
	if timeExists {
		confirmDuration = time.Since(publishTime)
	}

	// Record confirmation metrics if observability is available
	if ct.observability != nil {
		ct.observability.Metrics().ConfirmTotal("rabbitmq")
		if timeExists {
			ct.observability.Metrics().ConfirmDuration("rabbitmq", confirmDuration)
		}
	}

	result := messaging.PublishResult{
		DeliveryTag: confirm.DeliveryTag,
		Timestamp:   time.Now(),
		Success:     confirm.Ack,
	}

	var err error
	if confirm.Ack {
		if ct.observability != nil {
			ct.observability.Metrics().ConfirmSuccess("rabbitmq")
			ct.observability.RecordPublishMetrics("rabbitmq", "", confirmDuration, true, "")
		}
		receiptMgr.CompleteReceipt("", result, nil)
	} else {
		if ct.observability != nil {
			ct.observability.Metrics().ConfirmFailure("rabbitmq", "nacked")
			ct.observability.RecordPublishMetrics("rabbitmq", "", confirmDuration, false, "nacked")
		}
		err = messaging.NewError(messaging.ErrorCodePublish, "confirm_nack", "message was nacked by broker")
		receiptMgr.CompleteReceipt("", result, err)
	}
}

// handleReturns processes returned (unroutable) messages from RabbitMQ.
func (ct *ConfirmTracker) handleReturns() {
	defer ct.wg.Done()

	for returnMsg := range ct.returnChan {
		ct.processReturn(returnMsg)
	}
}

// processReturn processes a returned message.
func (ct *ConfirmTracker) processReturn(returnMsg amqp091.Return) {
	// Extract message ID from headers
	messageID := ""
	if returnMsg.Headers != nil {
		if id, ok := returnMsg.Headers["message_id"].(string); ok {
			messageID = id
		}
	}

	ct.mu.Lock()
	deliveryTag, exists := ct.deliveryTags[messageID]
	if exists {
		delete(ct.deliveryTags, messageID)
	}

	// Only process if we have a valid delivery tag
	var receiptMgr *messaging.ReceiptManager
	var publishTime time.Time
	var receiptExists, timeExists bool

	if deliveryTag > 0 {
		receiptMgr, receiptExists = ct.receipts[deliveryTag]
		publishTime, timeExists = ct.publishTimes[deliveryTag]
		if receiptExists {
			delete(ct.receipts, deliveryTag)
		}
		if timeExists {
			delete(ct.publishTimes, deliveryTag)
		}
	}
	ct.mu.Unlock()

	if !exists || !receiptExists || deliveryTag == 0 {
		if ct.observability != nil {
			ct.observability.Logger().Warn("received return for unknown message",
				logx.String("message_id", messageID),
				logx.Int64("delivery_tag", int64(deliveryTag)),
				logx.Int("reply_code", int(returnMsg.ReplyCode)),
				logx.String("reply_text", returnMsg.ReplyText))
		}
		return
	}

	// Calculate return duration
	var returnDuration time.Duration
	if timeExists {
		returnDuration = time.Since(publishTime)
	}

	// Record return metrics if observability is available
	if ct.observability != nil {
		ct.observability.Metrics().ReturnTotal("rabbitmq")
		if timeExists {
			ct.observability.Metrics().ReturnDuration("rabbitmq", returnDuration)
		}
		ct.observability.RecordPublishMetrics("rabbitmq", returnMsg.Exchange, returnDuration, false, "returned")
		ct.observability.Logger().Warn("message returned as unroutable",
			logx.String("message_id", messageID),
			logx.String("exchange", returnMsg.Exchange),
			logx.String("routing_key", returnMsg.RoutingKey),
			logx.Int("reply_code", int(returnMsg.ReplyCode)),
			logx.String("reply_text", returnMsg.ReplyText))
	}

	// Complete the receipt with an error (always complete regardless of observability)
	result := messaging.PublishResult{
		DeliveryTag: deliveryTag,
		Timestamp:   time.Now(),
		Success:     false,
		Reason:      returnMsg.ReplyText,
	}

	err := messaging.NewError(messaging.ErrorCodePublish, "message_returned",
		"message was returned as unroutable: "+returnMsg.ReplyText)
	receiptMgr.CompleteReceipt("", result, err)
}

// Close closes the confirm tracker and cleans up resources.
func (ct *ConfirmTracker) Close() error {
	ct.mu.Lock()
	if ct.closed {
		ct.mu.Unlock()
		return nil
	}
	ct.closed = true

	// Collect pending receipts to complete them after releasing the lock
	pendingReceipts := make([]struct {
		deliveryTag uint64
		receiptMgr  *messaging.ReceiptManager
	}, 0, len(ct.receipts))

	for deliveryTag, receiptMgr := range ct.receipts {
		pendingReceipts = append(pendingReceipts, struct {
			deliveryTag uint64
			receiptMgr  *messaging.ReceiptManager
		}{deliveryTag, receiptMgr})
	}

	// Clear maps
	ct.receipts = make(map[uint64]*messaging.ReceiptManager)
	ct.deliveryTags = make(map[string]uint64)
	ct.publishTimes = make(map[uint64]time.Time)
	ct.mu.Unlock()

	// Complete all pending receipts with an error (without holding the lock)
	for _, pending := range pendingReceipts {
		err := messaging.NewError(messaging.ErrorCodePublish, "tracker_closed", "confirm tracker was closed")
		pending.receiptMgr.CompleteReceipt("", messaging.PublishResult{
			DeliveryTag: pending.deliveryTag,
			Timestamp:   time.Now(),
			Success:     false,
			Reason:      "tracker closed",
		}, err)
	}

	// Close notification channels
	close(ct.confirmChan)
	close(ct.returnChan)

	// Wait for goroutines to finish
	ct.wg.Wait()

	return nil
}

// PendingCount returns the number of pending confirmations.
func (ct *ConfirmTracker) PendingCount() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.receipts)
}

// ChannelConfirmManager manages confirm trackers for multiple channels.
type ChannelConfirmManager struct {
	trackers map[*amqp091.Channel]*ConfirmTracker
	mu       sync.RWMutex
}

// NewChannelConfirmManager creates a new channel confirm manager.
func NewChannelConfirmManager() *ChannelConfirmManager {
	return &ChannelConfirmManager{
		trackers: make(map[*amqp091.Channel]*ConfirmTracker),
	}
}

// GetOrCreateTracker gets or creates a confirm tracker for a channel.
func (ccm *ChannelConfirmManager) GetOrCreateTracker(channel *amqp091.Channel, observability *messaging.ObservabilityContext) (*ConfirmTracker, error) {
	if channel == nil {
		return nil, messaging.NewError(messaging.ErrorCodeChannel, "nil_channel", "channel cannot be nil")
	}

	ccm.mu.Lock()
	defer ccm.mu.Unlock()

	if tracker, exists := ccm.trackers[channel]; exists {
		return tracker, nil
	}

	tracker, err := NewConfirmTracker(channel, observability)
	if err != nil {
		return nil, err
	}

	ccm.trackers[channel] = tracker
	return tracker, nil
}

// RemoveTracker removes a tracker for a channel.
func (ccm *ChannelConfirmManager) RemoveTracker(channel *amqp091.Channel) error {
	if channel == nil {
		return messaging.NewError(messaging.ErrorCodeChannel, "nil_channel", "channel cannot be nil")
	}

	ccm.mu.Lock()
	tracker, exists := ccm.trackers[channel]
	if !exists {
		ccm.mu.Unlock()
		return nil
	}

	// Don't remove from map yet - only remove if Close() succeeds
	ccm.mu.Unlock()

	// Close the tracker
	err := tracker.Close()

	// Only remove from map if Close() succeeded
	if err == nil {
		ccm.mu.Lock()
		delete(ccm.trackers, channel)
		ccm.mu.Unlock()
	}

	return err
}

// Close closes all trackers.
func (ccm *ChannelConfirmManager) Close() error {
	ccm.mu.Lock()
	defer ccm.mu.Unlock()

	var lastErr error
	for channel, tracker := range ccm.trackers {
		if err := tracker.Close(); err != nil {
			lastErr = err
		}
		delete(ccm.trackers, channel)
	}

	return lastErr
}

// PendingCount returns the total number of pending confirmations across all trackers.
func (ccm *ChannelConfirmManager) PendingCount() int {
	ccm.mu.RLock()
	defer ccm.mu.RUnlock()

	total := 0
	for _, tracker := range ccm.trackers {
		total += tracker.PendingCount()
	}
	return total
}
