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

// Browser renders pages via CDP (Launch local Chrome or Attach to cdp_url).
type Browser interface {
	Status() domain.BrowserStatus
	RenderHTML(ctx context.Context, opts BrowserRenderOptions) (html string, finalURL string, err error)
	// Configure replaces policy and re-probes availability (e.g. after config save).
	Configure(cfg domain.ConfigBrowserSection)
	// Close shuts down any Launch-owned browser processes and temp dirs.
	// Attach mode only closes our CDP clients, never kills the remote browser.
	Close(ctx context.Context) error
}
