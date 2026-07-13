package auth

import (
	"testing"
	"time"
)

func TestLoginThrottle(t *testing.T) {
	tr := newLoginThrottle()
	now := time.Now()
	const ip = "10.0.0.5"

	// Below the threshold: never blocked.
	for i := 0; i < tr.maxFails-1; i++ {
		if d := tr.blockedFor(ip, now); d != 0 {
			t.Fatalf("blocked after %d failures, want allowed", i)
		}
		tr.recordFailure(ip, now)
	}
	// The threshold-crossing failure arms the lockout.
	tr.recordFailure(ip, now)
	if d := tr.blockedFor(ip, now); d <= 0 {
		t.Fatal("expected lockout after reaching maxFails")
	}

	// Still locked mid-window, cleared after the lockout expires.
	if d := tr.blockedFor(ip, now.Add(tr.lockout-time.Second)); d <= 0 {
		t.Fatal("expected still locked before lockout expiry")
	}
	if d := tr.blockedFor(ip, now.Add(tr.lockout+time.Second)); d != 0 {
		t.Fatal("expected unlocked after lockout expiry")
	}

	// A successful login resets the counter immediately.
	tr.recordFailure(ip, now)
	tr.reset(ip)
	if d := tr.blockedFor(ip, now); d != 0 {
		t.Fatal("expected allowed after reset")
	}

	// A different IP is tracked independently.
	for i := 0; i < tr.maxFails; i++ {
		tr.recordFailure("10.0.0.6", now)
	}
	if d := tr.blockedFor(ip, now); d != 0 {
		t.Fatal("one IP's lockout must not affect another IP")
	}
}
