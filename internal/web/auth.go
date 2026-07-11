package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // pure-Go driver: works with CGO_ENABLED=0

	"github.com/murad/unifi-collector/internal/web/respond"
)

const (
	sessionCookie = "unico_session"
	sessionTTL    = 12 * time.Hour
)

// userStore persists users in SQLite. There is intentionally NO register
// endpoint — accounts are created out-of-band via the create-user CLI. It also
// holds a persistent session-signing secret so signed cookies survive restarts.
type userStore struct {
	db     *sql.DB
	secret []byte
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
	// Serialize DB access to one connection: the background alert evaluator and
	// the HTTP handlers share this SQLite file, and a single connection avoids
	// "database is locked" errors under concurrent writes (traffic is low).
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'guest'
	)`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		return nil, err
	}
	secret, err := loadOrCreateSecret(db)
	if err != nil {
		return nil, err
	}
	return &userStore{db: db, secret: secret}, nil
}

// loadOrCreateSecret returns a stable random secret persisted in the DB, so
// session cookies stay valid across restarts and redeploys.
func loadOrCreateSecret(db *sql.DB) ([]byte, error) {
	var hexSecret string
	err := db.QueryRow(`SELECT v FROM meta WHERE k = 'session_secret'`).Scan(&hexSecret)
	if err == nil {
		return hex.DecodeString(hexSecret)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	hexSecret = hex.EncodeToString(buf)
	if _, err := db.Exec(`INSERT INTO meta (k, v) VALUES ('session_secret', ?)`, hexSecret); err != nil {
		return nil, err
	}
	return buf, nil
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

// --- sessions (stateless, HMAC-signed cookie; survives restarts) -----------

type session struct {
	username string
	role     string
}

// signer issues and verifies signed session cookies. The token is
// base64(payload).base64(hmac-sha256(payload)); payload is a JSON blob with the
// username, role and expiry. No server-side state, so a restart keeps sessions.
type signer struct {
	key []byte
}

type sessionPayload struct {
	U   string `json:"u"`
	R   string `json:"r"`
	Exp int64  `json:"exp"`
}

func newSigner(key []byte) *signer { return &signer{key: key} }

func (s *signer) sign(username, role string) string {
	payload, _ := json.Marshal(sessionPayload{U: username, R: role, Exp: time.Now().Add(sessionTTL).Unix()})
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(mac.Sum(nil))
}

func (s *signer) verify(tok string) (session, bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return session{}, false
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(parts[0])
	if err != nil {
		return session{}, false
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil {
		return session{}, false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return session{}, false
	}
	var p sessionPayload
	if err := json.Unmarshal(payload, &p); err != nil || time.Now().Unix() > p.Exp {
		return session{}, false
	}
	return session{username: p.U, role: p.R}, true
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
		respond.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	role, ok := s.users.verify(req.Username, req.Password)
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

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	respond.JSON(w, http.StatusOK, userResp{Username: sess.username, Role: sess.role})
}

func (s *Server) currentSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	return s.signer.verify(c.Value)
}

// requireAuth wraps a handler so only authenticated requests reach it.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentSession(r); !ok {
			respond.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// requireAdmin wraps a handler so only authenticated admins reach it. A valid
// non-admin session gets 403; no session gets 401.
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
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
