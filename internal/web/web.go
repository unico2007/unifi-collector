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
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
}

// Server is the BFF. It owns the user store, sessions, and upstream clients.
type Server struct {
	cfg     Config
	log     *zap.Logger
	http    *http.Server
	authn   *auth.Service
	alerts  *alert.Service
	eval    *alert.Evaluator
	prom    *query.Prometheus
	loki    *query.Loki
	aiProxy *httputil.ReverseProxy
}

// New wires the server. The caller owns the auth.Store lifecycle via Close.
func New(cfg Config, users *auth.Store, log *zap.Logger) (*Server, error) {
	astore, err := alert.NewStore(users.DB())
	if err != nil {
		return nil, err
	}
	prom := query.NewPrometheus(cfg.PrometheusURL)
	alerts := alert.NewService(prom, astore, cfg.TelegramToken, cfg.TelegramChatID, cfg.TelegramCriticalChatID)
	s := &Server{
		cfg:    cfg,
		log:    log,
		authn:  auth.NewService(users),
		alerts: alerts,
		eval:   alert.NewEvaluator(alerts, log),
		prom:   prom,
		loki:   query.NewLoki(cfg.LokiURL),
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
	mux.HandleFunc("GET /api/overview", s.authn.RequireAuth(s.handleOverview))
	mux.HandleFunc("GET /api/devices", s.authn.RequireAuth(s.handleDevices))
	mux.HandleFunc("GET /api/devices/{name}", s.authn.RequireAuth(s.handleDeviceDetail))
	mux.HandleFunc("GET /api/clients", s.authn.RequireAuth(s.handleClients))
	mux.HandleFunc("GET /api/wifi", s.authn.RequireAuth(s.handleWifi))
	mux.HandleFunc("GET /api/traffic", s.authn.RequireAuth(s.handleTraffic))
	mux.HandleFunc("GET /api/firewall", s.authn.RequireAuth(s.handleFirewall))
	mux.HandleFunc("GET /api/alerts", s.authn.RequireAuth(s.alerts.Alerts))
	mux.HandleFunc("GET /api/alerts/history", s.authn.RequireAuth(s.alerts.History))
	mux.HandleFunc("GET /api/alerts/settings", s.authn.RequireAuth(s.alerts.Settings))
	mux.HandleFunc("PUT /api/alerts/settings", s.authn.RequireAdmin(s.alerts.SettingsUpdate))
	mux.HandleFunc("POST /api/alerts/test-notify", s.authn.RequireAdmin(s.alerts.TestNotify))
	mux.HandleFunc("GET /api/topology", s.authn.RequireAuth(s.handleTopology))
	mux.HandleFunc("GET /api/logs/categories", s.authn.RequireAuth(s.handleLogsCategories))

	// AI (auth-gated proxy to the AI service).
	if s.aiProxy != nil {
		mux.HandleFunc("/api/ai/", s.authn.RequireAuth(s.handleAIProxy))
	}

	// Static frontend + SPA fallback (catch-all, must be last).
	mux.HandleFunc("/", s.handleStatic)

	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s, nil
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
	clean := filepath.Clean(r.URL.Path)
	full := filepath.Join(s.cfg.StaticDir, clean)
	// Serve the file if it exists; otherwise fall back to index.html (SPA).
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.cfg.StaticDir, "index.html"))
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
