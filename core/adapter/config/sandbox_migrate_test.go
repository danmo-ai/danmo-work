package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestMigrateLegacyEnvironmentContainerBackend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
runtime:
  sandbox:
    enabled: true
    mode: workspace-write
    network: deny
    backend: ""
  environment:
    backend: container
    engine: podman
    image: "localhost/my-env:v1"
    workspace_mount: "/workspace"
    resources:
      cpus: "2"
      memory: "1g"
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	sb := cfg.Runtime.Sandbox
	if sb.Backend != "podman" {
		t.Fatalf("backend=%q want podman", sb.Backend)
	}
	if sb.Image != "localhost/my-env:v1" {
		t.Fatalf("image=%q", sb.Image)
	}
	if sb.WorkspaceMount != "/workspace" {
		t.Fatalf("mount=%q", sb.WorkspaceMount)
	}
	if sb.Resources.CPUs != "2" || sb.Resources.Memory != "1g" {
		t.Fatalf("resources=%+v", sb.Resources)
	}
	if cfg.Runtime.Environment != nil {
		t.Fatal("legacy environment section should be cleared after migration")
	}
}

func TestMigrateLegacyEnvironmentDockerAppleAliases(t *testing.T) {
	for engine, want := range map[string]string{
		"docker":          "docker",
		"apple-container": "apple-container",
		"auto":            "", // auto never auto-picks container engines
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		yaml := `
runtime:
  environment:
    backend: oci
    engine: ` + engine + "\n"
		if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
			t.Fatal(err)
		}
		l := NewLoader(path)
		cfg, err := l.Load(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Runtime.Sandbox.Backend; got != want {
			t.Fatalf("engine=%s backend=%q want %q", engine, got, want)
		}
	}
}

func TestSaveOmitsLegacyEnvironmentSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Runtime.Sandbox = domain.ConfigSandboxSection{
		Enabled: true,
		Mode:    domain.SandboxModeWorkspaceWrite,
		Network: domain.SandboxNetworkDeny,
		Backend: "podman",
		Image:   "localhost/my-env:v2",
	}
	if err := l.Save(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "environment:") {
		t.Fatalf("saved config should not contain legacy environment section:\n%s", b)
	}
	if !strings.Contains(string(b), "podman") {
		t.Fatalf("saved config missing podman backend:\n%s", b)
	}
	cfg2, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Runtime.Sandbox.Backend != "podman" {
		t.Fatalf("round-trip backend=%q", cfg2.Runtime.Sandbox.Backend)
	}
}
