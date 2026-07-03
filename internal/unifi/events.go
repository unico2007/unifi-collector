package unifi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

// rawEvent mirrors the subset of the UniFi stat/event payload we use.
type rawEvent struct {
	Time      int64  `json:"time"` // epoch milliseconds
	Key       string `json:"key"`  // e.g. "EVT_WU_Connected"
	Msg       string `json:"msg"`
	Subsystem string `json:"subsystem"`
	AP        string `json:"ap"`
	APName    string `json:"ap_name"`
	SW        string `json:"sw"`
	SWName    string `json:"sw_name"`
	GW        string `json:"gw"`
	User      string `json:"user"`     // client MAC
	Hostname  string `json:"hostname"` // client hostname
	Model     string `json:"model"`
}

// classifyEvent maps a UniFi event key to a neutral EventType and a log level.
// Substring matching keeps it resilient to the many WU/WG/LU/LG key variants.
// Order matters: more specific patterns are checked before general ones.
func classifyEvent(key string) (models.EventType, string) {
	k := key
	switch {
	case strings.Contains(k, "Adopt"):
		return models.EventDeviceAdopted, "info"
	case strings.Contains(k, "Restart") || strings.Contains(k, "Reboot"):
		return models.EventAPRestart, "warning"
	case strings.Contains(k, "Upgrad") || strings.Contains(k, "Firmware"):
		return models.EventFirmwareUpdate, "info"
	case strings.Contains(k, "Lost_Contact") || strings.Contains(k, "WasOffline"):
		// Infrastructure device lost contact => it went offline.
		return models.EventAPOffline, "warning"
	case strings.Contains(k, "Isolated") || (strings.Contains(k, "Lost") && isDeviceKey(k)):
		return models.EventDeviceLost, "warning"
	case strings.Contains(k, "Disconnected") && isDeviceKey(k):
		return models.EventAPOffline, "warning"
	case strings.Contains(k, "Disconnected"):
		return models.EventClientDisconnected, "info"
	case strings.Contains(k, "Connected") && isDeviceKey(k):
		return models.EventAPOnline, "info"
	case strings.Contains(k, "Connected"):
		return models.EventClientConnected, "info"
	default:
		return models.EventUnknown, "info"
	}
}

// isDeviceKey reports whether the key concerns an infrastructure device (AP,
// switch, gateway) rather than an end-user client.
func isDeviceKey(k string) bool {
	return strings.Contains(k, "EVT_AP") || strings.Contains(k, "EVT_SW") ||
		strings.Contains(k, "EVT_GW") || strings.Contains(k, "EVT_DEV")
}

// eventCandidates are the endpoint variants tried, in order, to fetch events.
// Different UniFi controller versions expose events differently; the first one
// that returns a valid response is cached for subsequent calls.
var eventCandidates = []struct{ method, path string }{
	{http.MethodGet, pathEvents},   // stat/event (GET)
	{http.MethodPost, pathEvents},  // stat/event (POST with query)
	{http.MethodGet, "rest/event"}, // rest/event (GET)
}

// Events implements collector.EventSource. It fetches recent events and returns
// only those strictly newer than `since`.
func (c *Client) Events(ctx context.Context, since time.Time) ([]models.Event, error) {
	raw, err := c.fetchRawEvents(ctx, since)
	if err != nil {
		return nil, err
	}

	out := make([]models.Event, 0, len(raw))
	for _, e := range raw {
		ts := time.UnixMilli(e.Time)
		if !since.IsZero() && !ts.After(since) {
			continue
		}
		etype, level := classifyEvent(e.Key)

		devMAC, devName := e.AP, e.APName
		if devMAC == "" {
			devMAC, devName = e.SW, e.SWName
		}
		if devMAC == "" {
			devMAC = e.GW
		}

		out = append(out, models.Event{
			Vendor:     c.Name(),
			Site:       c.cfg.Site,
			Timestamp:  ts,
			Type:       etype,
			Level:      level,
			Message:    e.Msg,
			DeviceMAC:  devMAC,
			DeviceName: devName,
			Model:      e.Model,
			ClientMAC:  e.User,
			Hostname:   e.Hostname,
		})
	}
	return out, nil
}

// fetchRawEvents retrieves raw events, discovering the working endpoint on the
// first call and caching it. It reuses the live authenticated session, so no
// extra logins are triggered. If no candidate works, events are treated as
// optional (nil, nil) after logging once.
func (c *Client) fetchRawEvents(ctx context.Context, since time.Time) ([]rawEvent, error) {
	c.mu.RLock()
	unsupported := c.evUnsupported
	resolved := c.evResolved
	method := c.evResolvedMethod
	path := c.evResolvedPath
	c.mu.RUnlock()

	if unsupported {
		return nil, nil
	}
	if resolved {
		return c.fetchEventsVia(ctx, method, path, since)
	}

	var lastErr error
	for _, cand := range eventCandidates {
		raw, err := c.fetchEventsVia(ctx, cand.method, cand.path, since)
		if err == nil {
			c.mu.Lock()
			c.evResolved = true
			c.evResolvedMethod = cand.method
			c.evResolvedPath = cand.path
			c.mu.Unlock()
			c.log.Info("unifi: resolved events endpoint",
				zap.String("method", cand.method), zap.String("path", cand.path))
			return raw, nil
		}
		if isEndpointMissing(err) {
			lastErr = err
			continue // try the next candidate
		}
		return nil, err // real error (auth, network): surface it
	}

	c.mu.Lock()
	c.evUnsupported = true
	c.mu.Unlock()
	c.logEventsUnsupportedOnce(lastErr)
	return nil, nil
}

func (c *Client) fetchEventsVia(ctx context.Context, method, path string, since time.Time) ([]rawEvent, error) {
	var raw []rawEvent
	if method == http.MethodGet {
		return raw, c.GetJSON(ctx, path, &raw)
	}
	return raw, c.PostJSON(ctx, path, eventQuery(since), &raw)
}

// eventQuery builds the POST body for the events endpoint. UniFi's "within"
// parameter is expressed in HOURS; results are still filtered client-side by
// `since`, so this only bounds the server-side window.
func eventQuery(since time.Time) map[string]any {
	hours := 24
	if !since.IsZero() {
		h := int(time.Since(since).Hours()) + 1
		if h > hours {
			hours = h
		}
		if hours > 720 { // cap at 30 days
			hours = 720
		}
	}
	return map[string]any{"_limit": 3000, "within": hours}
}

// isEndpointMissing reports whether err indicates the candidate endpoint is not
// usable (so the next candidate should be tried, or events marked optional). It
// covers HTTP 400/404/405 and the UniFi "api.err.NotFound"/"api.err.InvalidObject"
// envelope errors, which different controller versions return for absent or
// differently-shaped event endpoints.
func isEndpointMissing(err error) bool {
	if isHTTPStatus(err, http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "NotFound") || strings.Contains(msg, "InvalidObject")
}

func isHTTPStatus(err error, codes ...int) bool {
	var statusErr *statusError
	if !errors.As(err, &statusErr) {
		return false
	}
	for _, code := range codes {
		if statusErr.StatusCode == code {
			return true
		}
	}
	return false
}

func (c *Client) logEventsUnsupportedOnce(err error) {
	c.mu.Lock()
	if c.eventsUnsupportedLogged {
		c.mu.Unlock()
		return
	}
	c.eventsUnsupportedLogged = true
	c.mu.Unlock()

	c.log.Warn("unifi: event endpoint unavailable; continuing without event logs", zap.Error(err))
}
