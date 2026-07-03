package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func newTestServer() *Server {
	return New(Config{Addr: ":0"}, http.NotFoundHandler(), zap.NewNop())
}

func TestLivenessAlwaysOK(t *testing.T) {
	s := newTestServer()
	rec := httptest.NewRecorder()
	s.handleLive(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d, want 200", rec.Code)
	}
}

func TestReadinessReflectsState(t *testing.T) {
	s := newTestServer()

	rec := httptest.NewRecorder()
	s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz before ready = %d, want 503", rec.Code)
	}

	s.SetReady(true)
	rec = httptest.NewRecorder()
	s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz after ready = %d, want 200", rec.Code)
	}
}

func TestRootAnd404(t *testing.T) {
	s := newTestServer()

	rec := httptest.NewRecorder()
	s.handleRoot(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/ = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleRoot(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("/nope = %d, want 404", rec.Code)
	}
}
