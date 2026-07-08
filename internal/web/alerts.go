package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
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
	Rules []ruleDTO `json:"rules"`
}

// rules are evaluated live against Prometheus on every request — no background
// state, so "active alerts" are always the current truth. Thresholds are fixed
// for now; user-configurable rules + history are a later phase.
var alertRules = []ruleDTO{
	{Name: "Cihaz offline", Condition: "unifi_device_up == 0", Level: "critical"},
	{Name: "CPU yüksək", Condition: "unifi_device_cpu_percent > 85", Level: "warning"},
	{Name: "Yaddaş yüksək", Condition: "unifi_device_memory_percent > 90", Level: "warning"},
	{Name: "Subsystem problemi", Condition: "unifi_health_status < 1", Level: "warning"},
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var out alertsDTO
	out.Active = []alertDTO{}
	out.Rules = alertRules

	out.Active = append(out.Active, s.alertDevices(ctx, `unifi_device_up == 0`, "critical", "Cihaz offline",
		func(l map[string]string, _ float64) (string, string) {
			return fmt.Sprintf("%s (%s) offline-dır", devName(l), l["type"]), "offline"
		})...)

	out.Active = append(out.Active, s.alertDevices(ctx, `unifi_device_cpu_percent > 85`, "warning", "CPU yüksək",
		func(l map[string]string, v float64) (string, string) {
			return fmt.Sprintf("%s: CPU %.0f%%", devName(l), v), fmt.Sprintf("%.0f%%", v)
		})...)

	out.Active = append(out.Active, s.alertDevices(ctx, `unifi_device_memory_percent > 90`, "warning", "Yaddaş yüksək",
		func(l map[string]string, v float64) (string, string) {
			return fmt.Sprintf("%s: yaddaş %.0f%%", devName(l), v), fmt.Sprintf("%.0f%%", v)
		})...)

	out.Active = append(out.Active, s.alertDevices(ctx, `unifi_health_status < 1`, "warning", "Subsystem problemi",
		func(l map[string]string, v float64) (string, string) {
			lvl := "xəbərdarlıq"
			if v == 0 {
				lvl = "xəta"
			}
			return fmt.Sprintf("%s subsystem: %s", l["subsystem"], lvl), lvl
		})...)

	// Escalate health=0 subsystems to critical.
	for i := range out.Active {
		if out.Active[i].Rule == "Subsystem problemi" && out.Active[i].Value == "xəta" {
			out.Active[i].Level = "critical"
		}
	}

	// Critical first, then warnings.
	sort.SliceStable(out.Active, func(i, j int) bool {
		return alertRank(out.Active[i].Level) < alertRank(out.Active[j].Level)
	})
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
