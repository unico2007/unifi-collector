package web

import (
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/murad/unifi-collector/internal/web/respond"
)

type attackDTO struct {
	Time   string `json:"time"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Action string `json:"action"`
}

type firewallDTO struct {
	Allow         []float64   `json:"allow"`
	Deny          []float64   `json:"deny"`
	BlockedToday  int         `json:"blockedToday"`
	TopBlockedIps []talker    `json:"topBlockedIps"`
	TopRules      []kv        `json:"topRules"`
	WebCategories []kv        `json:"webCategories"`
	Attacks       []attackDTO `json:"attacks"`
}

// handleFirewall builds the Firewall page from Kerio Control firewall logs in
// Loki (vendor="kerio"). Kerio ships every filter action over syslog; the
// collector tags those lines vendor="kerio" (see internal/syslog). Until Kerio
// logging is enabled the query returns nothing and every field is an honest zero
// — no fabricated data.
func (s *Server) handleFirewall(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lines := s.loki.Recent(ctx, `{vendor="kerio"}`, 24*time.Hour, 5000)

	out := firewallDTO{
		Allow:         make([]float64, 24),
		Deny:          make([]float64, 24),
		TopBlockedIps: []talker{},
		TopRules:      []kv{},
		WebCategories: []kv{},
		Attacks:       []attackDTO{},
	}

	ipCounts := map[string]int{}
	ruleCounts := map[string]int{}
	contentCounts := map[string]int{}

	for _, ln := range lines {
		ev, ok := parseKerio(ln.Msg)
		if !ok {
			continue
		}
		hour := hourOf(ln.Time)
		if ev.action == "allow" {
			if hour >= 0 {
				out.Allow[hour]++
			}
			continue
		}
		// deny
		if hour >= 0 {
			out.Deny[hour]++
		}
		out.BlockedToday++
		if ev.rule != "" {
			ruleCounts[ev.rule]++
		}
		if isPublicIP(ev.srcIP) {
			ipCounts[ev.srcIP]++
		}
		if ev.content != "" {
			contentCounts[ev.content]++
		}
		if len(out.Attacks) < 50 { // newest first (recent() sorts desc)
			out.Attacks = append(out.Attacks, attackDTO{
				Time:   ln.Time,
				Type:   ev.proto,
				Source: ev.srcIP,
				Action: "DENY",
			})
		}
	}

	out.TopBlockedIps = topTalkers(ipCounts, 8)
	out.TopRules = topKV(ruleCounts, 8)
	out.WebCategories = topKV(contentCounts, 8)

	respond.JSON(w, http.StatusOK, out)
}

// kerioEvent is the subset of a Kerio filter-log line the Firewall page needs.
type kerioEvent struct {
	action  string // "deny" | "allow"
	rule    string // rule name ("Admin panel block", "Peer to Peer traffic")
	srcIP   string // first IP in the line (the external attacker on inbound blocks)
	proto   string // TCP | UDP | ICMP
	content string // Kerio [Content] classification, e.g. "Suspected P2P"
}

var (
	kerioIPRe    = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
	kerioProtoRe = regexp.MustCompile(`proto:([A-Za-z]+)`)
	// src[:port] -> [peer ](dst)[:port] — tolerates both `IP:port` and
	// `host (IP):port` shapes, and portless ICMP.
	kerioConnRe     = regexp.MustCompile(`\(?(\d{1,3}(?:\.\d{1,3}){3})\)?(?::(\d+))?\s*->\s*(?:peer\s*)?\(?(\d{1,3}(?:\.\d{1,3}){3})\)?(?::(\d+))?`)
	kerioFlagRe     = regexp.MustCompile(`flags:\[\s*([A-Z ]+?)\s*\]`)
	kerioIcmpTypeRe = regexp.MustCompile(`type:(\d+)`)
)

// kerioConn returns "src[:port]" and "dst[:port]" from a Kerio line, or empty
// strings when the connection can't be located.
func kerioConn(msg string) (src, dst string) {
	m := kerioConnRe.FindStringSubmatch(msg)
	if m == nil {
		return "", ""
	}
	src = m[1]
	if m[2] != "" {
		src += ":" + m[2]
	}
	dst = m[3]
	if m[4] != "" {
		dst += ":" + m[4]
	}
	return src, dst
}

// kerioExtra returns the single most meaningful qualifier for a line: the
// [Content] classification, else the TCP flag, else the ICMP type.
func kerioExtra(msg string) string {
	if c := kerioContent(msg); c != "" {
		return c
	}
	if f := kerioFlagRe.FindStringSubmatch(msg); f != nil {
		return strings.TrimSpace(f[1])
	}
	if t := kerioIcmpTypeRe.FindStringSubmatch(msg); t != nil {
		return "type " + t[1]
	}
	return ""
}

// kerioCompact renders a raw Kerio filter line as a compact technical detail —
// "PROTO src → dst · EXTRA" — dropping len/seq/ack/win/tcplen/ttl noise. Falls
// back to the stripped raw line when the connection can't be parsed.
func kerioCompact(msg string) string {
	src, dst := kerioConn(msg)
	if src == "" || dst == "" {
		return kerioDetail(msg)
	}
	out := strings.TrimSpace(kerioProto(msg) + " " + src + " → " + dst)
	if e := kerioExtra(msg); e != "" {
		out += " · " + e
	}
	return out
}

// kerioTesvir renders a plain-Azerbaijani one-liner for non-technical readers:
// direction (internet vs LAN) + protocol + port/content. The verdict lives in the
// separate "Əməl" column so it isn't repeated here.
func kerioTesvir(ev kerioEvent, msg string) string {
	origin := "Daxili şəbəkədən"
	inbound := isPublicIP(ev.srcIP)
	if inbound {
		origin = "İnternetdən"
	}
	if ev.proto == "ICMP" {
		if t := kerioIcmpTypeRe.FindStringSubmatch(msg); t != nil && t[1] == "8" {
			return origin + " ping (ICMP)"
		}
		return origin + " ICMP paketi"
	}
	proto := ev.proto
	if proto == "" {
		proto = "paket"
	}
	desc := origin + " " + proto
	if inbound {
		desc += " cəhdi"
	}
	if ev.content != "" {
		return desc + " — " + ev.content
	}
	if _, dst := kerioConn(msg); dst != "" {
		if i := strings.LastIndexByte(dst, ':'); i >= 0 {
			return desc + " (port " + dst[i+1:] + ")"
		}
	}
	return desc
}

// parseKerio extracts the fields from one Kerio filter-log line. Handles both
// shapes seen in the wild:
//
//	DENY "Admin panel block" packet from internet, proto:TCP, len:40, 1.2.3.4:5 -> 6.7.8.9:80, ...
//	DENY [Rule] 'Peer to Peer traffic' [Connection] host (10.0.0.1):6881 -> peer (9.9.9.9):6881, UDP [Content] Suspected P2P
func parseKerio(msg string) (kerioEvent, bool) {
	if i := strings.Index(msg, "KerioControl:"); i >= 0 {
		msg = strings.TrimSpace(msg[i+len("KerioControl:"):])
	}
	var ev kerioEvent
	switch {
	case strings.Contains(msg, "DENY"), strings.Contains(msg, "DROP"):
		ev.action = "deny"
	case strings.Contains(msg, "PERMIT"), strings.Contains(msg, "ALLOW"):
		ev.action = "allow"
	default:
		return ev, false
	}
	ev.rule = kerioRule(msg)
	ev.proto = kerioProto(msg)
	ev.srcIP = kerioSrcIP(msg)
	ev.content = kerioContent(msg)
	return ev, true
}

// kerioRule pulls the rule name from either `[Rule] 'name'` or a `"name"` quote.
func kerioRule(msg string) string {
	if i := strings.Index(msg, "[Rule] '"); i >= 0 {
		rest := msg[i+len("[Rule] '"):]
		if j := strings.IndexByte(rest, '\''); j >= 0 {
			return rest[:j]
		}
	}
	if i := strings.IndexByte(msg, '"'); i >= 0 {
		rest := msg[i+1:]
		if j := strings.IndexByte(rest, '"'); j >= 0 {
			return rest[:j]
		}
	}
	return ""
}

func kerioProto(msg string) string {
	if m := kerioProtoRe.FindStringSubmatch(msg); m != nil {
		return strings.ToUpper(m[1])
	}
	for _, p := range []string{"ICMP", "TCP", "UDP"} {
		if strings.Contains(msg, " "+p+" ") || strings.Contains(msg, ", "+p+" ") || strings.HasSuffix(msg, " "+p) {
			return p
		}
	}
	return ""
}

func kerioSrcIP(msg string) string {
	if m := kerioIPRe.FindStringSubmatch(msg); m != nil {
		return m[1]
	}
	return ""
}

func kerioContent(msg string) string {
	if i := strings.Index(msg, "[Content] "); i >= 0 {
		return strings.TrimSpace(msg[i+len("[Content] "):])
	}
	return ""
}

// hourOf reads the hour (0-23) from a "15:04:05" time string, or -1.
func hourOf(t string) int {
	if len(t) < 2 {
		return -1
	}
	h, err := strconv.Atoi(t[:2])
	if err != nil || h < 0 || h > 23 {
		return -1
	}
	return h
}

// isPublicIP reports whether s is a routable public address (excludes RFC1918,
// loopback, link-local) — i.e. an external attacker worth ranking.
func isPublicIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}

// topTalkers / topKV rank a count map descending (ties broken by label) and cap.
func topTalkers(m map[string]int, n int) []talker {
	out := make([]talker, 0, len(m))
	for k, v := range m {
		out = append(out, talker{Label: k, Value: float64(v)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func topKV(m map[string]int, n int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{Label: k, Value: float64(v)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
