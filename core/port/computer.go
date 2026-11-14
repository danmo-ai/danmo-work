package port

import (
	"context"

	"danmo-work/core/domain"
)

// Computer is the OS-agnostic desktop-control surface used by the `computer`
// builtin tool. Pointer arguments are image-relative pixels from the latest
// screenshot with the same session/window scope; implementations map them to
// the target's native input coordinate system before dispatching the action.
type Computer interface {
	// Status reports the cached desktop availability probe.
	Status() domain.ComputerStatus
	// Configure replaces policy and re-probes availability (e.g. after config save).
	Configure(cfg domain.ConfigComputerSection)

	ListWindows(ctx context.Context, query string) ([]domain.WindowInfo, error)
	FocusWindow(ctx context.Context, id string) error
	WindowBounds(ctx context.Context, id string) (domain.WindowInfo, error)

	Screenshot(ctx context.Context, sessionID, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error)

	MouseMove(ctx context.Context, sessionID, windowID string, x, y int, modifiers []domain.KeyModifier) error
	MouseClick(ctx context.Context, sessionID, windowID string, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error
	MouseDrag(ctx context.Context, sessionID, windowID string, x0, y0, x1, y1 int) error
	MouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error
	Scroll(ctx context.Context, sessionID, windowID string, x, y int, dir domain.ScrollDirection, amount int) error

	TypeText(ctx context.Context, text string) error
	Key(ctx context.Context, key string, hold bool) error

	CursorPosition(ctx context.Context) (x, y int, err error)
}
