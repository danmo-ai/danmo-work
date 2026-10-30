package sandbox

import (
	"strings"
	"testing"

	"danmo-work/core/adapter/container"
	"danmo-work/core/domain"
)

func TestNormalizeSandboxProfile(t *testing.T) {
	if domain.NormalizeSandboxProfile("") != domain.SandboxProfileCLI {
		t.Fatal("empty")
	}
	if domain.NormalizeSandboxProfile("DESKTOP") != domain.SandboxProfileDesktop {
		t.Fatal("desktop")
	}
}

func TestDefaultImageTagByProfile(t *testing.T) {
	if container.DefaultImageTag("cli") != domain.EnvImageTagCLI {
		t.Fatal(container.DefaultImageTag("cli"))
	}
	if container.DefaultImageTag("desktop") != domain.EnvImageTagDesktop {
		t.Fatal(container.DefaultImageTag("desktop"))
	}
}

func TestEnvironmentStatusDesktopFields(t *testing.T) {
	m := New(domain.ConfigSandboxSection{
		Enabled: true,
		Backend: "docker",
		Profile: domain.SandboxProfileDesktop,
	})
	st := m.EnvironmentStatus()
	// Without a real docker daemon we may degrade to local; profile should still surface
	// when container backend is selected and not degraded — otherwise Backend=local.
	if st.Profile == domain.SandboxProfileDesktop {
		if st.Display != "" && st.Display != desktopDisplay {
			t.Fatalf("display=%q", st.Display)
		}
	}
	full := m.StatusWithTar("dev")
	if len(full.DesktopTarVariants) != 2 {
		t.Fatalf("desktop variants=%d", len(full.DesktopTarVariants))
	}
	for _, v := range full.DesktopTarVariants {
		if v.Profile != domain.SandboxProfileDesktop {
			t.Fatalf("%+v", v)
		}
		if !strings.Contains(v.AssetName, "desktop") {
			t.Fatalf("asset=%s", v.AssetName)
		}
	}
}

func TestBuildRuntimeEnvironmentDesktopPrompt(t *testing.T) {
	// Imported via runtime package test in prompt_builder_test; keep a lightweight
	// check that desktop preinstalled constant is non-empty.
	if domain.EnvDesktopImagePreinstalled == "" {
		t.Fatal("empty desktop preinstalled blurb")
	}
	if !strings.Contains(domain.EnvDesktopImagePreinstalled, "Xvfb") {
		t.Fatal(domain.EnvDesktopImagePreinstalled)
	}
}
