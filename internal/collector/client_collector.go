package collector

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ClientCollector reads connected clients from a ClientSource and records them
// into a ClientMetricSink.
type ClientCollector struct {
	src  ClientSource
	sink ClientMetricSink
	log  *zap.Logger
}

// NewClientCollector wires the dependencies.
func NewClientCollector(src ClientSource, sink ClientMetricSink, log *zap.Logger) *ClientCollector {
	return &ClientCollector{src: src, sink: sink, log: log}
}

func (c *ClientCollector) Name() string { return "clients" }

func (c *ClientCollector) Collect(ctx context.Context) error {
	clients, err := c.src.Clients(ctx)
	if err != nil {
		return fmt.Errorf("client collector: %w", err)
	}
	c.sink.RecordClients(clients)
	c.log.Debug("collected clients", zap.Int("count", len(clients)))
	return nil
}
