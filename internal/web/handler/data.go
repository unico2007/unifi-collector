package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/murad/unifi-collector/internal/web/query"
	"github.com/murad/unifi-collector/internal/web/respond"
)

// --- response DTOs (must mirror web/src/lib/api.ts) ------------------------

type deviceDTO struct {
	Name   string  `json:"name"`
	Vendor string  `json:"vendor"`
	Type   string  `json:"type"`
	Model  string  `json:"model"`
	IP     string  `json:"ip"`
	MAC    string  `json:"mac"`
	State  string  `json:"state"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Uptime string  `json:"uptime"`
}

type clientDTO struct {
	Name  string  `json:"name"`
	MAC   string  `json:"mac"`
	AP    string  `json:"ap"`
	VLAN  string  `json:"vlan"`
	RSSI  float64 `json:"rssi"`
	Rx    string  `json:"rx"`
	Tx    string  `json:"tx"`
	Data  string  `json:"data"`
	IP    string  `json:"ip"`
	Since string  `json:"since"`
}

type overviewDTO struct {
	Devices struct {
		Total   int `json:"total"`
		Online  int `json:"online"`
		Offline int `json:"offline"`
	} `json:"devices"`
	Clients      int             `json:"clients"`
	Health       int             `json:"health"`   // -1 = unknown (monitoring degraded / no data)
	Degraded     bool            `json:"degraded"` // Prometheus unreachable => don't render green-and-healthy
	Alerts       int             `json:"alerts"`
	ClientSeries []float64       `json:"clientSeries"`
	DeviceSeries []float64       `json:"deviceSeries"` // online devices over the range
	HealthSeries []float64       `json:"healthSeries"` // % of devices online over the range
	HealthBars   []float64       `json:"healthBars"`   // fixed last-24h health, 30m buckets (uptime strip)
	TopClients   []talker        `json:"topClients"`   // top 5 by real throughput (Mbps)
	VendorSplit  []vendorDTO     `json:"vendorSplit"`
	RecentLogs   []query.LogLine `json:"recentLogs"`
}

type vendorDTO struct {
	Vendor  string `json:"vendor"`
	Devices int    `json:"devices"`
	Online  int    `json:"online"`
	Clients int    `json:"clients"`
}

// --- handlers --------------------------------------------------------------

func (s *Handlers) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var o overviewDTO

	online, errOn := s.prom.Scalar(ctx, `count(unifi_device_up == 1)`)
	offline, errOff := s.prom.Scalar(ctx, `count(unifi_device_up == 0)`)
	o.Devices.Online = int(online)
	o.Devices.Offline = int(offline)
	o.Devices.Total = int(online + offline)

	clients, _ := s.prom.Scalar(ctx, `sum(unifi_clients_total)`)
	o.Clients = int(clients)

	// A query error here means Prometheus is unreachable — NOT that everything
	// is healthy. Without this guard total=0 => Health=100, so a full monitoring
	// outage rendered as "100% sağlam, 0 cihaz" (green + calm). Flag it instead.
	o.Degraded = errOn != nil || errOff != nil

	// Health: share of devices online (simple, explainable). -1 is the "unknown"
	// sentinel the frontend renders as "—": either monitoring is degraded or
	// there is genuinely no device data to assess.
	switch {
	case o.Degraded || o.Devices.Total == 0:
		o.Health = -1
	default:
		o.Health = int(float64(o.Devices.Online) / float64(o.Devices.Total) * 100)
	}
	o.Alerts = s.alerts.ActiveCount(ctx) // real active-alert count (same engine as the Alerts page)

	dur, step := query.ParseRange(r.URL.Query().Get("range"))
	o.ClientSeries, _ = s.prom.RangeSeries(ctx, `sum(unifi_clients_total)`, dur, step)
	if o.ClientSeries == nil {
		o.ClientSeries = []float64{}
	}
	// Online-device and health history feed the KPI sparklines/deltas.
	o.DeviceSeries, _ = s.prom.RangeSeries(ctx, `count(unifi_device_up == 1)`, dur, step)
	if o.DeviceSeries == nil {
		o.DeviceSeries = []float64{}
	}
	o.HealthSeries, _ = s.prom.RangeSeries(ctx, `count(unifi_device_up == 1) / count(unifi_device_up) * 100`, dur, step)
	if o.HealthSeries == nil {
		o.HealthSeries = []float64{}
	}
	// Uptime strip: always the last 24h in 30-minute buckets, independent of
	// the selected chart range, so the strip reads like a status page.
	o.HealthBars, _ = s.prom.RangeSeries(ctx, `count(unifi_device_up == 1) / count(unifi_device_up) * 100`, 24*time.Hour, 30*time.Minute)
	if o.HealthBars == nil {
		o.HealthBars = []float64{}
	}

	// Top clients by REAL throughput (rx+tx bytes counters, not the negotiated
	// PHY rate), so the list reflects who is actually moving data.
	o.TopClients = s.topClientThroughput(ctx)

	// Vendor split: devices + clients per vendor.
	o.VendorSplit = s.vendorSplit(ctx)
	if o.VendorSplit == nil {
		o.VendorSplit = []vendorDTO{}
	}

	// Recent logs across both vendors, newest first. Render each CEF payload as
	// a short readable line instead of the raw JSON-wrapped log.
	o.RecentLogs = s.loki.Recent(ctx, `{vendor=~"unifi|kerio"}`, time.Hour, 8)
	for i := range o.RecentLogs {
		o.RecentLogs[i].Msg = friendlyLog(o.RecentLogs[i].Msg)
	}
	if o.RecentLogs == nil {
		o.RecentLogs = []query.LogLine{}
	}

	respond.JSON(w, http.StatusOK, o)
}

// topClientThroughput returns the top-5 clients by current real throughput,
// measured from the per-client byte counters (rate over 10m), in Mbps.
func (s *Handlers) topClientThroughput(ctx context.Context) []talker {
	rows, err := s.prom.Query(ctx,
		`topk(5, sum by (name, mac) (rate(unifi_client_rx_bytes[10m]) + rate(unifi_client_tx_bytes[10m])))`)
	if err != nil {
		return []talker{}
	}
	out := make([]talker, 0, len(rows))
	for _, r := range rows {
		name := r.Labels["name"]
		if name == "" {
			name = r.Labels["mac"]
		}
		out = append(out, talker{Label: name, Value: round1(r.Value * 8 / 1e6), Sub: "Mbps"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value > out[j].Value })
	return out
}

func (s *Handlers) vendorSplit(ctx context.Context) []vendorDTO {
	devByVendor := map[string]int{}
	if rows, err := s.prom.Query(ctx, `sum by (vendor) (unifi_devices_total)`); err == nil {
		for _, r := range rows {
			devByVendor[r.Labels["vendor"]] = int(r.Value)
		}
	}
	cliByVendor := map[string]int{}
	if rows, err := s.prom.Query(ctx, `sum by (vendor) (unifi_clients_total)`); err == nil {
		for _, r := range rows {
			cliByVendor[r.Labels["vendor"]] = int(r.Value)
		}
	}
	onlineByVendor := map[string]int{}
	if rows, err := s.prom.Query(ctx, `count by (vendor) (unifi_device_up == 1)`); err == nil {
		for _, r := range rows {
			onlineByVendor[r.Labels["vendor"]] = int(r.Value)
		}
	}
	// Stable order: unifi first, then kerio, then any others.
	order := []string{"unifi", "kerio"}
	seen := map[string]bool{}
	var out []vendorDTO
	add := func(v string) {
		if seen[v] {
			return
		}
		seen[v] = true
		out = append(out, vendorDTO{Vendor: v, Devices: devByVendor[v], Online: onlineByVendor[v], Clients: cliByVendor[v]})
	}
	for _, v := range order {
		if _, ok := devByVendor[v]; ok {
			add(v)
		}
	}
	for v := range devByVendor {
		add(v)
	}
	return out
}

func (s *Handlers) Devices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cpu := s.byMAC(ctx, `unifi_device_cpu_percent`)
	mem := s.byMAC(ctx, `unifi_device_memory_percent`)
	uptime := s.byMAC(ctx, `unifi_device_uptime_seconds`)
	up := s.byMAC(ctx, `unifi_device_up`)

	infos, err := s.prom.Query(ctx, `unifi_device_info`)
	if err != nil {
		respond.JSON(w, http.StatusOK, []deviceDTO{})
		return
	}

	out := make([]deviceDTO, 0, len(infos))
	for _, in := range infos {
		mac := in.Labels["mac"]
		state := in.Labels["state"]
		if state == "" {
			if up[mac] == 1 {
				state = "online"
			} else {
				state = "offline"
			}
		}
		out = append(out, deviceDTO{
			Name:   in.Labels["name"],
			Vendor: in.Labels["vendor"],
			Type:   in.Labels["type"],
			Model:  in.Labels["model"],
			IP:     ipOrDash(in.Labels["ip"]),
			MAC:    mac,
			State:  state,
			CPU:    cpu[mac],
			Memory: mem[mac],
			Uptime: formatUptime(uptime[mac]),
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

func (s *Handlers) Clients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rssi, err := s.prom.Query(ctx, `unifi_client_rssi`)
	if err != nil {
		respond.JSON(w, http.StatusOK, []clientDTO{})
		return
	}
	rx := s.byMAC(ctx, `unifi_client_rx_rate`)
	tx := s.byMAC(ctx, `unifi_client_tx_rate`)
	rxb := s.byMAC(ctx, `unifi_client_rx_bytes`)
	txb := s.byMAC(ctx, `unifi_client_tx_bytes`)
	conn := s.byMAC(ctx, `unifi_client_connected_seconds`)
	names := s.apNames(ctx)
	ips := s.clientIPs(ctx)

	out := make([]clientDTO, 0, len(rssi))
	for _, c := range rssi {
		mac := c.Labels["mac"]
		out = append(out, clientDTO{
			Name:  c.Labels["name"],
			MAC:   mac,
			AP:    apLabel(names, c.Labels["ap"]),
			VLAN:  c.Labels["vlan"],
			RSSI:  c.Value,
			Rx:    formatRate(rx[mac]),
			Tx:    formatRate(tx[mac]),
			Data:  formatBytes(rxb[mac] + txb[mac]),
			IP:    ipOrDash(ips[mac]),
			Since: formatUptime(conn[mac]),
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

// apNames maps device MAC -> friendly name. The client "ap" label holds the
// AP/switch MAC (ap_mac/sw_mac); this lets us show a readable device name
// instead of a raw MAC across the clients, WiFi and device-detail views.
func (s *Handlers) apNames(ctx context.Context) map[string]string {
	m := map[string]string{}
	rows, err := s.prom.Query(ctx, `unifi_device_info`)
	if err != nil {
		return m
	}
	for _, r := range rows {
		if mac := r.Labels["mac"]; mac != "" && r.Labels["name"] != "" {
			m[mac] = r.Labels["name"]
		}
	}
	return m
}

// clientIPs maps client MAC -> IP from the unifi_client_info metric (mirrors how
// device IPs come from unifi_device_info).
func (s *Handlers) clientIPs(ctx context.Context) map[string]string {
	m := map[string]string{}
	rows, err := s.prom.Query(ctx, `unifi_client_info`)
	if err != nil {
		return m
	}
	for _, r := range rows {
		if mac := r.Labels["mac"]; mac != "" {
			m[mac] = r.Labels["ip"]
		}
	}
	return m
}

// apLabel translates an ap MAC to a device name, falling back to the MAC.
func apLabel(names map[string]string, ap string) string {
	if n, ok := names[ap]; ok {
		return n
	}
	return ap
}

// byMAC runs a query and indexes the scalar values by the "mac" label.
func (s *Handlers) byMAC(ctx context.Context, expr string) map[string]float64 {
	m := map[string]float64{}
	rows, err := s.prom.Query(ctx, expr)
	if err != nil {
		return m
	}
	for _, r := range rows {
		m[r.Labels["mac"]] = r.Value
	}
	return m
}

func ipOrDash(ip string) string {
	if ip == "" {
		return "-"
	}
	return ip
}

func formatUptime(seconds float64) string {
	if seconds <= 0 {
		return "-"
	}
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dg %ds", days, hours)
	}
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%ds %dd", hours, mins)
}

func formatRate(bitsPerSec float64) string {
	if bitsPerSec <= 0 {
		return "0"
	}
	mbps := bitsPerSec / 1e6
	if mbps < 1 {
		return fmt.Sprintf("%.0f Kbps", bitsPerSec/1e3)
	}
	return fmt.Sprintf("%.0f Mbps", mbps)
}
