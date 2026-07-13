package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/murad/unifi-collector/internal/web/respond"
)

// maxLoginBody caps the login request body so a client can't stream an
// unbounded payload into the JSON decoder.
const maxLoginBody = 4 << 10 // 4 KiB

// Service wires the user store to the session signer and exposes the auth
// HTTP handlers plus the route-guarding middleware.
type Service struct {
	store    *Store
	signer   *signer
	throttle *loginThrottle
	secure   bool // set the Secure flag on session cookies (enable behind HTTPS)
}

// NewService builds the auth service on an open user store. secureCookie should
// be true only when the dashboard is served over HTTPS (otherwise the browser
// won't send the cookie over plain-HTTP LAN and login breaks).
func NewService(store *Store, secureCookie bool) *Service {
	return &Service{
		store:    store,
		signer:   newSigner(store.secret),
		throttle: newLoginThrottle(),
		secure:   secureCookie,
	}
}

// clientIP extracts the best-effort client address for throttling.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResp struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Login verifies credentials and sets the session cookie.
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	now := time.Now()
	if wait := s.throttle.blockedFor(ip, now); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		respond.JSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "Çox sayda uğursuz cəhd. Bir azdan yenidən yoxlayın.",
		})
		return
	}

	var req loginReq
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	role, ok := s.store.verify(req.Username, req.Password)
	if !ok {
		s.throttle.recordFailure(ip, now)
		respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "İstifadəçi adı və ya parol yanlışdır"})
		return
	}
	s.throttle.reset(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signer.sign(req.Username, role),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	respond.JSON(w, http.StatusOK, userResp{Username: req.Username, Role: role})
}

// Logout clears the session cookie. Flags mirror the ones set at login so the
// browser reliably matches and removes the cookie.
func (s *Service) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me returns the current session's user, or 401.
func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	respond.JSON(w, http.StatusOK, userResp{Username: sess.username, Role: sess.role})
}

func (s *Service) currentSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	return s.signer.verify(c.Value)
}

// RequireAuth wraps a handler so only authenticated requests reach it.
func (s *Service) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentSession(r); !ok {
			respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// RequireAdmin wraps a handler so only authenticated admins reach it. A valid
// non-admin session gets 403; no session gets 401.
func (s *Service) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.currentSession(r)
		if !ok {
			respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if sess.role != "admin" {
			respond.JSON(w, http.StatusForbidden, map[string]string{"error": "yalnız admin"})
			return
		}
		next(w, r)
	}
}
