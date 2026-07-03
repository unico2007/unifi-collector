package unifi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// unifiOSServer serves login plus a single site endpoint returning fixture.
func unifiOSServer(t *testing.T, endpoint, fixture string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, endpoint):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixture))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := NewClient(Config{BaseURL: url, Username: "u", Password: "p", Site: "default", Timeout: 2 * time.Second}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestDevices_Mapping(t *testing.T) {
	srv := unifiOSServer(t, "stat/device", `{"meta":{"rc":"ok"},"data":[
		{"name":"AP1","mac":"aa","model":"U6","type":"uap","ip":"10.0.0.1","version":"6.5","state":1,"adopted":true,"uptime":3600,"rx_bytes":100,"tx_bytes":200,"system-stats":{"cpu":"5.5","mem":"40"}},
		{"mac":"bb","state":0}
	]}`)
	defer srv.Close()

	devs, err := testClient(t, srv.URL).Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 2 {
		t.Fatalf("got %d devices, want 2", len(devs))
	}
	d := devs[0]
	if d.Name != "AP1" || d.State != "online" || d.CPUPercent != 5.5 || d.Adopted != true {
		t.Errorf("device0 mapped wrong: %+v", d)
	}
	if d.Uptime != time.Hour {
		t.Errorf("uptime = %v, want 1h", d.Uptime)
	}
	// Name falls back to MAC when empty; state 0 -> offline.
	if devs[1].Name != "bb" || devs[1].State != "offline" {
		t.Errorf("device1 fallback wrong: %+v", devs[1])
	}
	if d.Vendor != "unifi" {
		t.Errorf("vendor = %q, want unifi", d.Vendor)
	}
}

func TestClients_RateConversion(t *testing.T) {
	srv := unifiOSServer(t, "stat/sta", `{"meta":{"rc":"ok"},"data":[
		{"hostname":"phone","mac":"cc","ap_mac":"aa","rssi":-50,"tx_rate":1000,"rx_rate":2000,"vlan":10,"uptime":60}
	]}`)
	defer srv.Close()

	clients, err := testClient(t, srv.URL).Clients(context.Background())
	if err != nil {
		t.Fatalf("Clients: %v", err)
	}
	c := clients[0]
	// kbps -> bits/s (x1000).
	if c.TxRate != 1_000_000 || c.RxRate != 2_000_000 {
		t.Errorf("rates = tx %v rx %v, want 1e6/2e6", c.TxRate, c.RxRate)
	}
	if c.Name != "phone" || c.VLAN != "10" || c.ConnectedAP != "aa" {
		t.Errorf("client mapped wrong: %+v", c)
	}
}

func TestHealth_Aggregation(t *testing.T) {
	srv := unifiOSServer(t, "stat/health", `{"meta":{"rc":"ok"},"data":[
		{"subsystem":"wlan","status":"ok","num_user":3,"num_guest":2,"num_ap":4,"num_sw":1,"num_gw":1}
	]}`)
	defer srv.Close()

	h, err := testClient(t, srv.URL).Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if h[0].NumClients != 5 || h[0].NumDevices != 6 {
		t.Errorf("aggregation wrong: clients=%d devices=%d", h[0].NumClients, h[0].NumDevices)
	}
}

func TestEvents_FilterAndClassify(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour).UnixMilli()
	recent := now.Add(-1 * time.Minute).UnixMilli()
	srv := unifiOSServer(t, "stat/event", `{"meta":{"rc":"ok"},"data":[
		{"time":`+strconv.FormatInt(old, 10)+`,"key":"EVT_WU_Connected","user":"cc"},
		{"time":`+strconv.FormatInt(recent, 10)+`,"key":"EVT_AP_Lost_Contact","ap":"aa","ap_name":"AP1"}
	]}`)
	defer srv.Close()

	// since = 1h ago -> only the recent event survives.
	events, err := testClient(t, srv.URL).Events(context.Background(), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (old filtered out)", len(events))
	}
	if events[0].Type != models.EventAPOffline || events[0].DeviceName != "AP1" {
		t.Errorf("event mapped wrong: %+v", events[0])
	}
}

func TestEvents_FallsBackToPOSTWhenGETUnsupported(t *testing.T) {
	now := time.Now()
	recent := now.Add(-1 * time.Minute).UnixMilli()
	var sawPost bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "stat/event") && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.NotFound"},"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "stat/event") && r.Method == http.MethodPost:
			sawPost = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode POST body: %v", err)
			}
			if body["_limit"] == nil || body["within"] == nil {
				t.Fatalf("POST body missing query fields: %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[
				{"time":` + strconv.FormatInt(recent, 10) + `,"key":"EVT_WU_Connected","user":"cc"}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	events, err := testClient(t, srv.URL).Events(context.Background(), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if !sawPost {
		t.Fatal("expected POST fallback to be used")
	}
	if len(events) != 1 || events[0].Type != models.EventClientConnected {
		t.Fatalf("events = %+v, want one client connected event", events)
	}
}

func TestEvents_UnsupportedEndpointIsOptional(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "stat/event"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.NotFound"},"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	events, err := testClient(t, srv.URL).Events(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want none", events)
	}
}

func TestClassifyEvent(t *testing.T) {
	cases := []struct {
		key  string
		want models.EventType
	}{
		{"EVT_WU_Connected", models.EventClientConnected},
		{"EVT_LU_Disconnected", models.EventClientDisconnected},
		{"EVT_AP_Connected", models.EventAPOnline},
		{"EVT_AP_Lost_Contact", models.EventAPOffline},
		{"EVT_AP_Restarted", models.EventAPRestart},
		{"EVT_AP_Adopted", models.EventDeviceAdopted},
		{"EVT_AP_Upgraded", models.EventFirmwareUpdate},
		{"EVT_SomethingElse", models.EventUnknown},
	}
	for _, tc := range cases {
		if got, _ := classifyEvent(tc.key); got != tc.want {
			t.Errorf("classifyEvent(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
