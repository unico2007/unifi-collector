package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Config configures the BFF server.
type Config struct {
	Addr          string // listen address, e.g. ":8080"
	StaticDir     string // path to the built frontend (web/dist)
	PrometheusURL string // e.g. http://prometheus:9090
	LokiURL       string // e.g. http://loki:3100
	AIURL         string // e.g. http://ai:8090
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
}

// Server is the BFF. It owns the user store, sessions, and upstream clients.
type Server struct {
	cfg      Config
	log      *zap.Logger
	http     *http.Server
	users    *userStore
	sessions *sessionStore
	prom     *promClient
	loki     *lokiClient
	aiProxy  *httputil.ReverseProxy
}

// New wires the server. The caller owns the userStore lifecycle via Close.
func New(cfg Config, users *userStore, log *zap.Logger) (*Server, error) {
	s := &Server{
		cfg:      cfg,
		log:      log,
		users:    users,
		sessions: newSessionStore(),
		prom:     newPromClient(cfg.PrometheusURL),
		loki:     newLokiClient(cfg.LokiURL),
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
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)

	// Data (auth-gated).
	mux.HandleFunc("GET /api/overview", s.requireAuth(s.handleOverview))
	mux.HandleFunc("GET /api/devices", s.requireAuth(s.handleDevices))
	mux.HandleFunc("GET /api/devices/{name}", s.requireAuth(s.handleDeviceDetail))
	mux.HandleFunc("GET /api/clients", s.requireAuth(s.handleClients))
	mux.HandleFunc("GET /api/wifi", s.requireAuth(s.handleWifi))
	mux.HandleFunc("GET /api/traffic", s.requireAuth(s.handleTraffic))
	mux.HandleFunc("GET /api/firewall", s.requireAuth(s.handleFirewall))
	mux.HandleFunc("GET /api/alerts", s.requireAuth(s.handleAlerts))
	mux.HandleFunc("GET /api/topology", s.requireAuth(s.handleTopology))
	mux.HandleFunc("GET /api/logs/categories", s.requireAuth(s.handleLogsCategories))

	// AI (auth-gated proxy to the AI service).
	if s.aiProxy != nil {
		mux.HandleFunc("/api/ai/", s.requireAuth(s.handleAIProxy))
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
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if s.cfg.StaticDir == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "unico BFF: frontend not bundled"})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
