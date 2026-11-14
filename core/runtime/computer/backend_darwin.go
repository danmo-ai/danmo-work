//go:build darwin

package computer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"danmo-work/core/domain"
)

func newBackend() backend { return &darwinBackend{} }

// darwinBackend uses a build-time CoreGraphics helper for native input and
// stable window/display geometry. osascript is only used for accessibility
// focus and key codes; screencapture uses the same logical-point space.
type darwinBackend struct{}

func (b *darwinBackend) native(ctx context.Context, args ...string) (string, error) {
	return runCmd(ctx, nil, darwinInputPath(), args...)
}

func darwinInputPath() string {
	if override := strings.TrimSpace(os.Getenv("DQ_DARWIN_INPUT")); override != "" {
		return override
	}
	if exe, err := os.Executable(); err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(exe), "dq-darwin-input"),
			filepath.Join(filepath.Dir(exe), "..", "Resources", "darwin-input", "dq-darwin-input"),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".danmo-work", "bin", "dq-darwin-input")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "dq-darwin-input"
}

func (b *darwinBackend) probe(_ domain.ConfigComputerSection) domain.ComputerStatus {
	st := domain.ComputerStatus{Platform: "darwin", Backend: "macos"}
	if !hasBinary("screencapture") || !hasBinary("osascript") {
		st.DegradedReason = "macOS screencapture and osascript are required"
		return st
	}
	path := darwinInputPath()
	if _, err := os.Stat(path); err != nil && !hasBinary(path) {
		st.DegradedReason = "native input helper is missing; install cliclick for limited mouse fallback"
		return st
	}
	out, err := b.native(context.Background(), "probe")
	if err != nil {
		st.DegradedReason = "native input helper failed: " + err.Error()
		return st
	}
	var screen, accessibility int
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "screen=%d accessibility=%d", &screen, &accessibility); err != nil {
		st.DegradedReason = "unexpected native input probe output"
		return st
	}
	if screen != 1 || accessibility != 1 {
		var missing []string
		if screen != 1 {
			missing = append(missing, "Screen Recording")
		}
		if accessibility != 1 {
			missing = append(missing, "Accessibility")
		}
		st.DegradedReason = "grant " + strings.Join(missing, " and ") + " permission to the Danmo Work process in System Settings → Privacy & Security"
		return st
	}
	st.Available = true
	return st
}

func (b *darwinBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	out, err := b.native(ctx, "windows")
	if err != nil {
		return nil, err
	}
	return parseDarwinWindows(out, query), nil
}

// native output: windowID pid app x y width height title (logical points).
func parseDarwinWindows(out, query string) []domain.WindowInfo {
	q := strings.ToLower(strings.TrimSpace(query))
	var wins []domain.WindowInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 8)
		if len(parts) != 8 {
			continue
		}
		info := domain.WindowInfo{ID: parts[0], App: parts[2], Title: parts[7]}
		info.PID, _ = strconv.Atoi(parts[1])
		info.X, _ = strconv.Atoi(parts[3])
		info.Y, _ = strconv.Atoi(parts[4])
		info.Width, _ = strconv.Atoi(parts[5])
		info.Height, _ = strconv.Atoi(parts[6])
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) && !strings.Contains(strings.ToLower(info.App), q) {
			continue
		}
		wins = append(wins, info)
	}
	return wins
}

func (b *darwinBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	wins, err := b.listWindows(ctx, "")
	if err != nil {
		return domain.WindowInfo{}, err
	}
	for _, w := range wins {
		if w.ID == id {
			return w, nil
		}
	}
	return domain.WindowInfo{}, fmt.Errorf("window %q not found", id)
}

func (b *darwinBackend) focusWindow(ctx context.Context, id string) error {
	info, err := b.windowBounds(ctx, id)
	if err != nil {
		return err
	}
	// CGWindowList order is front-to-back; index selects the exact window even
	// when an application owns several windows with identical titles.
	wins, err := b.listWindows(ctx, "")
	if err != nil {
		return err
	}
	index := 0
	for _, w := range wins {
		if w.PID == info.PID {
			index++
			if w.ID == id {
				break
			}
		}
	}
	script := fmt.Sprintf(`tell application "System Events" to tell process (name of (first process whose unix id is %d))
	set frontmost to true
	try
		perform action "AXRaise" of window %d
	end try
end tell`, info.PID, index)
	_, err = runCmd(ctx, nil, "osascript", "-e", script)
	return err
}

type darwinDisplay struct {
	id, x, y, width, height int
}

type darwinCapture struct {
	display darwinDisplay
	img     domain.ComputerImage
	data    image.Image
}

