// Command collector is the composition root. It constructs every component and
// injects dependencies (constructor injection), then runs the long-lived
// services (Loki shipper, scheduler, HTTP server) bound to a single context so
// that a shutdown signal stops all of them gracefully.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/config"
	"github.com/murad/unifi-collector/internal/logger"
	"github.com/murad/unifi-collector/internal/loki"
	"github.com/murad/unifi-collector/internal/metrics"
	"github.com/murad/unifi-collector/internal/scheduler"
	"github.com/murad/unifi-collector/internal/server"
	"github.com/murad/unifi-collector/internal/syslog"
	"github.com/murad/unifi-collector/internal/unifi"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// metricNamespace is the Prometheus namespace for vendor metrics (unifi_*).
const metricNamespace = "unifi"

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz and exit (for container HEALTHCHECK)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if *healthcheck {
		return doHealthcheck(cfg.Server.Addr)
	}

	log, err := logger.New(logger.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format})
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	log.Info("collector starting",
		zap.String("version", version),
		zap.String("unifi_url", cfg.UniFi.URL),
		zap.String("site", cfg.UniFi.Site))

	// --- Exporters (sinks) ---------------------------------------------------
	mx := metrics.New(metricNamespace)

	var lokiExp *loki.Exporter
	if cfg.Loki.Enabled {
		lokiExp = loki.New(loki.Config{
			URL:        cfg.Loki.URL,
			BatchSize:  cfg.Loki.BatchSize,
			BatchWait:  cfg.Loki.BatchWait,
			Tenant:     cfg.Loki.Tenant,
			Timeout:    10 * time.Second,
			MaxRetries: 3,
		}, log)
	}

	// --- Vendor adapter ------------------------------------------------------
	uclient, err := unifi.NewClient(unifi.Config{
		BaseURL:             cfg.UniFi.URL,
		Username:            cfg.UniFi.Username,
		Password:            cfg.UniFi.Password,
		Site:                cfg.UniFi.Site,
		VerifyTLS:           cfg.UniFi.VerifyTLS,
		Timeout:             cfg.UniFi.Timeout,
		MaxRetries:          cfg.UniFi.MaxRetries,
		AuthFailureCooldown: cfg.UniFi.AuthFailureCooldown,
	}, log)
	if err != nil {
		return err
	}

	// --- Collectors (registered per config) ----------------------------------
	reg := collector.NewRegistry()
	if err := registerCollectors(reg, cfg, uclient, mx, lokiExp, log); err != nil {
		return err
	}

	// --- Scheduler -----------------------------------------------------------
	sched := scheduler.New(log, mx)
	for _, c := range reg.All() {
		cc := cfg.Collectors[c.Name()]
		if err := sched.Register(c, cc.Interval, cc.Interval); err != nil {
			return err
		}
	}

	// --- HTTP server ---------------------------------------------------------
	srv := server.New(server.Config{
		Addr:         cfg.Server.Addr,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}, mx.Handler(), log)

	// --- Lifecycle -----------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)

	if lokiExp != nil {
		g.Go(func() error { return lokiExp.Run(gctx) })
	}
	g.Go(func() error { return sched.Run(gctx) })
	g.Go(func() error { return srv.Run(gctx) })
	g.Go(func() error { markReadyWhenLoggedIn(gctx, uclient, srv, log); return nil })

	// Push-based syslog receiver (optional). It shares the Loki LogSink.
	if cfg.Syslog.Enabled {
		if lokiExp == nil {
			log.Warn("syslog enabled but Loki is disabled; skipping (syslog has no sink)")
		} else {
			recv := syslog.New(syslog.Config{
				UDPAddr: cfg.Syslog.UDPAddr,
				TCPAddr: cfg.Syslog.TCPAddr,
				Vendor:  cfg.Syslog.Vendor,
				Site:    cfg.Syslog.Site,
			}, lokiExp, log)
			g.Go(func() error { return recv.Run(gctx) })
		}
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info("collector stopped cleanly")
	return nil
}

// doHealthcheck performs a one-shot GET of /healthz and returns an error unless
// it gets 200. Used by the container HEALTHCHECK against the distroless image.
func doHealthcheck(addr string) error {
	host := addr
	if len(host) > 0 && host[0] == ':' {
		host = "127.0.0.1" + host
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/healthz")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: status %d", resp.StatusCode)
	}
	return nil
}

// registerCollectors builds and registers the enabled collectors. Adding a new
// vendor later means adding its adapter and a few lines here — nothing in the
// core packages changes.
func registerCollectors(
	reg *collector.Registry,
	cfg *config.Config,
	src *unifi.Client,
	mx *metrics.Metrics,
	lokiExp *loki.Exporter,
	log *zap.Logger,
) error {
	enabled := func(name string) bool { return cfg.Collectors[name].Enabled }

	if enabled("devices") {
		if err := reg.Register(collector.NewDeviceCollector(src, mx, log)); err != nil {
			return err
		}
	}
	if enabled("clients") {
		if err := reg.Register(collector.NewClientCollector(src, mx, log)); err != nil {
			return err
		}
	}
	if enabled("health") {
		if err := reg.Register(collector.NewHealthCollector(src, mx, log)); err != nil {
			return err
		}
	}
	if enabled("events") {
		if lokiExp == nil {
			log.Warn("events collector enabled but Loki is disabled; skipping (events have no sink)")
		} else if err := reg.Register(collector.NewEventCollector(src, lokiExp, log, time.Hour)); err != nil {
			return err
		}
	}
	return nil
}

// markReadyWhenLoggedIn retries the UniFi login until it succeeds (or ctx is
// cancelled), then flips the server's readiness probe to ready.
func markReadyWhenLoggedIn(ctx context.Context, c *unifi.Client, srv *server.Server, log *zap.Logger) {
	backoff := time.Second
	for {
		if err := c.Login(ctx); err == nil {
			srv.SetReady(true)
			log.Info("readiness: UniFi login succeeded")
			return
		} else {
			log.Warn("readiness: UniFi login failed, retrying", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}
