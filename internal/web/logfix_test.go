package web

import (
	"testing"

	"github.com/murad/unifi-collector/internal/web/query"
)

func TestDecodeAndParseNoJSONTail(t *testing.T) {
	raw := `{"event":"unknown","level":"info","msg":"CEF:0|Ubiquiti|UniFi OS|4.3.1|1|Admin Accessed UniFi OS|1|UNIFIdeviceName=Unico UCK G2 UNIFIcategory=admin msg=admin accessed the UniFi OS using the console's IP. Source IP: 10.10.0.243","vendor":"unifi","site":"default"}`

	msg, level := query.DecodeLogLine(raw, "warn")
	if level != "info" {
		t.Fatalf("level = %q, want info (inner JSON level)", level)
	}
	name, device, m, lvl, ok := parseCEF(msg)
	if !ok {
		t.Fatalf("parseCEF failed on decoded msg: %q", msg)
	}
	if name != "Admin Accessed UniFi OS" {
		t.Errorf("name = %q", name)
	}
	if device != "Unico UCK G2" {
		t.Errorf("device = %q", device)
	}
	if m != "admin accessed the UniFi OS using the console's IP. Source IP: 10.10.0.243" {
		t.Errorf("msg leaked JSON tail: %q", m)
	}
	if lvl != "info" {
		t.Errorf("cef level = %q", lvl)
	}

	// friendlyLog should render "<device>: <msg>" with no JSON residue.
	f := friendlyLog(msg)
	want := "Unico UCK G2: admin accessed the UniFi OS using the console's IP. Source IP: 10.10.0.243"
	if f != want {
		t.Errorf("friendlyLog = %q, want %q", f, want)
	}
}
