package alert

import (
	"database/sql"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return st
}

func sub(level, msg string) alertDTO {
	return alertDTO{Level: level, Rule: "Subsystem problemi", Target: "wlan", Message: msg, Value: msg}
}

func rec(t *testing.T, s *Store, active ...alertDTO) (fired, escalated, resolved []alertDTO) {
	t.Helper()
	f, e, r, err := s.recordTransitions(active)
	if err != nil {
		t.Fatalf("recordTransitions: %v", err)
	}
	return f, e, r
}

func TestRecordTransitions_Escalation(t *testing.T) {
	s := testStore(t)

	// 1. First seen as a warning: one fired warning, one open row.
	fired, escalated, resolved := rec(t, s, sub("warning", "xəbərdarlıq"))
	if len(fired) != 1 || fired[0].Level != "warning" {
		t.Fatalf("first tick: fired = %+v, want one warning", fired)
	}
	if len(escalated) != 0 {
		t.Fatalf("first tick: escalated = %+v, want none", escalated)
	}
	if len(resolved) != 0 {
		t.Fatalf("first tick: resolved = %+v, want none", resolved)
	}

	// 2. Same rule+target worsens to critical: must re-notify as an escalation
	//    (not a fresh fire), must NOT open a second row, must NOT resolve anything.
	fired, escalated, resolved = rec(t, s, sub("critical", "xəta"))
	if len(fired) != 0 {
		t.Fatalf("escalation: fired = %+v, want none (should be an escalation)", fired)
	}
	if len(escalated) != 1 || escalated[0].Level != "critical" {
		t.Fatalf("escalation: escalated = %+v, want one critical", escalated)
	}
	if len(resolved) != 0 {
		t.Fatalf("escalation: resolved = %+v, want none", resolved)
	}
	h := s.history(0)
	if len(h) != 1 {
		t.Fatalf("escalation must reuse the open row, history = %d rows, want 1", len(h))
	}
	if h[0].Level != "critical" || h[0].ResolvedAt != 0 {
		t.Fatalf("escalated row = %+v, want level critical and still active", h[0])
	}
}

func TestRecordTransitions_DeEscalationIsSilent(t *testing.T) {
	s := testStore(t)
	rec(t, s, sub("critical", "xəta"))

	// Critical eases back to warning: the row must reflect warning, but we must
	// not page anyone for a downgrade.
	fired, escalated, resolved := rec(t, s, sub("warning", "xəbərdarlıq"))
	if len(fired) != 0 {
		t.Fatalf("de-escalation must not notify, fired = %+v", fired)
	}
	if len(escalated) != 0 {
		t.Fatalf("de-escalation must not escalate, escalated = %+v", escalated)
	}
	if len(resolved) != 0 {
		t.Fatalf("de-escalation must not resolve, resolved = %+v", resolved)
	}
	if h := s.history(0); len(h) != 1 || h[0].Level != "warning" {
		t.Fatalf("de-escalated row = %+v, want single warning row", h)
	}
}

func TestRecordTransitions_UnchangedAndResolve(t *testing.T) {
	s := testStore(t)
	rec(t, s, sub("warning", "xəbərdarlıq"))

	// Same level again: no new fire.
	if fired, _, _ := rec(t, s, sub("warning", "xəbərdarlıq")); len(fired) != 0 {
		t.Fatalf("unchanged tick must not re-fire, fired = %+v", fired)
	}

	// Now gone: resolve fires once.
	fired, _, resolved := rec(t, s)
	if len(fired) != 0 {
		t.Fatalf("resolve tick: fired = %+v, want none", fired)
	}
	if len(resolved) != 1 || resolved[0].Rule != "Subsystem problemi" {
		t.Fatalf("resolve tick: resolved = %+v, want the subsystem alert", resolved)
	}
}
