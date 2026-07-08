package web

import "testing"

func TestParseKerioInboundDeny(t *testing.T) {
	msg := `KerioControl: DENY "Admin panel block" packet from internet, proto:TCP, len:40, 62.238.41.57:65301 -> 89.147.252.244:80, flags:[ SYN ], seq:1502936152 ack:0, win:65535, tcplen:0`
	ev, ok := parseKerio(msg)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if ev.action != "deny" {
		t.Errorf("action = %q", ev.action)
	}
	if ev.rule != "Admin panel block" {
		t.Errorf("rule = %q", ev.rule)
	}
	if ev.proto != "TCP" {
		t.Errorf("proto = %q", ev.proto)
	}
	if ev.srcIP != "62.238.41.57" {
		t.Errorf("srcIP = %q", ev.srcIP)
	}
	if !isPublicIP(ev.srcIP) {
		t.Errorf("expected %s public", ev.srcIP)
	}
}

func TestParseKerioInboundICMP(t *testing.T) {
	msg := `KerioControl: DENY "Admin panel block" packet from internet, proto:ICMP, len:84, 115.67.98.136 -> 89.147.252.244, type:8 code:0 id:4 seq:9 ttl:54`
	ev, ok := parseKerio(msg)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if ev.proto != "ICMP" {
		t.Errorf("proto = %q", ev.proto)
	}
	if ev.srcIP != "115.67.98.136" {
		t.Errorf("srcIP = %q", ev.srcIP)
	}
}

func TestParseKerioOutboundP2P(t *testing.T) {
	msg := `KerioControl: DENY [Rule] 'Peer to Peer traffic' [Connection] win-g6v9dt7o8g9 (10.10.0.250):6881 -> 59.114.239.221.broad (221.239.114.59):6881, UDP [Content] Suspected P2P`
	ev, ok := parseKerio(msg)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if ev.rule != "Peer to Peer traffic" {
		t.Errorf("rule = %q", ev.rule)
	}
	if ev.proto != "UDP" {
		t.Errorf("proto = %q", ev.proto)
	}
	if ev.content != "Suspected P2P" {
		t.Errorf("content = %q", ev.content)
	}
	// First IP is the internal host — must be excluded from public attacker ranking.
	if ev.srcIP != "10.10.0.250" {
		t.Errorf("srcIP = %q", ev.srcIP)
	}
	if isPublicIP(ev.srcIP) {
		t.Errorf("internal IP %s must not count as public", ev.srcIP)
	}
}

func TestParseKerioNonFirewallLineSkipped(t *testing.T) {
	if _, ok := parseKerio("KerioControl: some status message with no verdict"); ok {
		t.Error("expected non-firewall line to be skipped")
	}
}

func TestHourOf(t *testing.T) {
	if h := hourOf("15:43:24"); h != 15 {
		t.Errorf("hourOf = %d", h)
	}
	if h := hourOf(""); h != -1 {
		t.Errorf("hourOf empty = %d", h)
	}
}
