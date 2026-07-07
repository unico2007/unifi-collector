package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
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

func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var d trafficDTO

	// Throughput over 24h (bytes/s -> Mbps).
	d.Rx = toMbps(mustSeries(s.prom.rangeSeries(ctx, `sum(rate(unifi_device_rx_bytes[5m]))`, 24*time.Hour, time.Hour)))
	d.Tx = toMbps(mustSeries(s.prom.rangeSeries(ctx, `sum(rate(unifi_device_tx_bytes[5m]))`, 24*time.Hour, time.Hour)))

	// Cumulative totals.
	rxTotal, _ := s.prom.scalar(ctx, `sum(unifi_device_rx_bytes)`)
	txTotal, _ := s.prom.scalar(ctx, `sum(unifi_device_tx_bytes)`)
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
