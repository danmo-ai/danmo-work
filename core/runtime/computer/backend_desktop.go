package computer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"strconv"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// desktopBackend drives X11 inside a sandbox desktop container via ExecDesktop.
type desktopBackend struct {
	mu      sync.Mutex
	desktop port.DesktopEnvironment
	opts    port.SandboxRunOptions
	display string
}

func newDesktopBackend(desktop port.DesktopEnvironment) *desktopBackend {
	return &desktopBackend{
		desktop: desktop,
		display: ":1",
	}
}

// SetSession updates the project/workdir used for container exec.
func (b *desktopBackend) SetSession(opts port.SandboxRunOptions) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.opts = opts
}

func (b *desktopBackend) session() port.SandboxRunOptions {
	b.mu.Lock()
	defer b.mu.Unlock()
	opts := b.opts
	if opts.ProjectID == "" {
		opts.ProjectID = "default"
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	return opts
}

func (b *desktopBackend) run(ctx context.Context, name string, args ...string) (string, error) {
	opts := b.session()
	quoted := make([]string, 0, 1+len(args))
	quoted = append(quoted, shellQuote(name))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	cmd := strings.Join(quoted, " ")
	out, err := b.desktop.ExecDesktop(ctx, opts, cmd, []string{"DISPLAY=" + b.display})
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s failed: %s", name, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (b *desktopBackend) probe(cfg domain.ConfigComputerSection) domain.ComputerStatus {
	st := domain.ComputerStatus{
		Platform: "linux",
		Backend:  "sandbox-desktop",
		Display:  b.display,
	}
	if b.desktop == nil || b.desktop.DesktopProfile() != domain.SandboxProfileDesktop {
		st.DegradedReason = "sandbox desktop profile not active"
		return st
	}
	st.Available = true
	if d := strings.TrimSpace(cfg.Display); d != "" {
		b.display = d
		st.Display = d
	}
	return st
}

func (b *desktopBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	out, err := b.run(ctx, "xdotool", "search", "--onlyvisible", "--name", "")
	if err != nil {
		return nil, nil
	}
	active, _ := b.run(ctx, "xdotool", "getactivewindow")
	var out2 []domain.WindowInfo
	q := strings.ToLower(strings.TrimSpace(query))
	for _, line := range strings.Split(out, "\n") {
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		info, err := b.windowBounds(ctx, id)
		if err != nil || info.Title == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) && !strings.Contains(strings.ToLower(info.App), q) {
			continue
		}
		if id == active {
			info.Active = true
		}
		out2 = append(out2, info)
	}
	return out2, nil
}

func (b *desktopBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	geo, err := b.run(ctx, "xdotool", "getwindowgeometry", "--shell", id)
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
	if name, err := b.run(ctx, "xdotool", "getwindowname", id); err == nil {
		info.Title = name
	}
	if pidStr, err := b.run(ctx, "xdotool", "getwindowpid", id); err == nil {
		info.PID, _ = strconv.Atoi(pidStr)
	}
	if cls, err := b.run(ctx, "xdotool", "getwindowclassname", id); err == nil {
		info.App = cls
	}
	return info, nil
}

func (b *desktopBackend) focusWindow(ctx context.Context, id string) error {
	if _, err := b.run(ctx, "xdotool", "windowactivate", "--sync", id); err != nil {
		if _, err2 := b.run(ctx, "xdotool", "windowraise", id); err2 != nil {
			return err
		}
		_, _ = b.run(ctx, "xdotool", "windowfocus", id)
	}
	return nil
}

func (b *desktopBackend) screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	origin := image.Point{}
	if windowID != "" {
		info, err := b.windowBounds(ctx, windowID)
		if err != nil {
			return domain.ComputerImage{}, err
		}
		origin = image.Point{X: info.X, Y: info.Y}
	}
	path := "/tmp/dq-computer-shot.png"
	var captureCmd string
	if windowID != "" {
		captureCmd = fmt.Sprintf("import -silent -window %s %s", shellQuote(windowID), shellQuote(path))
	} else {
		captureCmd = fmt.Sprintf("(import -silent -window root %s || scrot -o %s)", shellQuote(path), shellQuote(path))
	}
	opts := b.session()
	// Prefer GNU base64 -w0; fall back to tr -d for BusyBox.
	script := captureCmd + " && (base64 -w0 " + shellQuote(path) + " 2>/dev/null || base64 " + shellQuote(path) + " | tr -d '\\n')"
	out, err := b.desktop.ExecDesktop(ctx, opts, script, []string{"DISPLAY=" + b.display})
	if err != nil {
		return domain.ComputerImage{}, err
	}
	return decodeShot(out, origin)
}

func decodeShot(out []byte, origin image.Point) (domain.ComputerImage, error) {
	raw := strings.TrimSpace(string(out))
	// strip command noise: take last whitespace-separated token that looks like base64
	fields := strings.Fields(raw)
	if len(fields) > 0 {
		raw = fields[len(fields)-1]
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return domain.ComputerImage{}, fmt.Errorf("decode screenshot: %w", err)
		}
	}
	cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data))
	w, h := 0, 0
	if decErr == nil {
		w, h = cfg.Width, cfg.Height
	}
	return domain.ComputerImage{
		MimeType: "image/png",
		Data:     data,
		Width:    w,
		Height:   h,
		OriginX:  origin.X,
		OriginY:  origin.Y,
	}, nil
}

func (b *desktopBackend) mouseMove(ctx context.Context, x, y int) error {
	_, err := b.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

func (b *desktopBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	_, err := b.run(ctx, "xdotool", "click", "--repeat", strconv.Itoa(clicks), xdoButton(button))
	return err
}

func (b *desktopBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if _, err := b.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x0), strconv.Itoa(y0)); err != nil {
		return err
	}
	if _, err := b.run(ctx, "xdotool", "mousedown", "1"); err != nil {
		return err
	}
	if _, err := b.run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x1), strconv.Itoa(y1)); err != nil {
		_, _ = b.run(ctx, "xdotool", "mouseup", "1")
		return err
	}
	_, err := b.run(ctx, "xdotool", "mouseup", "1")
	return err
}

func (b *desktopBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	button := "5"
	switch dir {
	case domain.ScrollUp:
		button = "4"
	case domain.ScrollDown:
		button = "5"
	case domain.ScrollLeft:
		button = "6"
	case domain.ScrollRight:
		button = "7"
	}
	_, err := b.run(ctx, "xdotool", "click", "--repeat", strconv.Itoa(amount), button)
	return err
}

func (b *desktopBackend) typeText(ctx context.Context, text string) error {
	_, err := b.run(ctx, "xdotool", "type", "--clearmodifiers", "--", text)
	return err
}

func (b *desktopBackend) key(ctx context.Context, key string) error {
	_, err := b.run(ctx, "xdotool", "key", "--clearmodifiers", key)
	return err
}

func (b *desktopBackend) cursorPosition(ctx context.Context) (int, int, error) {
	out, err := b.run(ctx, "xdotool", "getmouselocation", "--shell")
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
		switch k {
		case "X":
			x = n
		case "Y":
			y = n
		}
	}
	return x, y, nil
}
