// Package handler holds the read-only dashboard and log view handlers
// (overview, devices, clients, wifi, traffic, firewall, topology, logs). They
// translate Prometheus/Loki queries into the compact JSON the frontend charts
// consume. Stateful features (auth, alerting) live in their own packages; this
// package only reads.
package handler

import (
	"github.com/murad/unifi-collector/internal/web/alert"
	"github.com/murad/unifi-collector/internal/web/query"
)

// Handlers carries the read dependencies shared by every view handler.
type Handlers struct {
	prom   *query.Prometheus
	loki   *query.Loki
	alerts *alert.Service // Overview reads the live active-alert count
}

// New builds the handler set.
func New(prom *query.Prometheus, loki *query.Loki, alerts *alert.Service) *Handlers {
	return &Handlers{prom: prom, loki: loki, alerts: alerts}
}
