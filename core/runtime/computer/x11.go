package computer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"danmo-work/core/domain"
)

// x11CommandRunner is the only difference between host and container X11.
type x11CommandRunner interface {
	run(context.Context, string, ...string) (string, error)
}

func joinShellArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func (b *linuxBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	return x11ListWindows(ctx, b, query)
}

func (b *desktopBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	return x11ListWindows(ctx, b, query)
}

func x11ListWindows(ctx context.Context, r x11CommandRunner, query string) ([]domain.WindowInfo, error) {
	out, err := r.run(ctx, "wmctrl", "-lpGx")
	if err == nil {
		wins := parseWmctrl(out, query)
		if active, activeErr := r.run(ctx, "xdotool", "getactivewindow"); activeErr == nil {
			active = strings.TrimSpace(active)
			for i := range wins {
				wins[i].Active = wins[i].ID == active
			}
		}
		return wins, nil
	}
	// Older minimal images may lack wmctrl; retain a functional xdotool fallback.
	out, err = r.run(ctx, "xdotool", "search", "--onlyvisible", "--name", "")
	if err != nil {
		return nil, nil
	}
	active, _ := r.run(ctx, "xdotool", "getactivewindow")
	q := strings.ToLower(strings.TrimSpace(query))
	var wins []domain.WindowInfo
	for _, line := range strings.Split(out, "\n") {
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		info, err := x11WindowBounds(ctx, r, id)
		if err != nil || info.Title == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) && !strings.Contains(strings.ToLower(info.App), q) {
			continue
		}
		info.Active = id == active
		wins = append(wins, info)
	}
	return wins, nil
}

// wmctrl -lpGx: id desktop pid x y width height class title (host may be blank).
func parseWmctrl(out, query string) []domain.WindowInfo {
	q := strings.ToLower(strings.TrimSpace(query))
	var wins []domain.WindowInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 || !strings.HasPrefix(fields[0], "0x") {
			continue
		}
		id := strings.TrimPrefix(fields[0], "0x")
		if n, err := strconv.ParseInt(id, 16, 64); err == nil {
			id = strconv.FormatInt(n, 10)
		}
		info := domain.WindowInfo{ID: id, App: fields[7], Title: strings.Join(fields[8:], " ")}
		info.PID, _ = strconv.Atoi(fields[2])
		info.X, _ = strconv.Atoi(fields[3])
		info.Y, _ = strconv.Atoi(fields[4])
		info.Width, _ = strconv.Atoi(fields[5])
		info.Height, _ = strconv.Atoi(fields[6])
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) && !strings.Contains(strings.ToLower(info.App), q) {
			continue
		}
		wins = append(wins, info)
	}
	return wins
}

func (b *linuxBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	return x11WindowBounds(ctx, b, id)
}

func (b *desktopBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	return x11WindowBounds(ctx, b, id)
}

func x11WindowBounds(ctx context.Context, r x11CommandRunner, id string) (domain.WindowInfo, error) {
	geo, err := r.run(ctx, "xdotool", "getwindowgeometry", "--shell", id)
	if err != nil {
		return domain.WindowInfo{}, err
	}
	info := domain.WindowInfo{ID: id}
	for _, line := range strings.Split(geo, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		switch k {
		case "X":
			info.X = n
		case "Y":
			info.Y = n
		case "WIDTH":
			info.Width = n
		case "HEIGHT":
			info.Height = n
		}
	}
	if name, err := r.run(ctx, "xdotool", "getwindowname", id); err == nil {
		info.Title = name
	}
	if pidStr, err := r.run(ctx, "xdotool", "getwindowpid", id); err == nil {
		info.PID, _ = strconv.Atoi(pidStr)
	}
	if cls, err := r.run(ctx, "xdotool", "getwindowclassname", id); err == nil {
		info.App = cls
	}
	return info, nil
}

func (b *linuxBackend) focusWindow(ctx context.Context, id string) error {
	return x11FocusWindow(ctx, b, id)
}

func (b *desktopBackend) focusWindow(ctx context.Context, id string) error {
	return x11FocusWindow(ctx, b, id)
}

func x11FocusWindow(ctx context.Context, r x11CommandRunner, id string) error {
	if _, err := r.run(ctx, "xdotool", "windowactivate", "--sync", id); err != nil {
		if _, err2 := r.run(ctx, "xdotool", "windowraise", id); err2 != nil {
			return err
		}
		_, _ = r.run(ctx, "xdotool", "windowfocus", id)
	}
	return nil
}

