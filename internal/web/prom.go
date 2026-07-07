// Package web is the BFF (backend-for-frontend) for the Unico dashboard. It
// serves the built React app, proxies the AI service, authenticates users, and
// exposes /api/* JSON built from Prometheus + Loki — so the browser never talks
// to those systems directly.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// promClient is a tiny Prometheus HTTP API client: just enough for instant and
// range queries. It deliberately avoids the official client to keep deps light.
type promClient struct {
	base string
	http *http.Client
}

func newPromClient(base string) *promClient {
	return &promClient{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

// sample is one Prometheus series with its label set and a scalar value.
type sample struct {
	labels map[string]string
	value  float64
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

// query runs an instant PromQL query and returns the vector as samples.
func (c *promClient) query(ctx context.Context, expr string) ([]sample, error) {
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

	out := make([]sample, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		v := 0.0
		if len(r.Value) == 2 {
			if s, ok := r.Value[1].(string); ok {
				v, _ = strconv.ParseFloat(s, 64)
			}
		}
		out = append(out, sample{labels: r.Metric, value: v})
	}
	return out, nil
}

// scalar runs an instant query expected to return a single value (e.g. a
// count) and returns it, or 0 if the result is empty.
func (c *promClient) scalar(ctx context.Context, expr string) (float64, error) {
	s, err := c.query(ctx, expr)
	if err != nil {
		return 0, err
	}
	if len(s) == 0 {
		return 0, nil
	}
	return s[0].value, nil
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

// rangeSeries runs a range query over the last dur (single-series expected) and
// returns the values as a plain float slice for the frontend charts.
func (c *promClient) rangeSeries(ctx context.Context, expr string, dur time.Duration, step time.Duration) ([]float64, error) {
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
