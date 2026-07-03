package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/models"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordDevices_RemovesStaleSeries(t *testing.T) {
	m := New("unifi")

	m.RecordDevices([]models.Device{
		{Vendor: "unifi", Site: "default", MAC: "aa", Name: "ap1", Type: "uap", State: "online", CPUPercent: 10},
		{Vendor: "unifi", Site: "default", MAC: "bb", Name: "ap2", Type: "uap", State: "online", CPUPercent: 20},
	})
	if got := testutil.CollectAndCount(m.deviceCPU); got != 2 {
		t.Fatalf("after first scrape: cpu series = %d, want 2", got)
	}

	// Second scrape has only one device: the disappeared device's series must go.
	m.RecordDevices([]models.Device{
		{Vendor: "unifi", Site: "default", MAC: "aa", Name: "ap1", Type: "uap", State: "online", CPUPercent: 15},
	})
	if got := testutil.CollectAndCount(m.deviceCPU); got != 1 {
		t.Fatalf("after second scrape: cpu series = %d, want 1 (stale removed)", got)
	}
	if got := testutil.ToFloat64(m.deviceCPU.WithLabelValues("unifi", "default", "aa", "ap1", "", "uap")); got != 15 {
		t.Errorf("ap1 cpu = %v, want 15", got)
	}
}

func TestObserveScrape_CountsErrors(t *testing.T) {
	m := New("unifi")

	m.ObserveScrape("devices", 5*time.Millisecond, nil)
	if got := testutil.ToFloat64(m.scrapeSuccess.WithLabelValues("devices")); got != 1 {
		t.Errorf("success = %v, want 1", got)
	}

	m.ObserveScrape("devices", 5*time.Millisecond, errors.New("boom"))
	if got := testutil.ToFloat64(m.scrapeErrors.WithLabelValues("devices")); got != 1 {
		t.Errorf("errors_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.scrapeSuccess.WithLabelValues("devices")); got != 0 {
		t.Errorf("success after error = %v, want 0", got)
	}
}

func TestRecordDevices_DevicesTotalByType(t *testing.T) {
	m := New("unifi")
	m.RecordDevices([]models.Device{
		{Vendor: "unifi", Site: "default", MAC: "a", Type: "uap"},
		{Vendor: "unifi", Site: "default", MAC: "b", Type: "uap"},
		{Vendor: "unifi", Site: "default", MAC: "c", Type: "usw"},
	})
	if got := testutil.ToFloat64(m.devicesTotal.WithLabelValues("unifi", "default", "uap")); got != 2 {
		t.Errorf("uap total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.devicesTotal.WithLabelValues("unifi", "default", "usw")); got != 1 {
		t.Errorf("usw total = %v, want 1", got)
	}
}
