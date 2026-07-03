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
		out = append(out, models.Health{
			Vendor:     c.Name(),
			Site:       c.cfg.Site,
			Subsystem:  h.Subsystem,
			Status:     h.Status,
			NumDevices: h.NumAP + h.NumSW + h.NumGW,
			NumClients: h.NumUser + h.NumGuest,
		})
	}
	return out, nil
}
