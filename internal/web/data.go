package web

import (
	"context"
	"fmt"
	"net/http"
	"time"
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
	IP    string  `json:"ip"`
	Since string  `json:"since"`
}

type overviewDTO struct {
	Devices struct {
		Total   int `json:"total"`
		Online  int `json:"online"`
		Offline int `json:"offline"`
	} `json:"devices"`
	Clients      int         `json:"clients"`
	Health       int         `json:"health"`
	Alerts       int         `json:"alerts"`
	ClientSeries []float64   `json:"clientSeries"`
	VendorSplit  []vendorDTO `json:"vendorSplit"`
	RecentLogs   []logLine   `json:"recentLogs"`
}

type vendorDTO struct {
	Vendor  string `json:"vendor"`
	Devices int    `json:"devices"`
	Clients int    `json:"clients"`
}

// --- handlers --------------------------------------------------------------

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var o overviewDTO

	online, _ := s.prom.scalar(ctx, `count(unifi_device_up == 1)`)
	offline, _ := s.prom.scalar(ctx, `count(unifi_device_up == 0)`)
	o.Devices.Online = int(online)
	o.Devices.Offline = int(offline)
	o.Devices.Total = int(online + offline)

	clients, _ := s.prom.scalar(ctx, `sum(unifi_clients_total)`)
	o.Clients = int(clients)

	// Health: share of devices online (simple, explainable).
	if o.Devices.Total > 0 {
		o.Health = int(float64(o.Devices.Online) / float64(o.Devices.Total) * 100)
	} else {
		o.Health = 100
	}
	o.Alerts = 0 // alert engine lands in a later phase

	o.ClientSeries, _ = s.prom.rangeSeries(ctx, `sum(unifi_clients_total)`, 24*time.Hour, time.Hour)
	if o.ClientSeries == nil {
		o.ClientSeries = []float64{}
	}

	// Vendor split: devices + clients per vendor.
	o.VendorSplit = s.vendorSplit(ctx)
	if o.VendorSplit == nil {
		o.VendorSplit = []vendorDTO{}
	}

	// Recent logs across both vendors, newest first.
	o.RecentLogs = s.loki.recent(ctx, `{vendor=~"unifi|kerio"}`, time.Hour, 8)
	if o.RecentLogs == nil {
		o.RecentLogs = []logLine{}
	}

	writeJSON(w, http.StatusOK, o)
}

func (s *Server) vendorSplit(ctx context.Context) []vendorDTO {
	devByVendor := map[string]int{}
	if rows, err := s.prom.query(ctx, `sum by (vendor) (unifi_devices_total)`); err == nil {
		for _, r := range rows {
			devByVendor[r.labels["vendor"]] = int(r.value)
		}
	}
	cliByVendor := map[string]int{}
	if rows, err := s.prom.query(ctx, `sum by (vendor) (unifi_clients_total)`); err == nil {
		for _, r := range rows {
			cliByVendor[r.labels["vendor"]] = int(r.value)
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
		out = append(out, vendorDTO{Vendor: v, Devices: devByVendor[v], Clients: cliByVendor[v]})
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

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cpu := s.byMAC(ctx, `unifi_device_cpu_percent`)
	mem := s.byMAC(ctx, `unifi_device_memory_percent`)
	uptime := s.byMAC(ctx, `unifi_device_uptime_seconds`)
	up := s.byMAC(ctx, `unifi_device_up`)

	infos, err := s.prom.query(ctx, `unifi_device_info`)
	if err != nil {
		writeJSON(w, http.StatusOK, []deviceDTO{})
		return
	}

	out := make([]deviceDTO, 0, len(infos))
	for _, in := range infos {
		mac := in.labels["mac"]
		state := in.labels["state"]
		if state == "" {
			if up[mac] == 1 {
				state = "online"
			} else {
				state = "offline"
			}
		}
		out = append(out, deviceDTO{
			Name:   in.labels["name"],
			Vendor: in.labels["vendor"],
			Type:   in.labels["type"],
			Model:  in.labels["model"],
			IP:     "-", // not exported as a metric label
			MAC:    mac,
			State:  state,
			CPU:    cpu[mac],
			Memory: mem[mac],
			Uptime: formatUptime(uptime[mac]),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rssi, err := s.prom.query(ctx, `unifi_client_rssi`)
	if err != nil {
		writeJSON(w, http.StatusOK, []clientDTO{})
		return
	}
	rx := s.byMAC(ctx, `unifi_client_rx_rate`)
	tx := s.byMAC(ctx, `unifi_client_tx_rate`)
	conn := s.byMAC(ctx, `unifi_client_connected_seconds`)

	out := make([]clientDTO, 0, len(rssi))
	for _, c := range rssi {
		mac := c.labels["mac"]
		out = append(out, clientDTO{
			Name:  c.labels["name"],
			MAC:   mac,
			AP:    c.labels["ap"],
			VLAN:  c.labels["vlan"],
			RSSI:  c.value,
			Rx:    formatRate(rx[mac]),
			Tx:    formatRate(tx[mac]),
			IP:    "-",
			Since: formatUptime(conn[mac]),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// byMAC runs a query and indexes the scalar values by the "mac" label.
func (s *Server) byMAC(ctx context.Context, expr string) map[string]float64 {
	m := map[string]float64{}
	rows, err := s.prom.query(ctx, expr)
	if err != nil {
		return m
	}
	for _, r := range rows {
		m[r.labels["mac"]] = r.value
	}
	return m
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
