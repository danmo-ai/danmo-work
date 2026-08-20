package port

import (
	"context"

	"danmo-work/core/domain"
)

// Computer is the OS-agnostic desktop-control surface used by the `computer`
// builtin tool. Implementations drive the host GUI: enumerate/focus windows,
// move and click the mouse, type on the keyboard, and capture screenshots.
//
// All coordinates are absolute pixels on the virtual screen unless a method
// documents otherwise. Backends are expected to be safe to call concurrently
// only in the sense that the runtime serializes computer tool calls per turn.
type Computer interface {
	// Status reports whether desktop control is available and why not.
	Status() domain.ComputerStatus
	// Configure replaces policy and re-probes availability (e.g. after config save).
	Configure(cfg domain.ConfigComputerSection)

	// ListWindows returns on-screen windows, optionally filtered by a
	// case-insensitive substring match against title/app.
	ListWindows(ctx context.Context, query string) ([]domain.WindowInfo, error)
	// FocusWindow raises and focuses the window with the given backend id.
	FocusWindow(ctx context.Context, id string) error
	// WindowBounds returns the current absolute bounds of a window.
	WindowBounds(ctx context.Context, id string) (domain.WindowInfo, error)

	// Screenshot captures the full primary display when windowID is empty, or a
	// single window when set. The returned image carries its origin so callers
	// can map image-relative coordinates to absolute screen coordinates.
	Screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error)

	// MouseMove moves the cursor to an absolute screen coordinate.
	MouseMove(ctx context.Context, x, y int) error
	// MouseClick clicks a button at an absolute coordinate. clicks>=1 controls
	// single/double/triple clicks.
	MouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error
	// MouseDrag presses at (x0,y0), moves to (x1,y1), and releases (left button).
	MouseDrag(ctx context.Context, x0, y0, x1, y1 int) error
	// Scroll scrolls by amount "ticks" in a direction, at an absolute coordinate.
	Scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error

	// TypeText types a Unicode string at the current focus.
	TypeText(ctx context.Context, text string) error
	// Key presses a key or chord such as "Return", "ctrl+s", "alt+Tab".
	Key(ctx context.Context, key string) error

	// CursorPosition returns the current absolute cursor position.
	CursorPosition(ctx context.Context) (x, y int, err error)
}
