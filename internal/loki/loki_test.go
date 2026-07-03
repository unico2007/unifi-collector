package loki

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// captureServer records the last decoded push payload and signals its arrival.
func captureServer(t *testing.T, got chan<- lokiPush) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p lokiPush
		if err := json.Unmarshal(body, &p); err != nil {
			t.Errorf("server: bad payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		got <- p
	}))
}

func sampleEvents() []models.Event {
	base := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	return []models.Event{
		{Vendor: "unifi", Site: "default", Level: "info", Type: models.EventClientConnected,
			Timestamp: base.Add(2 * time.Second), ClientMAC: "cc:cc", Hostname: "phone", Message: "connected"},
		{Vendor: "unifi", Site: "default", Level: "info", Type: models.EventClientConnected,
			Timestamp: base, ClientMAC: "dd:dd", Hostname: "laptop", Message: "connected"},
	}
}

func TestExporter_FlushesBatchBySize(t *testing.T) {
	got := make(chan lokiPush, 1)
	srv := captureServer(t, got)
	defer srv.Close()

	e := New(Config{URL: srv.URL, BatchSize: 2, BatchWait: time.Hour}, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = e.Run(ctx) }()

	if err := e.WriteEvents(ctx, sampleEvents()); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}

	select {
	case p := <-got:
		if len(p.Streams) != 1 {
			t.Fatalf("streams = %d, want 1 (same label set)", len(p.Streams))
		}
		s := p.Streams[0]
		if s.Stream["event"] != "client_connected" || s.Stream["site"] != "default" {
			t.Errorf("unexpected stream labels: %v", s.Stream)
		}
		if len(s.Values) != 2 {
			t.Fatalf("values = %d, want 2", len(s.Values))
		}
		// Ascending timestamp order enforced.
		if s.Values[0][0] >= s.Values[1][0] {
			t.Errorf("values not sorted ascending: %s then %s", s.Values[0][0], s.Values[1][0])
		}
		// High-cardinality field is in the line, not a label.
		if _, isLabel := s.Stream["mac"]; isLabel {
			t.Error("mac should not be a stream label")
		}
		var ll logLine
		if err := json.Unmarshal([]byte(s.Values[0][1]), &ll); err != nil {
			t.Fatalf("log line not JSON: %v", err)
		}
		if ll.ClientMAC == "" {
			t.Error("client_mac missing from log line")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for push")
	}
}

func TestExporter_FlushesOnShutdown(t *testing.T) {
	got := make(chan lokiPush, 1)
	srv := captureServer(t, got)
	defer srv.Close()

	// Large batch + long wait: only a shutdown drain can trigger the flush.
	e := New(Config{URL: srv.URL, BatchSize: 1000, BatchWait: time.Hour}, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { _ = e.Run(ctx); close(done) }()

	if err := e.WriteEvents(context.Background(), sampleEvents()); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}
	// Give the run loop a moment to buffer the events, then shut down.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case p := <-got:
		if len(p.Streams) == 0 || len(p.Streams[0].Values) != 2 {
			t.Fatalf("shutdown flush payload wrong: %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown flush")
	}
	<-done
}
