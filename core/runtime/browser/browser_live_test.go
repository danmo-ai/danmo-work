//go:build browser

package browser

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// TestRenderHTML_LaunchLive requires a local Chrome/Edge/Chromium.
// Run: go test -tags=browser ./core/runtime/browser/ -count=1
func TestRenderHTML_LaunchLive(t *testing.T) {
	m := New(domain.ConfigBrowserSection{Enabled: true})
	st := m.Status()
	if !st.Available || st.Mode != "launch" {
		t.Skipf("no local browser: %+v", st)
	}
	html, finalURL, err := m.RenderHTML(context.Background(), port.BrowserRenderOptions{
		URL:       "https://example.com/",
		Timeout:   45 * time.Second,
		WaitUntil: "load",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(html), "html") {
		t.Fatalf("unexpected html len=%d", len(html))
	}
	if finalURL == "" {
		t.Fatal("empty finalURL")
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStickyPage_NavigateSnapshotScreenshotLive(t *testing.T) {
	home, err := os.MkdirTemp("", "danmo-browser-live-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	t.Setenv("WORK_HOME", home)

	m := New(domain.ConfigBrowserSection{Enabled: true})
	st := m.Status()
	if !st.Available || st.Mode != "launch" {
		t.Skipf("no local browser: %+v", st)
	}

	page, err := m.AcquirePage(context.Background(), "live-session")
	if err != nil {
		t.Fatal(err)
	}
	nav, err := page.Navigate(context.Background(), port.BrowserNavigateOptions{
		URL:       "https://example.com/",
		Timeout:   45 * time.Second,
		WaitUntil: "load",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nav.Snapshot, "URL:") {
		t.Fatalf("snapshot missing URL: %s", nav.Snapshot)
	}
	page2, err := m.AcquirePage(context.Background(), "live-session")
	if err != nil {
		t.Fatal(err)
	}
	if page2 != page {
		t.Fatal("expected sticky page reuse")
	}
	png, err := page.Screenshot(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 8 || png[0] != 0x89 || png[1] != 0x50 {
		t.Fatalf("expected PNG magic, len=%d", len(png))
	}
	if err := m.ClosePage(context.Background(), "live-session"); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Give Chromium a moment to release profile locks before RemoveAll.
	time.Sleep(500 * time.Millisecond)
}
