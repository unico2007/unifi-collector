// Package server exposes the operational HTTP surface: Prometheus metrics plus
// Kubernetes-style liveness (/healthz) and readiness (/readyz) probes. It owns
// its own Config so it stays decoupled from internal/config.
package server

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Config configures the HTTP server.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Server serves /metrics, /healthz and /readyz.
type Server struct {
	log  *zap.Logger
	http *http.Server

	// ready flips to true once the app can serve real data (e.g. after the
	// first successful UniFi login). Liveness is independent of this.
	ready atomic.Bool
}

// New builds a server. metricsHandler is mounted at /metrics.
func New(cfg Config, metricsHandler http.Handler, log *zap.Logger) *Server {
	s := &Server{log: log}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/healthz", s.handleLive)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/", s.handleRoot)

	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s
}

// SetReady marks the server ready (or not) to serve real data.
func (s *Server) SetReady(ready bool) { s.ready.Store(ready) }

// Run starts the server and blocks until ctx is cancelled, then shuts it down
// gracefully. It returns a non-nil error only on an unexpected serve failure.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", zap.String("addr", s.http.Addr))
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
		s.log.Info("http server shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}

func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeText(w, http.StatusOK, "ok")
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	if s.ready.Load() {
		writeText(w, http.StatusOK, "ready")
		return
	}
	writeText(w, http.StatusServiceUnavailable, "not ready")
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeText(w, http.StatusNotFound, "not found")
		return
	}
	writeText(w, http.StatusOK, "collector: see /metrics, /healthz, /readyz")
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body + "\n"))
}
