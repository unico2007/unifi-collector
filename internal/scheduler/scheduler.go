// Package scheduler runs registered collectors periodically, each on its own
// ticker and with its own per-cycle timeout, and reports the outcome of every
// cycle to a ScrapeObserver. It is decoupled from both the config and the
// metrics packages: callers supply intervals/timeouts explicitly, and the
// observer is a narrow interface satisfied structurally by internal/metrics.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/murad/unifi-collector/internal/collector"
	"go.uber.org/zap"
)

// ScrapeObserver receives the outcome of each collection cycle. *metrics.Metrics
// satisfies this via its ObserveScrape method.
type ScrapeObserver interface {
	ObserveScrape(collectorName string, duration time.Duration, err error)
}

// job binds a collector to its schedule.
type job struct {
	collector collector.Collector
	interval  time.Duration
	timeout   time.Duration
}

// Scheduler drives a set of collectors.
type Scheduler struct {
	log      *zap.Logger
	observer ScrapeObserver
	jobs     []job
}

// New builds a scheduler. observer may be nil (outcomes are then only logged).
func New(log *zap.Logger, observer ScrapeObserver) *Scheduler {
	return &Scheduler{log: log, observer: observer}
}

// Register schedules c to run every interval, with each cycle bounded by
// timeout. If timeout <= 0 it defaults to interval.
func (s *Scheduler) Register(c collector.Collector, interval, timeout time.Duration) error {
	if c == nil {
		return fmt.Errorf("scheduler: nil collector")
	}
	if interval <= 0 {
		return fmt.Errorf("scheduler: collector %q has non-positive interval", c.Name())
	}
	if timeout <= 0 {
		timeout = interval
	}
	s.jobs = append(s.jobs, job{collector: c, interval: interval, timeout: timeout})
	return nil
}

// Run starts every registered job and blocks until ctx is cancelled, after
// which it waits for all in-flight cycles to finish before returning.
func (s *Scheduler) Run(ctx context.Context) error {
	if len(s.jobs) == 0 {
		s.log.Warn("scheduler: no collectors registered")
	}

	var wg sync.WaitGroup
	for _, j := range s.jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			s.runJob(ctx, j)
		}(j)
	}

	s.log.Info("scheduler started", zap.Int("jobs", len(s.jobs)))
	wg.Wait()
	s.log.Info("scheduler stopped")
	return nil
}

// runJob runs one collector immediately, then on its ticker until ctx is done.
func (s *Scheduler) runJob(ctx context.Context, j job) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Immediate first cycle (unless already shutting down).
	select {
	case <-ctx.Done():
		return
	default:
		s.execute(ctx, j)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.execute(ctx, j)
		}
	}
}

// execute runs a single Collect with a bounded context and reports the outcome.
func (s *Scheduler) execute(ctx context.Context, j job) {
	cctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()

	start := time.Now()
	err := j.collector.Collect(cctx)
	dur := time.Since(start)

	if s.observer != nil {
		s.observer.ObserveScrape(j.collector.Name(), dur, err)
	}
	if err != nil {
		s.log.Warn("collection cycle failed",
			zap.String("collector", j.collector.Name()),
			zap.Duration("duration", dur),
			zap.Error(err))
		return
	}
	s.log.Debug("collection cycle ok",
		zap.String("collector", j.collector.Name()),
		zap.Duration("duration", dur))
}
