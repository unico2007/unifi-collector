package collector

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// HealthCollector reads health snapshots from a HealthSource and records them
// into a HealthMetricSink.
type HealthCollector struct {
	src  HealthSource
	sink HealthMetricSink
	log  *zap.Logger
}

// NewHealthCollector wires the dependencies.
func NewHealthCollector(src HealthSource, sink HealthMetricSink, log *zap.Logger) *HealthCollector {
	return &HealthCollector{src: src, sink: sink, log: log}
}

func (c *HealthCollector) Name() string { return "health" }

func (c *HealthCollector) Collect(ctx context.Context) error {
	health, err := c.src.Health(ctx)
	if err != nil {
		return fmt.Errorf("health collector: %w", err)
	}
	c.sink.RecordHealth(health)
	c.log.Debug("collected health", zap.Int("count", len(health)))
	return nil
}
