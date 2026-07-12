package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/murad/unifi-collector/internal/web/query"
	"github.com/murad/unifi-collector/internal/web/respond"
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
	Rules                   []ruleDTO  `json:"rules"`
	Thresholds              thresholds `json:"thresholds"`
	TelegramEnabled         bool       `json:"telegramEnabled"`
	TelegramCriticalRouting bool       `json:"telegramCriticalRouting"`
}

// Service exposes the alert HTTP handlers and the live rule evaluation. It
// depends on Prometheus (for live rule queries), the alert Store (thresholds +
// history) and the Telegram notifier.
type Service struct {
	prom     *query.Prometheus
	store    *Store
	notifier *notifier
}

// NewService builds the alert service. Telegram parameters are optional; when
// blank, notifications are disabled.
func NewService(prom *query.Prometheus, store *Store, tgToken, tgChat, tgCriticalChat string) *Service {
	return &Service{prom: prom, store: store, notifier: newNotifier(tgToken, tgChat, tgCriticalChat)}
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

// Alerts returns the live active alerts plus the rule list and Telegram status.
func (s *Service) Alerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	th := s.store.thresholds()
	var out alertsDTO
	out.Active = s.activeAlerts(ctx, th)
	out.Rules = rulesFor(th)
	out.Thresholds = th
	out.TelegramEnabled = s.notifier.enabled()
	out.TelegramCriticalRouting = s.notifier.criticalRouting()

	for _, a := range out.Active {
		switch a.Level {
		case "critical":
			out.Counts.Critical++
		case "warning":
			out.Counts.Warning++
		}
	}

	respond.JSON(w, http.StatusOK, out)
}

// History returns the recent fire/resolve timeline, newest first.
func (s *Service) History(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	respond.JSON(w, http.StatusOK, s.store.history(limit))
}

// Settings returns the current configurable thresholds.
func (s *Service) Settings(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, s.store.thresholds())
}

// SettingsUpdate persists new thresholds (admin-only; gated by the route
// wrapper). Patch semantics: an omitted field keeps its stored value — decoding
// it into a plain struct would zero it and silently revert the admin's setting
// to the default. Out-of-range values are rejected, not silently clamped, so a
// typo (150 for 15) gets feedback instead of a 200.
func (s *Service) SettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var patch struct {
		CPU    *float64 `json:"cpuPercent"`
		Memory *float64 `json:"memoryPercent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		respond.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	th := s.store.thresholds()
	if patch.CPU != nil {
		th.CPU = *patch.CPU
	}
	if patch.Memory != nil {
		th.Memory = *patch.Memory
	}
	if th.CPU < 1 || th.CPU > 100 || th.Memory < 1 || th.Memory > 100 {
		respond.JSON(w, http.StatusBadRequest, map[string]string{"error": "thresholds must be between 1 and 100"})
		return
	}
	if err := s.store.setThresholds(th); err != nil {
		respond.JSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
		return
	}
	respond.JSON(w, http.StatusOK, s.store.thresholds())
}

// TestNotify sends a test Telegram message so an admin can verify the bot token
// + chat id are correct (admin-only; gated by the route wrapper).
func (s *Service) TestNotify(w http.ResponseWriter, r *http.Request) {
	if !s.notifier.enabled() {
		respond.JSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	// Send the sample through the real alert renderer so the admin sees exactly
	// how a live alert will look (structure, bold headers, timestamp).
	sample := alertDTO{
		Level:   "warning",
		Rule:    "Test bildirişi",
		Message: "Telegram inteqrasiyası işləyir. Bu, real alertlərin göndəriləcəyi formatın nümunəsidir — real hadisə deyil.",
	}
	if err := s.notifier.send(ctx, firedMessage(sample)); err != nil {
		respond.JSON(w, http.StatusBadGateway, map[string]any{"enabled": true, "error": err.Error()})
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"enabled": true, "sent": true})
}

// ActiveCount returns the number of currently-active alerts, evaluated live
// with the same engine as the Alerts page so the Overview badge always agrees.
func (s *Service) ActiveCount(ctx context.Context) int {
	return len(s.activeAlerts(ctx, s.store.thresholds()))
}

// activeAlerts evaluates every rule live against Prometheus and returns the
// current active alerts, critical first. Shared by the Alerts page, the Overview
// alert count, and the background history evaluator so they always agree.
func (s *Service) activeAlerts(ctx context.Context, th thresholds) []alertDTO {
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
func (s *Service) alertDevices(ctx context.Context, expr, level, rule string,
	build func(labels map[string]string, value float64) (msg, val string)) []alertDTO {
	rows, err := s.prom.Query(ctx, expr)
	if err != nil {
		return nil
	}
	out := make([]alertDTO, 0, len(rows))
	for _, row := range rows {
		msg, val := build(row.Labels, row.Value)
		target := devName(row.Labels)
		if row.Labels["subsystem"] != "" {
			target = row.Labels["subsystem"]
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
