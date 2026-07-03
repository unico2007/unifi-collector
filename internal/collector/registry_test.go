package collector_test

import (
	"context"
	"testing"

	"github.com/murad/unifi-collector/internal/collector"
)

// fakeCollector is a tiny test double — proving collectors are trivially
// mockable thanks to the small interface.
type fakeCollector struct {
	name    string
	collect func(context.Context) error
}

func (f fakeCollector) Name() string { return f.name }
func (f fakeCollector) Collect(ctx context.Context) error {
	if f.collect != nil {
		return f.collect(ctx)
	}
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := collector.NewRegistry()

	if err := r.Register(fakeCollector{name: "devices"}); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	got, ok := r.Get("devices")
	if !ok {
		t.Fatal("Get(devices): not found")
	}
	if got.Name() != "devices" {
		t.Fatalf("Get(devices): name = %q, want devices", got.Name())
	}
}

func TestRegistry_RejectsDuplicateAndInvalid(t *testing.T) {
	r := collector.NewRegistry()
	_ = r.Register(fakeCollector{name: "clients"})

	if err := r.Register(fakeCollector{name: "clients"}); err == nil {
		t.Error("duplicate registration: expected error, got nil")
	}
	if err := r.Register(fakeCollector{name: ""}); err == nil {
		t.Error("empty name: expected error, got nil")
	}
	if err := r.Register(nil); err == nil {
		t.Error("nil collector: expected error, got nil")
	}
}

func TestRegistry_AllIsSorted(t *testing.T) {
	r := collector.NewRegistry()
	_ = r.Register(fakeCollector{name: "events"})
	_ = r.Register(fakeCollector{name: "devices"})
	_ = r.Register(fakeCollector{name: "clients"})

	all := r.All()
	want := []string{"clients", "devices", "events"}
	if len(all) != len(want) {
		t.Fatalf("All(): len = %d, want %d", len(all), len(want))
	}
	for i, c := range all {
		if c.Name() != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, c.Name(), want[i])
		}
	}
}