func (b *linuxBackend) mouseMove(ctx context.Context, x, y int, modifiers []domain.KeyModifier) error {
	return x11MouseMove(ctx, b, x, y, modifiers)
}

func (b *desktopBackend) mouseMove(ctx context.Context, x, y int, modifiers []domain.KeyModifier) error {
	return x11MouseMove(ctx, b, x, y, modifiers)
}

func x11MouseMove(ctx context.Context, r x11CommandRunner, x, y int, modifiers []domain.KeyModifier) error {
	if err := x11Modifiers(ctx, r, modifiers, true); err != nil {
		return err
	}
	_, err := r.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	releaseErr := x11Modifiers(ctx, r, modifiers, false)
	if err != nil {
		return err
	}
	return releaseErr
}

func (b *linuxBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	return x11MouseClick(ctx, b, button, x, y, clicks, modifiers)
}

func (b *desktopBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	return x11MouseClick(ctx, b, button, x, y, clicks, modifiers)
}

func x11MouseClick(ctx context.Context, r x11CommandRunner, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	if x >= 0 && y >= 0 {
		if _, err := r.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y)); err != nil {
			return err
		}
	}
	if err := x11Modifiers(ctx, r, modifiers, true); err != nil {
		return err
	}
	_, err := r.run(ctx, "xdotool", "click", "--repeat", strconv.Itoa(clicks), "--delay", "50", xdoButton(button))
	releaseErr := x11Modifiers(ctx, r, modifiers, false)
	if err != nil {
		return err
	}
	return releaseErr
}

func (b *linuxBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	return x11MouseDrag(ctx, b, x0, y0, x1, y1)
}

func (b *desktopBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	return x11MouseDrag(ctx, b, x0, y0, x1, y1)
}

func x11MouseDrag(ctx context.Context, r x11CommandRunner, x0, y0, x1, y1 int) error {
	if _, err := r.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x0), strconv.Itoa(y0)); err != nil {
		return err
	}
	if _, err := r.run(ctx, "xdotool", "mousedown", "1"); err != nil {
		return err
	}
	// Multiple movement steps give toolkits a real drag gesture, not a teleport.
	for step := 1; step <= 8; step++ {
		x := x0 + (x1-x0)*step/8
		y := y0 + (y1-y0)*step/8
		if _, err := r.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y)); err != nil {
			_, _ = r.run(ctx, "xdotool", "mouseup", "1")
			return err
		}
	}
	_, err := r.run(ctx, "xdotool", "mouseup", "1")
	return err
}

func (b *linuxBackend) mouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error {
	return x11MouseButton(ctx, b, button, pressed)
}

func (b *desktopBackend) mouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error {
	return x11MouseButton(ctx, b, button, pressed)
}

func x11MouseButton(ctx context.Context, r x11CommandRunner, button domain.MouseButton, pressed bool) error {
	action := "mouseup"
	if pressed {
		action = "mousedown"
	}
	_, err := r.run(ctx, "xdotool", action, xdoButton(button))
	return err
}

func (b *linuxBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	return x11Scroll(ctx, b, x, y, dir, amount)
}

func (b *desktopBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	return x11Scroll(ctx, b, x, y, dir, amount)
}

func x11Scroll(ctx context.Context, r x11CommandRunner, x, y int, dir domain.ScrollDirection, amount int) error {
	if x >= 0 && y >= 0 {
		if _, err := r.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y)); err != nil {
			return err
		}
	}
	button := "5"
	switch dir {
	case domain.ScrollUp:
		button = "4"
	case domain.ScrollLeft:
		button = "6"
	case domain.ScrollRight:
		button = "7"
	}
	_, err := r.run(ctx, "xdotool", "click", "--repeat", strconv.Itoa(amount), "--delay", "40", button)
	return err
}

func (b *linuxBackend) typeText(ctx context.Context, text string) error {
	return x11TypeText(ctx, b, text)
}

func (b *desktopBackend) typeText(ctx context.Context, text string) error {
	return x11TypeText(ctx, b, text)
}

func x11TypeText(ctx context.Context, r x11CommandRunner, text string) error {
	_, err := r.run(ctx, "xdotool", "type", "--clearmodifiers", "--delay", "12", "--", text)
	return err
}

func (b *linuxBackend) key(ctx context.Context, key string, hold bool) error {
	return x11Key(ctx, b, key, hold)
}

func (b *desktopBackend) key(ctx context.Context, key string, hold bool) error {
	return x11Key(ctx, b, key, hold)
}

