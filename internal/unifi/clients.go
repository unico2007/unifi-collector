package unifi

import (
	"context"
	"strconv"
	"time"

	"github.com/murad/unifi-collector/internal/models"
)

// rawClient mirrors the subset of the UniFi stat/sta (station) payload we use.
type rawClient struct {
	Name     string  `json:"name"`
	Hostname string  `json:"hostname"`
	Mac      string  `json:"mac"`
	IP       string  `json:"ip"`
	APMac    string  `json:"ap_mac"` // AP the client is associated with
	SWMac    string  `json:"sw_mac"` // switch, for wired clients
	RSSI     float64 `json:"rssi"`
	TxRate   float64 `json:"tx_rate"` // kbps
	RxRate   float64 `json:"rx_rate"` // kbps
	VLAN     int     `json:"vlan"`
	Uptime   int64   `json:"uptime"` // seconds connected
}

// Clients implements collector.ClientSource.
func (c *Client) Clients(ctx context.Context) ([]models.Client, error) {
	var raw []rawClient
	if err := c.GetJSON(ctx, pathClients, &raw); err != nil {
		return nil, err
	}

	out := make([]models.Client, 0, len(raw))
	for _, cl := range raw {
		name := cl.Name
		if name == "" {
			name = cl.Hostname
		}
		if name == "" {
			name = cl.Mac
		}
		ap := cl.APMac
		if ap == "" {
			ap = cl.SWMac
		}
		out = append(out, models.Client{
			Vendor:      c.Name(),
			Site:        c.cfg.Site,
			ID:          cl.Mac,
			Name:        name,
			MAC:         cl.Mac,
			IP:          cl.IP,
			ConnectedAP: ap,
			VLAN:        strconv.Itoa(cl.VLAN),
			RSSI:        cl.RSSI,
			// UniFi reports negotiated rates in kbps; normalize to bits/s.
			TxRate:        cl.TxRate * 1000,
			RxRate:        cl.RxRate * 1000,
			ConnectedTime: time.Duration(cl.Uptime) * time.Second,
		})
	}
	return out, nil
}
