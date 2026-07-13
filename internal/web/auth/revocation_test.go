package auth

import (
	"net/http"
	"path/filepath"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "u.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewService(store, false, nil)
}

// reqWithToken builds a request carrying the given session cookie value.
func reqWithToken(token string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	return r
}

func TestSessionRevocation(t *testing.T) {
	svc := newTestService(t)
	if err := svc.store.UpsertUser("bob", "pw", "guest"); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	// A freshly issued token authenticates.
	role, ver, ok := svc.store.verify("bob", "pw")
	if !ok || role != "guest" {
		t.Fatalf("verify = %q,%v; want guest,true", role, ok)
	}
	token := svc.signer.sign("bob", role, ver)
	if _, ok := svc.currentSession(reqWithToken(token)); !ok {
		t.Fatal("fresh token should authenticate")
	}

	// Logout (bump) revokes that exact token immediately.
	if err := svc.store.bumpTokenVersion("bob"); err != nil {
		t.Fatalf("bump: %v", err)
	}
	if _, ok := svc.currentSession(reqWithToken(token)); ok {
		t.Fatal("token must be rejected after logout/bump")
	}
}

func TestRoleChangeRevokesOldToken(t *testing.T) {
	svc := newTestService(t)
	if err := svc.store.UpsertUser("carol", "pw", "guest"); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	_, ver, _ := svc.store.verify("carol", "pw")
	oldToken := svc.signer.sign("carol", "guest", ver)
	if _, ok := svc.currentSession(reqWithToken(oldToken)); !ok {
		t.Fatal("guest token should work initially")
	}

	// Admin promotes carol via the same store the CLI uses.
	if err := svc.store.UpsertUser("carol", "pw", "admin"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	// The old (guest) token is now invalid — carol must re-login.
	if _, ok := svc.currentSession(reqWithToken(oldToken)); ok {
		t.Fatal("old token must be revoked after role change")
	}
	// Re-login yields a new token with the new role and version.
	role, newVer, ok := svc.store.verify("carol", "pw")
	if !ok || role != "admin" || newVer == ver {
		t.Fatalf("re-verify = %q,ver=%d,ok=%v; want admin, bumped version", role, newVer, ok)
	}
	newToken := svc.signer.sign("carol", role, newVer)
	sess, ok := svc.currentSession(reqWithToken(newToken))
	if !ok || sess.role != "admin" {
		t.Fatalf("new token session = %+v,ok=%v; want admin", sess, ok)
	}
}

func TestDeletedOrUnknownUserRejected(t *testing.T) {
	svc := newTestService(t)
	// A validly signed token for a user that doesn't exist must be rejected
	// (tokenVersion lookup fails).
	token := svc.signer.sign("ghost", "admin", 1)
	if _, ok := svc.currentSession(reqWithToken(token)); ok {
		t.Fatal("token for nonexistent user must be rejected")
	}
}
