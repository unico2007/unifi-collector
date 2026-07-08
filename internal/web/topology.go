package web

import (
	"net/http"
	"sort"
)

type topoNode struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Vendor  string `json:"vendor"`
	Model   string `json:"model"`
	IP      string `json:"ip"`
	State   string `json:"state"`
	Clients int    `json:"clients"` // wireless clients, for APs
}

type topoClient struct {
	Name string  `json:"name"`
	MAC  string  `json:"mac"`
	RSSI float64 `json:"rssi"`
}

type topologyDTO struct {
	Edge        []topoNode              `json:"edge"`     // firewall + gateway (network exit)
	Switches    []topoNode              `json:"switches"` // usw
	APs         []topoNode              `json:"aps"`      // uap
	ClientsByAp map[string][]topoClient `json:"clientsByAp"`
	Stats       struct {
		Switches int `json:"switches"`
		APs      int `json:"aps"`
		Clients  int `json:"clients"`
	} `json:"stats"`
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := topologyDTO{
		Edge: []topoNode{}, Switches: []topoNode{}, APs: []topoNode{},
		ClientsByAp: map[string][]topoClient{},
	}

	// Clients grouped under their AP (ap label is a MAC → resolve to name).
	names := s.apNames(ctx)
	if rows, err := s.prom.query(ctx, `unifi_client_rssi`); err == nil {
		for _, c := range rows {
			ap := apLabel(names, c.labels["ap"])
			out.ClientsByAp[ap] = append(out.ClientsByAp[ap], topoClient{
				Name: c.labels["name"], MAC: c.labels["mac"], RSSI: c.value,
			})
			out.Stats.Clients++
		}
	}

	infos, err := s.prom.query(ctx, `unifi_device_info`)
	if err != nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	for _, in := range infos {
		n := topoNode{
			Name: in.labels["name"], Type: in.labels["type"], Vendor: in.labels["vendor"],
			Model: in.labels["model"], IP: ipOrDash(in.labels["ip"]), State: in.labels["state"],
		}
		switch {
		case n.Vendor == "kerio" || n.Type == "ugw" || n.Type == "interface":
			out.Edge = append(out.Edge, n)
		case n.Type == "usw":
			out.Switches = append(out.Switches, n)
			out.Stats.Switches++
		case n.Type == "uap":
			n.Clients = len(out.ClientsByAp[n.Name])
			out.APs = append(out.APs, n)
			out.Stats.APs++
		default:
			out.Edge = append(out.Edge, n)
		}
	}

	byName := func(a, b topoNode) bool { return a.Name < b.Name }
	sort.SliceStable(out.Edge, func(i, j int) bool { return byName(out.Edge[i], out.Edge[j]) })
	sort.SliceStable(out.Switches, func(i, j int) bool { return byName(out.Switches[i], out.Switches[j]) })
	sort.SliceStable(out.APs, func(i, j int) bool { return out.APs[i].Clients > out.APs[j].Clients })

	writeJSON(w, http.StatusOK, out)
}
