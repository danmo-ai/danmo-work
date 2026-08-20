//go:build darwin

package computer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder for image.DecodeConfig
	"os"
	"strconv"
	"strings"

	"danmo-work/core/domain"
)

func newBackend() backend { return &darwinBackend{} }

// darwinBackend drives macOS via osascript (AppleScript / System Events) for
// window discovery, focus, and keyboard, plus screencapture for screenshots.
// Mouse actions require the `cliclick` helper (brew install cliclick) because
// AppleScript cannot move the pointer to absolute coordinates reliably.
//
// Note: macOS gates automation behind TCC. The user must grant the host app
// Accessibility and Screen Recording permission; otherwise actions fail at
// runtime with an OS error surfaced through the returned message.
type darwinBackend struct{}

func (b *darwinBackend) probe(_ domain.ConfigComputerSection) domain.ComputerStatus {
	st := domain.ComputerStatus{Platform: "darwin", Backend: "macos"}
	if !hasBinary("osascript") {
		st.DegradedReason = "osascript not found"
		return st
	}
	st.Available = true
	if !hasBinary("cliclick") {
		st.DegradedReason = "mouse actions need cliclick (brew install cliclick); keyboard/screenshot still work"
	}
	return st
}

// windowScript enumerates visible app windows via System Events. Each line:
// pid\tapp\tx\ty\tw\th\ttitle
const windowScript = `tell application "System Events"
	set out to ""
	repeat with proc in (every process whose visible is true)
		set pid to unix id of proc
		set appName to name of proc
		repeat with w in (every window of proc)
			try
				set p to position of w
				set s to size of w
				set t to name of w
				set out to out & pid & tab & appName & tab & (item 1 of p) & tab & (item 2 of p) & tab & (item 1 of s) & tab & (item 2 of s) & tab & t & linefeed
			end try
		end repeat
	end repeat
	return out
end tell`

func (b *darwinBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	out, err := runCmd(ctx, nil, "osascript", "-e", windowScript)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var wins []domain.WindowInfo
	counts := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}
		app := parts[1]
		idx := counts[app]
		counts[app] = idx + 1
		info := domain.WindowInfo{
			ID:    fmt.Sprintf("%s\x1f%d", app, idx),
			App:   app,
			Title: parts[6],
		}
		info.PID, _ = strconv.Atoi(parts[0])
		info.X, _ = strconv.Atoi(parts[2])
		info.Y, _ = strconv.Atoi(parts[3])
		info.Width, _ = strconv.Atoi(parts[4])
		info.Height, _ = strconv.Atoi(parts[5])
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) && !strings.Contains(strings.ToLower(info.App), q) {
			continue
		}
		wins = append(wins, info)
	}
	return wins, nil
}

func parseDarwinID(id string) (app string, idx int) {
	app, idxStr, ok := strings.Cut(id, "\x1f")
	if !ok {
		return id, 0
	}
	idx, _ = strconv.Atoi(idxStr)
	return app, idx
}

func (b *darwinBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	app, idx := parseDarwinID(id)
	wins, err := b.listWindows(ctx, "")
	if err != nil {
		return domain.WindowInfo{}, err
	}
	seen := 0
	for _, w := range wins {
		if w.App == app {
			if seen == idx {
				return w, nil
			}
			seen++
		}
	}
	return domain.WindowInfo{}, fmt.Errorf("window %q not found", id)
}

func (b *darwinBackend) focusWindow(ctx context.Context, id string) error {
	app, _ := parseDarwinID(id)
	_, err := runCmd(ctx, nil, "osascript", "-e", fmt.Sprintf("tell application %q to activate", app))
	return err
}

