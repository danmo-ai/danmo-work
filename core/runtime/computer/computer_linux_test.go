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

	// MaxShotEdge -1 keeps the raw capture (no JPEG downscale) for smoke asserts.
	m := New(domain.ConfigComputerSection{Enabled: true, Display: display, MaxShotEdge: -1})
	st := m.Status()
	if !st.Available {
		t.Skipf("desktop control unavailable: %s", st.DegradedReason)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Screenshot of the full display must return image bytes.
	img, err := m.Screenshot(ctx, "smoke", "", nil)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if len(img.Data) == 0 || (img.MimeType != "image/png" && img.MimeType != "image/jpeg") {
		t.Fatalf("bad screenshot: %d bytes, mime=%s", len(img.Data), img.MimeType)
	}

	// Cursor move + position round-trip.
	if err := m.MouseMove(ctx, "smoke", "", 40, 40, nil); err != nil {
		t.Fatalf("mouse move: %v", err)
	}
	x, y, err := m.CursorPosition(ctx)
	if err != nil {
		t.Fatalf("cursor position: %v", err)
	}
	if x != 40 || y != 40 {
		t.Fatalf("cursor moved to (%d,%d), want (40,40)", x, y)
	}

	// Verify downscaled coordinates map back to the physical X11 display.
	scaled := New(domain.ConfigComputerSection{Enabled: true, Display: display, MaxShotEdge: 512})
	img, err = scaled.Screenshot(ctx, "scaled-smoke", "", nil)
	if err != nil {
		t.Fatalf("scaled screenshot: %v", err)
	}
	targetX, targetY := img.Width/2, img.Height/2
	if err := scaled.MouseMove(ctx, "scaled-smoke", "", targetX, targetY, nil); err != nil {
		t.Fatalf("scaled mouse move: %v", err)
	}
	x, y, err = scaled.CursorPosition(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantX, wantY := targetX*img.DisplayWidth/img.Width, targetY*img.DisplayHeight/img.Height
	if x != wantX || y != wantY {
		t.Fatalf("scaled cursor moved to (%d,%d), want (%d,%d)", x, y, wantX, wantY)
	}

	wins, err := m.ListWindows(ctx, "")
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	for _, win := range wins {
		if win.Width <= 0 || win.Height <= 0 {
			continue
		}
		windowShot, err := m.Screenshot(ctx, "window-smoke", win.ID, nil)
		if err != nil {
			t.Fatalf("window screenshot: %v", err)
		}
		if windowShot.Width != win.Width || windowShot.Height != win.Height || windowShot.OriginX != win.X || windowShot.OriginY != win.Y {
			t.Fatalf("window capture=%dx%d at %d,%d; bounds=%+v", windowShot.Width, windowShot.Height, windowShot.OriginX, windowShot.OriginY, win)
		}
		break
	}
}
