package connector

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreateIdentityStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "remote.json")
	a, err := LoadOrCreateIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.DeviceID != b.DeviceID || a.DeviceSecret != b.DeviceSecret {
		t.Fatalf("identity not stable: %+v vs %+v", a, b)
	}
}

func TestNormalizeConnectorURL(t *testing.T) {
	got := NormalizeConnectorURL("https://hub.example.com")
	want := "wss://hub.example.com/v1/connector"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if HubHTTPSBase(got) != "https://hub.example.com" {
		t.Fatalf("https base: %s", HubHTTPSBase(got))
	}
}
