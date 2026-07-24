package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"github.com/chromedp/chromedp"
)

func (m *Manager) renderOnce(
	ctx context.Context,
	cfg domain.ConfigBrowserSection,
	st domain.BrowserStatus,
	opts port.BrowserRenderOptions,
	timeout time.Duration,
	userDataDir string,
) (html string, finalURL string, err error) {
	parent := context.Background()
	if ctx != nil {
		parent = ctx
	}

	var allocCtx context.Context
	var cancelAlloc context.CancelFunc

	if st.Mode == "attach" {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(parent, cfg.CDPURL)
	} else {
		execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(st.Path),
			chromedp.UserDataDir(userDataDir),
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		)
		if ua := strings.TrimSpace(opts.UserAgent); ua != "" {
			execOpts = append(execOpts, chromedp.UserAgent(ua))
		}
		if proxy := strings.TrimSpace(opts.Proxy); proxy != "" {
			execOpts = append(execOpts, chromedp.ProxyServer(proxy))
		}
		allocCtx, cancelAlloc = chromedp.NewExecAllocator(parent, execOpts...)
	}

	// Track allocator cancel so Manager.Close can tear down in-flight launches.
	_, untrack := m.track(cancelAlloc)
	defer func() {
		untrack()
		cancelAlloc()
	}()

	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, timeout)
	defer cancelTimeout()

	var outerHTML, loc string
	tasks := chromedp.Tasks{
		chromedp.Navigate(opts.URL),
		waitAction(opts.WaitUntil),
		chromedp.Evaluate(`document.documentElement ? document.documentElement.outerHTML : document.body ? document.body.outerHTML : ""`, &outerHTML),
		chromedp.Location(&loc),
	}
	if err := chromedp.Run(tabCtx, tasks); err != nil {
		return "", "", fmt.Errorf("browser render failed: %w", err)
	}
	if loc == "" {
		loc = opts.URL
	}
	return outerHTML, loc, nil
}

func waitAction(waitUntil string) chromedp.Action {
	switch strings.ToLower(strings.TrimSpace(waitUntil)) {
	case "domcontentloaded":
		return chromedp.WaitReady("body", chromedp.ByQuery)
	case "load":
		return chromedp.WaitReady("body", chromedp.ByQuery)
	case "networkidle", "":
		// Best-effort settle: wait for body then a short idle pause.
		return chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return nil
			}
		})
	default:
		return chromedp.WaitReady("body", chromedp.ByQuery)
	}
}
