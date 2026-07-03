package unifi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := NewClient(Config{
		BaseURL:             baseURL,
		Username:            "admin",
		Password:            "secret",
		Site:                "default",
		Timeout:             2 * time.Second,
		MaxRetries:          2,
		AuthFailureCooldown: time.Minute,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// Verifies UniFi-OS login + that an expired session (HTTP 401) triggers exactly
// one transparent re-login and a successful retry.
func TestClient_LoginAndAutoReauth(t *testing.T) {
	var logins int32
	var dataCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			atomic.AddInt32(&logins, 1)
			w.Header().Set("X-CSRF-Token", "tok123")
			w.WriteHeader(http.StatusOK)
		case "/proxy/network/api/s/default/stat/device":
			// First data call: pretend the session expired.
			if atomic.AddInt32(&dataCalls, 1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"name":"ap1"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	var out []struct {
		Name string `json:"name"`
	}
	if err := c.GetJSON(context.Background(), "stat/device", &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	if len(out) != 1 || out[0].Name != "ap1" {
		t.Fatalf("unexpected data: %+v", out)
	}
	if got := atomic.LoadInt32(&logins); got != 2 {
		t.Errorf("logins = %d, want 2 (initial + re-auth)", got)
	}
	if got := atomic.LoadInt32(&dataCalls); got != 2 {
		t.Errorf("data calls = %d, want 2", got)
	}
}

// Verifies fallback to the classic controller when the UniFi-OS login path 404s.
func TestClient_ClassicFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			w.WriteHeader(http.StatusNotFound) // not a UniFi-OS device
		case "/api/login":
			w.WriteHeader(http.StatusOK)
		case "/api/s/default/stat/device":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if err := c.GetJSON(context.Background(), "stat/device", nil); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if c.unifiOS {
		t.Error("expected classic controller (unifiOS=false)")
	}
}

// Verifies bad credentials surface as ErrAuth and are not retried as transient.
func TestClient_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	err := c.Login(context.Background())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("error = %v, want ErrAuth", err)
	}
}

func TestClient_AuthFailureCooldownSuppressesConcurrentLogins(t *testing.T) {
	var logins int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			atomic.AddInt32(&logins, 1)
			time.Sleep(25 * time.Millisecond)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	const callers = 12
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- c.Login(context.Background())
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if !errors.Is(err, ErrAuth) {
			t.Fatalf("Login error = %v, want ErrAuth", err)
		}
	}
	if got := atomic.LoadInt32(&logins); got != 1 {
		t.Fatalf("login attempts = %d, want 1", got)
	}
}

func TestClient_Login429IsNotRetried(t *testing.T) {
	var logins int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			atomic.AddInt32(&logins, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.cfg.MaxRetries = 5

	err := c.Login(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("Login error = %v, want ErrAuth", err)
	}
	if got := atomic.LoadInt32(&logins); got != 1 {
		t.Fatalf("login attempts = %d, want 1", got)
	}

	err = c.Login(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("cached Login error = %v, want ErrAuth", err)
	}
	if got := atomic.LoadInt32(&logins); got != 1 {
		t.Fatalf("login attempts after cached failure = %d, want 1", got)
	}
}