func (b *darwinBackend) displays(ctx context.Context) ([]darwinDisplay, error) {
	out, err := b.native(ctx, "displays")
	if err != nil {
		return nil, err
	}
	var displays []darwinDisplay
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 5 {
			continue
		}
		var d darwinDisplay
		d.id, _ = strconv.Atoi(parts[0])
		d.x, _ = strconv.Atoi(parts[1])
		d.y, _ = strconv.Atoi(parts[2])
		d.width, _ = strconv.Atoi(parts[3])
		d.height, _ = strconv.Atoi(parts[4])
		displays = append(displays, d)
	}
	if len(displays) == 0 {
		return nil, fmt.Errorf("no active macOS displays")
	}
	return displays, nil
}

func (b *darwinBackend) captureRegion(ctx context.Context, windowID string) (darwinDisplay, error) {
	if windowID != "" {
		info, err := b.windowBounds(ctx, windowID)
		if err != nil {
			return darwinDisplay{}, err
		}
		return darwinDisplay{x: info.X, y: info.Y, width: info.Width, height: info.Height}, nil
	}
	displays, err := b.displays(ctx)
	if err != nil {
		return darwinDisplay{}, err
	}
	left, top, right, bottom := displays[0].x, displays[0].y, displays[0].x+displays[0].width, displays[0].y+displays[0].height
	for _, d := range displays[1:] {
		left = min(left, d.x)
		top = min(top, d.y)
		right = max(right, d.x+d.width)
		bottom = max(bottom, d.y+d.height)
	}
	return darwinDisplay{x: left, y: top, width: right - left, height: bottom - top}, nil
}

func (b *darwinBackend) screenshot(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	if windowID == "" && zoom == nil {
		return b.screenshotDisplays(ctx)
	}
	return b.screenshotRegion(ctx, windowID, zoom)
}

func (b *darwinBackend) screenshotDisplays(ctx context.Context) (domain.ComputerImage, error) {
	displays, err := b.displays(ctx)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	if len(displays) == 1 {
		return b.screenshotRegion(ctx, "", nil)
	}
	// Capture each display separately so non-contiguous arrangements do not
	// create a misleading black rectangle or crop another monitor incorrectly.
	captures := make([]darwinCapture, len(displays))
	for i, d := range displays {
		img, err := b.screenshotRegion(ctx, "", &domain.ScreenRect{X: d.x, Y: d.y, Width: d.width, Height: d.height})
		if err != nil {
			return domain.ComputerImage{}, err
		}
		decoded, err := decodeRaster(img.Data)
		if err != nil {
			return domain.ComputerImage{}, err
		}
		captures[i] = darwinCapture{display: d, img: img, data: decoded}
	}
	canvas, outputW, outputH, screenW, screenH, originX, originY := stitchDisplays(captures)
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return domain.ComputerImage{}, err
	}
	return domain.ComputerImage{MimeType: "image/png", Data: buf.Bytes(), Width: outputW, Height: outputH, OriginX: originX, OriginY: originY, DisplayWidth: screenW, DisplayHeight: screenH}, nil
}

// stitchDisplays places each physical-pixel capture according to its logical
// point origin. The resulting image can be mapped linearly to screen points,
// including negative coordinates and vertically offset displays.
func stitchDisplays(captures []darwinCapture) (*image.RGBA, int, int, int, int, int, int) {
	left, top := captures[0].display.x, captures[0].display.y
	right, bottom := left+captures[0].display.width, top+captures[0].display.height
	for _, c := range captures {
		left = min(left, c.display.x)
		top = min(top, c.display.y)
		right = max(right, c.display.x+c.display.width)
		bottom = max(bottom, c.display.y+c.display.height)
	}
	screenW, screenH := right-left, bottom-top
	scale := captures[0].img.DisplayWidth
	pixelsW, pixelsH := 0, 0
	if scale > 0 {
		pixelsW = captures[0].img.Width * screenW / scale
	}
	if captures[0].img.DisplayHeight > 0 {
		pixelsH = captures[0].img.Height * screenH / captures[0].img.DisplayHeight
	}
	canvas := image.NewRGBA(image.Rect(0, 0, pixelsW, pixelsH))
	for _, c := range captures {
		x := (c.display.x - left) * pixelsW / screenW
		y := (c.display.y - top) * pixelsH / screenH
		draw.Draw(canvas, image.Rect(x, y, x+c.img.Width, y+c.img.Height), c.data, c.data.Bounds().Min, draw.Src)
	}
	return canvas, pixelsW, pixelsH, screenW, screenH, left, top
}

