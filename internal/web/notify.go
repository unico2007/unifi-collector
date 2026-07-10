package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// notifier sends alert notifications to a Telegram chat. It is nil when no
// token/chat is configured, so every call site must tolerate a nil receiver
// (the methods below do). Secrets come from the server .env (WEB_TELEGRAM_*),
// never from code or git.
type notifier struct {
	token          string
	chatID         string
	criticalChatID string // optional: critical alerts route here instead
	http           *http.Client
}

// newNotifier returns a configured notifier, or nil (disabled) when the token or
// default chat is missing. criticalChatID is optional: when set, critical alerts
// (fire and resolve) go there instead of the default chat; warnings always use
// the default chat.
func newNotifier(token, chatID, criticalChatID string) *notifier {
	if token == "" || chatID == "" {
		return nil
	}
	return &notifier{token: token, chatID: chatID, criticalChatID: criticalChatID, http: &http.Client{Timeout: 10 * time.Second}}
}

// enabled reports whether notifications are configured.
func (n *notifier) enabled() bool { return n != nil }

// criticalRouting reports whether a separate critical-severity chat is set.
func (n *notifier) criticalRouting() bool { return n != nil && n.criticalChatID != "" }

// chatFor returns the destination chat for an alert of the given severity:
// the critical chat for "critical" when configured, otherwise the default chat.
func (n *notifier) chatFor(level string) string {
	if level == "critical" && n.criticalChatID != "" {
		return n.criticalChatID
	}
	return n.chatID
}

// send posts one plain-text message to the default chat. A nil notifier is a
// no-op so callers don't need to branch.
func (n *notifier) send(ctx context.Context, text string) error {
	if n == nil {
		return nil
	}
	return n.sendTo(ctx, n.chatID, text)
}

// sendTo posts one plain-text message to a specific chat. A nil notifier is a
// no-op.
func (n *notifier) sendTo(ctx context.Context, chatID, text string) error {
	if n == nil {
		return nil
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)
	form := url.Values{"chat_id": {chatID}, "text": {text}, "disable_web_page_preview": {"true"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// notifyTransitions sends one message per fired/resolved alert. Called from the
// background evaluator; safe on a nil notifier.
func (n *notifier) notifyTransitions(ctx context.Context, fired, resolved []alertDTO) {
	if n == nil {
		return
	}
	for _, a := range fired {
		icon := "🟠"
		if a.Level == "critical" {
			icon = "🔴"
		}
		_ = n.sendTo(ctx, n.chatFor(a.Level), fmt.Sprintf("%s Unico alert: %s\n%s", icon, a.Rule, a.Message))
	}
	for _, a := range resolved {
		// Route the resolve to the same chat the fire went to, so a critical
		// alert clears where the team saw it raised.
		_ = n.sendTo(ctx, n.chatFor(a.Level), fmt.Sprintf("✅ Həll olundu: %s — %s", a.Rule, a.Target))
	}
}
