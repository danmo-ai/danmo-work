//go:build linux

package computer

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
)

// TestLinuxSmoke exercises the real X11 backend when a display and tooling are
// present (e.g. the Cursor Cloud XFCE desktop). It is skipped on headless CI.
func TestLinuxSmoke(t *testing.T) {
	display := strings.TrimSpace(os.Getenv("DISPLAY"))
	if display == "" {
		t.Skip("no DISPLAY set")
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Skip("xdotool not installed")
	}

	m := New(domain.ConfigComputerSection{Enabled: true, Display: display})
	st := m.Status()
	if !st.Available {
		t.Skipf("desktop control unavailable: %s", st.DegradedReason)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Screenshot of the full display must return PNG bytes.
	img, err := m.Screenshot(ctx, "")
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if len(img.Data) == 0 || img.MimeType != "image/png" {
		t.Fatalf("bad screenshot: %d bytes, mime=%s", len(img.Data), img.MimeType)
	}

	// Cursor move + position round-trip.
	if err := m.MouseMove(ctx, 40, 40); err != nil {
		t.Fatalf("mouse move: %v", err)
	}
	x, y, err := m.CursorPosition(ctx)
	if err != nil {
		t.Fatalf("cursor position: %v", err)
	}
	if x < 0 || y < 0 {
		t.Fatalf("unexpected cursor position (%d,%d)", x, y)
	}

	// Window enumeration should not error (may be empty in a bare session).
	if _, err := m.ListWindows(ctx, ""); err != nil {
		t.Fatalf("list windows: %v", err)
	}
}
