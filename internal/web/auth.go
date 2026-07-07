package web

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // pure-Go driver: works with CGO_ENABLED=0
)

const (
	sessionCookie = "unico_session"
	sessionTTL    = 12 * time.Hour
)

// userStore persists users in SQLite. There is intentionally NO register
// endpoint — accounts are created out-of-band via the create-user CLI.
type userStore struct {
	db *sql.DB
}

// OpenUserStore opens (creating if needed) the SQLite user database.
func OpenUserStore(path string) (*userStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'guest'
	)`); err != nil {
		return nil, err
	}
	return &userStore{db: db}, nil
}

// Close closes the underlying database.
func (u *userStore) Close() error { return u.db.Close() }

// UpsertUser creates or updates a user (used by the CLI). role is "admin" or
// "guest".
func (u *userStore) UpsertUser(username, password, role string) error {
	if username == "" || password == "" {
		return errors.New("username and password required")
	}
	if role != "admin" && role != "guest" {
		return errors.New(`role must be "admin" or "guest"`)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = u.db.Exec(`INSERT INTO users (username, password_hash, role)
		VALUES (?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash, role=excluded.role`,
		username, string(hash), role)
	return err
}

// verify checks credentials and returns the stored role on success.
func (u *userStore) verify(username, password string) (role string, ok bool) {
	var hash string
	err := u.db.QueryRow(`SELECT password_hash, role FROM users WHERE username = ?`, username).
		Scan(&hash, &role)
	if err != nil {
		return "", false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", false
	}
	return role, true
}

// --- sessions (in-memory; a restart simply forces re-login) ----------------

type session struct {
	username string
	role     string
	expires  time.Time
}

type sessionStore struct {
	mu sync.Mutex
	m  map[string]session
}

func newSessionStore() *sessionStore { return &sessionStore{m: map[string]session{}} }

func (s *sessionStore) create(username, role string) string {
	tok := randomToken()
	s.mu.Lock()
	s.m[tok] = session{username: username, role: role, expires: time.Now().Add(sessionTTL)}
	s.mu.Unlock()
	return tok
}

func (s *sessionStore) get(tok string) (session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[tok]
	if !ok || time.Now().After(sess.expires) {
		delete(s.m, tok)
		return session{}, false
	}
	return sess, true
}

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	delete(s.m, tok)
	s.mu.Unlock()
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- handlers --------------------------------------------------------------

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResp struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	role, ok := s.users.verify(req.Username, req.Password)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "İstifadəçi adı və ya parol yanlışdır"})
		return
	}
	tok := s.sessions.create(req.Username, role)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	writeJSON(w, http.StatusOK, userResp{Username: req.Username, Role: role})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, userResp{Username: sess.username, Role: sess.role})
}

func (s *Server) currentSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	return s.sessions.get(c.Value)
}

// requireAuth wraps a handler so only authenticated requests reach it.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentSession(r); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}
