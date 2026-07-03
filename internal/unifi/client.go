// Package unifi is the UniFi vendor adapter. This file implements the
// low-level authenticated HTTP client: login, session-cookie management with
// automatic re-authentication, CSRF handling, retries and timeouts. The
// higher-level Devices/Clients/Events/Health methods (which satisfy the
// collector.Source interfaces) are built on top of this core in a later stage.
//
// This package deliberately does NOT import internal/config: it owns its own
// Config struct so the vendor adapter stays decoupled from the app's config
// shape. Wiring code maps one onto the other.
package unifi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Config holds everything the client needs to reach a UniFi controller.
type Config struct {
	BaseURL    string // e.g. https://192.168.1.1:8443
	Username   string
	Password   string
	Site       string // e.g. "default"
	VerifyTLS  bool
	Timeout    time.Duration // per-request timeout
	MaxRetries int           // retry attempts for transient failures
	// AuthFailureCooldown suppresses repeated login attempts after credentials
	// are rejected, protecting controllers from auth-rate-limit lockouts.
	AuthFailureCooldown time.Duration
}

// ErrAuth is returned when the controller rejects the supplied credentials.
var ErrAuth = errors.New("unifi: authentication failed")

// Client is a concurrency-safe UniFi HTTP client.
type Client struct {
	cfg Config
	log *zap.Logger
	hc  *http.Client

	loginMu sync.Mutex // serializes login so concurrent callers don't stampede

	mu        sync.RWMutex // guards the fields below
	loggedIn  bool
	unifiOS   bool   // true => UDM/UniFi-OS style (/proxy/network, CSRF)
	csrfToken string // captured from responses, sent on mutating requests

	authFailureErr error
	authFailureAt  time.Time

	eventsUnsupportedLogged bool

	// Events endpoint auto-discovery: resolved lazily on the first successful
	// Events() call using the live authenticated session (no extra logins).
	evResolved       bool
	evUnsupported    bool
	evResolvedMethod string
	evResolvedPath   string
}

// apiResponse is the standard UniFi JSON envelope.
type apiResponse struct {
	Meta struct {
		RC  string `json:"rc"`  // "ok" | "error"
		Msg string `json:"msg"` // e.g. "api.err.LoginRequired"
	} `json:"meta"`
	Data json.RawMessage `json:"data"`
}

type statusError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unifi: %s %s: status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

// NewClient constructs a client. The logger must be non-nil.
func NewClient(cfg Config, log *zap.Logger) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("unifi: BaseURL is required")
	}
	if log == nil {
		return nil, fmt.Errorf("unifi: logger is required")
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.AuthFailureCooldown <= 0 {
		cfg.AuthFailureCooldown = time.Minute
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("unifi: cookie jar: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.VerifyTLS}, //nolint:gosec // controlled by config
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}

	return &Client{
		cfg: cfg,
		log: log,
		hc: &http.Client{
			Jar:       jar,
			Transport: transport,
			// Per-request timeout is enforced via context; keep the client
			// timeout as a hard upper bound safety net.
			Timeout: cfg.Timeout + 5*time.Second,
		},
	}, nil
}

// ---- Public API ----------------------------------------------------------

// Login authenticates and stores the session. It is safe to call concurrently;
// only one login runs at a time.
func (c *Client) Login(ctx context.Context) error {
	return c.ensureLoggedIn(ctx)
}

// GetJSON performs an authenticated GET against an API path (relative to the
// site API root, e.g. "stat/device") and unmarshals the envelope's data field
// into out. It transparently re-authenticates once if the session expired.
func (c *Client) GetJSON(ctx context.Context, apiPath string, out any) error {
	resp, err := c.doAPI(ctx, http.MethodGet, apiPath, nil, true)
	if err != nil {
		return err
	}
	return decodeData(apiPath, resp, out)
}

// PostJSON performs an authenticated POST against a site API path and decodes
// the envelope's data field into out.
func (c *Client) PostJSON(ctx context.Context, apiPath string, payload any, out any) error {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("unifi: encoding %s request: %w", apiPath, err)
		}
	}
	resp, err := c.doAPI(ctx, http.MethodPost, apiPath, body, true)
	if err != nil {
		return err
	}
	return decodeData(apiPath, resp, out)
}

