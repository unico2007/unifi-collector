package unifi

import (
	"context"

	"github.com/murad/unifi-collector/internal/models"
)

// rawHealth mirrors the subset of the UniFi stat/health payload we use. The
// endpoint returns one entry per subsystem (wlan, wan, lan, ...).
type rawHealth struct {
	Subsystem string `json:"subsystem"`
	Status    string `json:"status"`
	NumUser   int    `json:"num_user"`
	NumGuest  int    `json:"num_guest"`
	NumAP     int    `json:"num_ap"`
	NumSW     int    `json:"num_sw"`
	NumGW     int    `json:"num_gw"`
}

// Health implements collector.HealthSource.
func (c *Client) Health(ctx context.Context) ([]models.Health, error) {
	var raw []rawHealth
	if err := c.GetJSON(ctx, pathHealth, &raw); err != nil {
		return nil, err
	}

	out := make([]models.Health, 0, len(raw))
	for _, h := range raw {
		numDevices := h.NumAP + h.NumSW + h.NumGW
		// Skip subsystems UniFi has no device managing. When the site's gateway
		// isn't UniFi (here it's Kerio), the wan/lan/vpn/www subsystems come back
		// with no managed device and a non-"ok" status, which would otherwise map
		// to 0 (error) and raise false alerts. A subsystem UniFi truly manages
		// (e.g. wlan with the APs) reports NumDevices > 0 and still surfaces real
		// problems.
		if numDevices == 0 && h.Status != "ok" && h.Status != "warning" {
			continue
		}
		out = append(out, models.Health{
			Vendor:     c.Name(),
			Site:       c.cfg.Site,
			Subsystem:  h.Subsystem,
			Status:     h.Status,
			NumDevices: numDevices,
			NumClients: h.NumUser + h.NumGuest,
		})
	}
	return out, nil
}
