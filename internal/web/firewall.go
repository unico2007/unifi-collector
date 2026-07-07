package web

import "net/http"

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

// handleFirewall currently has no real data source: Kerio firewall logs are not
// shipped to Loki yet, and no firewall metrics exist. It returns an honest empty
// structure (no fabricated attacks/IPs) rather than mock data. Wire this up once
// Kerio firewall syslog -> Loki is configured (see the gap report).
func (s *Server) handleFirewall(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, firewallDTO{
		Allow:         []float64{},
		Deny:          []float64{},
		BlockedToday:  0,
		TopBlockedIps: []talker{},
		TopRules:      []kv{},
		WebCategories: []kv{},
		Attacks:       []attackDTO{},
	})
}
