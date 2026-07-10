// Command web is the Unico BFF (backend-for-frontend). It serves the built
// React dashboard, proxies the AI service, authenticates users against a SQLite
// store, and exposes /api/* JSON built from Prometheus + Loki.
//
// It also doubles as the user-admin CLI (there is no self-register endpoint):
//
//	web -create-user -user admin -pass 's3cret' -role admin
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/murad/unifi-collector/internal/logger"
	"github.com/murad/unifi-collector/internal/web"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr       = flag.String("addr", env("WEB_ADDR", ":8080"), "listen address")
		staticDir  = flag.String("static", env("WEB_STATIC_DIR", "web/dist"), "built frontend directory")
		promURL    = flag.String("prometheus", env("WEB_PROMETHEUS_URL", "http://prometheus:9090"), "Prometheus base URL")
		lokiURL    = flag.String("loki", env("WEB_LOKI_URL", "http://loki:3100"), "Loki base URL")
		aiURL      = flag.String("ai", env("WEB_AI_URL", "http://ai:8090"), "AI service base URL")
		dbPath     = flag.String("db", env("WEB_DB", "data/users.db"), "SQLite user database path")
		tgToken    = flag.String("telegram-token", env("WEB_TELEGRAM_TOKEN", ""), "Telegram bot token for alert notifications (optional)")
		tgChat     = flag.String("telegram-chat", env("WEB_TELEGRAM_CHAT_ID", ""), "Telegram chat id to notify (optional)")
		tgCritChat = flag.String("telegram-critical-chat", env("WEB_TELEGRAM_CRITICAL_CHAT_ID", ""), "Telegram chat id for critical alerts (optional; falls back to -telegram-chat)")
		createUser = flag.Bool("create-user", false, "create/update a user, then exit")
		username   = flag.String("user", "", "username (with -create-user)")
		password   = flag.String("pass", "", "password (with -create-user)")
		role       = flag.String("role", "guest", "role: admin or guest (with -create-user)")
	)
	flag.Parse()

	users, err := web.OpenUserStore(*dbPath)
	if err != nil {
		return fmt.Errorf("open user store: %w", err)
	}
	defer func() { _ = users.Close() }()

	// CLI mode: create/update a user and exit.
	if *createUser {
		if err := users.UpsertUser(*username, *password, *role); err != nil {
			return err
		}
		fmt.Printf("user %q (%s) saved\n", *username, *role)
		return nil
	}

	log, err := logger.New(logger.Config{Level: "info", Format: "json"})
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	log.Info("unico bff starting", zap.String("version", version))

	srv, err := web.New(web.Config{
		Addr:                   *addr,
		StaticDir:              *staticDir,
		PrometheusURL:          *promURL,
		LokiURL:                *lokiURL,
		AIURL:                  *aiURL,
		TelegramToken:          *tgToken,
		TelegramChatID:         *tgChat,
		TelegramCriticalChatID: *tgCritChat,
		ReadTimeout:            15 * time.Second,
		WriteTimeout:           30 * time.Second,
	}, users, log)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return srv.Run(ctx)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
