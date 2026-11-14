package port

import (
	"context"
	"errors"
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
var ErrBrowserPageNotFound = errors.New("browser page not found for session")
var ErrBrowserPageClosed = errors.New("browser page is closed")

// BrowserWatchOptions limits an ephemeral, read-only page screencast.
type BrowserWatchOptions struct {
	MaxWidth  int
	MaxHeight int
	Quality   int
}

// BrowserPageStatus describes the live sticky page without creating it.
type BrowserPageStatus struct {
	SessionID string `json:"sessionId"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	State     string `json:"state"` // live | closed
}

// BrowserFrame is one JPEG screencast frame. Frames are ephemeral and are not
// written to turn logs or persisted browser history.
type BrowserFrame struct {
	Data     []byte
	MimeType string
	Width    int
	Height   int
}

// BrowserWatchEvent carries either a status transition or a new frame.
type BrowserWatchEvent struct {
	Status BrowserPageStatus
	Frame  *BrowserFrame
}

// BrowserLastFrame is the process-local final visual state of a closed page.
type BrowserLastFrame struct {
	Status BrowserPageStatus
	Frame  BrowserFrame
}

// BrowserWatcher is implemented by browsers that can mirror an existing sticky
// page. Watching must never call AcquirePage or launch a browser on its own.
type BrowserWatcher interface {
	WatchPage(ctx context.Context, sessionID string, opts BrowserWatchOptions) (<-chan BrowserWatchEvent, error)
	LastPageFrame(sessionID string) (BrowserLastFrame, bool)
}

// BrowserPage interface controls the Agent-owned sticky tab.
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
