package domain

// Computer use domain types. These describe an OS-agnostic desktop control
// surface (window discovery, mouse, keyboard, screenshot) that the agent drives
// through a single `computer` builtin tool. The concrete OS backends live in
// core/runtime/computer.

// ConfigComputerSection is persisted under runtime.computer in config.yaml.
// Desktop control is off by default and must be explicitly enabled because it
// grants the agent full control of the host GUI (no sandbox boundary).
type ConfigComputerSection struct {
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	// Display optionally overrides the X11 DISPLAY used on Linux (e.g. ":1").
	// Empty inherits the process environment.
	Display string `json:"display,omitempty" mapstructure:"display" yaml:"display,omitempty"`
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
}

// MouseButton identifies which mouse button an action targets.
type MouseButton string

const (
	MouseLeft   MouseButton = "left"
	MouseRight  MouseButton = "right"
	MouseMiddle MouseButton = "middle"
)

// ScrollDirection identifies a scroll axis/direction.
type ScrollDirection string

const (
	ScrollUp    ScrollDirection = "up"
	ScrollDown  ScrollDirection = "down"
	ScrollLeft  ScrollDirection = "left"
	ScrollRight ScrollDirection = "right"
)
