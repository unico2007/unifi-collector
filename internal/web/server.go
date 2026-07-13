// Package web is the BFF (backend-for-frontend) for the Unico dashboard. It
// serves the built React app, proxies the AI service, authenticates users, and
// exposes /api/* JSON built from Prometheus + Loki — so the browser never talks
// to those systems directly. It is a thin composition root over the feature
// packages: auth, alert, handler, query and respond.
package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/murad/unifi-collector/internal/web/alert"
	"github.com/murad/unifi-collector/internal/web/auth"
	"github.com/murad/unifi-collector/internal/web/handler"
	"github.com/murad/unifi-collector/internal/web/query"
	"github.com/murad/unifi-collector/internal/web/respond"
)

// Config configures the BFF server.
type Config struct {
	Addr                   string // listen address, e.g. ":8080"
	StaticDir              string // path to the built frontend (web/dist)
	PrometheusURL          string // e.g. http://prometheus:9090
	LokiURL                string // e.g. http://loki:3100
	AIURL                  string // e.g. http://ai:8090
	TelegramToken          string // optional: Telegram bot token for alert notifications
	TelegramChatID         string // optional: Telegram chat id to notify
	TelegramCriticalChatID string // optional: route critical alerts to this chat instead
	CookieSecure           bool   // set Secure flag on the session cookie (enable when served over HTTPS)
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
}

// Server is the BFF composition root: it wires the feature packages (auth,
// alert, handler) to the HTTP router, serves the static frontend, proxies the
// AI service, and runs the background alert evaluator. The features own their
// own logic and state; this type only assembles and routes them.
type Server struct {
	cfg     Config
	log     *zap.Logger
	http    *http.Server
	authn   *auth.Service
	alerts  *alert.Service
	eval    *alert.Evaluator
	h       *handler.Handlers
	aiProxy *httputil.ReverseProxy
}

// New wires the server. The caller owns the auth.Store lifecycle via Close.
func New(cfg Config, users *auth.Store, log *zap.Logger) (*Server, error) {
	astore, err := alert.NewStore(users.DB())
	if err != nil {
		return nil, err
	}
	prom := query.NewPrometheus(cfg.PrometheusURL)
	loki := query.NewLoki(cfg.LokiURL)
	alerts := alert.NewService(prom, astore, cfg.TelegramToken, cfg.TelegramChatID, cfg.TelegramCriticalChatID)
	s := &Server{
		cfg:    cfg,
		log:    log,
		authn:  auth.NewService(users, cfg.CookieSecure),
		alerts: alerts,
		eval:   alert.NewEvaluator(alerts, log),
		h:      handler.New(prom, loki, alerts),
	}

	if cfg.AIURL != "" {
		target, err := url.Parse(cfg.AIURL)
		if err != nil {
			return nil, err
		}
		s.aiProxy = httputil.NewSingleHostReverseProxy(target)
	}

	mux := http.NewServeMux()

	// Auth.
	mux.HandleFunc("POST /api/login", s.authn.Login)
	mux.HandleFunc("POST /api/logout", s.authn.Logout)
	mux.HandleFunc("GET /api/me", s.authn.Me)

	// Data (auth-gated).
	mux.HandleFunc("GET /api/overview", s.authn.RequireAuth(s.h.Overview))
	mux.HandleFunc("GET /api/devices", s.authn.RequireAuth(s.h.Devices))
	mux.HandleFunc("GET /api/devices/{name}", s.authn.RequireAuth(s.h.DeviceDetail))
	mux.HandleFunc("GET /api/clients", s.authn.RequireAuth(s.h.Clients))
	mux.HandleFunc("GET /api/wifi", s.authn.RequireAuth(s.h.Wifi))
	mux.HandleFunc("GET /api/traffic", s.authn.RequireAuth(s.h.Traffic))
	mux.HandleFunc("GET /api/firewall", s.authn.RequireAuth(s.h.Firewall))
	mux.HandleFunc("GET /api/alerts", s.authn.RequireAuth(s.alerts.Alerts))
	mux.HandleFunc("GET /api/alerts/history", s.authn.RequireAuth(s.alerts.History))
	mux.HandleFunc("GET /api/alerts/settings", s.authn.RequireAuth(s.alerts.Settings))
	mux.HandleFunc("PUT /api/alerts/settings", s.authn.RequireAdmin(s.alerts.SettingsUpdate))
	mux.HandleFunc("POST /api/alerts/test-notify", s.authn.RequireAdmin(s.alerts.TestNotify))
	mux.HandleFunc("GET /api/topology", s.authn.RequireAuth(s.h.Topology))
	mux.HandleFunc("GET /api/logs/categories", s.authn.RequireAuth(s.h.LogsCategories))

	// AI (auth-gated proxy to the AI service).
	if s.aiProxy != nil {
		mux.HandleFunc("/api/ai/", s.authn.RequireAuth(s.handleAIProxy))
	}

	// Static frontend + SPA fallback (catch-all, must be last).
	mux.HandleFunc("/", s.handleStatic)

	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      securityHeaders(mux),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s, nil
}

// cspThemeScriptHash is the sha256 of the one inline <script> in web/index.html
// (the pre-paint theme-init snippet). It is pinned so the CSP can forbid all
// other inline scripts without an 'unsafe-inline' escape hatch. If that snippet
// is ever edited, recompute this hash (else the theme just flashes on load; the
// app still works).
const cspThemeScriptHash = "sha256-2mgJOBxM3lhZD3R9LzEcasRocw97lWvXwzftCs/a8z8="

// securityHeaders adds baseline hardening headers to every response: block MIME
// sniffing, deny framing (clickjacking), trim the referrer, and a conservative
// CSP for the self-hosted SPA. The app loads only same-origin assets and talks
// only to its own /api, so everything is locked to 'self'.
func securityHeaders(next http.Handler) http.Handler {
	csp := "default-src 'self'; " +
		"script-src 'self' '" + cspThemeScriptHash + "'; " +
		"style-src 'self' 'unsafe-inline'; " + // Tailwind + component inline styles
		"img-src 'self' data:; " +
		"connect-src 'self'; object-src 'none'; base-uri 'self'; " +
		"form-action 'self'; frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

// handleAIProxy forwards /api/ai/* to the AI service, rewriting the path from
// /api/ai/chat to /ai/chat.
func (s *Server) handleAIProxy(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
	s.aiProxy.ServeHTTP(w, r)
}

// handleStatic serves the built SPA. Unknown non-API paths fall back to
// index.html so client-side routing works on refresh/deep-link.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		respond.JSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if s.cfg.StaticDir == "" {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "unico BFF: frontend not bundled"})
		return
	}
	// Resolve the request against the static root and confirm it stays inside it,
	// so a crafted path (e.g. "..%2f..") can never escape to serve arbitrary
	// host files. net/http already cleans paths, but this is explicit defense.
	root := s.cfg.StaticDir
	full := filepath.Join(root, filepath.Clean("/"+r.URL.Path))
	if rel, err := filepath.Rel(root, full); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
		return
	}
	// Serve the file if it exists; otherwise fall back to index.html (SPA).
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, "index.html"))
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	go s.eval.Run(ctx)
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("bff listening",
			zap.String("addr", s.http.Addr),
			zap.String("prometheus", s.cfg.PrometheusURL),
			zap.String("loki", s.cfg.LokiURL),
			zap.String("ai", s.cfg.AIURL))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.log.Info("bff shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}