func x11Key(ctx context.Context, r x11CommandRunner, key string, hold bool) error {
	if !hold {
		_, err := r.run(ctx, "xdotool", "key", "--clearmodifiers", key)
		return err
	}
	seconds, err := keyHoldSeconds(key)
	if err != nil {
		return err
	}
	chord := strings.TrimSuffix(key, fmt.Sprintf("\x00%d", seconds))
	if _, err := r.run(ctx, "xdotool", "keydown", "--clearmodifiers", chord); err != nil {
		return err
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		_, _ = r.run(context.Background(), "xdotool", "keyup", chord)
		return ctx.Err()
	case <-timer.C:
	}
	_, err = r.run(ctx, "xdotool", "keyup", chord)
	return err
}

func (b *linuxBackend) cursorPosition(ctx context.Context) (int, int, error) {
	return x11CursorPosition(ctx, b)
}

func (b *desktopBackend) cursorPosition(ctx context.Context) (int, int, error) {
	return x11CursorPosition(ctx, b)
}

func x11CursorPosition(ctx context.Context, r x11CommandRunner) (int, int, error) {
	out, err := r.run(ctx, "xdotool", "getmouselocation", "--shell")
	if err != nil {
		return 0, 0, err
	}
	x, y := 0, 0
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		if k == "X" {
			x = n
		} else if k == "Y" {
			y = n
		}
	}
	return x, y, nil
}

func x11Modifiers(ctx context.Context, r x11CommandRunner, modifiers []domain.KeyModifier, down bool) error {
	action := "keyup"
	if down {
		action = "keydown"
	}
	for _, mod := range modifiers {
		var key string
		switch mod {
		case domain.ModifierShift:
			key = "Shift_L"
		case domain.ModifierCtrl:
			key = "Control_L"
		case domain.ModifierAlt:
			key = "Alt_L"
		case domain.ModifierMeta:
			key = "Super_L"
		default:
			return fmt.Errorf("unsupported modifier %q", mod)
		}
		if _, err := r.run(ctx, "xdotool", action, key); err != nil {
			return err
		}
	}
	return nil
}

func captureGeometry(ctx context.Context, r x11CommandRunner, windowID string) (image.Point, int, int, error) {
	if windowID != "" {
		info, err := x11WindowBounds(ctx, r, windowID)
		if err != nil {
			return image.Point{}, 0, 0, err
		}
		if info.Width <= 0 || info.Height <= 0 {
			return image.Point{}, 0, 0, fmt.Errorf("window %q has invalid bounds", windowID)
		}
		return image.Point{X: info.X, Y: info.Y}, info.Width, info.Height, nil
	}
	out, err := r.run(ctx, "xdpyinfo")
	if err != nil {
		return image.Point{}, 0, 0, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "dimensions:") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "dimensions:" && i+1 < len(fields) {
					var w, h int
					if _, err := fmt.Sscanf(fields[i+1], "%dx%d", &w, &h); err == nil {
						return image.Point{}, w, h, nil
					}
				}
			}
		}
	}
	return image.Point{}, 0, 0, fmt.Errorf("could not read X11 display dimensions")
}

func decodeShot(out []byte, origin image.Point) (domain.ComputerImage, error) {
	raw := strings.TrimSpace(string(out))
	// base64 output can be wrapped even with -w0 on minimal coreutils builds.
	var compact strings.Builder
	for _, field := range strings.Fields(raw) {
		if len(field) >= 4 && isBase64Token(field) {
			compact.WriteString(field)
		}
	}
	data, err := base64.StdEncoding.DecodeString(compact.String())
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(compact.String())
		if err != nil {
			return domain.ComputerImage{}, fmt.Errorf("decode screenshot: %w", err)
		}
	}
	cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data))
	if decErr != nil {
		return domain.ComputerImage{}, fmt.Errorf("decode screenshot image: %w", decErr)
	}
	return domain.ComputerImage{MimeType: "image/png", Data: data, Width: cfg.Width, Height: cfg.Height, OriginX: origin.X, OriginY: origin.Y}, nil
}

func isBase64Token(s string) bool {
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			continue
		}
		return false
	}
	return true
}

func keyHoldSeconds(key string) (int, error) {
	idx := strings.LastIndexByte(key, 0)
	if idx < 0 {
		return 0, fmt.Errorf("hold_key duration is missing")
	}
	seconds, err := strconv.Atoi(key[idx+1:])
	if err != nil || seconds < 1 || seconds > 10 {
		return 0, fmt.Errorf("hold_key duration must be 1-10 seconds")
	}
	return seconds, nil
}
