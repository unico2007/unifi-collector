package auth

import (
	"sync"
	"time"
)

// loginThrottle is a small in-memory brute-force guard for the login endpoint.
// After maxFails failed attempts for a key (client IP) within the sliding
// window, further attempts are refused for lockout, regardless of correctness,
// so an attacker on the LAN can't spray passwords quickly. A successful login
// clears the key immediately. State is per-process (the BFF is a single
// instance) and self-expiring, so it needs no cleanup goroutine.
type loginThrottle struct {
	mu       sync.Mutex
	entries  map[string]*throttleEntry
	maxFails int
	window   time.Duration
	lockout  time.Duration
}

type throttleEntry struct {
	count      int
	windowFrom time.Time
	lockedTill time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{
		entries:  make(map[string]*throttleEntry),
		maxFails: 8,
		window:   5 * time.Minute,
		lockout:  10 * time.Minute,
	}
}

// blockedFor reports how long the key must wait before another attempt is
// allowed (0 = allowed now).
func (t *loginThrottle) blockedFor(key string, now time.Time) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.entries[key]
	if e == nil {
		return 0
	}
	if now.Before(e.lockedTill) {
		return e.lockedTill.Sub(now)
	}
	return 0
}

// recordFailure counts a failed attempt and arms the lockout once the threshold
// is crossed within the window.
func (t *loginThrottle) recordFailure(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.entries[key]
	if e == nil || now.Sub(e.windowFrom) > t.window {
		e = &throttleEntry{windowFrom: now}
		t.entries[key] = e
	}
	e.count++
	if e.count >= t.maxFails {
		e.lockedTill = now.Add(t.lockout)
		e.count = 0
		e.windowFrom = now
	}
}

// reset clears the key after a successful login.
func (t *loginThrottle) reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}
