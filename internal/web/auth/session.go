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

// session is the authenticated identity carried by a signed cookie.
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
