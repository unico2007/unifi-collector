package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/scheduler"
	"go.uber.org/zap"
)

type fakeCollector struct {
	name  string
	calls int32
	fn    func(context.Context) error
}

func (f *fakeCollector) Name() string { return f.name }
func (f *fakeCollector) Collect(ctx context.Context) error {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		return f.fn(ctx)
	}
	return nil
}

type recordingObserver struct {
	mu   sync.Mutex
	errs map[string]error
	seen map[string]int
}

func newObserver() *recordingObserver {
	return &recordingObserver{errs: map[string]error{}, seen: map[string]int{}}
}
func (o *recordingObserver) ObserveScrape(name string, _ time.Duration, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.seen[name]++
	o.errs[name] = err
}
func (o *recordingObserver) count(name string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.seen[name]
}
func (o *recordingObserver) lastErr(name string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.errs[name]
}

func TestScheduler_RunsPeriodicallyAndObserves(t *testing.T) {
	c := &fakeCollector{name: "devices"}
	obs := newObserver()
	s := scheduler.New(zap.NewNop(), obs)
	if err := s.Register(c, 20*time.Millisecond, 0); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx) // blocks until ctx expires

	// Immediate run + ~3 ticks; be lenient to avoid timing flakiness.
	if got := atomic.LoadInt32(&c.calls); got < 2 {
		t.Errorf("collect calls = %d, want >= 2", got)
	}
	if obs.count("devices") < 2 {
		t.Errorf("observations = %d, want >= 2", obs.count("devices"))
	}
	if obs.lastErr("devices") != nil {
		t.Errorf("unexpected error: %v", obs.lastErr("devices"))
	}
}

func TestScheduler_PerCollectorTimeout(t *testing.T) {
	// Collector blocks until its context is cancelled, then returns that error.
	c := &fakeCollector{name: "slow", fn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	obs := newObserver()
	s := scheduler.New(zap.NewNop(), obs)
	// Long interval so only the immediate run happens; short per-cycle timeout.
	if err := s.Register(c, time.Hour, 20*time.Millisecond); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx)

	if got := obs.lastErr("slow"); !errors.Is(got, context.DeadlineExceeded) {
		t.Errorf("timeout error = %v, want context.DeadlineExceeded", got)
	}
}

func TestScheduler_RegisterValidation(t *testing.T) {
	s := scheduler.New(zap.NewNop(), nil)
	if err := s.Register(nil, time.Second, 0); err == nil {
		t.Error("nil collector: expected error")
	}
	if err := s.Register(&fakeCollector{name: "x"}, 0, 0); err == nil {
		t.Error("zero interval: expected error")
	}
}
