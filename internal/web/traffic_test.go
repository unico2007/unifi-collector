package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubProm serves a Prometheus instant-query API returning one series per
// given device type, so trafficTier can be exercised end-to-end.
func stubProm(t *testing.T, types []string) *promClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		results := make([]map[string]any, 0, len(types))
		for _, typ := range types {
			results = append(results, map[string]any{
				"metric": map[string]string{"type": typ},
				"value":  [2]any{0, "1"},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]any{"resultType": "vector", "result": results},
		})
	}))
	t.Cleanup(srv.Close)
	return newPromClient(srv.URL)
}

func TestTrafficTier(t *testing.T) {
	cases := []struct {
		types []string
		want  string
	}{
		{[]string{"ugw", "usw", "uap"}, "ugw"}, // gateway wins when present
		{[]string{"usw", "uap"}, "usw"},        // Kerio-gateway site: switches next
		{[]string{"uap"}, "uap"},               // AP-only site
		{[]string{}, "uap"},                    // no data at all: safe default
	}
	for _, c := range cases {
		s := &Server{prom: stubProm(t, c.types)}
		if got := s.trafficTier(context.Background()); got != c.want {
			t.Errorf("types %v: tier = %q, want %q", c.types, got, c.want)
		}
	}
}

func TestTrafficTierPromDown(t *testing.T) {
	s := &Server{prom: newPromClient("http://127.0.0.1:1")} // nothing listens
	if got := s.trafficTier(context.Background()); got != "uap" {
		t.Errorf("prom unreachable: tier = %q, want uap fallback", got)
	}
}

func TestTrafficQueries(t *testing.T) {
	t.Run("gateway and switch keep counter direction", func(t *testing.T) {
		for _, tier := range []string{"ugw", "usw"} {
			down, up, downT, upT := trafficQueries(tier)
			wantDown := fmt.Sprintf(`sum(rate(unifi_device_rx_bytes{type=%q}[5m]))`, tier)
			if down != wantDown {
				t.Errorf("%s down = %q, want %q", tier, down, wantDown)
			}
			if up == down || downT == upT {
				t.Errorf("%s: up/down queries must differ", tier)
			}
			for _, q := range []string{down, up, downT, upT} {
				if want := fmt.Sprintf("type=%q", tier); !strings.Contains(q, want) {
					t.Errorf("%s query %q missing tier filter %s", tier, q, want)
				}
			}
		}
	})

	t.Run("AP tier swaps direction (AP tx = client download)", func(t *testing.T) {
		down, up, downT, upT := trafficQueries("uap")
		if want := `sum(rate(unifi_device_tx_bytes{type="uap"}[5m]))`; down != want {
			t.Errorf("uap down = %q, want %q", down, want)
		}
		if want := `sum(rate(unifi_device_rx_bytes{type="uap"}[5m]))`; up != want {
			t.Errorf("uap up = %q, want %q", up, want)
		}
		if want := `sum(unifi_device_tx_bytes{type="uap"})`; downT != want {
			t.Errorf("uap downTotal = %q, want %q", downT, want)
		}
		if want := `sum(unifi_device_rx_bytes{type="uap"})`; upT != want {
			t.Errorf("uap upTotal = %q, want %q", upT, want)
		}
	})
}
