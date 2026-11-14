package computer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type fakeDesktop struct {
	mu       sync.Mutex
	profile  string
	commands []string
	ensure   error
	exec     error
	probes   int
}

func (f *fakeDesktop) DesktopProfile() string  { return f.profile }
func (f *fakeDesktop) DesktopMaxShotEdge() int { return 1024 }
func (f *fakeDesktop) EnsureDesktop(context.Context, port.SandboxRunOptions) (domain.DesktopEndpoints, error) {
	return domain.DesktopEndpoints{}, f.ensure
}
func (f *fakeDesktop) ExecDesktop(_ context.Context, _ port.SandboxRunOptions, command string, _ []string) ([]byte, error) {
	f.mu.Lock()
	f.commands = append(f.commands, command)
	if strings.Contains(command, "command -v xdotool") {
		f.probes++
	}
	f.mu.Unlock()
	return []byte("true"), f.exec
}
func (f *fakeDesktop) DesktopEndpoints(string) (domain.DesktopEndpoints, bool) {
	return domain.DesktopEndpoints{}, false
}

func TestDesktopProbeChecksContainerAndTools(t *testing.T) {
	f := &fakeDesktop{profile: domain.SandboxProfileDesktop, ensure: errors.New("container missing")}
	b := newDesktopBackend(f)
	st := b.probe(domain.ConfigComputerSection{})
	if st.Available || !strings.Contains(st.DegradedReason, "container missing") {
		t.Fatalf("probe=%+v", st)
	}
	f.ensure = nil
	st = b.probe(domain.ConfigComputerSection{})
	if !st.Available {
		t.Fatalf("probe=%+v", st)
	}
	f.exec = errors.New("xdotool missing")
	st = b.probe(domain.ConfigComputerSection{})
	if st.Available {
		t.Fatal("probe must fail when image tools are missing")
	}
}

func TestDesktopSessionsAreIndependent(t *testing.T) {
	b := newDesktopBackend(&fakeDesktop{})
	b.SetSession("s1", port.SandboxRunOptions{ProjectID: "project-1"})
	b.SetSession("s2", port.SandboxRunOptions{ProjectID: "project-2"})
	if got := b.session(withSession(context.Background(), "s1")).ProjectID; got != "project-1" {
		t.Fatalf("project=%q", got)
	}
	if got := b.session(withSession(context.Background(), "s2")).ProjectID; got != "project-2" {
		t.Fatalf("project=%q", got)
	}
}

func TestManagerDoesNotProbePerAction(t *testing.T) {
	f := &fakeDesktop{profile: domain.SandboxProfileDesktop}
	m := New(domain.ConfigComputerSection{Enabled: true, Target: domain.ComputerTargetHost})
	m.SetDesktopEnvironment(f)
	f.mu.Lock()
	f.probes = 0
	f.mu.Unlock()
	m.mu.Lock()
	m.status.Available = true
	m.desktop.probeFn = func(domain.ConfigComputerSection) domain.ComputerStatus {
		return domain.ComputerStatus{Platform: "linux", Backend: "sandbox-desktop", Available: true}
	}
	m.backend = m.desktop
	m.mu.Unlock()
	f.mu.Lock()
	f.probes = 0
	f.mu.Unlock()
	if err := m.Key(context.Background(), "Escape", false); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	count := f.probes
	f.mu.Unlock()
	if count != 0 {
		t.Fatalf("ready() unexpectedly executed %d container probes", count)
	}
}

type podmanDesktop struct{ container string }

func (p podmanDesktop) DesktopProfile() string  { return domain.SandboxProfileDesktop }
func (p podmanDesktop) DesktopMaxShotEdge() int { return 1024 }
func (p podmanDesktop) EnsureDesktop(context.Context, port.SandboxRunOptions) (domain.DesktopEndpoints, error) {
	return domain.DesktopEndpoints{}, nil
}
func (p podmanDesktop) ExecDesktop(ctx context.Context, _ port.SandboxRunOptions, command string, env []string) ([]byte, error) {
	args := []string{"exec"}
	for _, item := range env {
		args = append(args, "-e", item)
	}
	args = append(args, p.container, "sh", "-c", command)
	return exec.CommandContext(ctx, "podman", args...).Output()
}
func (p podmanDesktop) DesktopEndpoints(string) (domain.DesktopEndpoints, bool) {
	return domain.DesktopEndpoints{}, false
}

func TestDesktopSandboxIntegration(t *testing.T) {
	container := os.Getenv("DQ_COMPUTER_TEST_DESKTOP")
	if container == "" {
		t.Skip("set DQ_COMPUTER_TEST_DESKTOP to an isolated X11 test container")
	}
	env := podmanDesktop{container: container}
	m := New(domain.ConfigComputerSection{Enabled: true, Target: domain.ComputerTargetSandbox, MaxShotEdge: 512})
	m.SetDesktopEnvironment(env)
	if !m.Status().Available {
		t.Fatalf("sandbox unavailable: %+v", m.Status())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m.SetSession(port.SandboxRunOptions{ProjectID: "integration"}, "integration")
	img, err := m.Screenshot(ctx, "integration", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if img.Width > 512 || img.DisplayWidth != 1024 || img.DisplayHeight != 768 {
		t.Fatalf("capture geometry: %+v", img)
	}
	if err := m.MouseMove(ctx, "integration", "", img.Width/2, img.Height/2, nil); err != nil {
		t.Fatal(err)
	}
	x, y, err := m.CursorPosition(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if x != 512 || y != 384 {
		t.Fatalf("pointer at (%d,%d), want (512,384)", x, y)
	}
	if err := m.MouseClick(ctx, "integration", "", domain.MouseLeft, img.Width/2, img.Height/2, 1, nil); err != nil {
		t.Fatal(err)
	}
	wins, err := m.ListWindows(ctx, "xterm")
	if err != nil || len(wins) == 0 {
		t.Fatalf("windows=%v err=%v", wins, err)
	}
	windowShot, err := m.Screenshot(ctx, "integration", wins[0].ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if windowShot.DisplayWidth != wins[0].Width || windowShot.DisplayHeight != wins[0].Height || windowShot.OriginX != wins[0].X || windowShot.OriginY != wins[0].Y {
		t.Fatalf("window capture geometry mismatch: shot=%+v bounds=%+v", windowShot, wins[0])
	}
	if err := m.Scroll(ctx, "integration", "", img.Width/2, img.Height/2, domain.ScrollDown, 2); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopProbeHasBoundedTimeout(t *testing.T) {
	f := &fakeDesktop{profile: domain.SandboxProfileDesktop}
	b := newDesktopBackend(f)
	start := time.Now()
	_ = b.probe(domain.ConfigComputerSection{})
	if time.Since(start) > 30*time.Second {
		t.Fatal("probe should not wait indefinitely")
	}
}