// ---- Authentication ------------------------------------------------------

func (c *Client) ensureLoggedIn(ctx context.Context) error {
	now := time.Now()

	c.mu.RLock()
	ok := c.loggedIn
	cachedErr := c.cachedAuthFailureLocked(now)
	c.mu.RUnlock()
	if ok {
		return nil
	}
	if cachedErr != nil {
		return cachedErr
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	// Re-check under the login lock: another goroutine may have just logged in.
	now = time.Now()
	c.mu.RLock()
	ok = c.loggedIn
	cachedErr = c.cachedAuthFailureLocked(now)
	c.mu.RUnlock()
	if ok {
		return nil
	}
	if cachedErr != nil {
		return cachedErr
	}

	if err := c.doLogin(ctx); err != nil {
		if errors.Is(err, ErrAuth) {
			c.recordAuthFailure(err)
		}
		return err
	}
	c.clearAuthFailure()
	return nil
}

func (c *Client) doLogin(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	})

	// Try UniFi-OS first; fall back to the classic controller on 404.
	for _, attempt := range []struct {
		os   bool
		path string
	}{
		{true, "/api/auth/login"},
		{false, "/api/login"},
	} {
		resp, err := c.sendWithRetry(ctx, http.MethodPost, c.cfg.BaseURL+attempt.path, body, false)
		if err != nil {
			return fmt.Errorf("unifi: login request: %w", err)
		}
		func() { defer resp.Body.Close(); io.Copy(io.Discard, resp.Body) }()

		switch {
		case resp.StatusCode == http.StatusOK:
			c.mu.Lock()
			c.loggedIn = true
			c.unifiOS = attempt.os
			if t := resp.Header.Get("X-CSRF-Token"); t != "" {
				c.csrfToken = t
			}
			c.mu.Unlock()
			c.log.Info("unifi: logged in", zap.Bool("unifi_os", attempt.os))
			return nil
		case resp.StatusCode == http.StatusNotFound && attempt.os:
			// Not a UniFi-OS device — try the classic path.
			continue
		case resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusBadRequest ||
			resp.StatusCode == http.StatusTooManyRequests:
			return fmt.Errorf("%w (status %d)", ErrAuth, resp.StatusCode)
		default:
			return fmt.Errorf("unifi: login failed with status %d", resp.StatusCode)
		}
	}
	return fmt.Errorf("unifi: login failed: no compatible API endpoint")
}

func (c *Client) cachedAuthFailureLocked(now time.Time) error {
	if c.authFailureErr == nil {
		return nil
	}
	if now.Sub(c.authFailureAt) >= c.cfg.AuthFailureCooldown {
		return nil
	}
	return c.authFailureErr
}

func (c *Client) recordAuthFailure(err error) {
	c.mu.Lock()
	c.loggedIn = false
	c.authFailureErr = err
	c.authFailureAt = time.Now()
	c.mu.Unlock()
}

func (c *Client) clearAuthFailure() {
	c.mu.Lock()
	c.authFailureErr = nil
	c.authFailureAt = time.Time{}
	c.mu.Unlock()
}

// ---- Request execution ---------------------------------------------------

