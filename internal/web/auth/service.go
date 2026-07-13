package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

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
	log      *zap.Logger
}

// NewService builds the auth service on an open user store. secureCookie should
// be true only when the dashboard is served over HTTPS (otherwise the browser
// won't send the cookie over plain-HTTP LAN and login breaks). log may be nil.
func NewService(store *Store, secureCookie bool, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{
		store:    store,
		signer:   newSigner(store.secret),
		throttle: newLoginThrottle(),
		secure:   secureCookie,
		log:      log,
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
	role, version, ok := s.store.verify(req.Username, req.Password)
	if !ok {
		s.throttle.recordFailure(ip, now)
		respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "İstifadəçi adı və ya parol yanlışdır"})
		return
	}
	s.throttle.reset(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signer.sign(req.Username, role, version),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	respond.JSON(w, http.StatusOK, userResp{Username: req.Username, Role: role})
}

// Logout revokes the user's sessions (bumps their token_version so every
// outstanding cookie is rejected) and clears the cookie. Flags mirror the ones
// set at login so the browser reliably matches and removes the cookie.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	// Use verifyIgnoringExpiry so even a just-expired (but authentic) cookie still
	// revokes the user's other sessions. Log a bump failure instead of dropping
	// it silently — on failure the other sessions live until their 12h expiry.
	if sess, ok := s.signer.verifyIgnoringExpiry(cookieValue(r)); ok {
		if err := s.store.bumpTokenVersion(sess.username); err != nil {
			s.log.Warn("logout: could not revoke sessions (token_version bump failed)",
				zap.String("user", sess.username), zap.Error(err))
		}
	}
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

// cookieValue returns the raw session cookie value, or "" if absent.
func cookieValue(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

// currentSession validates the cookie's signature AND confirms the token's
// version still matches the user's current token_version in the DB. The second
// check is what makes logout / role / password changes revoke a token
// immediately, instead of it staying valid until the 12h expiry.
func (s *Service) currentSession(r *http.Request) (session, bool) {
	sess, ok := s.signer.verify(cookieValue(r))
	if !ok {
		return session{}, false
	}
	current, exists, err := s.store.tokenVersion(sess.username)
	if err != nil {
		// Transient DB error (lock/IO). Don't deauthenticate a validly-signed
		// session over a blip — fail open and log. Real revocations are a
		// SUCCESSFUL query with a mismatched version, so they still take effect.
		s.log.Warn("session check: token_version read failed; accepting valid signature",
			zap.String("user", sess.username), zap.Error(err))
		return sess, true
	}
	if !exists || current != sess.version {
		return session{}, false
	}
	return sess, true
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
