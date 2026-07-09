package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type alertDTO struct {
	Level   string `json:"level"`  // "critical" | "warning"
	Rule    string `json:"rule"`   // human rule name
	Target  string `json:"target"` // device / subsystem the alert is about
	Message string `json:"message"`
	Value   string `json:"value"`
}

type ruleDTO struct {
	Name      string `json:"name"`
	Condition string `json:"condition"`
	Level     string `json:"level"`
}

type alertsDTO struct {
	Active []alertDTO `json:"active"`
	Counts struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
	} `json:"counts"`
	Rules           []ruleDTO  `json:"rules"`
	Thresholds      thresholds `json:"thresholds"`
	TelegramEnabled bool       `json:"telegramEnabled"`
}

// rulesFor builds the rule list for the given thresholds. Rules are evaluated
// live against Prometheus on every request — no background state, so "active
// alerts" are always the current truth. CPU/memory limits are user-configurable
// (persisted in SQLite); offline + subsystem-health are boolean and fixed.
func rulesFor(th thresholds) []ruleDTO {
	return []ruleDTO{
		{Name: "Cihaz offline", Condition: "unifi_device_up == 0", Level: "critical"},
		{Name: "CPU yüksək", Condition: fmt.Sprintf("unifi_device_cpu_percent > %g", th.CPU), Level: "warning"},
		{Name: "Yaddaş yüksək", Condition: fmt.Sprintf("unifi_device_memory_percent > %g", th.Memory), Level: "warning"},
		{Name: "Subsystem problemi", Condition: "unifi_health_status < 1", Level: "warning"},
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	th := s.astore.thresholds()
	var out alertsDTO
	out.Active = s.activeAlerts(ctx, th)
	out.Rules = rulesFor(th)
	out.Thresholds = th
	out.TelegramEnabled = s.notifier.enabled()

	for _, a := range out.Active {
		switch a.Level {
		case "critical":
			out.Counts.Critical++
		case "warning":
			out.Counts.Warning++
		}
	}

	writeJSON(w, http.StatusOK, out)
}

// handleAlertHistory returns the recent fire/resolve timeline, newest first.
func (s *Server) handleAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, s.astore.history(limit))
}

// handleAlertSettings returns the current configurable thresholds.
func (s *Server) handleAlertSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.astore.thresholds())
}

// handleAlertSettingsUpdate persists new thresholds (admin-only; gated by the
// route wrapper). Values out of 1..100 fall back to the previous defaults.
func (s *Server) handleAlertSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var th thresholds
	if err := json.NewDecoder(r.Body).Decode(&th); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := s.astore.setThresholds(th); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
		return
	}
	writeJSON(w, http.StatusOK, s.astore.thresholds())
}

// handleTestNotify sends a test Telegram message so an admin can verify the
// bot token + chat id are correct (admin-only; gated by the route wrapper).
func (s *Server) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	if !s.notifier.enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.notifier.send(ctx, "✅ Unico test bildirişi — Telegram inteqrasiyası işləyir."); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"enabled": true, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "sent": true})
}

// activeAlerts evaluates every rule live against Prometheus and returns the
// current active alerts, critical first. Shared by the Alerts page, the Overview
// alert count, and the background history evaluator so they always agree.
func (s *Server) activeAlerts(ctx context.Context, th thresholds) []alertDTO {
	active := []alertDTO{}

	active = append(active, s.alertDevices(ctx, `unifi_device_up == 0`, "critical", "Cihaz offline",
		func(l map[string]string, _ float64) (string, string) {
			return fmt.Sprintf("%s (%s) offline-dır", devName(l), l["type"]), "offline"
		})...)

	active = append(active, s.alertDevices(ctx, fmt.Sprintf(`unifi_device_cpu_percent > %g`, th.CPU), "warning", "CPU yüksək",
		func(l map[string]string, v float64) (string, string) {
			return fmt.Sprintf("%s: CPU %.0f%%", devName(l), v), fmt.Sprintf("%.0f%%", v)
		})...)

	active = append(active, s.alertDevices(ctx, fmt.Sprintf(`unifi_device_memory_percent > %g`, th.Memory), "warning", "Yaddaş yüksək",
		func(l map[string]string, v float64) (string, string) {
			return fmt.Sprintf("%s: yaddaş %.0f%%", devName(l), v), fmt.Sprintf("%.0f%%", v)
		})...)

	active = append(active, s.alertDevices(ctx, `unifi_health_status < 1`, "warning", "Subsystem problemi",
		func(l map[string]string, v float64) (string, string) {
			lvl := "xəbərdarlıq"
			if v == 0 {
				lvl = "xəta"
			}
			return fmt.Sprintf("%s subsystem: %s", l["subsystem"], lvl), lvl
		})...)

	// Escalate health=0 subsystems to critical.
	for i := range active {
		if active[i].Rule == "Subsystem problemi" && active[i].Value == "xəta" {
			active[i].Level = "critical"
		}
	}

	// Critical first, then warnings.
	sort.SliceStable(active, func(i, j int) bool {
		return alertRank(active[i].Level) < alertRank(active[j].Level)
	})
	return active
}

// alertDevices runs a threshold query and builds one alert per matching series.
func (s *Server) alertDevices(ctx context.Context, expr, level, rule string,
	build func(labels map[string]string, value float64) (msg, val string)) []alertDTO {
	rows, err := s.prom.query(ctx, expr)
	if err != nil {
		return nil
	}
	out := make([]alertDTO, 0, len(rows))
	for _, row := range rows {
		msg, val := build(row.labels, row.value)
		target := devName(row.labels)
		if row.labels["subsystem"] != "" {
			target = row.labels["subsystem"]
		}
		out = append(out, alertDTO{Level: level, Rule: rule, Target: target, Message: msg, Value: val})
	}
	return out
}

func devName(l map[string]string) string {
	if n := l["name"]; n != "" {
		return n
	}
	return l["mac"]
}

func alertRank(level string) int {
	if level == "critical" {
		return 0
	}
	return 1
}
