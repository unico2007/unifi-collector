// Package kerio is the Kerio Control vendor adapter. It talks to the Kerio
// Control Administration API (JSON-RPC 2.0 over HTTPS, default port 4081) and
// maps the responses into the neutral models, satisfying the collector.Source
// capability interfaces — exactly like internal/unifi, proving the framework
// extends to a new vendor without touching the core.
//
// Like the other vendor adapters, this package owns its own Config and does not
// import internal/config.
package kerio

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Config holds everything needed to reach a Kerio Control appliance.
type Config struct {
	BaseURL   string // e.g. https://10.10.0.2:4081
	Username  string
	Password  string
	VerifyTLS bool
	Timeout   time.Duration
}

// ErrAuth is returned when Kerio rejects the credentials.
var ErrAuth = errors.New("kerio: authentication failed")

// Client is a concurrency-safe Kerio Control JSON-RPC client.
type Client struct {
	cfg Config
	log *zap.Logger
	hc  *http.Client

	loginMu sync.Mutex
	mu      sync.RWMutex
	token   string
	ready   bool
	nextID  int
}

// rpcRequest / rpcResponse model the JSON-RPC 2.0 envelope.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// NewClient constructs a client. logger must be non-nil.
func NewClient(cfg Config, log *zap.Logger) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("kerio: BaseURL is required")
	}
	if log == nil {
		return nil, fmt.Errorf("kerio: logger is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("kerio: cookie jar: %w", err)
	}
	return &Client{
		cfg: cfg,
		log: log,
		hc: &http.Client{
			Jar:       jar,
			Timeout:   cfg.Timeout + 5*time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.VerifyTLS}}, //nolint:gosec
		},
	}, nil
}

func (c *Client) endpoint() string { return c.cfg.BaseURL + "/admin/api/jsonrpc/" }

// Login authenticates via Session.login and stores the session token.
func (c *Client) Login(ctx context.Context) error {
	return c.ensureLoggedIn(ctx)
}

func (c *Client) ensureLoggedIn(ctx context.Context) error {
	c.mu.RLock()
	ok := c.ready
	c.mu.RUnlock()
	if ok {
		return nil
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.mu.RLock()
	ok = c.ready
	c.mu.RUnlock()
	if ok {
		return nil
	}
	return c.doLogin(ctx)
}

func (c *Client) doLogin(ctx context.Context) error {
	params := map[string]any{
		"userName": c.cfg.Username,
		"password": c.cfg.Password,
		"application": map[string]string{
			"name":    "unifi-collector",
			"vendor":  "murad",
			"version": "1.0",
		},
	}
	// Login itself carries no X-Token yet.
	resp, err := c.rawCall(ctx, "Session.login", params, "")
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("%w: %s", ErrAuth, resp.Error.Message)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return fmt.Errorf("kerio: decoding login result: %w", err)
	}
	c.mu.Lock()
	c.token = out.Token
	c.ready = true
	c.mu.Unlock()
	c.log.Info("kerio: logged in")
	return nil
}

// call performs an authenticated JSON-RPC call and decodes result into out.
// It transparently re-authenticates once if the session expired.
func (c *Client) call(ctx context.Context, method string, params, out any) error {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	resp, err := c.rawCall(ctx, method, params, token)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		// Kerio uses error code for auth/session issues; re-login once.
		if isSessionError(resp.Error) {
			c.invalidate()
			if err := c.ensureLoggedIn(ctx); err != nil {
				return err
			}
			c.mu.RLock()
			token = c.token
			c.mu.RUnlock()
			resp, err = c.rawCall(ctx, method, params, token)
			if err != nil {
				return err
			}
			if resp.Error != nil {
				return fmt.Errorf("kerio: %s: %s", method, resp.Error.Message)
			}
		} else {
			return fmt.Errorf("kerio: %s: %s", method, resp.Error.Message)
		}
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("kerio: decoding %s result: %w", method, err)
		}
	}
	return nil
}

// rawCall sends a single JSON-RPC request. token may be empty (login).
func (c *Client) rawCall(ctx context.Context, method string, params any, token string) (*rpcResponse, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("kerio: encoding request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("X-Token", token)
	}

	httpResp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, httpResp.Body); _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("kerio: %s: HTTP %d", method, httpResp.StatusCode)
	}
	var out rpcResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("kerio: decoding response: %w", err)
	}
	return &out, nil
}

func (c *Client) invalidate() {
	c.mu.Lock()
	c.ready = false
	c.mu.Unlock()
}

// isSessionError reports whether an RPC error indicates an expired/invalid
// session (so a re-login should be attempted).
func isSessionError(e *rpcError) bool {
	m := strings.ToLower(e.Message)
	return strings.Contains(m, "session") || strings.Contains(m, "login") || strings.Contains(m, "token")
}
