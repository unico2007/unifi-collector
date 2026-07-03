// Package syslog is a push-based log receiver. Unlike the pull-based collectors,
// devices send syslog messages TO us; the receiver parses them (RFC3164 and
// RFC5424), maps them to neutral models.Event values, and forwards them to a
// collector.LogSink (Loki). It runs as a long-lived service, like the Loki
// exporter, bound to the app context for graceful shutdown.
package syslog

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// Config configures the syslog receiver.
type Config struct {
	UDPAddr string // e.g. ":1514" (empty disables UDP)
	TCPAddr string // e.g. ":1514" (empty disables TCP)
	Vendor  string // label applied to received logs
	Site    string // site label applied to received logs
}

// Receiver listens for syslog messages and forwards them to a LogSink.
type Receiver struct {
	cfg  Config
	sink collector.LogSink
	log  *zap.Logger
}

// New builds a receiver. sink must be non-nil.
func New(cfg Config, sink collector.LogSink, log *zap.Logger) *Receiver {
	if cfg.Vendor == "" {
		cfg.Vendor = "syslog"
	}
	if cfg.Site == "" {
		cfg.Site = "default"
	}
	return &Receiver{cfg: cfg, sink: sink, log: log}
}

// Run starts the configured listeners and blocks until ctx is cancelled.
func (r *Receiver) Run(ctx context.Context) error {
	errCh := make(chan error, 2)
	started := 0

	if r.cfg.UDPAddr != "" {
		started++
		go func() { errCh <- r.serveUDP(ctx) }()
	}
	if r.cfg.TCPAddr != "" {
		started++
		go func() { errCh <- r.serveTCP(ctx) }()
	}
	if started == 0 {
		r.log.Warn("syslog: no listeners configured")
		<-ctx.Done()
		return nil
	}

	for i := 0; i < started; i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

func (r *Receiver) serveUDP(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", r.cfg.UDPAddr)
	if err != nil {
		return err
	}
	r.log.Info("syslog: UDP listener started", zap.String("addr", r.cfg.UDPAddr))

	go func() { <-ctx.Done(); _ = pc.Close() }()

	buf := make([]byte, 16*1024)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				r.log.Info("syslog: UDP listener stopped")
				return nil
			}
			continue
		}
		r.handle(ctx, string(buf[:n]))
	}
}

func (r *Receiver) serveTCP(ctx context.Context) error {
	ln, err := net.Listen("tcp", r.cfg.TCPAddr)
	if err != nil {
		return err
	}
	r.log.Info("syslog: TCP listener started", zap.String("addr", r.cfg.TCPAddr))

	go func() { <-ctx.Done(); _ = ln.Close() }()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				r.log.Info("syslog: TCP listener stopped")
				return nil
			}
			continue
		}
		go r.handleConn(ctx, conn)
	}
}

// handleConn reads newline-delimited syslog lines from a TCP connection.
func (r *Receiver) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	go func() { <-ctx.Done(); _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			r.handle(ctx, line)
		}
	}
}

// handle parses one raw message and forwards it as an Event.
func (r *Receiver) handle(ctx context.Context, raw string) {
	m := parse(raw)
	ev := models.Event{
		Vendor:     r.cfg.Vendor,
		Site:       r.cfg.Site,
		Timestamp:  m.timestamp,
		Type:       models.EventUnknown,
		Level:      severityToLevel(m.severity),
		Message:    m.message,
		Hostname:   m.hostname,
		DeviceName: m.hostname,
	}
	if err := r.sink.WriteEvents(ctx, []models.Event{ev}); err != nil {
		r.log.Warn("syslog: forwarding message failed", zap.Error(err))
	}
}

// parsed holds the fields extracted from a syslog line.
type parsed struct {
	severity  int
	timestamp time.Time
	hostname  string
	message   string
}

// parse extracts a best-effort structure from an RFC3164 or RFC5424 message.
// On anything unexpected it degrades to severity=info and the whole line as the
// message, so no log is ever dropped.
func parse(raw string) parsed {
	p := parsed{severity: 6, timestamp: time.Now(), message: strings.TrimSpace(raw)}

	if !strings.HasPrefix(raw, "<") {
		return p
	}
	gt := strings.IndexByte(raw, '>')
	if gt < 0 {
		return p
	}
	pri, err := strconv.Atoi(raw[1:gt])
	if err != nil || pri < 0 {
		return p
	}
	p.severity = pri % 8
	rest := raw[gt+1:]

	// RFC5424: "1 TIMESTAMP HOSTNAME APP PROCID MSGID [SD] MSG"
	if strings.HasPrefix(rest, "1 ") {
		f := strings.SplitN(rest, " ", 8)
		if len(f) >= 7 {
			if t, e := time.Parse(time.RFC3339, f[1]); e == nil {
				p.timestamp = t
			}
			p.hostname = notNil(f[2])
			app := notNil(f[3])
			msg := ""
			if len(f) == 8 {
				msg = f[7]
			}
			p.message = strings.TrimSpace(app + ": " + msg)
			return p
		}
	}

	// RFC3164: "Mon _2 15:04:05 HOSTNAME TAG: MSG"
	if len(rest) > 16 {
		if t, e := time.Parse(time.Stamp, strings.TrimSpace(rest[:15])); e == nil {
			now := time.Now()
			p.timestamp = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
		}
		remainder := strings.TrimSpace(rest[15:])
		if host, msg, ok := strings.Cut(remainder, " "); ok {
			p.hostname = host
			p.message = msg
			return p
		}
	}

	p.message = strings.TrimSpace(rest)
	return p
}

// notNil converts the RFC5424 "-" placeholder to an empty string.
func notNil(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

// severityToLevel maps a syslog severity (0-7) to a log level.
func severityToLevel(sev int) string {
	switch {
	case sev <= 3:
		return "error"
	case sev == 4:
		return "warning"
	case sev == 7:
		return "debug"
	default:
		return "info"
	}
}
