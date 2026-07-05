package collector_test

import (
	"context"
	"errors"
	"testing"

	"github.com/murad/unifi-collector/internal/collector"
	"github.com/murad/unifi-collector/internal/models"
	"go.uber.org/zap"
)

type stubDeviceSource struct {
	name    string
	devices []models.Device
	err     error
}

func (s *stubDeviceSource) Name() string { return s.name }
func (s *stubDeviceSource) Devices(context.Context) ([]models.Device, error) {
	return s.devices, s.err
}

func TestMultiDeviceSource_Concatenates(t *testing.T) {
	a := &stubDeviceSource{name: "unifi", devices: []models.Device{{ID: "a"}}}
	b := &stubDeviceSource{name: "kerio", devices: []models.Device{{ID: "b"}, {ID: "c"}}}
	m := collector.NewMultiDeviceSource(zap.NewNop(), a, b)

	got, err := m.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d devices, want 3", len(got))
	}
}

func TestMultiDeviceSource_PartialFailureReturnsRest(t *testing.T) {
	good := &stubDeviceSource{name: "unifi", devices: []models.Device{{ID: "a"}}}
	bad := &stubDeviceSource{name: "kerio", err: errors.New("boom")}
	m := collector.NewMultiDeviceSource(zap.NewNop(), good, bad)

	got, err := m.Devices(context.Background())
	if err != nil {
		t.Fatalf("one source failing should not error, got: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %v, want only the healthy device", got)
	}
}

func TestMultiDeviceSource_AllFailReturnsError(t *testing.T) {
	m := collector.NewMultiDeviceSource(zap.NewNop(),
		&stubDeviceSource{name: "unifi", err: errors.New("boom1")},
		&stubDeviceSource{name: "kerio", err: errors.New("boom2")},
	)
	if _, err := m.Devices(context.Background()); err == nil {
		t.Fatal("expected an error when every source fails")
	}
}

func TestMultiDeviceSource_IgnoresNil(t *testing.T) {
	good := &stubDeviceSource{name: "unifi", devices: []models.Device{{ID: "a"}}}
	m := collector.NewMultiDeviceSource(zap.NewNop(), nil, good, nil)

	got, err := m.Devices(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v err %v, want 1 device and no error", got, err)
	}
}