// doAPI performs a request against a site-scoped API path (e.g. "stat/device")
// that returns the UniFi JSON envelope. The full URL is built AFTER login so
// the correct (classic vs UniFi-OS) path prefix is used. If allowReauth is set
// and the session is found to be expired, it re-authenticates once and retries.
func (c *Client) doAPI(ctx context.Context, method, apiPath string, body []byte, allowReauth bool) (*apiResponse, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}

	fullURL := c.siteURL(apiPath)
	resp, err := c.sendWithRetry(ctx, method, fullURL, body, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Capture a refreshed CSRF token if present.
	if t := resp.Header.Get("X-CSRF-Token"); t != "" {
		c.mu.Lock()
		c.csrfToken = t
		c.mu.Unlock()
	}

	// UniFi-OS signals an expired session with HTTP 401.
	if resp.StatusCode == http.StatusUnauthorized && allowReauth {
		c.invalidateSession()
		return c.doAPI(ctx, method, apiPath, body, false)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unifi: reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &statusError{
			Method:     method,
			URL:        fullURL,
			StatusCode: resp.StatusCode,
			Body:       snippet(raw),
		}
	}

	var out apiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unifi: decoding envelope: %w", err)
	}

	// Classic controllers signal an expired session inside the body.
	if out.Meta.RC != "ok" {
		if allowReauth && strings.Contains(out.Meta.Msg, "LoginRequired") {
			c.invalidateSession()
			return c.doAPI(ctx, method, apiPath, body, false)
		}
		return nil, fmt.Errorf("unifi: api error: %s", out.Meta.Msg)
	}
	return &out, nil
}

func (c *Client) invalidateSession() {
	c.mu.Lock()
	c.loggedIn = false
	c.mu.Unlock()
}

func decodeData(apiPath string, resp *apiResponse, out any) error {
	if out != nil && len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, out); err != nil {
			return fmt.Errorf("unifi: decoding %s data: %w", apiPath, err)
		}
	}
	return nil
}

// sendWithRetry sends a request, retrying transient failures with exponential
// backoff. body may be nil; it is re-read on every attempt.
func (c *Client) sendWithRetry(ctx context.Context, method, fullURL string, body []byte, retryTooManyRequests bool) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := backoff(ctx, attempt); err != nil {
				return nil, err
			}
			c.log.Debug("unifi: retrying request",
				zap.String("url", fullURL), zap.Int("attempt", attempt))
		}

		resp, err := c.send(ctx, method, fullURL, body)
		if err != nil {
			lastErr = err
			continue // network-level error: retry
		}
		if resp.StatusCode >= 500 || (retryTooManyRequests && resp.StatusCode == http.StatusTooManyRequests) {
			lastErr = fmt.Errorf("unifi: transient status %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("unifi: request failed after %d attempts: %w", c.cfg.MaxRetries+1, lastErr)
}

// send builds and executes a single HTTP request with a per-request timeout.
func (c *Client) send(ctx context.Context, method, fullURL string, body []byte) (*http.Response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	// NOTE: cancel is tied to the response lifetime; we cancel when the body
	// is closed by wrapping via context on the request only. To keep it simple
	// and avoid leaking, we cancel after the caller finishes by attaching to a
	// closer below.
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, fullURL, reader)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unifi: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		c.mu.RLock()
		if c.unifiOS && c.csrfToken != "" {
			req.Header.Set("X-CSRF-Token", c.csrfToken)
		}
		c.mu.RUnlock()
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	// Ensure the per-request context is released when the body is closed.
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// ---- URL helpers ---------------------------------------------------------

// siteURL builds the full URL for a site-scoped API path, accounting for the
// UniFi-OS /proxy/network prefix.
func (c *Client) siteURL(apiPath string) string {
	c.mu.RLock()
	os := c.unifiOS
	c.mu.RUnlock()
	prefix := ""
	if os {
		prefix = "/proxy/network"
	}
	return fmt.Sprintf("%s%s/api/s/%s/%s", c.cfg.BaseURL, prefix, c.cfg.Site, strings.TrimLeft(apiPath, "/"))
}

// ---- small helpers -------------------------------------------------------

// cancelReadCloser cancels the per-request context once the body is closed.
type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func backoff(ctx context.Context, attempt int) error {
	d := time.Duration(math.Min(float64(200*time.Millisecond)*math.Pow(2, float64(attempt-1)), float64(5*time.Second)))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}
