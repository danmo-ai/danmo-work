package builtin

import (
	"context"
	"testing"

	"danmo-work/core/domain"
)

// fakeComputer records calls and returns canned data for handler tests.
type fakeComputer struct {
	windows   []domain.WindowInfo
	bounds    domain.WindowInfo
	img       domain.ComputerImage
	lastClick struct {
		button domain.MouseButton
		x, y   int
		clicks int
	}
	lastMove   [2]int
	lastDrag   [4]int
	lastType   string
	lastKey    string
	lastHold   bool
	lastButton bool
	lastMods   []domain.KeyModifier
	lastZoom   *domain.ScreenRect
	lastScroll struct {
		x, y   int
		dir    domain.ScrollDirection
		amount int
	}
	focused string
}

func (f *fakeComputer) Status() domain.ComputerStatus          { return domain.ComputerStatus{Available: true} }
func (f *fakeComputer) Configure(domain.ConfigComputerSection) {}
func (f *fakeComputer) ListWindows(_ context.Context, _ string) ([]domain.WindowInfo, error) {
	return f.windows, nil
}
func (f *fakeComputer) FocusWindow(_ context.Context, id string) error {
	f.focused = id
	return nil
}
func (f *fakeComputer) WindowBounds(_ context.Context, _ string) (domain.WindowInfo, error) {
	return f.bounds, nil
}
func (f *fakeComputer) Screenshot(_ context.Context, _, _ string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	f.lastZoom = zoom
	if f.img.DisplayWidth == 0 {
		f.img.DisplayWidth = f.img.Width
		f.img.DisplayHeight = f.img.Height
	}
	return f.img, nil
}
func (f *fakeComputer) MouseMove(_ context.Context, _, _ string, x, y int, _ []domain.KeyModifier) error {
	f.lastMove = [2]int{x, y}
	return nil
}
func (f *fakeComputer) MouseClick(_ context.Context, _, _ string, b domain.MouseButton, x, y, clicks int, mods []domain.KeyModifier) error {
	f.lastMods = mods
	f.lastClick.button, f.lastClick.x, f.lastClick.y, f.lastClick.clicks = b, x, y, clicks
	return nil
}
func (f *fakeComputer) MouseDrag(_ context.Context, _, _ string, x0, y0, x1, y1 int) error {
	f.lastDrag = [4]int{x0, y0, x1, y1}
	return nil
}
func (f *fakeComputer) MouseButton(_ context.Context, _ domain.MouseButton, pressed bool) error {
	f.lastButton = pressed
	return nil
}
func (f *fakeComputer) Scroll(_ context.Context, _, _ string, x, y int, dir domain.ScrollDirection, amount int) error {
	f.lastScroll.x, f.lastScroll.y, f.lastScroll.dir, f.lastScroll.amount = x, y, dir, amount
	return nil
}
func (f *fakeComputer) TypeText(_ context.Context, text string) error { f.lastType = text; return nil }
func (f *fakeComputer) Key(_ context.Context, key string, hold bool) error {
	f.lastKey = key
	f.lastHold = hold
	return nil
}
func (f *fakeComputer) CursorPosition(_ context.Context) (int, int, error) {
	return 7, 9, nil
}

