package computer

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestShrinkComputerImageDownscales(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	in := domain.ComputerImage{MimeType: "image/png", Data: buf.Bytes(), Width: 1280, Height: 800}
	out := shrinkComputerImage(in, 1024)
	if out.Width != 1024 || out.Height != 640 {
		t.Fatalf("got %dx%d", out.Width, out.Height)
	}
	if out.DisplayWidth != 1280 || out.DisplayHeight != 800 {
		t.Fatalf("display %dx%d", out.DisplayWidth, out.DisplayHeight)
	}
	if out.MimeType != "image/jpeg" {
		t.Fatalf("mime=%s", out.MimeType)
	}
	x, y := mapShotCoord(512, 320, out.Width, out.Height, out.DisplayWidth, out.DisplayHeight)
	if x != 640 || y != 400 {
		t.Fatalf("mapped %d,%d", x, y)
	}
}

func TestShrinkComputerImageNoopWhenSmall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	in := domain.ComputerImage{MimeType: "image/png", Data: buf.Bytes(), Width: 100, Height: 80}
	out := shrinkComputerImage(in, 1024)
	if out.Width != 100 || out.MimeType != "image/png" {
		t.Fatalf("%+v", out)
	}
}

func TestMapShotCoordIdentity(t *testing.T) {
	x, y := mapShotCoord(10, 20, 100, 100, 100, 100)
	if x != 10 || y != 20 {
		t.Fatal(x, y)
	}
}

func TestMapImagePointRetinaDownscaled(t *testing.T) {
	// A 3456×2234 Retina image downscaled for the model represents a 1728×1117
	// logical-point display. The midpoint must remain at the midpoint, not be
	// mapped to physical pixels and sent to a point-based mouse API.
	img := domain.ComputerImage{Width: 1024, Height: 662, DisplayWidth: 1728, DisplayHeight: 1117}
	x, y, err := mapImagePoint(img, 512, 331)
	if err != nil {
		t.Fatal(err)
	}
	if x != 864 || y != 558 {
		t.Fatalf("mapped center to (%d,%d)", x, y)
	}
}

func TestMapImagePointRetinaWithoutDownscale(t *testing.T) {
	img := domain.ComputerImage{Width: 3456, Height: 2234, DisplayWidth: 1728, DisplayHeight: 1117}
	x, y, err := mapImagePoint(img, 1728, 1117)
	if err != nil {
		t.Fatal(err)
	}
	if x != 864 || y != 558 {
		t.Fatalf("mapped center to (%d,%d)", x, y)
	}
}

func TestMapImagePointWindowOriginNotScaled(t *testing.T) {
	// Window lives on a secondary display at a negative logical-point origin.
	// Its Retina pixels are downscaled again for the model.
	img := domain.ComputerImage{Width: 512, Height: 256, DisplayWidth: 1280, DisplayHeight: 640, OriginX: -2000, OriginY: -300}
	x, y, err := mapImagePoint(img, 256, 128)
	if err != nil {
		t.Fatal(err)
	}
	if x != -1360 || y != 20 {
		t.Fatalf("mapped window center to (%d,%d)", x, y)
	}
}

func TestMapImagePointZoomUsesOwnGeometry(t *testing.T) {
	img := domain.ComputerImage{Width: 500, Height: 300, DisplayWidth: 250, DisplayHeight: 150, OriginX: 700, OriginY: 400}
	x, y, err := mapImagePoint(img, 250, 150)
	if err != nil {
		t.Fatal(err)
	}
	if x != 825 || y != 475 {
		t.Fatalf("mapped zoom center to (%d,%d)", x, y)
	}
}

func TestMapImagePointRejectsOutsideScreenshot(t *testing.T) {
	img := domain.ComputerImage{Width: 100, Height: 80, DisplayWidth: 200, DisplayHeight: 160}
	if _, _, err := mapImagePoint(img, -1, 0); err == nil {
		t.Fatal("expected negative coordinate to fail")
	}
	if _, _, err := mapImagePoint(img, 100, 0); err == nil {
		t.Fatal("expected boundary coordinate to fail")
	}
}

