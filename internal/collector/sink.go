package collector

import (
	"context"

	"github.com/murad/unifi-collector/internal/models"
)

// Sinks are the destinations a collector writes to. Metric sinks are
// synchronous and cannot fail (they only update in-memory gauges), so they
// return nothing. The log sink talks to a remote system (Loki) and may fail,
// so it takes a context and returns an error.

// DeviceMetricSink records device metrics. Implemented by internal/metrics.
type DeviceMetricSink interface {
	RecordDevices(devices []models.Device)
}

// ClientMetricSink records client metrics.
type ClientMetricSink interface {
	RecordClients(clients []models.Client)
}

// HealthMetricSink records health/summary metrics.
type HealthMetricSink interface {
	RecordHealth(health []models.Health)
}

// LogSink ships normalized events to a log backend (Loki). Implemented by
// internal/loki.
type LogSink interface {
	WriteEvents(ctx context.Context, events []models.Event) error
}
