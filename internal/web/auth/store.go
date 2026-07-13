// Package auth is the BFF's authentication feature: the SQLite-backed user
// store, HMAC-signed session cookies, the login/logout/me handlers, and the
// RequireAuth/RequireAdmin middleware. It owns the shared *sql.DB (the alert
// feature builds its tables on the same handle via Store.DB).
package auth

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite" // pure-Go driver: works with CGO_ENABLED=0

	"crypto/rand"
)

// Store persists users in SQLite. There is intentionally NO register endpoint —
// accounts are created out-of-band via the create-user CLI. It also holds a
// persistent session-signing secret so signed cookies survive restarts.
type Store struct {
	db     *sql.DB
	secret []byte
	// dummyHash is a valid bcrypt hash compared against when the requested user
	// does not exist, so a failed login takes the same time whether the username
	// is real or not — this closes the timing side-channel that would otherwise
	// let an attacker enumerate valid usernames.
	dummyHash []byte
}

// OpenStore opens (creating if needed) the SQLite user database.
func OpenStore(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	// WAL + a busy timeout so a read never fails outright while another process
	// (the create-user CLI opens the same file) holds a write lock — that would
	// otherwise surface as a spurious SQLITE_BUSY on the per-request token_version
	// check and bounce a valid session to login. WAL lets readers run during a
	// write; busy_timeout waits out any remaining lock instead of erroring.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
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
		role TEXT NOT NULL DEFAULT 'guest',
		token_version INTEGER NOT NULL DEFAULT 1
	)`); err != nil {
		return nil, err
	}
	// Migration for databases created before token_version existed. ALTER ADD
	// COLUMN with a constant default is safe in SQLite; ignore the error when the
	// column is already present (fresh DBs created by the statement above).
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		return nil, err
	}
	secret, err := loadOrCreateSecret(db)
	if err != nil {
		return nil, err
	}
	// A throwaway hash of a random password, used to equalize verify() timing for
	// unknown usernames. Generated once at cost=DefaultCost so it matches real
	// hashes' comparison cost.
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	dummyHash, err := bcrypt.GenerateFromPassword(rnd, bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, secret: secret, dummyHash: dummyHash}, nil
}

// DB returns the underlying handle so sibling features (alerting) can create
// their own tables on the same single-connection SQLite database.
func (u *Store) DB() *sql.DB { return u.db }

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
func (u *Store) Close() error { return u.db.Close() }

// UpsertUser creates or updates a user (used by the CLI). role is "admin" or
// "guest".
func (u *Store) UpsertUser(username, password, role string) error {
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
	// On update (password or role change) bump token_version so any outstanding
	// sessions for this user are revoked and they must re-authenticate — that is
	// how a role change or password reset takes effect immediately.
	_, err = u.db.Exec(`INSERT INTO users (username, password_hash, role, token_version)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(username) DO UPDATE SET
			password_hash=excluded.password_hash,
			role=excluded.role,
			token_version=users.token_version + 1`,
		username, string(hash), role)
	return err
}

// verify checks credentials and returns the stored role and current token
// version on success.
func (u *Store) verify(username, password string) (role string, version int, ok bool) {
	var hash string
	err := u.db.QueryRow(`SELECT password_hash, role, token_version FROM users WHERE username = ?`, username).
		Scan(&hash, &role, &version)
	if err != nil {
		// Unknown user: still run one bcrypt comparison against the dummy hash so
		// the response time doesn't reveal whether the username exists.
		_ = bcrypt.CompareHashAndPassword(u.dummyHash, []byte(password))
		return "", 0, false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", 0, false
	}
	return role, version, true
}

// tokenVersion returns the user's current token version. Three outcomes are
// kept distinct so the caller doesn't confuse a real revocation with a DB blip:
//   - (v, true, nil)    user exists, v is current
//   - (0, false, nil)   user gone (deleted) => genuine revocation
//   - (0, false, err)   transient DB error (e.g. lock/IO) => NOT a revocation
func (u *Store) tokenVersion(username string) (version int, ok bool, err error) {
	err = u.db.QueryRow(`SELECT token_version FROM users WHERE username = ?`, username).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return version, true, nil
}

// bumpTokenVersion invalidates all of a user's outstanding sessions (used by
// logout). A no-op if the user doesn't exist.
func (u *Store) bumpTokenVersion(username string) error {
	_, err := u.db.Exec(`UPDATE users SET token_version = token_version + 1 WHERE username = ?`, username)
	return err
}