func (b *darwinBackend) screenshotRegion(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	region, err := b.captureRegion(ctx, windowID)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	if zoom != nil {
		if zoom.X < region.x || zoom.Y < region.y || zoom.X+zoom.Width > region.x+region.width || zoom.Y+zoom.Height > region.y+region.height {
			return domain.ComputerImage{}, fmt.Errorf("zoom rectangle is outside the selected screen region")
		}
		region.x, region.y, region.width, region.height = zoom.X, zoom.Y, zoom.Width, zoom.Height
	}
	if region.width <= 0 || region.height <= 0 {
		return domain.ComputerImage{}, fmt.Errorf("invalid capture region")
	}
	tmp, err := os.CreateTemp("", "dq-computer-*.png")
	if err != nil {
		return domain.ComputerImage{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)
	rect := fmt.Sprintf("%d,%d,%d,%d", region.x, region.y, region.width, region.height)
	if _, err := runCmd(ctx, nil, "screencapture", "-x", "-o", "-R", rect, path); err != nil {
		return domain.ComputerImage{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return domain.ComputerImage{}, fmt.Errorf("decode screenshot: %w", err)
	}
	return domain.ComputerImage{
		MimeType: "image/png", Data: data, Width: cfg.Width, Height: cfg.Height,
		OriginX: region.x, OriginY: region.y, DisplayWidth: region.width, DisplayHeight: region.height,
	}, nil
}

func (b *darwinBackend) mouseMove(ctx context.Context, x, y int, modifiers []domain.KeyModifier) error {
	if b.helperAvailable() {
		_, err := b.native(ctx, "move", strconv.Itoa(x), strconv.Itoa(y), joinModifiers(modifiers))
		return err
	}
	_, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("m:%d,%d", x, y))
	return err
}

func (b *darwinBackend) helperAvailable() bool {
	path := darwinInputPath()
	_, err := os.Stat(path)
	return err == nil || hasBinary(path)
}

func (b *darwinBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	if x < 0 || y < 0 {
		var err error
		x, y, err = b.cursorPosition(ctx)
		if err != nil {
			return err
		}
	}
	if b.helperAvailable() {
		_, err := b.native(ctx, "click", string(button), strconv.Itoa(x), strconv.Itoa(y), strconv.Itoa(clicks), joinModifiers(modifiers))
		return err
	}
	if button == domain.MouseMiddle {
		return fmt.Errorf("middle click requires the bundled native input helper")
	}
	if len(modifiers) > 0 {
		return fmt.Errorf("modified clicks require the bundled native input helper")
	}
	verb := "c"
	if button == domain.MouseRight {
		verb = "rc"
	} else if clicks == 2 {
		verb = "dc"
	} else if clicks > 2 {
		return fmt.Errorf("triple click requires the bundled native input helper")
	}
	_, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("%s:%d,%d", verb, x, y))
	return err
}

func (b *darwinBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if b.helperAvailable() {
		_, err := b.native(ctx, "drag", strconv.Itoa(x0), strconv.Itoa(y0), strconv.Itoa(x1), strconv.Itoa(y1))
		return err
	}
	_, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("dd:%d,%d", x0, y0), fmt.Sprintf("dm:%d,%d", (x0+x1)/2, (y0+y1)/2), fmt.Sprintf("dm:%d,%d", x1, y1), fmt.Sprintf("du:%d,%d", x1, y1))
	return err
}

func (b *darwinBackend) mouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error {
	if button != domain.MouseLeft {
		return fmt.Errorf("only left mouse button holds are supported")
	}
	if !b.helperAvailable() {
		return fmt.Errorf("mouse button holds require the bundled native input helper")
	}
	x, y, err := b.cursorPosition(ctx)
	if err != nil {
		return err
	}
	action := "up"
	if pressed {
		action = "down"
	}
	_, err = b.native(ctx, "button", action, strconv.Itoa(x), strconv.Itoa(y))
	return err
}

func (b *darwinBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if x < 0 || y < 0 {
		var err error
		x, y, err = b.cursorPosition(ctx)
		if err != nil {
			return err
		}
	}
	if !b.helperAvailable() {
		return fmt.Errorf("scroll requires the bundled native input helper")
	}
	_, err := b.native(ctx, "scroll", strconv.Itoa(x), strconv.Itoa(y), string(dir), strconv.Itoa(amount))
	return err
}

func (b *darwinBackend) typeText(ctx context.Context, text string) error {
	if b.helperAvailable() {
		_, err := b.native(ctx, "type", text)
		return err
	}
	_, err := runCmd(ctx, nil, "osascript", "-e", "on run argv\ntell application \"System Events\" to keystroke (item 1 of argv)\nend run", text)
	return err
}

func darwinKeyBase(key string) string {
	base, _, _ := splitKeyChord(key)
	return base
}

