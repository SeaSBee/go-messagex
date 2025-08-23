// Package rabbitmq provides RabbitMQ transport implementation for the messaging system.
package rabbitmq

import (
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/seasbee/go-logx"

	"github.com/seasbee/go-messagex/pkg/messaging"
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

// NewConfirmTracker creates a new confirm tracker for a channel.
func NewConfirmTracker(channel *amqp091.Channel, observability *messaging.ObservabilityContext) (*ConfirmTracker, error) {
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
		confirmChan:   make(chan amqp091.Confirmation, 1000),
		returnChan:    make(chan amqp091.Return, 100),
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
		ct.observability.Logger().Warn("received confirmation for unknown delivery tag",
			logx.Int64("delivery_tag", int64(confirm.DeliveryTag)))
		return
	}

	// Record confirmation metrics
	ct.observability.Metrics().ConfirmTotal("rabbitmq")

	// Calculate confirmation duration
	var confirmDuration time.Duration
	if timeExists {
		confirmDuration = time.Since(publishTime)
		ct.observability.Metrics().ConfirmDuration("rabbitmq", confirmDuration)
	}

	result := messaging.PublishResult{
		DeliveryTag: confirm.DeliveryTag,
		Timestamp:   time.Now(),
		Success:     confirm.Ack,
	}

	var err error
	if confirm.Ack {
		ct.observability.Metrics().ConfirmSuccess("rabbitmq")
		ct.observability.RecordPublishMetrics("rabbitmq", "", confirmDuration, true, "")
		receiptMgr.CompleteReceipt("", result, nil)
	} else {
		ct.observability.Metrics().ConfirmFailure("rabbitmq", "nacked")
		err = messaging.NewError(messaging.ErrorCodePublish, "confirm_nack", "message was nacked by broker")
		ct.observability.RecordPublishMetrics("rabbitmq", "", confirmDuration, false, "nacked")
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
	receiptMgr, receiptExists := ct.receipts[deliveryTag]
	publishTime, timeExists := ct.publishTimes[deliveryTag]
	if receiptExists {
		delete(ct.receipts, deliveryTag)
	}
	if timeExists {
		delete(ct.publishTimes, deliveryTag)
	}
	ct.mu.Unlock()

	if !exists || !receiptExists {
		ct.observability.Logger().Warn("received return for unknown message",
			logx.String("message_id", messageID),
			logx.Int64("delivery_tag", int64(deliveryTag)),
			logx.Int("reply_code", int(returnMsg.ReplyCode)),
			logx.String("reply_text", returnMsg.ReplyText))
		return
	}

	// Record return metrics
	ct.observability.Metrics().ReturnTotal("rabbitmq")

	// Calculate return duration
	var returnDuration time.Duration
	if timeExists {
		returnDuration = time.Since(publishTime)
		ct.observability.Metrics().ReturnDuration("rabbitmq", returnDuration)
	}

	ct.observability.RecordPublishMetrics("rabbitmq", returnMsg.Exchange, returnDuration, false, "returned")
	ct.observability.Logger().Warn("message returned as unroutable",
		logx.String("message_id", messageID),
		logx.String("exchange", returnMsg.Exchange),
		logx.String("routing_key", returnMsg.RoutingKey),
		logx.Int("reply_code", int(returnMsg.ReplyCode)),
		logx.String("reply_text", returnMsg.ReplyText))

	// Complete the receipt with an error
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

	// Complete all pending receipts with an error
	for deliveryTag, receiptMgr := range ct.receipts {
		err := messaging.NewError(messaging.ErrorCodePublish, "tracker_closed", "confirm tracker was closed")
		receiptMgr.CompleteReceipt("", messaging.PublishResult{
			DeliveryTag: deliveryTag,
			Timestamp:   time.Now(),
			Success:     false,
			Reason:      "tracker closed",
		}, err)
	}

	// Clear maps
	ct.receipts = make(map[uint64]*messaging.ReceiptManager)
	ct.deliveryTags = make(map[string]uint64)
	ct.publishTimes = make(map[uint64]time.Time)
	ct.mu.Unlock()

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
	ccm.mu.Lock()
	defer ccm.mu.Unlock()

	if tracker, exists := ccm.trackers[channel]; exists {
		delete(ccm.trackers, channel)
		return tracker.Close()
	}

	return nil
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
