// Package loki is the Loki push exporter. It implements collector.LogSink by
// buffering events and shipping them to Loki's push API in batches. A single
// background goroutine (Run) owns the HTTP I/O; WriteEvents merely enqueues,
// so collectors are never blocked on network latency.
//
// This package does not import internal/metrics; asynchronous push failures are
// logged rather than surfaced synchronously.
package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// Compile-time proof that *Exporter satisfies the LogSink contract.
var _ collector.LogSink = (*Exporter)(nil)

// Config configures the exporter.
type Config struct {
	URL        string            // full push URL, e.g. http://loki:3100/loki/api/v1/push
	BatchSize  int               // flush when this many events are buffered
	BatchWait  time.Duration     // flush at least this often
	Tenant     string            // optional X-Scope-OrgID
	Timeout    time.Duration     // per-push HTTP timeout
	MaxRetries int               // push retry attempts
	Labels     map[string]string // static extra stream labels (e.g. job=...)
}

// Exporter buffers and ships events to Loki.
type Exporter struct {
	cfg Config
	log *zap.Logger
	hc  *http.Client
	ch  chan models.Event
}

// lokiPush is the Loki push API request body.
type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"` // [ nanosecond_ts, line ]
}

// logLine is the JSON structure written as each Loki log line. High-cardinality
// fields live here (not as labels) so LogQL can filter them via `| json`.
type logLine struct {
	Event     string `json:"event"`
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	Vendor    string `json:"vendor,omitempty"`
	Site      string `json:"site,omitempty"`
	Device    string `json:"device,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Model     string `json:"model,omitempty"`
	ClientMAC string `json:"client_mac,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

// New builds an exporter. It does not start shipping until Run is called.
func New(cfg Config, log *zap.Logger) *Exporter {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.BatchWait <= 0 {
		cfg.BatchWait = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	return &Exporter{
		cfg: cfg,
		log: log,
		hc:  &http.Client{Timeout: cfg.Timeout},
		// Generous buffer so brief Loki hiccups don't block collectors.
		ch: make(chan models.Event, cfg.BatchSize*10),
	}
}

// WriteEvents implements collector.LogSink. It enqueues events for the
// background shipper, honoring ctx cancellation.
func (e *Exporter) WriteEvents(ctx context.Context, events []models.Event) error {
	for _, ev := range events {
		select {
		case e.ch <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Run owns the shipping loop until ctx is cancelled, after which it drains the
// buffer, performs a final flush, and returns. Intended to run in its own
// goroutine for the lifetime of the app.
func (e *Exporter) Run(ctx context.Context) error {
	ticker := time.NewTicker(e.cfg.BatchWait)
	defer ticker.Stop()

	batch := make([]models.Event, 0, e.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		e.pushWithRetry(batch)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// Drain whatever is still buffered, then final flush.
			for {
				select {
				case ev := <-e.ch:
					batch = append(batch, ev)
					if len(batch) >= e.cfg.BatchSize {
						flush()
					}
				default:
					flush()
					e.log.Info("loki exporter stopped")
					return nil
				}
			}
		case ev := <-e.ch:
			batch = append(batch, ev)
			if len(batch) >= e.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// pushWithRetry sends a batch, retrying transient failures.
func (e *Exporter) pushWithRetry(events []models.Event) {
	payload, err := json.Marshal(e.buildPayload(events))
	if err != nil {
		e.log.Error("loki: marshaling payload", zap.Error(err))
		return
	}

	for attempt := 0; attempt <= e.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err = e.push(payload); err == nil {
			return
		}
		e.log.Warn("loki: push failed",
			zap.Int("attempt", attempt), zap.Int("events", len(events)), zap.Error(err))
	}
	e.log.Error("loki: dropping batch after retries",
		zap.Int("events", len(events)), zap.Error(err))
}

func (e *Exporter) push(payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.cfg.Tenant != "" {
		req.Header.Set("X-Scope-OrgID", e.cfg.Tenant)
	}

	resp, err := e.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("loki: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// buildPayload groups events into streams by their label set.
func (e *Exporter) buildPayload(events []models.Event) lokiPush {
	streams := map[string]*lokiStream{}
	for _, ev := range events {
		labels := e.streamLabels(ev)
		key := labelsKey(labels)

		s, ok := streams[key]
		if !ok {
			s = &lokiStream{Stream: labels}
			streams[key] = s
		}

		ts := ev.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		line, _ := json.Marshal(toLogLine(ev))
		s.Values = append(s.Values, [2]string{
			strconv.FormatInt(ts.UnixNano(), 10),
			string(line),
		})
	}

	out := lokiPush{Streams: make([]lokiStream, 0, len(streams))}
	for _, s := range streams {
		// Loki requires ascending timestamps within a stream.
		sort.Slice(s.Values, func(i, j int) bool { return s.Values[i][0] < s.Values[j][0] })
		out.Streams = append(out.Streams, *s)
	}
	return out
}

// streamLabels returns the low-cardinality label set for an event, merged with
// any static labels from config.
func (e *Exporter) streamLabels(ev models.Event) map[string]string {
	labels := map[string]string{
		"job":    "collector",
		"vendor": ev.Vendor,
		"site":   ev.Site,
		"level":  ev.Level,
		"event":  string(ev.Type),
	}
	if ev.Model != "" {
		labels["model"] = ev.Model
	}
	for k, v := range e.cfg.Labels {
		labels[k] = v
	}
	// Drop empties: Loki rejects empty label values.
	for k, v := range labels {
		if v == "" {
			delete(labels, k)
		}
	}
	return labels
}

func toLogLine(ev models.Event) logLine {
	return logLine{
		Event:     string(ev.Type),
		Level:     ev.Level,
		Msg:       ev.Message,
		Vendor:    ev.Vendor,
		Site:      ev.Site,
		Device:    ev.DeviceName,
		MAC:       ev.DeviceMAC,
		Model:     ev.Model,
		ClientMAC: ev.ClientMAC,
		Hostname:  ev.Hostname,
	}
}

// labelsKey builds a deterministic key from a label set for grouping.
func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}
	return b.String()
}
