package syslog

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

func TestVendorFor(t *testing.T) {
	kerio := `<134>Jul  8 15:39:56 control KerioControl: DENY "Admin panel block" packet from internet`
	if v := vendorFor(kerio, "unifi"); v != "kerio" {
		t.Errorf("kerio line → %q, want kerio", v)
	}
	unifi := `<134>1 2026-07-08T15:39:56Z ap CEF:0|Ubiquiti|UniFi OS|...`
	if v := vendorFor(unifi, "unifi"); v != "unifi" {
		t.Errorf("unifi line → %q, want unifi (fallback)", v)
	}
}

func TestParse_RFC3164(t *testing.T) {
	p := parse("<34>Oct 11 22:14:15 aphost sshd: login failed for admin")
	if p.severity != 2 { // 34 % 8 = 2 (critical/error range)
		t.Errorf("severity = %d, want 2", p.severity)
	}
	if p.hostname != "aphost" {
		t.Errorf("hostname = %q, want aphost", p.hostname)
	}
	if !strings.Contains(p.message, "login failed") {
		t.Errorf("message = %q, want it to contain 'login failed'", p.message)
	}
}

func TestParse_RFC5424(t *testing.T) {
	p := parse(`<134>1 2026-07-02T12:00:00Z gw1 hostapd 123 - - client connected`)
	if p.severity != 6 { // 134 % 8 = 6 (info)
		t.Errorf("severity = %d, want 6", p.severity)
	}
	if p.hostname != "gw1" {
		t.Errorf("hostname = %q, want gw1", p.hostname)
	}
	if !strings.Contains(p.message, "client connected") {
		t.Errorf("message = %q, want it to contain 'client connected'", p.message)
	}
	if p.timestamp.Year() != 2026 {
		t.Errorf("timestamp year = %d, want 2026", p.timestamp.Year())
	}
}

func TestParse_PlainFallback(t *testing.T) {
	p := parse("just some text without priority")
	if p.severity != 6 {
		t.Errorf("severity = %d, want default 6", p.severity)
	}
	if p.message != "just some text without priority" {
		t.Errorf("message = %q", p.message)
	}
}

func TestSeverityToLevel(t *testing.T) {
	cases := map[int]string{0: "error", 3: "error", 4: "warning", 5: "info", 6: "info", 7: "debug"}
	for sev, want := range cases {
		if got := severityToLevel(sev); got != want {
			t.Errorf("severityToLevel(%d) = %q, want %q", sev, got, want)
		}
	}
}

type fakeSink struct{ ch chan models.Event }

func (f *fakeSink) WriteEvents(_ context.Context, events []models.Event) error {
	for _, e := range events {
		f.ch <- e
	}
	return nil
}

func TestReceiver_UDPRoundTrip(t *testing.T) {
	sink := &fakeSink{ch: make(chan models.Event, 1)}
	r := New(Config{UDPAddr: "127.0.0.1:15514", Vendor: "unifi", Site: "default"}, sink, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	time.Sleep(150 * time.Millisecond) // let the listener bind

	conn, err := net.Dial("udp", "127.0.0.1:15514")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("<34>Oct 11 22:14:15 aphost sshd: boom")); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case ev := <-sink.ch:
		if ev.Vendor != "unifi" || ev.Site != "default" {
			t.Errorf("labels wrong: %+v", ev)
		}
		if ev.Level != "error" {
			t.Errorf("level = %q, want error", ev.Level)
		}
		if ev.Hostname != "aphost" {
			t.Errorf("hostname = %q, want aphost", ev.Hostname)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarded event")
	}
}
