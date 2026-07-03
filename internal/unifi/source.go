package unifi

import (
	"strconv"

	"github.com/murad/unifi-collector/internal/collector"
)

// Compile-time proof that *Client satisfies the full collector.Source
// aggregate (Name + Devices + Clients + Events + Health). If a mapping method
// is missing or has the wrong signature, the build breaks here.
var _ collector.Source = (*Client)(nil)

// Name identifies this vendor adapter; used as a metric/log label.
func (c *Client) Name() string { return "unifi" }

// UniFi API endpoint paths (relative to the site API root).
const (
	pathDevices = "stat/device"
	pathClients = "stat/sta"
	pathEvents  = "stat/event"
	pathHealth  = "stat/health"
)

// atof parses a UniFi numeric-string (e.g. cpu "3.2") into a float, returning
// 0 for empty or malformed input.
func atof(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
