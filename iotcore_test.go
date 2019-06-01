package iotcore

import (
	"testing"
)

var device = Device{
	ProjectID:   "myproject",
	RegistryID:  "myregistery",
	DeviceID:    "foo",
	PrivKeyPath: "key.pem",
	Region:      "us-central1",
}

func TestClientID(t *testing.T) {
	want := "projects/myproject/locations/us-central1/registries/myregistery/devices/foo"
	got := device.ClientID()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigTopic(t *testing.T) {
	want := "/devices/foo/config"
	got := device.ConfigTopic()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCommandTopic(t *testing.T) {
	want := "/devices/foo/commands/#"
	got := device.CommandTopic()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTelemetryTopic(t *testing.T) {
	want := "/devices/foo/events"
	got := device.TelemetryTopic()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStateTopic(t *testing.T) {
	want := "/devices/foo/state"
	got := device.StateTopic()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
