package unifi

import (
	"context"
	"time"

	"github.com/murad/unifi-collector/internal/models"
)

// rawDevice mirrors the subset of the UniFi stat/device payload we consume.
type rawDevice struct {
	Name     string  `json:"name"`
	Mac      string  `json:"mac"`
	Model    string  `json:"model"`
	Type     string  `json:"type"`
	IP       string  `json:"ip"`
	Version  string  `json:"version"`
	State    int     `json:"state"`
	Adopted  bool    `json:"adopted"`
	Uptime   int64   `json:"uptime"`   // seconds
	RxBytes  float64 `json:"rx_bytes"` // cumulative
	TxBytes  float64 `json:"tx_bytes"` // cumulative
	SysStats struct {
		CPU string `json:"cpu"` // percent, as string
		Mem string `json:"mem"` // percent, as string
	} `json:"system-stats"`
}

// deviceState maps UniFi's numeric state to a human-readable string.
func deviceState(s int) string {
	switch s {
	case 0:
		return "offline"
	case 1:
		return "online"
	case 4:
		return "upgrading"
	case 5:
		return "provisioning"
	default:
		return "unknown"
	}
}

// Devices implements collector.DeviceSource.
func (c *Client) Devices(ctx context.Context) ([]models.Device, error) {
	var raw []rawDevice
	if err := c.GetJSON(ctx, pathDevices, &raw); err != nil {
		return nil, err
	}

	out := make([]models.Device, 0, len(raw))
	for _, d := range raw {
		name := d.Name
		if name == "" {
			name = d.Mac
		}
		out = append(out, models.Device{
			Vendor:        c.Name(),
			Site:          c.cfg.Site,
			ID:            d.Mac,
			Name:          name,
			MAC:           d.Mac,
			Model:         d.Model,
			Type:          d.Type,
			IP:            d.IP,
			Version:       d.Version,
			State:         deviceState(d.State),
			Adopted:       d.Adopted,
			CPUPercent:    atof(d.SysStats.CPU),
			MemoryPercent: atof(d.SysStats.Mem),
			Uptime:        time.Duration(d.Uptime) * time.Second,
			RxBytes:       d.RxBytes,
			TxBytes:       d.TxBytes,
		})
	}
	return out, nil
}
