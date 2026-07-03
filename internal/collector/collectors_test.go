package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// --- test doubles ---------------------------------------------------------

type fakeSource struct {
	devices []models.Device
	events  []models.Event
	// records the `since` argument passed to Events.
	lastSince time.Time
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Devices(context.Context) ([]models.Device, error) {
	return f.devices, nil
}
func (f *fakeSource) Clients(context.Context) ([]models.Client, error) { return nil, nil }
func (f *fakeSource) Health(context.Context) ([]models.Health, error)  { return nil, nil }
func (f *fakeSource) Events(_ context.Context, since time.Time) ([]models.Event, error) {
	f.lastSince = since
	return f.events, nil
}

type fakeDeviceSink struct{ recorded []models.Device }

func (s *fakeDeviceSink) RecordDevices(d []models.Device) { s.recorded = d }

type fakeLogSink struct{ writes [][]models.Event }

func (s *fakeLogSink) WriteEvents(_ context.Context, e []models.Event) error {
	s.writes = append(s.writes, e)
	return nil
}

// --- tests ----------------------------------------------------------------

func TestDeviceCollector_RecordsToSink(t *testing.T) {
	src := &fakeSource{devices: []models.Device{{MAC: "aa"}, {MAC: "bb"}}}
	sink := &fakeDeviceSink{}
	c := collector.NewDeviceCollector(src, sink, zap.NewNop())

	if c.Name() != "devices" {
		t.Fatalf("Name = %q, want devices", c.Name())
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(sink.recorded) != 2 {
		t.Fatalf("recorded %d devices, want 2", len(sink.recorded))
	}
}

func TestEventCollector_AdvancesWatermark(t *testing.T) {
	t0 := time.Now().Add(-10 * time.Minute)
	src := &fakeSource{events: []models.Event{
		{Timestamp: t0},
		{Timestamp: t0.Add(2 * time.Minute)}, // newest
		{Timestamp: t0.Add(1 * time.Minute)},
	}}
	sink := &fakeLogSink{}
	c := collector.NewEventCollector(src, sink, zap.NewNop(), time.Hour)

	// First cycle: forwards all events and sets the watermark to the newest.
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect#1: %v", err)
	}
	if len(sink.writes) != 1 || len(sink.writes[0]) != 3 {
		t.Fatalf("cycle#1 should forward 3 events, got %+v", sink.writes)
	}

	// Second cycle: no new events; source should be queried with since == newest.
	src.events = nil
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect#2: %v", err)
	}
	if len(sink.writes) != 1 {
		t.Fatalf("cycle#2 should not write (no new events), got %d writes", len(sink.writes))
	}
	wantSince := t0.Add(2 * time.Minute)
	if !src.lastSince.Equal(wantSince) {
		t.Errorf("cycle#2 since = %v, want watermark %v", src.lastSince, wantSince)
	}
}
