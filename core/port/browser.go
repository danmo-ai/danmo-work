package port

import (
	"context"
	"time"

	"danmo-work/core/domain"
)

// BrowserRenderOptions configures a single headless page render.
type BrowserRenderOptions struct {
	URL       string
	Timeout   time.Duration
	WaitUntil string // networkidle | load | domcontentloaded
	Proxy     string
	UserAgent string
}

// BrowserNavigateOptions configures a sticky-page navigation.
type BrowserNavigateOptions struct {
	URL       string
	Timeout   time.Duration
	WaitUntil string // networkidle | load | domcontentloaded
}

// BrowserNavigateResult is returned after navigate (includes a fresh snapshot).
type BrowserNavigateResult struct {
	URL      string
	Title    string
	Snapshot string
}

// BrowserSnapshotResult is a compact accessibility-oriented page summary.
type BrowserSnapshotResult struct {
	URL      string
	Title    string
	Snapshot string
}

// BrowserActRequest is a semantic page action against a snapshot ref.
type BrowserActRequest struct {
	Action    string // click | type | press | scroll | select | hover
	Ref       string
	Text      string
	Key       string
	Direction string // up | down (scroll)
	Amount    int    // scroll pixels; 0 = default
	Value     string // select option
	Timeout   time.Duration
}

// BrowserActResult is returned after an action (includes a fresh snapshot).
type BrowserActResult struct {
	URL      string
	Title    string
	Snapshot string
}

// BrowserPage is a sticky tab keyed by Danmo session ID.
type BrowserPage interface {
	Navigate(ctx context.Context, opts BrowserNavigateOptions) (BrowserNavigateResult, error)
	Snapshot(ctx context.Context) (BrowserSnapshotResult, error)
	Act(ctx context.Context, req BrowserActRequest) (BrowserActResult, error)
	Screenshot(ctx context.Context, fullPage bool) (png []byte, err error)
	Close(ctx context.Context) error
}

// Browser renders pages via CDP (Launch local Chrome or Attach to cdp_url)
// and manages sticky interactive sessions for browser_* tools.
type Browser interface {
	Status() domain.BrowserStatus
	RenderHTML(ctx context.Context, opts BrowserRenderOptions) (html string, finalURL string, err error)
	// AcquirePage returns the sticky page for sessionID, creating it if needed.
	AcquirePage(ctx context.Context, sessionID string) (BrowserPage, error)
	// ClosePage tears down the sticky page for sessionID (idempotent).
	ClosePage(ctx context.Context, sessionID string) error
	// CloseAll tears down every sticky page and shared allocator state.
	CloseAll(ctx context.Context) error
	// Configure replaces policy and re-probes availability (e.g. after config save).
	Configure(cfg domain.ConfigBrowserSection)
	// Close shuts down sticky pages plus any in-flight one-shot renders.
	// Attach mode never kills the remote browser process.
	Close(ctx context.Context) error
}
