package kerio

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// Compile-time proof that *Client satisfies the DeviceSource capability. Kerio
// exposes device/interface data but not UniFi-style client/event/health feeds,
// so it implements only the capabilities it actually supports (Interface
// Segregation in action — the scheduler wires just this collector).
var _ collector.DeviceSource = (*Client)(nil)

// Name identifies this vendor adapter.
func (c *Client) Name() string { return "kerio" }

// rawInterface mirrors an entry from Interfaces.get.
//
// NOTE: these JSON field names are a best-effort starting point. Confirm them
// against a real Interfaces.get response (browser DevTools on the Kerio admin,
// or `scripts/` discovery) and adjust — the mapping is the only vendor-specific
// part that needs the live device.
type rawInterface struct {
	Name    string `json:"name"`
	Type    string `json:"type"`    // e.g. "Ethernet", "VPNTunnel"
	Enabled bool   `json:"enabled"` // administratively up
	Status  string `json:"status"`  // e.g. "Up" / "Down"
	IP      string `json:"ip"`
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

	// TEMP DEBUG: capture the raw Interfaces.get result so we can see Kerio's
	// real field names and fix the rawInterface mapping. Remove after fixing.
	var raw json.RawMessage
	if err := c.call(ctx, "Interfaces.get", params, &raw); err != nil {
		return nil, err
	}
	c.log.Info("kerio Interfaces.get raw (temp debug)", zap.ByteString("raw", raw))

	var res interfacesResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("kerio: decoding interfaces: %w", err)
	}

	out := make([]models.Device, 0, len(res.List))
	for _, i := range res.List {
		state := "offline"
		if i.Status == "Up" || (i.Status == "" && i.Enabled) {
			state = "online"
		}
		out = append(out, models.Device{
			Vendor:  c.Name(),
			ID:      i.Name,
			Name:    i.Name,
			Type:    "interface",
			IP:      i.IP,
			State:   state,
			Adopted: i.Enabled,
			Labels:  map[string]string{"iface_type": i.Type},
		})
	}
	return out, nil
}
