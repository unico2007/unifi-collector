package alert

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Evaluator periodically evaluates the alert rules and records fire/resolve
// transitions to the history table, so the timeline reflects real events even
// when nobody is viewing the Alerts page.
type Evaluator struct {
	svc *Service
	log *zap.Logger
}

// NewEvaluator builds the background evaluator for a service.
func NewEvaluator(svc *Service, log *zap.Logger) *Evaluator {
	return &Evaluator{svc: svc, log: log}
}

// Run evaluates once immediately, then every 30s until ctx is done.
func (e *Evaluator) Run(ctx context.Context) {
	const interval = 30 * time.Second
	first := true
	evaluate := func() {
		ectx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		// Guard: if Prometheus is unreachable, activeAlerts() returns empty and we
		// would wrongly resolve every open alert (then re-fire on recovery). Skip
		// the tick instead so the history stays clean during a Prometheus blip.
		if _, err := e.svc.prom.Scalar(ectx, "vector(1)"); err != nil {
			return
		}
		active := e.svc.activeAlerts(ectx, e.svc.store.thresholds())
		fired, escalated, resolved, err := e.svc.store.recordTransitions(active)
		if err != nil {
			e.log.Warn("alert history record failed", zap.Error(err))
			return
		}
		// Skip notifications on the very first tick: on a fresh DB every current
		// alert would look "newly fired" and spam the chat. After that, restarts
		// don't re-fire (open rows persist in SQLite), so this only mutes genuine
		// pre-existing state at first boot.
		if !first {
			e.svc.notifier.notifyTransitions(ectx, fired, escalated, resolved)
		}
		first = false
	}
	// prune keeps the history table bounded (resolved spans older than 90 days,
	// hard cap 5000 resolved rows). Runs at startup and once a day.
	prune := func() {
		n, err := e.svc.store.pruneHistory(90*24*time.Hour, 5000)
		if err != nil {
			e.log.Warn("alert history prune failed", zap.Error(err))
		} else if n > 0 {
			e.log.Info("alert history pruned", zap.Int64("removed", n))
		}
	}
	evaluate() // once at startup so history starts immediately
	prune()
	ticker := time.NewTicker(interval)
	pruneTicker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	defer pruneTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evaluate()
		case <-pruneTicker.C:
			prune()
		}
	}
}
