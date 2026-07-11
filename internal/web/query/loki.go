package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// Loki is a minimal Loki query client used for the "recent logs" panels.
type Loki struct {
	base string
	http *http.Client
}

// NewLoki builds a Loki client pointed at base (e.g. http://loki:3100).
func NewLoki(base string) *Loki {
	return &Loki{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

// LogLine is a single decoded Loki log line ready for a panel.
type LogLine struct {
	Time   string            `json:"time"`
	Level  string            `json:"level"`
	Msg    string            `json:"msg"`
	Labels map[string]string `json:"-"`
}

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"` // [ [ns, line], ... ]
		} `json:"result"`
	} `json:"data"`
}

// Recent runs a LogQL query over the last dur and returns up to limit lines,
// newest first. Best-effort: returns nil on any error so callers can fall back.
func (c *Loki) Recent(ctx context.Context, logql string, dur time.Duration, limit int) []LogLine {
	end := time.Now()
	start := end.Add(-dur)
	u := fmt.Sprintf("%s/loki/api/v1/query_range?query=%s&start=%d&end=%d&limit=%d&direction=backward",
		c.base, url.QueryEscape(logql), start.UnixNano(), end.UnixNano(), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	var lr lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil || lr.Status != "success" {
		return nil
	}

	var lines []LogLine
	for _, st := range lr.Data.Result {
		level := st.Stream["level"]
		for _, v := range st.Values {
			ns, _ := strconv.ParseInt(v[0], 10, 64)
			msg, lvl := DecodeLogLine(v[1], level)
			lines = append(lines, LogLine{
				Time:   time.Unix(0, ns).Format("15:04:05"),
				Level:  NormalizeLevel(lvl),
				Msg:    msg,
				Labels: st.Stream,
			})
		}
	}
	// Newest first, then cap.
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time > lines[j].Time })
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines
}

// DecodeLogLine unwraps a Loki log line. Collector pushes each line as a JSON
// object {"event","level","msg","vendor","site"} where "msg" carries the real
// payload (a UniFi CEF string). Returning the inner msg keeps the JSON tail
// (`","vendor":..}`) out of downstream CEF parsing and the Overview panel. Falls
// back to the raw line and stream level for anything that isn't our JSON shape.
func DecodeLogLine(raw, streamLevel string) (msg, level string) {
	var w struct {
		Msg   string `json:"msg"`
		Level string `json:"level"`
	}
	if err := json.Unmarshal([]byte(raw), &w); err == nil && w.Msg != "" {
		lvl := w.Level
		if lvl == "" {
			lvl = streamLevel
		}
		return w.Msg, lvl
	}
	return raw, streamLevel
}

// NormalizeLevel folds the various level spellings into info/warn/error.
func NormalizeLevel(l string) string {
	switch l {
	case "info", "warn", "error":
		return l
	case "warning":
		return "warn"
	case "":
		return "info"
	default:
		return l
	}
}
