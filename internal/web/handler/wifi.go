package handler

import (
	"net/http"
	"sort"

	"github.com/murad/unifi-collector/internal/web/respond"
)

// kv is the {label,value} shape the frontend charts consume.
type kv struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type wifiDTO struct {
	RSSIBins     []int    `json:"rssiBins"`
	RSSILabels   []string `json:"rssiLabels"`
	ClientsPerAp []kv     `json:"clientsPerAp"`
	BandSplit    []kv     `json:"bandSplit"`
	VLANSplit    []kv     `json:"vlanSplit"`
	Quality      struct {
		Good int `json:"good"`
		Fair int `json:"fair"`
		Poor int `json:"poor"`
	} `json:"quality"`
}

// rssiBinLabels defines the histogram buckets (upper dBm edge per bin).
var rssiBinLabels = []string{"-90", "-80", "-75", "-70", "-65", "-60", "-55", "-45"}
var rssiBinEdges = []float64{-85, -80, -75, -70, -65, -60, -55, 0} // client falls in first bin whose edge >= rssi

func (s *Handlers) Wifi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var d wifiDTO
	d.RSSILabels = rssiBinLabels
	d.RSSIBins = make([]int, len(rssiBinLabels))

	// band!="" keeps this to actual WiFi clients: wired clients used to be
	// exported with rssi=0, which lands in the strongest bin and the "good"
	// quality bucket. The collector no longer emits those series, but the
	// filter also shields against data scraped by older collector versions.
	clients, err := s.prom.Query(ctx, `unifi_client_rssi{band!=""}`)
	if err != nil {
		d.ClientsPerAp, d.VLANSplit, d.BandSplit = []kv{}, []kv{}, []kv{}
		respond.JSON(w, http.StatusOK, d)
		return
	}
	names := s.apNames(ctx)

	perAp := map[string]float64{}
	perVlan := map[string]float64{}
	perBand := map[string]float64{}
	for _, c := range clients {
		rssi := c.Value
		// histogram
		for i, edge := range rssiBinEdges {
			if rssi <= edge {
				d.RSSIBins[i]++
				break
			}
		}
		// quality buckets
		switch {
		case rssi >= -60:
			d.Quality.Good++
		case rssi >= -72:
			d.Quality.Fair++
		default:
			d.Quality.Poor++
		}
		if ap := c.Labels["ap"]; ap != "" {
			perAp[apLabel(names, ap)]++
		}
		if vlan := c.Labels["vlan"]; vlan != "" {
			perVlan[vlan]++
		}
		if band := c.Labels["band"]; band != "" {
			perBand[band]++
		}
	}

	d.ClientsPerAp = sortedKV(perAp, "")
	d.VLANSplit = sortedKV(perVlan, "VLAN ")
	d.BandSplit = sortedKV(perBand, "")
	respond.JSON(w, http.StatusOK, d)
}

// sortedKV turns a label->count map into a value-descending kv slice, with an
// optional label prefix.
func sortedKV(m map[string]float64, prefix string) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{Label: prefix + k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Label < out[j].Label
	})
	return out
}
