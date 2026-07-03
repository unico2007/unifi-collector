package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventCollector reads events from an EventSource and ships them to a LogSink.
// It tracks the timestamp of the most recent event it has seen so that each
// cycle only forwards genuinely new events (avoiding duplicate log lines).
type EventCollector struct {
	src  EventSource
	sink LogSink
	log  *zap.Logger

	// lookback bounds the very first fetch (before any lastSeen exists).
	lookback time.Duration

	mu       sync.Mutex
	lastSeen time.Time
}

// NewEventCollector wires the dependencies. lookback controls how far back the
// first collection reaches (e.g. 1h); pass 0 for a sensible default.
func NewEventCollector(src EventSource, sink LogSink, log *zap.Logger, lookback time.Duration) *EventCollector {
	if lookback <= 0 {
		lookback = time.Hour
	}
	return &EventCollector{src: src, sink: sink, log: log, lookback: lookback}
}

func (c *EventCollector) Name() string { return "events" }

func (c *EventCollector) Collect(ctx context.Context) error {
	c.mu.Lock()
	since := c.lastSeen
	c.mu.Unlock()
	if since.IsZero() {
		since = time.Now().Add(-c.lookback)
	}

	events, err := c.src.Events(ctx, since)
	if err != nil {
		return fmt.Errorf("event collector: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	if err := c.sink.WriteEvents(ctx, events); err != nil {
		return fmt.Errorf("event collector: writing events: %w", err)
	}

	// Advance the watermark to the newest event timestamp.
	newest := since
	for _, e := range events {
		if e.Timestamp.After(newest) {
			newest = e.Timestamp
		}
	}
	c.mu.Lock()
	c.lastSeen = newest
	c.mu.Unlock()

	c.log.Debug("collected events", zap.Int("count", len(events)))
	return nil
}
