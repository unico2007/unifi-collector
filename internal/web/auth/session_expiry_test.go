package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// signWithExp forges a token with an arbitrary expiry, using the signer's real
// key + algorithm, so we can build an authentic-but-expired cookie.
func signWithExp(s *signer, u, r string, v int, exp int64) string {
	payload, _ := json.Marshal(sessionPayload{U: u, R: r, V: v, Exp: exp})
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(mac.Sum(nil))
}

func TestVerifyIgnoringExpiry(t *testing.T) {
	s := newSigner([]byte("test-key-0123456789"))
	expired := signWithExp(s, "dave", "guest", 1, time.Now().Add(-time.Hour).Unix())

	if _, ok := s.verify(expired); ok {
		t.Fatal("verify() must reject an expired token (no access)")
	}
	sess, ok := s.verifyIgnoringExpiry(expired)
	if !ok || sess.username != "dave" {
		t.Fatalf("verifyIgnoringExpiry() must accept an authentic expired token; got %+v ok=%v", sess, ok)
	}
	// A tampered token is rejected even ignoring expiry.
	if _, ok := s.verifyIgnoringExpiry(expired + "x"); ok {
		t.Fatal("verifyIgnoringExpiry() must reject a tampered token")
	}
}

// Logout with an expired-but-authentic cookie must still revoke the user's
// other (unexpired) sessions.
func TestLogoutRevokesWithExpiredCookie(t *testing.T) {
	svc := newTestService(t)
	if err := svc.store.UpsertUser("dave", "pw", "guest"); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	_, ver, _ := svc.store.verify("dave", "pw")

	// A live sibling session (unexpired) + the logging-out device's expired cookie.
	liveToken := svc.signer.sign("dave", "guest", ver)
	expiredCookie := signWithExp(svc.signer, "dave", "guest", ver, time.Now().Add(-time.Minute).Unix())

	if _, ok := svc.currentSession(reqWithToken(liveToken)); !ok {
		t.Fatal("sibling session should be valid before logout")
	}
	svc.Logout(httptest.NewRecorder(), reqWithToken(expiredCookie))
	if _, ok := svc.currentSession(reqWithToken(liveToken)); ok {
		t.Fatal("sibling session must be revoked after logout with expired cookie")
	}
}
