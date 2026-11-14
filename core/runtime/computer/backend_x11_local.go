package computer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strconv"
	"strings"

	"danmo-work/core/domain"
)

// linuxBackend drives local X11 through the shared x11.go implementation.
type linuxBackend struct{ display string }

func (b *linuxBackend) run(ctx context.Context, name string, args ...string) (string, error) {
	return runCmd(ctx, b.env(), name, args...)
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
	display := strings.TrimSpace(cfg.Display)
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
	if !hasBinary("xdpyinfo") {
		st.Backend = "x11"
		st.DegradedReason = "xdpyinfo not installed (needed to determine display geometry)"
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

func (b *linuxBackend) captureTool() string {
	switch {
	case hasBinary("import"):
		return "import"
	case hasBinary("scrot"):
		return "scrot"
	case hasBinary("ffmpeg"):
		return "ffmpeg"
	default:
		return ""
	}
}

func (b *linuxBackend) screenshot(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
	origin, w, h, err := captureGeometry(ctx, b, windowID)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	if zoom != nil {
		if zoom.X < origin.X || zoom.Y < origin.Y || zoom.X+zoom.Width > origin.X+w || zoom.Y+zoom.Height > origin.Y+h {
			return domain.ComputerImage{}, fmt.Errorf("zoom rectangle is outside the selected screen region")
		}
		origin = image.Point{X: zoom.X, Y: zoom.Y}
		w, h = zoom.Width, zoom.Height
	}
	tmp, err := os.CreateTemp("", "dq-computer-*.png")
	if err != nil {
		return domain.ComputerImage{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)
	if err := b.capture(ctx, origin, w, h, path); err != nil {
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
	return domain.ComputerImage{MimeType: "image/png", Data: data, Width: cfg.Width, Height: cfg.Height, OriginX: origin.X, OriginY: origin.Y, DisplayWidth: w, DisplayHeight: h}, nil
}

// capture always crops the full root capture in the same pixel coordinate space
// as xdotool; window captures never mix client rectangles with frame origins.
func (b *linuxBackend) capture(ctx context.Context, origin image.Point, width, height int, path string) error {
	crop := fmt.Sprintf("%dx%d+%d+%d", width, height, origin.X, origin.Y)
	switch b.captureTool() {
	case "import":
		_, err := b.run(ctx, "import", "-silent", "-window", "root", "-crop", crop, "+repage", path)
		return err
	case "scrot":
		_, err := b.run(ctx, "scrot", "-o", "-a", fmt.Sprintf("%d,%d,%d,%d", origin.X, origin.Y, width, height), path)
		return err
	case "ffmpeg":
		display := b.display
		if display == "" {
			display = ":0"
		}
		_, err := b.run(ctx, "ffmpeg", "-y", "-loglevel", "error", "-f", "x11grab", "-video_size", strconv.Itoa(width)+"x"+strconv.Itoa(height), "-i", fmt.Sprintf("%s+%d,%d", display, origin.X, origin.Y), "-frames:v", "1", path)
		return err
	default:
		return fmt.Errorf("no screenshot tool available")
	}
}
