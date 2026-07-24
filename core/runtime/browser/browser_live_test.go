//go:build browser

package browser

import (
	"context"
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
