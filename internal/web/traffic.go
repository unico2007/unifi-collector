package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

type talker struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Sub   string  `json:"sub,omitempty"`
}

type trafficDTO struct {
	Rx         []float64 `json:"rx"`
	Tx         []float64 `json:"tx"`
	TotalRx    string    `json:"totalRx"`
	TotalTx    string    `json:"totalTx"`
	TopTalkers []talker  `json:"topTalkers"`
	PerAp      []kv      `json:"perAp"`
}

// trafficTier picks the single device tier used for site-wide totals. Summing
// every tier counts the same flow once per hop (gateway + switch + AP), which
// inflates the chart by the topology depth. Prefer the gateway (true WAN
// view); this site's gateway is a Kerio box outside UniFi, so fall back to
// switches, then APs.
func (s *Server) trafficTier(ctx context.Context) string {
	rows, err := s.prom.query(ctx, `count by (type) (unifi_device_rx_bytes)`)
	if err != nil {
		return "uap"
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[r.labels["type"]] = true
	}
	for _, t := range []string{"ugw", "usw", "uap"} {
		if have[t] {
			return t
		}
	}
	return "uap"
}

// trafficQueries builds the PromQL for the site totals at one tier, mapping
// download/upload onto the right counter direction: a gateway/switch receives
// downloads on rx, but an AP *transmits* downloads to its clients, so at the
// AP tier the directions swap.
func trafficQueries(tier string) (downRate, upRate, downTotal, upTotal string) {
	rx := fmt.Sprintf(`sum(rate(unifi_device_rx_bytes{type=%q}[5m]))`, tier)
	tx := fmt.Sprintf(`sum(rate(unifi_device_tx_bytes{type=%q}[5m]))`, tier)
	rxT := fmt.Sprintf(`sum(unifi_device_rx_bytes{type=%q})`, tier)
	txT := fmt.Sprintf(`sum(unifi_device_tx_bytes{type=%q})`, tier)
	if tier == "uap" {
		return tx, rx, txT, rxT
	}
	return rx, tx, rxT, txT
}

func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var d trafficDTO

	// Throughput over the selected range (bytes/s -> Mbps), measured at a
	// single device tier so multi-hop flows aren't double-counted.
	dur, step := parseRange(r.URL.Query().Get("range"))
	downRate, upRate, downTotal, upTotal := trafficQueries(s.trafficTier(ctx))
	d.Rx = toMbps(mustSeries(s.prom.rangeSeries(ctx, downRate, dur, step)))
	d.Tx = toMbps(mustSeries(s.prom.rangeSeries(ctx, upRate, dur, step)))

	// Cumulative totals (same tier and direction mapping as the chart).
	rxTotal, _ := s.prom.scalar(ctx, downTotal)
	txTotal, _ := s.prom.scalar(ctx, upTotal)
	d.TotalRx = formatBytes(rxTotal)
	d.TotalTx = formatBytes(txTotal)

	// Top talkers by current client throughput (rx+tx rate, bits/s). We only
	// have instantaneous rates, not cumulative per-client volume, so this is
	// "top by current speed", labelled in Mbps.
	d.TopTalkers = s.topTalkers(ctx)

	// Per-AP current throughput (Mbps).
	d.PerAp = []kv{}
	if rows, err := s.prom.query(ctx,
		`sum by (name) (rate(unifi_device_rx_bytes{type="uap"}[5m]) + rate(unifi_device_tx_bytes{type="uap"}[5m]))`); err == nil {
		m := map[string]float64{}
		for _, r := range rows {
			m[r.labels["name"]] = r.value * 8 / 1e6
		}
		d.PerAp = sortedKV(m, "")
	}

	writeJSON(w, http.StatusOK, d)
}

func (s *Server) topTalkers(ctx context.Context) []talker {
	rate := map[string]float64{} // name -> bits/s
	for _, metric := range []string{`unifi_client_rx_rate`, `unifi_client_tx_rate`} {
		rows, err := s.prom.query(ctx, metric)
		if err != nil {
			continue
		}
		for _, r := range rows {
			name := r.labels["name"]
			if name == "" {
				name = r.labels["mac"]
			}
			rate[name] += r.value
		}
	}
	out := make([]talker, 0, len(rate))
	for name, bits := range rate {
		out = append(out, talker{Label: name, Value: round1(bits / 1e6), Sub: "Mbps"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value > out[j].Value })
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func mustSeries(s []float64, _ error) []float64 {
	if s == nil {
		return []float64{}
	}
	return s
}

func toMbps(bytesPerSec []float64) []float64 {
	out := make([]float64, len(bytesPerSec))
	for i, v := range bytesPerSec {
		out[i] = round1(v * 8 / 1e6)
	}
	return out
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }

func formatBytes(b float64) string {
	const unit = 1024.0
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := b / unit
	for _, u := range units {
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= unit
	}
	return fmt.Sprintf("%.1f EB", v)
}
