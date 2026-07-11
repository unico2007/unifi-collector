package alert

import "testing"

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
