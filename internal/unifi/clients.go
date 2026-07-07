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
	Uptime   int64   `json:"uptime"`   // seconds connected
	Radio    string  `json:"radio"`    // "ng"=2.4GHz, "na"=5GHz, "6e"=6GHz
	Channel  int     `json:"channel"`  // radio channel (fallback for band)
	IsWired  bool    `json:"is_wired"` // true for wired clients (no band)
}

// wifiBand derives a human band label from the station's radio/channel. Returns
// "" for wired clients (they have no radio band).
func wifiBand(radio string, channel int, wired bool) string {
	if wired {
		return ""
	}
	switch radio {
	case "ng":
		return "2.4 GHz"
	case "na":
		return "5 GHz"
	case "6e", "6g":
		return "6 GHz"
	}
	switch {
	case channel == 0:
		return ""
	case channel <= 14:
		return "2.4 GHz"
	case channel >= 32 && channel <= 177:
		return "5 GHz"
	default:
		return "6 GHz"
	}
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
			Band:        wifiBand(cl.Radio, cl.Channel, cl.IsWired),
			RSSI:        cl.RSSI,
			// UniFi reports negotiated rates in kbps; normalize to bits/s.
			TxRate:        cl.TxRate * 1000,
			RxRate:        cl.RxRate * 1000,
			ConnectedTime: time.Duration(cl.Uptime) * time.Second,
		})
	}
	return out, nil
}
