package kerio

import (
	"context"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
)

// Compile-time proof that *Client satisfies the DeviceSource capability. Kerio
// exposes device/interface data but not UniFi-style client/event/health feeds,
// so it implements only the capabilities it actually supports (Interface
// Segregation in action — the scheduler wires just this collector).
var _ collector.DeviceSource = (*Client)(nil)

// Name identifies this vendor adapter.
func (c *Client) Name() string { return "kerio" }

// rawInterface mirrors an entry from Interfaces.get. Field names verified
// against a live Kerio Control response: link connectivity is reported by
// `linkStatus` ("Up"/"Down"); the `status` field is a config-store state
// ("StoreStatusClean", ...) and must NOT be used for up/down.
type rawInterface struct {
	Name       string `json:"name"`
	Type       string `json:"type"`       // e.g. "Ethernet", "VPNTunnel"
	Enabled    bool   `json:"enabled"`    // administratively enabled
	LinkStatus string `json:"linkStatus"` // link connectivity: "Up" / "Down"
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
}

type interfacesResult struct {
	List []rawInterface `json:"list"`
}

// Devices implements collector.DeviceSource by listing the firewall's network
// interfaces (each surfaced as a neutral Device of type "interface").
func (c *Client) Devices(ctx context.Context) ([]models.Device, error) {
	params := map[string]any{
		"sortByGroup": true,
		"query": map[string]any{
			"start":   0,
			"limit":   -1,
			"orderBy": []map[string]string{{"columnName": "name", "direction": "Asc"}},
		},
	}

	var res interfacesResult
	if err := c.call(ctx, "Interfaces.get", params, &res); err != nil {
		return nil, err
	}

	out := make([]models.Device, 0, len(res.List))
	for _, i := range res.List {
		state := "offline"
		if i.LinkStatus == "Up" || (i.LinkStatus == "" && i.Enabled) {
			state = "online"
		}
		out = append(out, models.Device{
			Vendor:  c.Name(),
			ID:      i.Name,
			Name:    i.Name,
			MAC:     i.MAC,
			Type:    "interface",
			IP:      i.IP,
			State:   state,
			Adopted: i.Enabled,
			Labels:  map[string]string{"iface_type": i.Type},
		})
	}
	return out, nil
}
