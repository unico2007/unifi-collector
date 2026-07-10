// Package models defines the vendor-neutral domain types shared across the
// entire collector. No vendor-specific field (UniFi, QNAP, Cisco, ...) may
// leak into this package: vendor adapters are responsible for translating
// their proprietary payloads into these neutral shapes.
package models

import "time"

// Device is a monitored network device (access point, switch, gateway, ...),
// independent of the vendor that produced it.
type Device struct {
	Vendor  string // "unifi", "qnap", ... — which adapter produced this record
	Site    string // logical grouping the device belongs to
	ID      string // vendor-native stable identifier
	Name    string
	MAC     string
	Model   string
	Type    string // "uap", "usw", "ugw", ...
	IP      string
	Version string // firmware / software version
	State   string // human-readable state, e.g. "online", "offline"
	Adopted bool

	CPUPercent    float64
	MemoryPercent float64
	Uptime        time.Duration
	RxBytes       float64
	TxBytes       float64

	// Labels carries extra vendor-specific dimensions that should be kept
	// as metric/log labels without polluting the neutral struct.
	Labels map[string]string
}

// Client is an end-user device connected through a monitored Device.
type Client struct {
	Vendor      string
	Site        string
	ID          string
	Name        string
	MAC         string
	IP          string
	ConnectedAP string // MAC or name of the AP/switch the client is attached to
	VLAN        string
	Band        string // "2.4 GHz", "5 GHz", "6 GHz" for wireless; "" for wired

	RSSI          float64
	TxRate        float64 // bits/s
	RxRate        float64 // bits/s
	TxBytes       float64 // cumulative bytes this session
	RxBytes       float64 // cumulative bytes this session
	ConnectedTime time.Duration

	Labels map[string]string
}

// EventType is the normalized, vendor-independent category of an event.
type EventType string

const (
	EventClientConnected    EventType = "client_connected"
	EventClientDisconnected EventType = "client_disconnected"
	EventAPRestart          EventType = "ap_restart"
	EventAPOffline          EventType = "ap_offline"
	EventAPOnline           EventType = "ap_online"
	EventFirmwareUpdate     EventType = "firmware_update"
	EventDeviceAdopted      EventType = "device_adopted"
	EventDeviceLost         EventType = "device_lost"
	EventUnknown            EventType = "unknown"
)

// Event is a normalized log/event record destined for Loki.
type Event struct {
	Vendor    string
	Site      string
	Timestamp time.Time
	Type      EventType
	Level     string // "info", "warning", "error"
	Message   string

	// Device / client context (any may be empty depending on the event).
	DeviceMAC  string
	DeviceName string
	Model      string
	ClientMAC  string
	Hostname   string

	Labels map[string]string
}

// Health is a high-level, vendor-neutral health snapshot for a site/subsystem.
type Health struct {
	Vendor     string
	Site       string
	Subsystem  string // "wlan", "wan", "lan", ...
	Status     string // "ok", "warning", "error"
	NumDevices int
	NumClients int

	Labels map[string]string
}
