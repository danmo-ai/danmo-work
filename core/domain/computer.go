package domain

import "strings"

// Computer use domain types. These describe an OS-agnostic desktop control
// surface (window discovery, mouse, keyboard, screenshot) that the agent drives
// through a single `computer` builtin tool. The concrete OS backends live in
// core/runtime/computer.

// ConfigComputerSection is persisted under runtime.computer in config.yaml.
// Desktop control is off by default and must be explicitly enabled. When
// runtime.sandbox.profile=desktop with a container backend, actions target the
// sandboxed Xvfb display instead of the host GUI.
type ConfigComputerSection struct {
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	// Display optionally overrides the X11 DISPLAY used on Linux (e.g. ":1").
	// Empty inherits the process environment (host) or uses ":1" in desktop sandbox.
	Display string `json:"display,omitempty" mapstructure:"display" yaml:"display,omitempty"`
	// MaxShotEdge caps the longest screenshot edge sent to the model (JPEG
	// downscale). Zero uses 1568 for host targets, or the sandbox desktop cap.
	MaxShotEdge int `json:"maxShotEdge,omitempty" mapstructure:"max_shot_edge" yaml:"max_shot_edge,omitempty"`
	// Target selects the host GUI, a sandbox desktop, or automatic selection.
	Target string `json:"target,omitempty" mapstructure:"target" yaml:"target,omitempty"`
}

const (
	ComputerTargetAuto         = "auto"
	ComputerTargetHost         = "host"
	ComputerTargetSandbox      = "sandbox"
	DefaultComputerMaxShotEdge = 1568
)

func NormalizeComputerTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", ComputerTargetAuto:
		return ComputerTargetAuto
	case ComputerTargetHost:
		return ComputerTargetHost
	case ComputerTargetSandbox:
		return ComputerTargetSandbox
	default:
		return ComputerTargetAuto
	}
}

// ComputerStatus is the probed desktop-control capability surface.
type ComputerStatus struct {
	Available      bool   `json:"available"`
	Enabled        bool   `json:"enabled"`
	Platform       string `json:"platform"`                 // linux | darwin | windows
	Backend        string `json:"backend"`                  // x11 | wayland | macos | windows | none
	Display        string `json:"display,omitempty"`        // resolved DISPLAY (Linux)
	DegradedReason string `json:"degradedReason,omitempty"` // why control is unavailable
}

// WindowInfo describes one on-screen window discovered by the backend.
type WindowInfo struct {
	ID     string `json:"id"`               // backend-native window handle (string form)
	Title  string `json:"title"`            // window title
	App    string `json:"app,omitempty"`    // application / WM class when known
	PID    int    `json:"pid,omitempty"`    // owning process id when known
	X      int    `json:"x"`                // absolute top-left X on the virtual screen
	Y      int    `json:"y"`                // absolute top-left Y on the virtual screen
	Width  int    `json:"width"`            // window width in pixels
	Height int    `json:"height"`           // window height in pixels
	Active bool   `json:"active,omitempty"` // whether this is the focused window
}

// ComputerImage is a captured screenshot returned by a backend.
type ComputerImage struct {
	MimeType string `json:"mimeType"` // typically image/png
	Data     []byte `json:"-"`        // raw image bytes
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// OriginX/OriginY are the absolute screen coordinates of the image's
	// top-left pixel. Zero for a full-display capture; set to the window's
	// on-screen position for a window capture so the tool can translate
	// image-relative coordinates back to absolute screen coordinates.
	OriginX int `json:"originX"`
	OriginY int `json:"originY"`
	// DisplayWidth/DisplayHeight are the dimensions of the captured region in
	// mouse coordinate units. On macOS these are logical points, not Retina
	// pixels; on X11/Windows they are physical pixels. For a window or zoomed
	// capture, they describe that region rather than the whole display.
	// Map coordinates with: screen = Origin + image * (Display/Width).
	DisplayWidth  int `json:"displayWidth,omitempty"`
	DisplayHeight int `json:"displayHeight,omitempty"`
	// Scope uniquely identifies the latest screenshot used for coordinate
	// mapping. It includes the session, optional window, and optional zoom.
	Scope string `json:"-"`
}

// MouseButton identifies which mouse button an action targets.
type MouseButton string

const (
	MouseLeft   MouseButton = "left"
	MouseRight  MouseButton = "right"
	MouseMiddle MouseButton = "middle"
)

// ScreenRect is a rectangle in screen (mouse) coordinate units.
type ScreenRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// KeyModifier is a modifier held during a mouse action.
type KeyModifier string

const (
	ModifierShift KeyModifier = "shift"
	ModifierCtrl  KeyModifier = "ctrl"
	ModifierAlt   KeyModifier = "alt"
	ModifierMeta  KeyModifier = "meta"
)

// ScrollDirection identifies a scroll axis/direction.
type ScrollDirection string

const (
	ScrollUp    ScrollDirection = "up"
	ScrollDown  ScrollDirection = "down"
	ScrollLeft  ScrollDirection = "left"
	ScrollRight ScrollDirection = "right"
)
