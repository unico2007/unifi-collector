package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const (
	sessionCookie = "unico_session"
	sessionTTL    = 12 * time.Hour
)

// session is the authenticated identity carried by a signed cookie. version is
// the user's token_version at issue time; it is re-checked against the DB on
// every request so logout / role / password changes revoke old tokens at once.
type session struct {
	username string
	role     string
	version  int
}

// signer issues and verifies signed session cookies. The token is
// base64(payload).base64(hmac-sha256(payload)); payload is a JSON blob with the
// username, role, token version and expiry. The HMAC key is stateless (survives
// restarts); revocation is layered on top via the DB token_version check.
type signer struct {
	key []byte
}

type sessionPayload struct {
	U   string `json:"u"`
	R   string `json:"r"`
	V   int    `json:"v"`
	Exp int64  `json:"exp"`
}

func newSigner(key []byte) *signer { return &signer{key: key} }

func (s *signer) sign(username, role string, version int) string {
	payload, _ := json.Marshal(sessionPayload{U: username, R: role, V: version, Exp: time.Now().Add(sessionTTL).Unix()})
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(mac.Sum(nil))
}

// parse validates the token's base64 encoding, HMAC signature and JSON shape
// but does NOT check expiry. It is the shared core of verify/verifyIgnoringExpiry.
func (s *signer) parse(tok string) (sessionPayload, bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return sessionPayload{}, false
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(parts[0])
	if err != nil {
		return sessionPayload{}, false
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil {
		return sessionPayload{}, false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return sessionPayload{}, false
	}
	var p sessionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return sessionPayload{}, false
	}
	return p, true
}

// verify authenticates a token: valid signature AND not expired.
func (s *signer) verify(tok string) (session, bool) {
	p, ok := s.parse(tok)
	if !ok || time.Now().Unix() > p.Exp {
		return session{}, false
	}
	return session{username: p.U, role: p.R, version: p.V}, true
}

// verifyIgnoringExpiry authenticates the token's identity (signature) but skips
// the expiry check. Used only by Logout, so an expired-but-authentic cookie can
// still trigger server-side revocation (bump token_version). It must NEVER be
// used to grant access — that path stays on verify().
func (s *signer) verifyIgnoringExpiry(tok string) (session, bool) {
	p, ok := s.parse(tok)
	if !ok {
		return session{}, false
	}
	return session{username: p.U, role: p.R, version: p.V}, true
}
