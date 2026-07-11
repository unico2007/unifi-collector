package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/murad/unifi-collector/internal/web/query"
	"github.com/murad/unifi-collector/internal/web/respond"
)

type detailClient struct {
	Name string  `json:"name"`
	MAC  string  `json:"mac"`
	RSSI float64 `json:"rssi"`
	Rx   string  `json:"rx"`
	Tx   string  `json:"tx"`
}

type deviceDetailDTO struct {
	Device  deviceDTO      `json:"device"`
	CPU     []float64      `json:"cpu"`
	Memory  []float64      `json:"memory"`
	Rx      []float64      `json:"rx"`
	Tx      []float64      `json:"tx"`
	Clients []detailClient `json:"clients"`
}

func (s *Handlers) DeviceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	sel := fmt.Sprintf(`{name="%s"}`, escapeLabel(name))

	infos, err := s.prom.Query(ctx, `unifi_device_info`+sel)
	if err != nil || len(infos) == 0 {
		respond.JSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	in := infos[0]
	mac := in.Labels["mac"]

	dev := deviceDTO{
		Name:   in.Labels["name"],
		Vendor: in.Labels["vendor"],
		Type:   in.Labels["type"],
		Model:  in.Labels["model"],
		IP:     ipOrDash(in.Labels["ip"]),
		MAC:    mac,
		State:  in.Labels["state"],
	}
	if v, e := s.prom.Scalar(ctx, `unifi_device_cpu_percent`+sel); e == nil {
		dev.CPU = v
	}
	if v, e := s.prom.Scalar(ctx, `unifi_device_memory_percent`+sel); e == nil {
		dev.Memory = v
	}
	if v, e := s.prom.Scalar(ctx, `unifi_device_uptime_seconds`+sel); e == nil {
		dev.Uptime = formatUptime(v)
	}

	dur, step := query.ParseRange(r.URL.Query().Get("range"))
	d := deviceDetailDTO{Device: dev}
	d.CPU = mustSeries(s.prom.RangeSeries(ctx, `unifi_device_cpu_percent`+sel, dur, step))
	d.Memory = mustSeries(s.prom.RangeSeries(ctx, `unifi_device_memory_percent`+sel, dur, step))
	d.Rx = toMbps(mustSeries(s.prom.RangeSeries(ctx, `rate(unifi_device_rx_bytes`+sel+`[5m])`, dur, step)))
	d.Tx = toMbps(mustSeries(s.prom.RangeSeries(ctx, `rate(unifi_device_tx_bytes`+sel+`[5m])`, dur, step)))

	// Clients associated with this AP. The client "ap" label holds the AP's
	// MAC, so match on the device MAC rather than its name.
	d.Clients = []detailClient{}
	clientSel := fmt.Sprintf(`{ap="%s"}`, escapeLabel(mac))
	if rows, e := s.prom.Query(ctx, `unifi_client_rssi`+clientSel); e == nil {
		rx := s.byMAC(ctx, `unifi_client_rx_rate`+clientSel)
		tx := s.byMAC(ctx, `unifi_client_tx_rate`+clientSel)
		for _, c := range rows {
			cmac := c.Labels["mac"]
			d.Clients = append(d.Clients, detailClient{
				Name: c.Labels["name"],
				MAC:  cmac,
				RSSI: c.Value,
				Rx:   formatRate(rx[cmac]),
				Tx:   formatRate(tx[cmac]),
			})
		}
	}

	respond.JSON(w, http.StatusOK, d)
}

// escapeLabel escapes a value for safe inclusion inside a PromQL/LogQL double
// quoted label matcher.
func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}
