package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type pill struct {
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type logCategoryDTO struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Vendor  string   `json:"vendor"`
	Count   int      `json:"count"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// handleLogsCategories reads recent UniFi logs from Loki, parses their CEF
// payload, and groups them by CEF event name into categories the Logs page can
// render. Kerio categories are absent because Kerio does not ship logs to Loki
// yet (see the gap report).
func (s *Server) handleLogsCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lines := s.loki.recent(ctx, `{vendor="unifi"}`, 24*time.Hour, 800)

	cols := []string{"Vaxt", "Səviyyə", "Cihaz", "Detal"}
	order := []string{}
	cats := map[string]*logCategoryDTO{}

	get := func(label string) *logCategoryDTO {
		key := slug(label)
		c, ok := cats[key]
		if !ok {
			c = &logCategoryDTO{Key: key, Label: label, Vendor: "unifi", Columns: cols, Rows: [][]any{}}
			cats[key] = c
			order = append(order, key)
		}
		return c
	}

	for _, ln := range lines {
		name, device, msg, level, ok := parseCEF(ln.Msg)
		label := name
		detail := msg
		if !ok {
			label = "Digər"
			detail = ln.Msg
			level = "info"
		}
		if detail == "" {
			detail = name
		}
		c := get(label)
		c.Count++
		if len(c.Rows) < 120 { // cap payload per category
			c.Rows = append(c.Rows, []any{ln.Time, pill{Text: level, Kind: levelKind(level)}, device, detail})
		}
	}

	out := make([]logCategoryDTO, 0, len(order))
	for _, k := range order {
		out = append(out, *cats[k])
	}
	if out == nil {
		out = []logCategoryDTO{}
	}
	writeJSON(w, http.StatusOK, out)
}

// friendlyLog renders a (JSON-decoded) log line as a short human-readable string
// for the Overview "recent logs" panel: "<device>: <message>", falling back to
// the event name or the raw line when the CEF payload can't be parsed.
func friendlyLog(raw string) string {
	name, device, msg, _, ok := parseCEF(raw)
	if !ok {
		return raw
	}
	text := msg
	if text == "" {
		text = name
	}
	if device != "" && text != "" {
		return device + ": " + text
	}
	return text
}

// parseCEF extracts the event name, device, message and level from a UniFi CEF
// log line: "...CEF:0|Ubiquiti|UniFi OS|ver|sigId|Name|Severity|ext...".
func parseCEF(line string) (name, device, msg, level string, ok bool) {
	i := strings.Index(line, "CEF:0|")
	if i < 0 {
		return "", "", "", "", false
	}
	parts := strings.SplitN(line[i+len("CEF:0|"):], "|", 7)
	if len(parts) < 7 {
		return "", "", "", "", false
	}
	name = strings.TrimSpace(parts[4])
	level = cefSeverityLevel(parts[5])
	ext := parts[6]
	device = cefExt(ext, "UNIFIdeviceName=")
	msg = cefExt(ext, "msg=")
	return name, device, msg, level, true
}

// cefExt returns the value of a CEF extension key, ending at the next known key
// prefix (" UNIFI") or end of string.
func cefExt(ext, key string) string {
	i := strings.Index(ext, key)
	if i < 0 {
		return ""
	}
	v := ext[i+len(key):]
	if j := strings.Index(v, " UNIFI"); j >= 0 {
		v = v[:j]
	}
	return strings.TrimSpace(v)
}

func cefSeverityLevel(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return "info"
	}
	switch {
	case n >= 7:
		return "error"
	case n >= 4:
		return "warn"
	default:
		return "info"
	}
}

func levelKind(level string) string {
	switch level {
	case "error":
		return "no"
	case "warn":
		return "warn"
	default:
		return "info"
	}
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "log"
	}
	return out
}
