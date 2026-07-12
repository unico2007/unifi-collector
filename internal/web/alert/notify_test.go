package alert

import (
	"strings"
	"testing"
)

func TestNotifierChatRouting(t *testing.T) {
	t.Run("nil is disabled", func(t *testing.T) {
		var n *notifier
		if n.enabled() || n.criticalRouting() {
			t.Fatal("nil notifier must be disabled and non-routing")
		}
	})

	t.Run("no critical chat falls back to default", func(t *testing.T) {
		n := newNotifier("tok", "100", "")
		if n == nil {
			t.Fatal("expected configured notifier")
		}
		if n.criticalRouting() {
			t.Fatal("criticalRouting should be false without a critical chat")
		}
		if got := n.chatFor("critical"); got != "100" {
			t.Fatalf("critical should fall back to default chat, got %q", got)
		}
		if got := n.chatFor("warning"); got != "100" {
			t.Fatalf("warning should use default chat, got %q", got)
		}
	})

	t.Run("critical chat routes only critical", func(t *testing.T) {
		n := newNotifier("tok", "100", "999")
		if !n.criticalRouting() {
			t.Fatal("criticalRouting should be true")
		}
		if got := n.chatFor("critical"); got != "999" {
			t.Fatalf("critical should route to critical chat, got %q", got)
		}
		if got := n.chatFor("warning"); got != "100" {
			t.Fatalf("warning should stay on default chat, got %q", got)
		}
	})

	t.Run("missing token or default chat disables", func(t *testing.T) {
		if newNotifier("", "100", "") != nil {
			t.Fatal("missing token must disable")
		}
		if newNotifier("tok", "", "") != nil {
			t.Fatal("missing default chat must disable")
		}
	})
}

func TestMessageFormats(t *testing.T) {
	warn := alertDTO{Level: "warning", Rule: "CPU yüksək", Target: "AP-Ofis-1", Message: "AP-Ofis-1: CPU 85%", Value: "85%"}
	crit := alertDTO{Level: "critical", Rule: "Cihaz offline", Target: "AP-Ofis-1", Message: "AP-Ofis-1 (uap) offline-dır", Value: "offline"}

	fired := firedMessage(warn)
	for _, want := range []string{"🟠", "<b>XƏBƏRDARLIQ</b>", "<b>CPU yüksək</b>", "AP-Ofis-1: CPU 85%", "🕐"} {
		if !strings.Contains(fired, want) {
			t.Errorf("firedMessage missing %q:\n%s", want, fired)
		}
	}
	if strings.Contains(fired, "ESKALASİYA") {
		t.Errorf("a fresh fire must not be labelled an escalation:\n%s", fired)
	}

	esc := escalatedMessage(crit)
	for _, want := range []string{"🔴", "ESKALASİYA → KRİTİK", "<b>Cihaz offline</b>"} {
		if !strings.Contains(esc, want) {
			t.Errorf("escalatedMessage missing %q:\n%s", want, esc)
		}
	}

	res := resolvedMessage(warn)
	for _, want := range []string{"✅", "<b>HƏLL OLUNDU</b>", "<b>CPU yüksək</b>", "AP-Ofis-1", "🕐"} {
		if !strings.Contains(res, want) {
			t.Errorf("resolvedMessage missing %q:\n%s", want, res)
		}
	}

	// HTML-sensitive characters in a device name must be escaped so the markup
	// never breaks under parse_mode=HTML.
	tricky := alertDTO{Level: "warning", Rule: "CPU yüksək", Message: "AP <lab> & co: CPU 90%"}
	got := firedMessage(tricky)
	if strings.Contains(got, "<lab>") || !strings.Contains(got, "&lt;lab&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("device name not HTML-escaped:\n%s", got)
	}
}