func (b *darwinBackend) key(ctx context.Context, key string, hold bool) error {
	if b.helperAvailable() {
		if hold {
			seconds, err := keyHoldSeconds(key)
			if err != nil {
				return err
			}
			chord := strings.TrimSuffix(key, fmt.Sprintf("\x00%d", seconds))
			_, err = b.native(ctx, "key", darwinNativeKey(chord), strconv.Itoa(seconds))
			return err
		}
		_, err := b.native(ctx, "key", darwinNativeKey(key))
		return err
	}
	if hold {
		seconds, err := keyHoldSeconds(key)
		if err != nil {
			return err
		}
		chord := strings.TrimSuffix(key, fmt.Sprintf("\x00%d", seconds))
		script, err := darwinKeyScript(chord)
		if err != nil {
			return err
		}
		if _, err := runCmd(ctx, nil, "osascript", "-e", script, darwinKeyBase(chord)); err != nil {
			return err
		}
		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
		_, err = runCmd(ctx, nil, "osascript", "-e", script, darwinKeyBase(chord))
		return err
	}
	script, err := darwinKeyScript(key)
	if err != nil {
		return err
	}
	base, _, _ := splitKeyChord(key)
	_, err = runCmd(ctx, nil, "osascript", "-e", script, base)
	return err
}

func darwinNativeKey(key string) string {
	base, mods, _ := splitKeyChord(key)
	var names []string
	for _, mod := range mods {
		switch mod {
		case "command down":
			names = append(names, "meta")
		case "control down":
			names = append(names, "ctrl")
		case "option down":
			names = append(names, "alt")
		case "shift down":
			names = append(names, "shift")
		}
	}
	return base + "\t" + strings.Join(names, ",")
}

func splitKeyChord(key string) (base string, modifiers []string, err error) {
	var parts []string
	if strings.HasSuffix(key, "++") {
		parts = strings.Split(strings.TrimSuffix(key, "++"), "+")
		parts = append(parts, "+")
	} else {
		parts = strings.Split(key, "+")
	}
	base = parts[len(parts)-1]
	if base == "" {
		return "", nil, fmt.Errorf("missing key in chord %q", key)
	}
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "cmd", "command", "meta", "super":
			modifiers = append(modifiers, "command down")
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "alt", "option":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		default:
			return "", nil, fmt.Errorf("unsupported modifier %q", m)
		}
	}
	return base, modifiers, nil
}

// darwinKeyScript maps a chord like "cmd+s" or "Return" to a System Events call.
func darwinKeyScript(key string) (string, error) {
	base, mods, err := splitKeyChord(key)
	if err != nil {
		return "", err
	}
	special := map[string]string{
		"return": "36", "enter": "36", "tab": "48", "space": "49", "escape": "53", "esc": "53",
		"delete": "51", "backspace": "51", "up": "126", "down": "125", "left": "123", "right": "124",
		"home": "115", "end": "119", "pageup": "116", "pagedown": "121",
		"f1": "122", "f2": "120", "f3": "99", "f4": "118", "f5": "96", "f6": "97", "f7": "98",
		"f8": "100", "f9": "101", "f10": "109", "f11": "103", "f12": "111",
	}
	usingClause := ""
	if len(mods) > 0 {
		usingClause = " using {" + strings.Join(mods, ", ") + "}"
	}
	if code, ok := special[strings.ToLower(base)]; ok {
		return fmt.Sprintf("tell application \"System Events\" to key code %s%s", code, usingClause), nil
	}
	if len([]rune(base)) == 1 {
		return fmt.Sprintf("on run argv\ntell application \"System Events\" to keystroke (item 1 of argv)%s\nend run", usingClause), nil
	}
	return "", fmt.Errorf("unsupported key %q on macOS backend", key)
}

func (b *darwinBackend) cursorPosition(ctx context.Context) (int, int, error) {
	var out string
	var err error
	if b.helperAvailable() {
		out, err = b.native(ctx, "cursor")
	} else {
		out, err = runCmd(ctx, nil, "cliclick", "p")
	}
	if err != nil {
		return 0, 0, err
	}
	xStr, yStr, ok := strings.Cut(strings.TrimSpace(out), ",")
	if !ok {
		return 0, 0, fmt.Errorf("unexpected cursor output %q", out)
	}
	x, err := strconv.Atoi(strings.TrimSpace(xStr))
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(strings.TrimSpace(yStr))
	return x, y, err
}

func joinModifiers(modifiers []domain.KeyModifier) string {
	parts := make([]string, len(modifiers))
	for i, mod := range modifiers {
		parts[i] = string(mod)
	}
	return strings.Join(parts, ",")
}
