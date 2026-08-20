package builtin

import (
	"context"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type fakePage struct {
	navigated string
	closed    bool
	snap      string
}

func (p *fakePage) Navigate(_ context.Context, opts port.BrowserNavigateOptions) (port.BrowserNavigateResult, error) {
	p.navigated = opts.URL
	return port.BrowserNavigateResult{
		URL:      opts.URL,
		Title:    "Example",
		Snapshot: p.snap,
	}, nil
}
func (p *fakePage) Snapshot(context.Context) (port.BrowserSnapshotResult, error) {
	return port.BrowserSnapshotResult{URL: p.navigated, Title: "Example", Snapshot: p.snap}, nil
}
func (p *fakePage) Act(_ context.Context, req port.BrowserActRequest) (port.BrowserActResult, error) {
	return port.BrowserActResult{
		URL:      p.navigated,
		Title:    "Example",
		Snapshot: p.snap + "\n(after " + req.Action + ")",
	}, nil
}
func (p *fakePage) Screenshot(context.Context, bool) ([]byte, error) {
	// Minimal PNG header bytes for tests.
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, nil
}
func (p *fakePage) Close(context.Context) error { p.closed = true; return nil }

type fakeBrowser struct {
	page     *fakePage
	acquired map[string]int
	closed   map[string]int
}

func newFakeBrowser() *fakeBrowser {
	return &fakeBrowser{
		page:     &fakePage{snap: "- e1 link \"More\"\n- e2 button \"Go\""},
		acquired: map[string]int{},
		closed:   map[string]int{},
	}
}

func (b *fakeBrowser) Status() domain.BrowserStatus {
	return domain.BrowserStatus{Available: true, Enabled: true, Mode: "launch", Engine: "fake"}
}
func (b *fakeBrowser) Configure(domain.ConfigBrowserSection) {}
func (b *fakeBrowser) Close(context.Context) error           { return nil }
func (b *fakeBrowser) CloseAll(context.Context) error        { return nil }
func (b *fakeBrowser) RenderHTML(context.Context, port.BrowserRenderOptions) (string, string, error) {
	return "", "", nil
}
func (b *fakeBrowser) AcquirePage(_ context.Context, sessionID string) (port.BrowserPage, error) {
	b.acquired[sessionID]++
	return b.page, nil
}
func (b *fakeBrowser) ClosePage(_ context.Context, sessionID string) error {
	b.closed[sessionID]++
	return nil
}

func TestBrowserNavigate_RequiresURL(t *testing.T) {
	h := &BrowserNavigate{Browser: newFakeBrowser()}
	_, err := h.Execute(context.Background(), map[string]any{"__session_id": "s1"})
	if err == nil || !strings.Contains(err.Error(), "url") {
		t.Fatalf("expected url error, got %v", err)
	}
}

func TestBrowserNavigate_SSRFBlocked(t *testing.T) {
	h := &BrowserNavigate{Browser: newFakeBrowser()}
	_, err := h.Execute(context.Background(), map[string]any{
		"__session_id": "s1",
		"url":          "http://127.0.0.1/",
	})
	if err == nil {
		t.Fatal("expected SSRF block")
	}
}

func TestBrowserNavigate_HappyPath(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	br := newFakeBrowser()
	h := &BrowserNavigate{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{
		"__session_id": "s1",
		"url":          "https://example.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if br.acquired["s1"] != 1 {
		t.Fatalf("acquire count=%d", br.acquired["s1"])
	}
	if !strings.Contains(res.Content, "e1") || !strings.Contains(res.Content, "example.com") {
		t.Fatalf("unexpected content: %s", res.Content)
	}
}

func TestBrowserAct_RequiresAction(t *testing.T) {
	h := &BrowserAct{Browser: newFakeBrowser()}
	_, err := h.Execute(context.Background(), map[string]any{"__session_id": "s1"})
	if err == nil || !strings.Contains(err.Error(), "action") {
		t.Fatalf("expected action error, got %v", err)
	}
}

func TestBrowserAct_Click(t *testing.T) {
	br := newFakeBrowser()
	h := &BrowserAct{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{
		"__session_id": "s1",
		"action":       "click",
		"ref":          "e1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "after click") {
		t.Fatalf("unexpected content: %s", res.Content)
	}
}

func TestBrowserScreenshot_VisionGate(t *testing.T) {
	br := newFakeBrowser()
	h := &BrowserScreenshot{
		Browser:       br,
		SupportsImage: func(modelID string) bool { return false },
	}
	_, err := h.Execute(context.Background(), map[string]any{
		"__session_id": "s1",
		"__model_id":   "text-only",
	})
	if err == nil || !strings.Contains(err.Error(), "does not accept image") {
		t.Fatalf("expected vision gate error, got %v", err)
	}
}

func TestBrowserScreenshot_ReturnsImagePart(t *testing.T) {
	br := newFakeBrowser()
	h := &BrowserScreenshot{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{"__session_id": "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parts) != 1 || res.Parts[0].Type != "image" || res.Parts[0].MimeType != "image/png" {
		t.Fatalf("unexpected parts: %+v", res.Parts)
	}
}

func TestBrowserClose_Idempotent(t *testing.T) {
	br := newFakeBrowser()
	h := &BrowserClose{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{"__session_id": "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content == "" || br.closed["s1"] != 1 {
		t.Fatalf("content=%q closed=%d", res.Content, br.closed["s1"])
	}
}
