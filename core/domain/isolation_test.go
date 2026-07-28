package domain

import "testing"

func TestComputeEffectiveIsolationContainer(t *testing.T) {
	sb := SandboxStatus{
		Enabled: true,
		Mode:    SandboxModeWorkspaceWrite,
		Network: SandboxNetworkDeny,
		Backend: SandboxBackendHostWeak,
	}
	env := EnvironmentStatus{
		Backend: EnvironmentBackendContainer,
		Engine:  "docker",
	}
	iso := ComputeEffectiveIsolation(sb, env)
	if !iso.Strong || iso.Source != IsolationContainer {
		t.Fatalf("%+v", iso)
	}
	if iso.Network != SandboxNetworkDeny {
		t.Fatalf("network=%s", iso.Network)
	}
}

func TestComputeEffectiveIsolationOSSandbox(t *testing.T) {
	sb := SandboxStatus{
		Enabled: true,
		Mode:    SandboxModeWorkspaceWrite,
		Network: SandboxNetworkAllowlist,
		Backend: SandboxBackendSeatbelt,
	}
	iso := ComputeEffectiveIsolation(sb, EnvironmentStatus{Backend: EnvironmentBackendLocal})
	if !iso.Strong || iso.Source != IsolationOSSandbox {
		t.Fatalf("%+v", iso)
	}
}

func TestComputeEffectiveIsolationNone(t *testing.T) {
	sb := SandboxStatus{Enabled: false, Backend: SandboxBackendDisabled}
	iso := ComputeEffectiveIsolation(sb, EnvironmentStatus{})
	if iso.Strong || iso.Source != IsolationNone {
		t.Fatalf("%+v", iso)
	}
}
