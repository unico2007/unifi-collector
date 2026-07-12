package alert

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
	form := url.Values{
		"chat_id":                  {chatID},
		"text":                     {text},
		"parse_mode":               {"HTML"},
		"disable_web_page_preview": {"true"},
	}
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

// notifyTransitions sends one structured message per fired / escalated /
// resolved alert. Called from the background evaluator; safe on a nil notifier.
func (n *notifier) notifyTransitions(ctx context.Context, fired, escalated, resolved []alertDTO) {
	if n == nil {
		return
	}
	for _, a := range fired {
		_ = n.sendTo(ctx, n.chatFor(a.Level), firedMessage(a))
	}
	for _, a := range escalated {
		// Escalations route to the new (higher) severity's chat, so the critical
		// channel learns an existing warning just got worse.
		_ = n.sendTo(ctx, n.chatFor(a.Level), escalatedMessage(a))
	}
	for _, a := range resolved {
		// Route the resolve to the same chat the fire went to, so a critical
		// alert clears where the team saw it raised.
		_ = n.sendTo(ctx, n.chatFor(a.Level), resolvedMessage(a))
	}
}

// bakuLoc pins message timestamps to Azerbaijan time (UTC+4, no DST since 2016)
// regardless of the container's TZ, so we don't depend on tzdata being present.
var bakuLoc = time.FixedZone("AZT", 4*60*60)

// severityLabel maps an internal level to the Azerbaijani word shown in bold.
func severityLabel(level string) (icon, word string) {
	if level == "critical" {
		return "🔴", "KRİTİK"
	}
	return "🟠", "XƏBƏRDARLIQ"
}

// stamp formats "now" as a Baku-local wall-clock string for the message footer.
func stamp() string { return time.Now().In(bakuLoc).Format("02.01.2006 15:04") }

// esc escapes the three characters Telegram's HTML parse mode is sensitive to,
// so device names or messages containing <, > or & never break the markup.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// firedMessage renders a newly raised alert.
func firedMessage(a alertDTO) string {
	icon, word := severityLabel(a.Level)
	return fmt.Sprintf("%s <b>%s</b>\n<b>%s</b>\n\n%s\n🕐 %s",
		icon, word, esc(a.Rule), esc(a.Message), stamp())
}

// escalatedMessage renders an already-open alert that just worsened in severity.
func escalatedMessage(a alertDTO) string {
	icon, word := severityLabel(a.Level)
	return fmt.Sprintf("%s <b>ESKALASİYA → %s</b>\n<b>%s</b>\n\n%s\n🕐 %s",
		icon, word, esc(a.Rule), esc(a.Message), stamp())
}

// resolvedMessage renders an alert that has cleared. It shows the target rather
// than the last reading, which is no longer meaningful once resolved.
func resolvedMessage(a alertDTO) string {
	return fmt.Sprintf("✅ <b>HƏLL OLUNDU</b>\n<b>%s</b>\n\n%s\n🕐 %s",
		esc(a.Rule), esc(a.Target), stamp())
}