func TestComputerListWindows(t *testing.T) {
	f := &fakeComputer{windows: []domain.WindowInfo{{ID: "1", Title: "Mousepad", App: "mousepad", X: 10, Y: 20, Width: 100, Height: 50}}}
	h := &Computer{Ctl: f}
	res, err := h.Execute(context.Background(), map[string]any{"action": "list_windows"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Meta["count"].(int) != 1 {
		t.Fatalf("count=%v", res.Meta["count"])
	}
}

func TestComputerScreenshotReturnsImagePart(t *testing.T) {
	f := &fakeComputer{img: domain.ComputerImage{MimeType: "image/png", Data: []byte{1, 2, 3}, Width: 800, Height: 600}}
	h := &Computer{Ctl: f}
	res, err := h.Execute(context.Background(), map[string]any{"action": "screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parts) != 1 || res.Parts[0].Type != "image" || res.Parts[0].MimeType != "image/png" {
		t.Fatalf("parts=%+v", res.Parts)
	}
	if res.Meta["coordinate_space"] != "absolute" {
		t.Fatalf("space=%v", res.Meta["coordinate_space"])
	}
}

func TestComputerScreenshotNonVisionRejectedBeforeCapture(t *testing.T) {
	f := &fakeComputer{img: domain.ComputerImage{MimeType: "image/png", Data: []byte{1, 2, 3}}}
	h := &Computer{Ctl: f, SupportsImage: func(string) bool { return false }}
	_, err := h.Execute(context.Background(), map[string]any{"action": "screenshot", "__model_id": "text-only"})
	if err == nil {
		t.Fatal("expected non-vision model to be rejected before capture")
	}
}

func TestComputerClickUsesScreenshotScope(t *testing.T) {
	f := &fakeComputer{bounds: domain.WindowInfo{ID: "w1", X: 100, Y: 200}}
	h := &Computer{Ctl: f}
	_, err := h.Execute(context.Background(), map[string]any{
		"action": "left_click", "window_id": "w1", "coordinate": []any{float64(5), float64(6)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastClick.x != 5 || f.lastClick.y != 6 {
		t.Fatalf("expected unshifted image coordinates (5,6), got (%d,%d)", f.lastClick.x, f.lastClick.y)
	}
	if f.lastClick.button != domain.MouseLeft || f.lastClick.clicks != 1 {
		t.Fatalf("click=%+v", f.lastClick)
	}
}

func TestComputerDoubleClick(t *testing.T) {
	f := &fakeComputer{}
	h := &Computer{Ctl: f}
	_, err := h.Execute(context.Background(), map[string]any{
		"action":     "double_click",
		"coordinate": []any{float64(3), float64(4)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastClick.clicks != 2 || f.lastClick.x != 3 || f.lastClick.y != 4 {
		t.Fatalf("click=%+v", f.lastClick)
	}
}

func TestComputerDrag(t *testing.T) {
	f := &fakeComputer{}
	h := &Computer{Ctl: f}
	_, err := h.Execute(context.Background(), map[string]any{
		"action":           "left_click_drag",
		"start_coordinate": []any{float64(1), float64(2)},
		"coordinate":       []any{float64(3), float64(4)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastDrag != [4]int{1, 2, 3, 4} {
		t.Fatalf("drag=%v", f.lastDrag)
	}
}

func TestComputerScroll(t *testing.T) {
	f := &fakeComputer{}
	h := &Computer{Ctl: f}
	_, err := h.Execute(context.Background(), map[string]any{
		"action":           "scroll",
		"scroll_direction": "down",
		"scroll_amount":    float64(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastScroll.dir != domain.ScrollDown || f.lastScroll.amount != 5 {
		t.Fatalf("scroll=%+v", f.lastScroll)
	}
}

func TestComputerTypeAndKey(t *testing.T) {
	f := &fakeComputer{}
	h := &Computer{Ctl: f}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "type", "text": "hello"}); err != nil {
		t.Fatal(err)
	}
	if f.lastType != "hello" {
		t.Fatalf("type=%q", f.lastType)
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "key", "text": "ctrl+s"}); err != nil {
		t.Fatal(err)
	}
	if f.lastKey != "ctrl+s" {
		t.Fatalf("key=%q", f.lastKey)
	}
}

func TestComputerFocus(t *testing.T) {
	f := &fakeComputer{}
	h := &Computer{Ctl: f}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "focus_window", "window_id": "42"}); err != nil {
		t.Fatal(err)
	}
	if f.focused != "42" {
		t.Fatalf("focused=%q", f.focused)
	}
}

func TestComputerErrors(t *testing.T) {
	h := &Computer{Ctl: &fakeComputer{}}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "mouse_move"}); err == nil {
		t.Fatal("expected error for missing coordinate")
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "focus_window"}); err == nil {
		t.Fatal("expected error for missing window_id")
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "bogus"}); err == nil {
		t.Fatal("expected error for unknown action")
	}
	if _, err := (&Computer{}).Execute(context.Background(), map[string]any{"action": "screenshot"}); err == nil {
		t.Fatal("expected error when controller unset")
	}
}

func TestComputerExtendedActions(t *testing.T) {
	f := &fakeComputer{img: domain.ComputerImage{MimeType: "image/png", Data: []byte{1, 2, 3}, Width: 800, Height: 600}}
	h := &Computer{Ctl: f}
	if _, err := h.Execute(context.Background(), map[string]any{
		"action": "triple_click", "coordinate": []any{3.0, 4.0}, "modifiers": []any{"shift", "ctrl"},
	}); err != nil {
		t.Fatal(err)
	}
	if f.lastClick.clicks != 3 || len(f.lastMods) != 2 {
		t.Fatalf("triple/modifiers=%+v %+v", f.lastClick, f.lastMods)
	}
	for action, want := range map[string]bool{"left_mouse_down": true, "left_mouse_up": false} {
		if _, err := h.Execute(context.Background(), map[string]any{"action": action}); err != nil {
			t.Fatal(err)
		}
		if f.lastButton != want {
			t.Fatalf("%s: pressed=%v", action, f.lastButton)
		}
	}
	if _, err := h.Execute(context.Background(), map[string]any{
		"action": "zoom", "zoom": map[string]any{"x": 10.0, "y": 20.0, "width": 100.0, "height": 50.0},
	}); err != nil {
		t.Fatal(err)
	}
	if f.lastZoom == nil || f.lastZoom.X != 10 || f.lastZoom.Height != 50 {
		t.Fatalf("zoom=%+v", f.lastZoom)
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "hold_key", "text": "ctrl+s", "duration": 2.0}); err != nil {
		t.Fatal(err)
	}
	if !f.lastHold || f.lastKey != "ctrl+s\x002" {
		t.Fatalf("hold=%v key=%q", f.lastHold, f.lastKey)
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "left_click", "modifiers": []any{"bogus"}}); err == nil {
		t.Fatal("expected invalid modifier error")
	}
	if _, err := h.Execute(context.Background(), map[string]any{"action": "zoom", "zoom": map[string]any{"width": 0.0}}); err == nil {
		t.Fatal("expected invalid zoom error")
	}
}

func TestComputerRiskHigh(t *testing.T) {
	h := &Computer{Ctl: &fakeComputer{}}
	if h.RiskLevel() != domain.RiskHigh {
		t.Fatalf("risk=%v", h.RiskLevel())
	}
	if h.Name() != "computer" {
		t.Fatalf("name=%q", h.Name())
	}
}
