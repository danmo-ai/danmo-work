//go:build linux

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

func newBackend() backend { return &linuxBackend{} }

// linuxBackend drives X11 via xdotool for input/window ops and one of several
// CLI capture tools (ImageMagick import, scrot, ffmpeg, xwd) for screenshots.
type linuxBackend struct {
	display string
}

func (b *linuxBackend) env() []string {
	env := os.Environ()
	if b.display != "" {
		env = append(env, "DISPLAY="+b.display)
	}
	return env
}

func (b *linuxBackend) probe(cfg domain.ConfigComputerSection) domain.ComputerStatus {
	st := domain.ComputerStatus{Platform: "linux"}
	display := cfg.Display
	if display == "" {
		display = strings.TrimSpace(os.Getenv("DISPLAY"))
	}
	b.display = display
	st.Display = display

	if wl := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")); wl != "" && display == "" {
		st.Backend = "wayland"
		st.DegradedReason = "Wayland session without X11 DISPLAY; xdotool-based control is not supported"
		return st
	}
	if display == "" {
		st.Backend = "none"
		st.DegradedReason = "no X11 DISPLAY set"
		return st
	}
	if !hasBinary("xdotool") {
		st.Backend = "x11"
		st.DegradedReason = "xdotool not installed (apt-get install xdotool)"
		return st
	}
	if b.captureTool() == "" {
		st.Backend = "x11"
		st.DegradedReason = "no screenshot tool found (install imagemagick, scrot, or ffmpeg)"
		return st
	}
	st.Backend = "x11"
	st.Available = true
	return st
}

// captureTool picks the first available screenshot CLI.
func (b *linuxBackend) captureTool() string {
	switch {
	case hasBinary("import"): // ImageMagick
		return "import"
	case hasBinary("scrot"):
		return "scrot"
	case hasBinary("ffmpeg"):
		return "ffmpeg"
	case hasBinary("xwd") && hasBinary("convert"):
		return "xwd"
	default:
		return ""
	}
}