func (b *darwinBackend) screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	origin := image.Point{}
	if windowID != "" {
		if info, err := b.windowBounds(ctx, windowID); err == nil {
			origin = image.Point{X: info.X, Y: info.Y}
		}
	}
	tmp, err := os.CreateTemp("", "dq-computer-*.png")
	if err != nil {
		return domain.ComputerImage{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	// -x silences the capture sound; capture the whole display. Window-scoped
	// callers still get a correct origin from bounds above.
	if _, err := runCmd(ctx, nil, "screencapture", "-x", "-o", path); err != nil {
		return domain.ComputerImage{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	w, h := 0, 0
	if cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data)); decErr == nil {
		w, h = cfg.Width, cfg.Height
	}
	return domain.ComputerImage{MimeType: "image/png", Data: data, Width: w, Height: h, OriginX: origin.X, OriginY: origin.Y}, nil
}

func (b *darwinBackend) requireCliclick() error {
	if !hasBinary("cliclick") {
		return fmt.Errorf("mouse control requires cliclick (brew install cliclick)")
	}
	return nil
}

func (b *darwinBackend) mouseMove(ctx context.Context, x, y int) error {
	if err := b.requireCliclick(); err != nil {
		return err
	}
	_, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("m:%d,%d", x, y))
	return err
}

func (b *darwinBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if err := b.requireCliclick(); err != nil {
		return err
	}
	verb := "c"
	if button == domain.MouseRight {
		verb = "rc"
	}
	if x < 0 || y < 0 {
		x, y, _ = b.cursorPosition(ctx)
	}
	for i := 0; i < clicks; i++ {
		if _, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("%s:%d,%d", verb, x, y)); err != nil {
			return err
		}
	}
	return nil
}

func (b *darwinBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if err := b.requireCliclick(); err != nil {
		return err
	}
	_, err := runCmd(ctx, nil, "cliclick", fmt.Sprintf("dd:%d,%d", x0, y0), fmt.Sprintf("du:%d,%d", x1, y1))
	return err
}

func (b *darwinBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	// cliclick has no scroll verb across all versions; emulate with arrow keys
	// is unreliable, so surface a clear limitation instead of guessing.
	return fmt.Errorf("scroll is not supported on macOS backend")
}

func (b *darwinBackend) typeText(ctx context.Context, text string) error {
	script := fmt.Sprintf("tell application \"System Events\" to keystroke %q", text)
	_, err := runCmd(ctx, nil, "osascript", "-e", script)
	return err
}

func (b *darwinBackend) key(ctx context.Context, key string) error {
	script, err := darwinKeyScript(key)
	if err != nil {
		return err
	}
	_, err = runCmd(ctx, nil, "osascript", "-e", script)
	return err
}

// darwinKeyScript maps a chord like "cmd+s" or "Return" to a System Events call.
func darwinKeyScript(key string) (string, error) {
	parts := strings.Split(key, "+")
	base := parts[len(parts)-1]
	var mods []string
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "cmd", "command", "meta", "super":
			mods = append(mods, "command down")
		case "ctrl", "control":
			mods = append(mods, "control down")
		case "alt", "option":
			mods = append(mods, "option down")
		case "shift":
			mods = append(mods, "shift down")
		}
	}
	special := map[string]string{
		"return": "36", "enter": "36", "tab": "48", "space": "49",
		"escape": "53", "esc": "53", "delete": "51", "backspace": "51",
		"up": "126", "down": "125", "left": "123", "right": "124",
	}
	usingClause := ""
	if len(mods) > 0 {
		usingClause = " using {" + strings.Join(mods, ", ") + "}"
	}
	if code, ok := special[strings.ToLower(base)]; ok {
		return fmt.Sprintf("tell application \"System Events\" to key code %s%s", code, usingClause), nil
	}
	if len([]rune(base)) == 1 {
		return fmt.Sprintf("tell application \"System Events\" to keystroke %q%s", base, usingClause), nil
	}
	return "", fmt.Errorf("unsupported key %q on macOS backend", key)
}

func (b *darwinBackend) cursorPosition(ctx context.Context) (int, int, error) {
	if !hasBinary("cliclick") {
		return 0, 0, fmt.Errorf("cursor position requires cliclick (brew install cliclick)")
	}
	out, err := runCmd(ctx, nil, "cliclick", "p")
	if err != nil {
		return 0, 0, err
	}
	// cliclick p prints e.g. "123,456"
	xStr, yStr, ok := strings.Cut(strings.TrimSpace(out), ",")
	if !ok {
		return 0, 0, fmt.Errorf("unexpected cliclick output %q", out)
	}
	x, _ := strconv.Atoi(strings.TrimSpace(xStr))
	y, _ := strconv.Atoi(strings.TrimSpace(yStr))
	return x, y, nil
}
