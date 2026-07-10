// Package metrics is the Prometheus exporter. It implements the collector
// metric-sink interfaces and exposes an HTTP handler for /metrics. It owns a
// private *prometheus.Registry (no global default registry) so instances are
// isolated and unit-testable.
package metrics

import (
	"net/http"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// frameworkNamespace is used for collector self-observability metrics. It is
// intentionally vendor-neutral (not the per-vendor namespace).
const frameworkNamespace = "collector"

// Label sets. Order matters for the WithLabelValues calls below.
var (
	deviceLabels      = []string{"vendor", "site", "mac", "name", "model", "type"}
	deviceInfoLabels  = []string{"vendor", "site", "mac", "name", "model", "type", "version", "state", "ip"}
	devicesTotalLabel = []string{"vendor", "site", "type"}
	clientLabels      = []string{"vendor", "site", "mac", "name", "ap", "vlan", "band"}
	clientInfoLabels  = []string{"vendor", "site", "mac", "name", "ip"}
	clientsTotalLabel = []string{"vendor", "site"}
	healthLabels      = []string{"vendor", "site", "subsystem"}
	scrapeLabels      = []string{"collector"}
)

// Metrics holds all exported series and satisfies the collector metric sinks.
type Metrics struct {
	reg *prometheus.Registry

	// Devices.
	devicesTotal  *prometheus.GaugeVec
	deviceUp      *prometheus.GaugeVec
	deviceCPU     *prometheus.GaugeVec
	deviceMem     *prometheus.GaugeVec
	deviceUptime  *prometheus.GaugeVec
	deviceRxBytes *prometheus.GaugeVec
	deviceTxBytes *prometheus.GaugeVec
	deviceInfo    *prometheus.GaugeVec

	// Clients.
	clientsTotal    *prometheus.GaugeVec
	clientRSSI      *prometheus.GaugeVec
	clientTxRate    *prometheus.GaugeVec
	clientRxRate    *prometheus.GaugeVec
	clientConnected *prometheus.GaugeVec
	clientInfo      *prometheus.GaugeVec

	// Health.
	healthStatus  *prometheus.GaugeVec
	healthDevices *prometheus.GaugeVec
	healthClients *prometheus.GaugeVec

	// Framework self-observability.
	scrapeErrors   *prometheus.CounterVec
	scrapeDuration *prometheus.HistogramVec
	scrapeSuccess  *prometheus.GaugeVec
	scrapeLast     *prometheus.GaugeVec
}

// New builds a Metrics with all series registered under the given namespace
// (e.g. "unifi"). It also registers Go runtime and process collectors.
func New(namespace string) *Metrics {
	gauge := func(subsystem, name, help string, labels []string) *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem, Name: name, Help: help,
		}, labels)
	}

	m := &Metrics{
		reg: prometheus.NewRegistry(),

		devicesTotal:  gauge("", "devices_total", "Number of devices by type.", devicesTotalLabel),
		deviceUp:      gauge("device", "up", "1 if the device is online, else 0.", deviceLabels),
		deviceCPU:     gauge("device", "cpu_percent", "Device CPU utilization percent.", deviceLabels),
		deviceMem:     gauge("device", "memory_percent", "Device memory utilization percent.", deviceLabels),
		deviceUptime:  gauge("device", "uptime_seconds", "Device uptime in seconds.", deviceLabels),
		deviceRxBytes: gauge("device", "rx_bytes", "Device received bytes (cumulative).", deviceLabels),
		deviceTxBytes: gauge("device", "tx_bytes", "Device transmitted bytes (cumulative).", deviceLabels),
		deviceInfo:    gauge("device", "info", "Device metadata; value is always 1.", deviceInfoLabels),

		clientsTotal:    gauge("", "clients_total", "Number of connected clients.", clientsTotalLabel),
		clientRSSI:      gauge("client", "rssi", "Client signal strength (RSSI).", clientLabels),
		clientTxRate:    gauge("client", "tx_rate", "Client transmit rate in bits/s.", clientLabels),
		clientRxRate:    gauge("client", "rx_rate", "Client receive rate in bits/s.", clientLabels),
		clientConnected: gauge("client", "connected_seconds", "Client connected time in seconds.", clientLabels),
		clientInfo:      gauge("client", "info", "Client metadata (carries ip); value is always 1.", clientInfoLabels),

		healthStatus:  gauge("health", "status", "Subsystem status (1=ok, 0.5=warning, 0=error).", healthLabels),
		healthDevices: gauge("health", "devices", "Devices per subsystem.", healthLabels),
		healthClients: gauge("health", "clients", "Clients per subsystem.", healthLabels),

		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: frameworkNamespace, Subsystem: "scrape", Name: "errors_total",
			Help: "Total collection errors by collector.",
		}, scrapeLabels),
		scrapeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: frameworkNamespace, Subsystem: "scrape", Name: "duration_seconds",
			Help: "Collection duration by collector.", Buckets: prometheus.DefBuckets,
		}, scrapeLabels),
		scrapeSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: frameworkNamespace, Subsystem: "scrape", Name: "success",
			Help: "1 if the last collection succeeded, else 0.",
		}, scrapeLabels),
		scrapeLast: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: frameworkNamespace, Subsystem: "scrape", Name: "last_timestamp_seconds",
			Help: "Unix timestamp of the last collection.",
		}, scrapeLabels),
	}

	m.reg.MustRegister(
		m.devicesTotal, m.deviceUp, m.deviceCPU, m.deviceMem, m.deviceUptime,
		m.deviceRxBytes, m.deviceTxBytes, m.deviceInfo,
		m.clientsTotal, m.clientRSSI, m.clientTxRate, m.clientRxRate, m.clientConnected, m.clientInfo,
		m.healthStatus, m.healthDevices, m.healthClients,
		m.scrapeErrors, m.scrapeDuration, m.scrapeSuccess, m.scrapeLast,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Handler returns the /metrics HTTP handler backed by this instance's registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// RecordDevices implements collector.DeviceMetricSink. It resets the device
// gauges first so that devices which disappeared between scrapes do not leave
// stale series behind.
func (m *Metrics) RecordDevices(devices []models.Device) {
	m.devicesTotal.Reset()
	m.deviceUp.Reset()
	m.deviceCPU.Reset()
	m.deviceMem.Reset()
	m.deviceUptime.Reset()
	m.deviceRxBytes.Reset()
	m.deviceTxBytes.Reset()
	m.deviceInfo.Reset()

	byType := map[[3]string]int{}
	for _, d := range devices {
		lv := []string{d.Vendor, d.Site, d.MAC, d.Name, d.Model, d.Type}
		up := 0.0
		if d.State == "online" {
			up = 1
		}
		m.deviceUp.WithLabelValues(lv...).Set(up)
		m.deviceCPU.WithLabelValues(lv...).Set(d.CPUPercent)
		m.deviceMem.WithLabelValues(lv...).Set(d.MemoryPercent)
		m.deviceUptime.WithLabelValues(lv...).Set(d.Uptime.Seconds())
		m.deviceRxBytes.WithLabelValues(lv...).Set(d.RxBytes)
		m.deviceTxBytes.WithLabelValues(lv...).Set(d.TxBytes)
		m.deviceInfo.WithLabelValues(d.Vendor, d.Site, d.MAC, d.Name, d.Model, d.Type, d.Version, d.State, d.IP).Set(1)

		byType[[3]string{d.Vendor, d.Site, d.Type}]++
	}
	for k, n := range byType {
		m.devicesTotal.WithLabelValues(k[0], k[1], k[2]).Set(float64(n))
	}
}

// RecordClients implements collector.ClientMetricSink.
func (m *Metrics) RecordClients(clients []models.Client) {
	m.clientsTotal.Reset()
	m.clientRSSI.Reset()
	m.clientTxRate.Reset()
	m.clientRxRate.Reset()
	m.clientConnected.Reset()
	m.clientInfo.Reset()

	bySite := map[[2]string]int{}
	for _, c := range clients {
		lv := []string{c.Vendor, c.Site, c.MAC, c.Name, c.ConnectedAP, c.VLAN, c.Band}
		m.clientRSSI.WithLabelValues(lv...).Set(c.RSSI)
		m.clientTxRate.WithLabelValues(lv...).Set(c.TxRate)
		m.clientRxRate.WithLabelValues(lv...).Set(c.RxRate)
		m.clientConnected.WithLabelValues(lv...).Set(c.ConnectedTime.Seconds())
		m.clientInfo.WithLabelValues(c.Vendor, c.Site, c.MAC, c.Name, c.IP).Set(1)
		bySite[[2]string{c.Vendor, c.Site}]++
	}
	for k, n := range bySite {
		m.clientsTotal.WithLabelValues(k[0], k[1]).Set(float64(n))
	}
}

// RecordHealth implements collector.HealthMetricSink.
func (m *Metrics) RecordHealth(health []models.Health) {
	m.healthStatus.Reset()
	m.healthDevices.Reset()
	m.healthClients.Reset()

	for _, h := range health {
		lv := []string{h.Vendor, h.Site, h.Subsystem}
		m.healthStatus.WithLabelValues(lv...).Set(statusValue(h.Status))
		m.healthDevices.WithLabelValues(lv...).Set(float64(h.NumDevices))
		m.healthClients.WithLabelValues(lv...).Set(float64(h.NumClients))
	}
}

// ObserveScrape records the outcome of one collection cycle. The scheduler
// calls this after every Collect.
func (m *Metrics) ObserveScrape(collectorName string, duration time.Duration, err error) {
	m.scrapeDuration.WithLabelValues(collectorName).Observe(duration.Seconds())
	m.scrapeLast.WithLabelValues(collectorName).Set(float64(time.Now().Unix()))
	if err != nil {
		m.scrapeErrors.WithLabelValues(collectorName).Inc()
		m.scrapeSuccess.WithLabelValues(collectorName).Set(0)
		return
	}
	m.scrapeSuccess.WithLabelValues(collectorName).Set(1)
}

func statusValue(status string) float64 {
	switch status {
	case "ok":
		return 1
	case "warning":
		return 0.5
	default:
		return 0
	}
}