func TestManagerScopeMapping(t *testing.T) {
	m := New(domain.ConfigComputerSection{})
	m.shots[scopeFor("s1", "window", nil)] = domain.ComputerImage{Width: 200, Height: 100, DisplayWidth: 200, DisplayHeight: 100, OriginX: 50, OriginY: 60}
	m.shots[scopeFor("s2", "window", nil)] = domain.ComputerImage{Width: 400, Height: 200, DisplayWidth: 100, DisplayHeight: 50, OriginX: -500, OriginY: -100}
	x, y, err := m.mapXY("s2", "window", 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	if x != -450 || y != -75 {
		t.Fatalf("cross-session mapping returned (%d,%d)", x, y)
	}
	if _, _, err := m.mapXY("s1", "other", 10, 10); err == nil {
		t.Fatal("expected missing window scope to fail")
	}
	m.rememberShot(domain.ComputerImage{Scope: scopeFor("s1", "window", &domain.ScreenRect{X: 70, Y: 80, Width: 50, Height: 20}), Width: 100, Height: 40, DisplayWidth: 50, DisplayHeight: 20, OriginX: 70, OriginY: 80})
	x, y, err = m.mapXY("s1", "window", 50, 20)
	if err != nil {
		t.Fatal(err)
	}
	if x != 95 || y != 90 {
		t.Fatalf("zoom scope returned (%d,%d)", x, y)
	}
}

type fakeShotBackend struct{ img domain.ComputerImage }

func (f fakeShotBackend) probe(domain.ConfigComputerSection) domain.ComputerStatus {
	return domain.ComputerStatus{Available: true, Backend: "fake"}
}
func (f fakeShotBackend) listWindows(context.Context, string) ([]domain.WindowInfo, error) {
	return nil, nil
}
func (f fakeShotBackend) focusWindow(context.Context, string) error { return nil }
func (f fakeShotBackend) windowBounds(context.Context, string) (domain.WindowInfo, error) {
	return domain.WindowInfo{}, nil
}
func (f fakeShotBackend) screenshot(context.Context, string, *domain.ScreenRect) (domain.ComputerImage, error) {
	return f.img, nil
}
func (f fakeShotBackend) mouseMove(context.Context, int, int, []domain.KeyModifier) error { return nil }
func (f fakeShotBackend) mouseClick(context.Context, domain.MouseButton, int, int, int, []domain.KeyModifier) error {
	return nil
}
func (f fakeShotBackend) mouseDrag(context.Context, int, int, int, int) error { return nil }
func (f fakeShotBackend) mouseButton(context.Context, domain.MouseButton, bool) error {
	return nil
}
func (f fakeShotBackend) scroll(context.Context, int, int, domain.ScrollDirection, int) error {
	return nil
}
func (f fakeShotBackend) typeText(context.Context, string) error  { return nil }
func (f fakeShotBackend) key(context.Context, string, bool) error { return nil }
func (f fakeShotBackend) cursorPosition(context.Context) (int, int, error) {
	return 0, 0, nil
}

func TestScreenshotDedupIsPerTurnAndComparesPreviousCapture(t *testing.T) {
	m := New(domain.ConfigComputerSection{Enabled: true, MaxShotEdge: -1})
	m.host = fakeShotBackend{img: domain.ComputerImage{MimeType: "image/png", Data: []byte("pixels"), Width: 100, Height: 80}}
	m.backend = m.host
	m.status.Available = true
	ctx := context.Background()

	first, err := m.Screenshot(withTurn(withSession(ctx, "s"), "t1"), "s", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Unchanged {
		t.Fatal("first capture of a turn must never be marked unchanged")
	}
	second, err := m.Screenshot(withTurn(withSession(ctx, "s"), "t1"), "s", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Unchanged {
		t.Fatal("identical capture within the same turn should be unchanged")
	}
	newTurn, err := m.Screenshot(withTurn(withSession(ctx, "s"), "t2"), "s", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if newTurn.Unchanged {
		t.Fatal("first capture of a new turn must deliver image bytes")
	}
	m.host = fakeShotBackend{img: domain.ComputerImage{MimeType: "image/png", Data: []byte("changed"), Width: 100, Height: 80}}
	m.backend = m.host
	changed, err := m.Screenshot(withTurn(withSession(ctx, "s"), "t2"), "s", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Unchanged {
		t.Fatal("changed pixels must not be marked unchanged")
	}
	// Mapping state must survive dedup so follow-up clicks still work.
	if _, _, err := m.mapXY("s", "", 10, 10); err != nil {
		t.Fatalf("mapping after dedup: %v", err)
	}
}

func TestHostMaxShotEdgeIgnoresSandboxCap(t *testing.T) {
	m := New(domain.ConfigComputerSection{Enabled: true})
	m.SetDesktopEnvironment(fakeDesktopEnv{profile: domain.SandboxProfileCLI})
	if got := m.maxShotEdge(); got != domain.DefaultComputerMaxShotEdge {
		t.Fatalf("host edge=%d, want %d", got, domain.DefaultComputerMaxShotEdge)
	}
	m.SetDesktopEnvironment(fakeDesktopEnv{profile: domain.SandboxProfileDesktop})
	if got := m.maxShotEdge(); got != domain.DefaultDesktopMaxShotEdge {
		t.Fatalf("auto+sandbox edge=%d, want %d", got, domain.DefaultDesktopMaxShotEdge)
	}
	m.Configure(domain.ConfigComputerSection{Enabled: true, Target: domain.ComputerTargetHost})
	if got := m.maxShotEdge(); got != domain.DefaultComputerMaxShotEdge {
		t.Fatalf("explicit host edge=%d, want %d", got, domain.DefaultComputerMaxShotEdge)
	}
}

func TestScopeForZoom(t *testing.T) {
	base := scopeFor("s", "w", nil)
	zoom := scopeFor("s", "w", &domain.ScreenRect{X: 10, Y: 20, Width: 30, Height: 40})
	if !strings.HasPrefix(zoom, base+"\x00") {
		t.Fatalf("unexpected scope hierarchy: %q %q", base, zoom)
	}
}
