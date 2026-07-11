// Package query holds the read-only clients the BFF uses to pull data from
// Prometheus and Loki. These are the "repository" layer: they know how to talk
// to the upstream stores and return plain Go values, with no HTTP-handler or
// presentation concerns.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Prometheus is a tiny Prometheus HTTP API client: just enough for instant and
// range queries. It deliberately avoids the official client to keep deps light.
type Prometheus struct {
	base string
	http *http.Client
}

// NewPrometheus builds a Prometheus client pointed at base (e.g.
// http://prometheus:9090).
func NewPrometheus(base string) *Prometheus {
	return &Prometheus{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

// Sample is one Prometheus series with its label set and a scalar value.
type Sample struct {
	Labels map[string]string
	Value  float64
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// Query runs an instant PromQL query and returns the vector as samples.
func (c *Prometheus) Query(ctx context.Context, expr string) ([]Sample, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", c.base, url.QueryEscape(expr))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s", pr.Error)
	}

	out := make([]Sample, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		v := 0.0
		if len(r.Value) == 2 {
			if s, ok := r.Value[1].(string); ok {
				v, _ = strconv.ParseFloat(s, 64)
			}
		}
		out = append(out, Sample{Labels: r.Metric, Value: v})
	}
	return out, nil
}

// Scalar runs an instant query expected to return a single value (e.g. a
// count) and returns it, or 0 if the result is empty.
func (c *Prometheus) Scalar(ctx context.Context, expr string) (float64, error) {
	s, err := c.Query(ctx, expr)
	if err != nil {
		return 0, err
	}
	if len(s) == 0 {
		return 0, nil
	}
	return s[0].Value, nil
}

type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Values [][2]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// ParseRange maps a UI range key ("1h"/"6h"/"24h"/"7d") to a query duration and
// step. Steps are chosen to keep each series around 24-30 points so the charts
// stay readable at every zoom level. Unknown values default to 24h.
func ParseRange(r string) (dur, step time.Duration) {
	switch r {
	case "1h":
		return time.Hour, 2 * time.Minute
	case "6h":
		return 6 * time.Hour, 15 * time.Minute
	case "7d":
		return 7 * 24 * time.Hour, 6 * time.Hour
	default:
		return 24 * time.Hour, time.Hour
	}
}

// RangeSeries runs a range query over the last dur (single-series expected) and
// returns the values as a plain float slice for the frontend charts.
func (c *Prometheus) RangeSeries(ctx context.Context, expr string, dur time.Duration, step time.Duration) ([]float64, error) {
	end := time.Now()
	start := end.Add(-dur)
	u := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%d",
		c.base, url.QueryEscape(expr), start.Unix(), end.Unix(), int(step.Seconds()))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var pr promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	if pr.Status != "success" || len(pr.Data.Result) == 0 {
		return nil, nil
	}
	vals := pr.Data.Result[0].Values
	out := make([]float64, 0, len(vals))
	for _, v := range vals {
		if s, ok := v[1].(string); ok {
			f, _ := strconv.ParseFloat(s, 64)
			out = append(out, f)
		}
	}
	return out, nil
}
