package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestExtractPageContent_Readability(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Hello Article</title></head>
<body>
<article>
<h1>Hello Article</h1>
<p>This is a long enough paragraph about cats and dogs to satisfy readability extraction heuristics for unit testing purposes.</p>
<p>Another paragraph with more meaningful content so the article extractor has enough text to work with reliably.</p>
</article>
</body></html>`
	content, extractor, title, spa := extractPageContent(html, "https://example.com/post")
	if extractor != "readability" && extractor != "html" {
		t.Fatalf("extractor = %q", extractor)
	}
	if !strings.Contains(content, "cats and dogs") {
		t.Fatalf("content missing body text: %q", content)
	}
	if spa {
		t.Fatal("did not expect SPA hint for article page")
	}
	_ = title
}

func TestIsSPAShell(t *testing.T) {
	html := `<html><body><div id="root"></div><script></script><script></script><script></script></body></html>`
	if !isSPAShell(html, "ok") {
		// extracted is short (<200)
		if !isSPAShell(html, "x") {
			t.Fatal("expected SPA shell detection")
		}
	}
	if !isSPAShell(html, "short") {
		t.Fatal("expected SPA shell for short extract")
	}
	long := strings.Repeat("word ", 80)
	if isSPAShell(html, long) {
		t.Fatal("did not expect SPA hint when extract is long")
	}
}

func TestWebFetch_ExecuteJSON(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	h := &WebFetch{}
	res, err := h.Execute(context.Background(), map[string]any{
		"url":       srv.URL,
		"max_chars": float64(8000),
	})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Content), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["extractor"] != "json" {
		t.Fatalf("extractor = %v", parsed["extractor"])
	}
	if !strings.Contains(parsed["content"].(string), "hello") {
		t.Fatalf("content = %v", parsed["content"])
	}
}

func TestWebFetch_BlocksLocalhost(t *testing.T) {
	h := &WebFetch{}
	_, err := h.Execute(context.Background(), map[string]any{
		"url": "http://127.0.0.1:9/",
	})
	if err == nil {
		t.Fatal("expected SSRF error")
	}
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "local") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubBrowser struct {
	available bool
	html      string
	finalURL  string
	err       error
	calls     int
}

func (s *stubBrowser) Status() domain.BrowserStatus {
	return domain.BrowserStatus{Available: s.available, Enabled: true, Engine: "stub", Mode: "launch"}
}
func (s *stubBrowser) Configure(domain.ConfigBrowserSection) {}
func (s *stubBrowser) Close(context.Context) error           { return nil }
func (s *stubBrowser) CloseAll(context.Context) error        { return nil }
func (s *stubBrowser) ClosePage(context.Context, string) error {
	return nil
}
func (s *stubBrowser) AcquirePage(context.Context, string) (port.BrowserPage, error) {
	return nil, fmt.Errorf("stub browser does not support interactive pages")
}
func (s *stubBrowser) RenderHTML(ctx context.Context, opts port.BrowserRenderOptions) (string, string, error) {
	s.calls++
	if s.err != nil {
		return "", "", s.err
	}
	return s.html, s.finalURL, nil
}

func TestWebFetch_RenderNever_NoBrowser(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	spaHTML := `<html><body><div id="root"></div><script></script><script></script><script></script></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(spaHTML))
	}))
	defer srv.Close()

	br := &stubBrowser{available: true, html: "<html><body><p>Rendered content from browser with enough text to pass.</p></body></html>"}
	h := &WebFetch{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"render": "never",
	})
	if err != nil {
		t.Fatal(err)
	}
	if br.calls != 0 {
		t.Fatalf("browser should not be called with render=never, calls=%d", br.calls)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(res.Content), &parsed)
	msg, _ := parsed["message"].(string)
	if !strings.Contains(msg, "JavaScript-rendered") {
		t.Fatalf("expected SPA message, got %q", msg)
	}
}

func TestWebFetch_RenderAuto_UsesBrowserOnSPA(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	spaHTML := `<html><body><div id="root"></div><script></script><script></script><script></script></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(spaHTML))
	}))
	defer srv.Close()

	br := &stubBrowser{
		available: true,
		html:      `<!DOCTYPE html><html><body><article><h1>App</h1><p>This is enough readable text from a headless browser render for testing the auto path thoroughly.</p><p>Second paragraph keeps readability happy with additional sentences about the topic.</p></article></body></html>`,
		finalURL:  srv.URL + "/app",
	}
	h := &WebFetch{Browser: br}
	res, err := h.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"render": "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if br.calls != 1 {
		t.Fatalf("expected 1 browser call, got %d", br.calls)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(res.Content), &parsed)
	if parsed["rendered"] != true {
		t.Fatalf("expected rendered=true: %v", parsed)
	}
	ext, _ := parsed["extractor"].(string)
	if !strings.Contains(ext, "browser") {
		t.Fatalf("extractor = %q", ext)
	}
}

func TestWebFetch_RenderAlways_Unavailable(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	h := &WebFetch{Browser: &stubBrowser{available: false}}
	_, err := h.Execute(context.Background(), map[string]any{
		"url":    "https://example.com/",
		"render": "always",
	})
	if err == nil {
		t.Fatal("expected error when browser unavailable")
	}
	if !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