func (b *linuxBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	out, err := runCmd(ctx, b.env(), "xdotool", "search", "--onlyvisible", "--name", "")
	if err != nil {
		// xdotool exits non-zero when nothing matches; treat as empty.
		return nil, nil
	}
	active, _ := runCmd(ctx, b.env(), "xdotool", "getactivewindow")
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

func (b *linuxBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	geo, err := runCmd(ctx, b.env(), "xdotool", "getwindowgeometry", "--shell", id)
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
	if name, err := runCmd(ctx, b.env(), "xdotool", "getwindowname", id); err == nil {
		info.Title = name
	}
	if pidStr, err := runCmd(ctx, b.env(), "xdotool", "getwindowpid", id); err == nil {
		info.PID, _ = strconv.Atoi(pidStr)
	}
	if cls, err := runCmd(ctx, b.env(), "xdotool", "getwindowclassname", id); err == nil {
		info.App = cls
	}
	return info, nil
}

func (b *linuxBackend) focusWindow(ctx context.Context, id string) error {
	if _, err := runCmd(ctx, b.env(), "xdotool", "windowactivate", "--sync", id); err != nil {
		// windowactivate needs a WM; fall back to raise+focus.
		if _, err2 := runCmd(ctx, b.env(), "xdotool", "windowraise", id); err2 != nil {
			return err
		}
		_, _ = runCmd(ctx, b.env(), "xdotool", "windowfocus", id)
	}
	return nil
}

func (b *linuxBackend) screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	origin := image.Point{}
	if windowID != "" {
		info, err := b.windowBounds(ctx, windowID)
		if err != nil {
			return domain.ComputerImage{}, err
		}
		origin = image.Point{X: info.X, Y: info.Y}
	}

	tmp, err := os.CreateTemp("", "dq-computer-*.png")
	if err != nil {
		return domain.ComputerImage{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	if err := b.capture(ctx, windowID, path); err != nil {
		return domain.ComputerImage{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ComputerImage{}, err
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

func (b *linuxBackend) capture(ctx context.Context, windowID, path string) error {
	switch b.captureTool() {
	case "import":
		args := []string{"-silent"}
		if windowID != "" {
			args = append(args, "-window", windowID)
		} else {
			args = append(args, "-window", "root")
		}
		args = append(args, path)
		_, err := runCmd(ctx, b.env(), "import", args...)
		return err
	case "scrot":
		// scrot cannot target an arbitrary window id directly; capture full
		// screen. Window callers still get correct origin from bounds.
		_, err := runCmd(ctx, b.env(), "scrot", "-o", path)
		return err
	case "ffmpeg":
		return b.captureFFmpeg(ctx, windowID, path)
	case "xwd":
		xwdArgs := []string{"-silent", "-root", "-out", path + ".xwd"}
		if windowID != "" {
			xwdArgs = []string{"-silent", "-id", windowID, "-out", path + ".xwd"}
		}
		if _, err := runCmd(ctx, b.env(), "xwd", xwdArgs...); err != nil {
			return err
		}
		defer os.Remove(path + ".xwd")
		_, err := runCmd(ctx, b.env(), "convert", path+".xwd", path)
		return err
	default:
		return fmt.Errorf("no screenshot tool available")
	}
}

func (b *linuxBackend) captureFFmpeg(ctx context.Context, windowID, path string) error {
	display := b.display
	if display == "" {
		display = ":0"
	}
	args := []string{"-y", "-loglevel", "error", "-f", "x11grab"}
	if windowID != "" {
		info, err := b.windowBounds(ctx, windowID)
		if err == nil && info.Width > 0 && info.Height > 0 {
			args = append(args,
				"-video_size", fmt.Sprintf("%dx%d", info.Width, info.Height),
				"-i", fmt.Sprintf("%s+%d,%d", display, info.X, info.Y),
			)
		} else {
			args = append(args, "-i", display)
		}
	} else {
		args = append(args, "-i", display)
	}
	args = append(args, "-frames:v", "1", path)
	_, err := runCmd(ctx, b.env(), "ffmpeg", args...)
	return err
}

func (b *linuxBackend) mouseMove(ctx context.Context, x, y int) error {
	_, err := runCmd(ctx, b.env(), "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

func (b *linuxBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	_, err := runCmd(ctx, b.env(), "xdotool", "click", "--repeat", strconv.Itoa(clicks), xdoButton(button))
	return err
}

func (b *linuxBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	if _, err := runCmd(ctx, b.env(), "xdotool", "mousemove", "--sync", strconv.Itoa(x0), strconv.Itoa(y0)); err != nil {
		return err
	}
	if _, err := runCmd(ctx, b.env(), "xdotool", "mousedown", "1"); err != nil {
		return err
	}
	if _, err := runCmd(ctx, b.env(), "xdotool", "mousemove", "--sync", strconv.Itoa(x1), strconv.Itoa(y1)); err != nil {
		_, _ = runCmd(ctx, b.env(), "xdotool", "mouseup", "1")
		return err
	}
	_, err := runCmd(ctx, b.env(), "xdotool", "mouseup", "1")
	return err
}

func (b *linuxBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	button := "5" // down
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
	_, err := runCmd(ctx, b.env(), "xdotool", "click", "--repeat", strconv.Itoa(amount), button)
	return err
}

func (b *linuxBackend) typeText(ctx context.Context, text string) error {
	_, err := runCmd(ctx, b.env(), "xdotool", "type", "--clearmodifiers", "--", text)
	return err
}

func (b *linuxBackend) key(ctx context.Context, key string) error {
	_, err := runCmd(ctx, b.env(), "xdotool", "key", "--clearmodifiers", key)
	return err
}

func (b *linuxBackend) cursorPosition(ctx context.Context) (int, int, error) {
	out, err := runCmd(ctx, b.env(), "xdotool", "getmouselocation", "--shell")
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

func xdoButton(button domain.MouseButton) string {
	switch button {
	case domain.MouseRight:
		return "3"
	case domain.MouseMiddle:
		return "2"
	default:
		return "1"
	}
}
