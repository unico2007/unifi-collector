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
	token  string
	chatID string
	http   *http.Client
}

// newNotifier returns a configured notifier, or nil (disabled) when either
// credential is missing.
func newNotifier(token, chatID string) *notifier {
	if token == "" || chatID == "" {
		return nil
	}
	return &notifier{token: token, chatID: chatID, http: &http.Client{Timeout: 10 * time.Second}}
}

// enabled reports whether notifications are configured.
func (n *notifier) enabled() bool { return n != nil }

// send posts one plain-text message to the configured chat. A nil notifier is a
// no-op so callers don't need to branch.
func (n *notifier) send(ctx context.Context, text string) error {
	if n == nil {
		return nil
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)
	form := url.Values{"chat_id": {n.chatID}, "text": {text}, "disable_web_page_preview": {"true"}}
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
		_ = n.send(ctx, fmt.Sprintf("%s Unico alert: %s\n%s", icon, a.Rule, a.Message))
	}
	for _, a := range resolved {
		_ = n.send(ctx, fmt.Sprintf("✅ Həll olundu: %s — %s", a.Rule, a.Target))
	}
}
