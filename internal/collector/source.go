package collector

import (
	"context"
	"time"

	"github.com/murad/unifi-collector/internal/models"
)

// A vendor adapter (internal/unifi, and future internal/qnap, ...) implements
// one or more of the capability interfaces below. Splitting them (rather than a
// single fat Source interface) keeps each collector dependent only on what it
// actually needs, and lets a vendor expose only the capabilities it supports.

// DeviceSource fetches the current set of monitored devices.
type DeviceSource interface {
	Devices(ctx context.Context) ([]models.Device, error)
}

// ClientSource fetches the currently connected end-user clients.
type ClientSource interface {
	Clients(ctx context.Context) ([]models.Client, error)
}

// EventSource fetches normalized events that occurred at or after `since`.
type EventSource interface {
	Events(ctx context.Context, since time.Time) ([]models.Event, error)
}

// HealthSource fetches high-level health snapshots per site/subsystem.
type HealthSource interface {
	Health(ctx context.Context) ([]models.Health, error)
}

// Source is the convenience aggregate that a full-featured adapter (like the
// UniFi one) satisfies. Consumers should depend on the narrow interfaces above,
// not on this aggregate.
type Source interface {
	// Name identifies the vendor, e.g. "unifi". Used as a metric/log label.
	Name() string
	DeviceSource
	ClientSource
	EventSource
	HealthSource
}
