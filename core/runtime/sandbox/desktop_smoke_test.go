//go:build docker

package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// Smoke: build (or load) desktop image, start container, probe CDP + DISPLAY.
// Run with: go test -tags docker ./core/runtime/sandbox/ -run DesktopSmoke -count=1
func TestDesktopSmoke(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	root := findRepoRoot(t)
	outTar := filepath.Join(root, "out", "env", "danmo-work-env-desktop-linux-amd64.tar.gz")
	if _, err := os.Stat(outTar); err != nil {
		t.Log("building desktop env tar (may take several minutes)…")
		cmd := exec.Command("bash", filepath.Join(root, "scripts", "build_env_tar.sh"))
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"DQ_ENV_PROFILE=desktop",
			"DQ_ROOT="+root,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("build desktop tar: %v", err)
		}
	}

	cfg := domain.ConfigSandboxSection{
		Enabled: true,
		Backend: "docker",
		Profile: domain.SandboxProfileDesktop,
		Network: domain.SandboxNetworkAllow,
		TarPath: outTar,
		Image:   domain.EnvImageTagDesktop,
	}
	b, err := newContainerBackend(domain.EnvironmentEngineDocker, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	work := t.TempDir()
	_ = os.WriteFile(filepath.Join(work, "index.html"), []byte("<html><body>ok</body></html>"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	opts := port.SandboxRunOptions{
		ProjectID: "smoke",
		WorkDir:   work,
		Timeout:   60 * time.Second,
	}
	out, err := b.ExecDesktop(ctx, opts, "xdpyinfo >/dev/null && echo DISPLAY_OK", nil)
	if err != nil {
		t.Fatalf("display: %v out=%s", err, out)
	}
	if !strings.Contains(string(out), "DISPLAY_OK") {
		t.Fatalf("display output: %s", out)
	}

	ep, ok := b.DesktopEndpoints("smoke")
	if !ok || ep.CDPURL == "" {
		t.Fatalf("endpoints missing: %+v", ep)
	}
	t.Logf("CDP=%s noVNC=%s", ep.CDPURL, ep.NoVNCURL)

	// Chromium /json/version
	deadline := time.Now().Add(30 * time.Second)
	var versionOK bool
	for time.Now().Before(deadline) {
		resp, err := http.Get(ep.CDPURL + "/json/version")
		if err == nil {
			var body map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if resp.StatusCode == 200 && body["Browser"] != nil {
				versionOK = true
				t.Logf("browser=%v", body["Browser"])
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !versionOK {
		t.Fatal("CDP /json/version not ready")
	}

	shot, err := b.ExecDesktop(ctx, opts, `(import -silent -window root /tmp/s.png || scrot -o /tmp/s.png) && wc -c </tmp/s.png`, nil)
	if err != nil {
		t.Fatalf("screenshot: %v %s", err, shot)
	}
	t.Logf("screenshot bytes line: %s", strings.TrimSpace(string(shot)))
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}
