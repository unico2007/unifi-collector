package kerio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockKerio serves Session.login and Interfaces.get, asserting the X-Token flow.
func mockKerio(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "Session.login":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"token":"tok123"}}`))
		case "Interfaces.get":
			if r.Header.Get("X-Token") != "tok123" {
				w.Write([]byte(`{"jsonrpc":"2.0","id":2,"error":{"code":-32001,"message":"Invalid session token"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"list":[
				{"name":"WAN","type":"Ethernet","enabled":true,"linkStatus":"Up","ip":"10.10.0.2","mac":"aa-bb-cc-dd-ee-ff"},
				{"name":"Guest","type":"Ethernet","enabled":true,"linkStatus":"Down","ip":""}
			]}}`))
		default:
			w.Write([]byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-1,"message":"unknown method"}}`))
		}
	}))
}

func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := NewClient(Config{BaseURL: url, Username: "admin", Password: "pw", Timeout: 2 * time.Second}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestClient_LoginAndDevices(t *testing.T) {
	srv := mockKerio(t)
	defer srv.Close()

	devices, err := newTestClient(t, srv.URL).Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(devices))
	}
	if devices[0].Name != "WAN" || devices[0].State != "online" || devices[0].Vendor != "kerio" {
		t.Errorf("WAN mapped wrong: %+v", devices[0])
	}
	if devices[1].Name != "Guest" || devices[1].State != "offline" {
		t.Errorf("Guest mapped wrong: %+v", devices[1])
	}
	if devices[0].Labels["iface_type"] != "Ethernet" {
		t.Errorf("iface_type label = %q, want Ethernet", devices[0].Labels["iface_type"])
	}
	if devices[0].MAC != "aa-bb-cc-dd-ee-ff" {
		t.Errorf("WAN MAC = %q, want aa-bb-cc-dd-ee-ff", devices[0].MAC)
	}
}

func TestClient_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Invalid username or password"}}`))
	}))
	defer srv.Close()

	err := newTestClient(t, srv.URL).Login(context.Background())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
}
