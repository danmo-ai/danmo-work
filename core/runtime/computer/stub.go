package computer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"danmo-work/core/domain"
)

// stubBackend is used on unsupported platforms or when no backend applies. It
// reports an unavailable status and fails every action with a clear message.
type stubBackend struct {
	reason string
}

func (b stubBackend) probe(_ domain.ConfigComputerSection) domain.ComputerStatus {
	return domain.ComputerStatus{Available: false, Backend: "none", DegradedReason: b.reason}
}

func (b stubBackend) err() error { return fmt.Errorf("desktop control unsupported: %s", b.reason) }

func (b stubBackend) listWindows(context.Context, string) ([]domain.WindowInfo, error) {
	return nil, b.err()
}
func (b stubBackend) focusWindow(context.Context, string) error { return b.err() }
func (b stubBackend) windowBounds(context.Context, string) (domain.WindowInfo, error) {
	return domain.WindowInfo{}, b.err()
}
func (b stubBackend) screenshot(context.Context, string) (domain.ComputerImage, error) {
	return domain.ComputerImage{}, b.err()
}
func (b stubBackend) mouseMove(context.Context, int, int) error { return b.err() }
func (b stubBackend) mouseClick(context.Context, domain.MouseButton, int, int, int) error {
	return b.err()
}
func (b stubBackend) mouseDrag(context.Context, int, int, int, int) error { return b.err() }
func (b stubBackend) scroll(context.Context, int, int, domain.ScrollDirection, int) error {
	return b.err()
}
func (b stubBackend) typeText(context.Context, string) error { return b.err() }
func (b stubBackend) key(context.Context, string) error      { return b.err() }
func (b stubBackend) cursorPosition(context.Context) (int, int, error) {
	return 0, 0, b.err()
}

// runCmd runs a command with optional extra environment and returns trimmed
// stdout. It is shared by the OS backends.
func runCmd(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s failed: %s", name, msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// hasBinary reports whether an executable is on PATH.
func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
