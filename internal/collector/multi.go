package collector

import (
	"context"

	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// MultiDeviceSource fans one Devices() call out to several vendor DeviceSources
// and concatenates their results. The device metric sink rewrites every series
// each cycle, so a single collector must supply ALL vendors' devices at once;
// registering one DeviceCollector per vendor would make them clobber each
// other. This combiner is how a second vendor (e.g. Kerio) joins the existing
// "devices" collector without any change to the metrics or scheduler.
type MultiDeviceSource struct {
	sources []DeviceSource
	log     *zap.Logger
}

// namedSource is optionally satisfied by adapters that expose a vendor name,
// used only to label warnings.
type namedSource interface {
	Name() string
}

// NewMultiDeviceSource combines sources; nil entries are ignored.
func NewMultiDeviceSource(log *zap.Logger, sources ...DeviceSource) *MultiDeviceSource {
	filtered := make([]DeviceSource, 0, len(sources))
	for _, s := range sources {
		if s != nil {
			filtered = append(filtered, s)
		}
	}
	return &MultiDeviceSource{sources: filtered, log: log}
}

// Devices aggregates every source's devices. A single source's failure is
// logged and skipped so one vendor being unreachable does not blank out the
// others. Only when EVERY source fails is an error returned, so the collector
// records a scrape failure and keeps the last-good gauges (matching the
// single-source behaviour) instead of wiping them.
func (m *MultiDeviceSource) Devices(ctx context.Context) ([]models.Device, error) {
	var all []models.Device
	var firstErr error
	ok := 0
	for _, s := range m.sources {
		devices, err := s.Devices(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			m.log.Warn("device source failed; skipping this cycle",
				zap.String("source", sourceName(s)), zap.Error(err))
			continue
		}
		ok++
		all = append(all, devices...)
	}
	if ok == 0 && firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

func sourceName(s DeviceSource) string {
	if n, ok := s.(namedSource); ok {
		return n.Name()
	}
	return "unknown"
}
