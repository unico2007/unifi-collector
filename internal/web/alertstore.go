package web

import (
	"database/sql"
	"time"
)

// alertStore persists user-configurable alert thresholds and an alert-history
// timeline. It reuses the same SQLite database as the user store (on the
// web-data volume), so settings + history survive restarts/redeploys.
type alertStore struct {
	db *sql.DB
}

// thresholds are the two numeric alert limits a user can tune. The offline and
// subsystem-health rules are boolean and stay fixed.
type thresholds struct {
	CPU    float64 `json:"cpuPercent"`
	Memory float64 `json:"memoryPercent"`
}

var defaultThresholds = thresholds{CPU: 85, Memory: 90}

// historyRow is one fire→resolve span of an alert. ResolvedAt == 0 means the
// alert is still active.
type historyRow struct {
	Level      string `json:"level"`
	Rule       string `json:"rule"`
	Target     string `json:"target"`
	Message    string `json:"message"`
	FiredAt    int64  `json:"firedAt"`
	ResolvedAt int64  `json:"resolvedAt"`
}

func newAlertStore(db *sql.DB) (*alertStore, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS alert_settings (k TEXT PRIMARY KEY, v REAL NOT NULL)`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS alert_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fingerprint TEXT NOT NULL,
		level TEXT NOT NULL,
		rule TEXT NOT NULL,
		target TEXT NOT NULL,
		message TEXT NOT NULL,
		fired_at INTEGER NOT NULL,
		resolved_at INTEGER
	)`); err != nil {
		return nil, err
	}
	// Index used by the open-rows lookup on every evaluation tick.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_alert_history_open ON alert_history (resolved_at)`); err != nil {
		return nil, err
	}
	s := &alertStore{db: db}
	// Seed defaults once (INSERT OR IGNORE keeps user edits).
	for k, v := range map[string]float64{"cpu_percent": defaultThresholds.CPU, "memory_percent": defaultThresholds.Memory} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO alert_settings (k, v) VALUES (?, ?)`, k, v); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// thresholds returns the current limits, falling back to defaults on any error.
func (s *alertStore) thresholds() thresholds {
	th := defaultThresholds
	rows, err := s.db.Query(`SELECT k, v FROM alert_settings`)
	if err != nil {
		return th
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var k string
		var v float64
		if rows.Scan(&k, &v) != nil {
			continue
		}
		switch k {
		case "cpu_percent":
			th.CPU = v
		case "memory_percent":
			th.Memory = v
		}
	}
	return th
}

// setThresholds persists new limits (clamped to a sane 1..100 range).
func (s *alertStore) setThresholds(th thresholds) error {
	clamp := func(v, def float64) float64 {
		if v < 1 || v > 100 {
			return def
		}
		return v
	}
	th.CPU = clamp(th.CPU, defaultThresholds.CPU)
	th.Memory = clamp(th.Memory, defaultThresholds.Memory)
	_, err := s.db.Exec(`INSERT INTO alert_settings (k, v) VALUES ('cpu_percent', ?), ('memory_percent', ?)
		ON CONFLICT(k) DO UPDATE SET v = excluded.v`, th.CPU, th.Memory)
	return err
}

// recordTransitions diffs the current active alerts against the open (unresolved)
// history rows: newly-seen fingerprints are inserted as fired, and open rows no
// longer active are marked resolved. Called on a timer so the timeline reflects
// real fire/resolve events even when nobody is viewing the page. It returns the
// alerts that fired and resolved on this tick so the caller can notify.
func (s *alertStore) recordTransitions(active []alertDTO) (fired, resolved []alertDTO, err error) {
	now := time.Now().Unix()
	current := make(map[string]alertDTO, len(active))
	for _, a := range active {
		current[a.Rule+"|"+a.Target] = a
	}

	type openRow struct {
		id     int64
		detail alertDTO
	}
	open := map[string]openRow{} // fingerprint -> row
	rows, err := s.db.Query(`SELECT id, fingerprint, level, rule, target, message FROM alert_history WHERE resolved_at IS NULL`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int64
		var fp string
		var a alertDTO
		if rows.Scan(&id, &fp, &a.Level, &a.Rule, &a.Target, &a.Message) == nil {
			open[fp] = openRow{id: id, detail: a}
		}
	}
	_ = rows.Close()

	// Fire: active now but not already open.
	for fp, a := range current {
		if _, ok := open[fp]; ok {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT INTO alert_history (fingerprint, level, rule, target, message, fired_at) VALUES (?, ?, ?, ?, ?, ?)`,
			fp, a.Level, a.Rule, a.Target, a.Message, now); err != nil {
			return nil, nil, err
		}
		fired = append(fired, a)
	}
	// Resolve: open but no longer active.
	for fp, row := range open {
		if _, ok := current[fp]; ok {
			continue
		}
		if _, err := s.db.Exec(`UPDATE alert_history SET resolved_at = ? WHERE id = ?`, now, row.id); err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, row.detail)
	}
	return fired, resolved, nil
}

// history returns the most recent alert spans (active first via NULL resolved_at
// sorting high, then newest fired first).
func (s *alertStore) history(limit int) []historyRow {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT level, rule, target, message, fired_at, resolved_at
		FROM alert_history ORDER BY (resolved_at IS NOT NULL), fired_at DESC LIMIT ?`, limit)
	if err != nil {
		return []historyRow{}
	}
	defer func() { _ = rows.Close() }()
	out := []historyRow{}
	for rows.Next() {
		var h historyRow
		var resolved sql.NullInt64
		if rows.Scan(&h.Level, &h.Rule, &h.Target, &h.Message, &h.FiredAt, &resolved) != nil {
			continue
		}
		if resolved.Valid {
			h.ResolvedAt = resolved.Int64
		}
		out = append(out, h)
	}
	return out
}
