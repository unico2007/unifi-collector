package alert

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return &Service{store: store}
}

func TestSettingsUpdate(t *testing.T) {
	put := func(s *Service, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PUT", "/api/alerts/settings", strings.NewReader(body))
		s.SettingsUpdate(w, r)
		return w
	}

	t.Run("partial update keeps the omitted threshold", func(t *testing.T) {
		s := testService(t)
		if err := s.store.setThresholds(thresholds{CPU: 60, Memory: 75}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if w := put(s, `{"cpuPercent": 50}`); w.Code != 200 {
			t.Fatalf("partial update: status = %d, want 200", w.Code)
		}
		th := s.store.thresholds()
		if th.CPU != 50 {
			t.Errorf("cpu = %v, want 50", th.CPU)
		}
		if th.Memory != 75 {
			t.Errorf("memory = %v, want 75 (must NOT revert to default %v)", th.Memory, defaultThresholds.Memory)
		}
	})

	t.Run("out-of-range value is rejected with 400", func(t *testing.T) {
		s := testService(t)
		for _, body := range []string{
			`{"cpuPercent": 150}`,
			`{"cpuPercent": 0}`,
			`{"memoryPercent": -5}`,
			`{"cpuPercent": 50, "memoryPercent": 101}`,
		} {
			if w := put(s, body); w.Code != 400 {
				t.Errorf("body %s: status = %d, want 400", body, w.Code)
			}
		}
		// Nothing must have been persisted by the rejected requests.
		if th := s.store.thresholds(); th != defaultThresholds {
			t.Errorf("thresholds changed by rejected request: %+v", th)
		}
	})

	t.Run("full update still works", func(t *testing.T) {
		s := testService(t)
		if w := put(s, `{"cpuPercent": 42, "memoryPercent": 43}`); w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if th := s.store.thresholds(); th.CPU != 42 || th.Memory != 43 {
			t.Errorf("thresholds = %+v, want {42 43}", th)
		}
	})

	t.Run("malformed body is a 400", func(t *testing.T) {
		s := testService(t)
		if w := put(s, `{"cpuPercent": "high"}`); w.Code != 400 {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}
