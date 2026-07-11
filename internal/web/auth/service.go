package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/murad/unifi-collector/internal/web/respond"
)

// Service wires the user store to the session signer and exposes the auth
// HTTP handlers plus the route-guarding middleware.
type Service struct {
	store  *Store
	signer *signer
}

// NewService builds the auth service on an open user store.
func NewService(store *Store) *Service {
	return &Service{store: store, signer: newSigner(store.secret)}
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
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	role, ok := s.store.verify(req.Username, req.Password)
	if !ok {
		respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "İstifadəçi adı və ya parol yanlışdır"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signer.sign(req.Username, role),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	respond.JSON(w, http.StatusOK, userResp{Username: req.Username, Role: role})
}

// Logout clears the session cookie.
func (s *Service) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
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
