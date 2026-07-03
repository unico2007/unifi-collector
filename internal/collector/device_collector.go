package collector

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// DeviceCollector reads devices from a DeviceSource and records them into a
// DeviceMetricSink. It is vendor-neutral: it knows nothing about UniFi.
type DeviceCollector struct {
	src  DeviceSource
	sink DeviceMetricSink
	log  *zap.Logger
}

// NewDeviceCollector wires the dependencies (constructor injection).
func NewDeviceCollector(src DeviceSource, sink DeviceMetricSink, log *zap.Logger) *DeviceCollector {
	return &DeviceCollector{src: src, sink: sink, log: log}
}

func (c *DeviceCollector) Name() string { return "devices" }

func (c *DeviceCollector) Collect(ctx context.Context) error {
	devices, err := c.src.Devices(ctx)
	if err != nil {
		return fmt.Errorf("device collector: %w", err)
	}
	c.sink.RecordDevices(devices)
	c.log.Debug("collected devices", zap.Int("count", len(devices)))
	return nil
}
