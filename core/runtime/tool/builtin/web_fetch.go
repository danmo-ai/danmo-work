package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	readability "github.com/go-shiori/go-readability"
)

// WebFetch fetches a URL and returns readable text or JSON.
type WebFetch struct {
	ConfigFunc func(context.Context) (domain.SearchConfig, error)
	Browser    port.Browser
	Egress     HostEgressChecker
}

func (h *WebFetch) Name() string                { return "web_fetch" }
func (h *WebFetch) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *WebFetch) Describe(args map[string]any) string {
	urlStr, _ := args["url"].(string)
	if len(urlStr) > 80 {
		urlStr = urlStr[:80] + "..."
	}
	return urlStr
}
func (h *WebFetch) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "web_fetch",
		Description: "Fetch the content of a web page or API endpoint and return readable text.\n\n" +
			"- url: must be a valid HTTP/HTTPS URL (required).\n" +
			"- max_chars: limits returned content length (default 8000, min 100).\n" +
			"- timeout_ms: timeout in milliseconds (default 15000, max 60000).\n" +
			"- render: auto (default) | always | never. auto uses a headless browser when the HTTP response looks like a JS SPA shell; always requires a browser; never is HTTP-only.\n" +
			"- HTML pages are extracted via readability (with a simple HTML fallback); JSON is returned as-is.\n" +
			"- Use after web_search to get full content from interesting results.\n" +
			"- HTTP URLs on port 80 are automatically upgraded to HTTPS.\n" +
			"- Private/local addresses are blocked (SSRF protection).\n" +
			"- JavaScript rendering requires a local Chrome/Edge/Chromium or runtime.browser.cdp_url.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":        map[string]any{"type": "string", "description": "URL to fetch"},
				"max_chars":  map[string]any{"type": "integer", "description": "Maximum characters to return (default 8000)"},
				"timeout_ms": map[string]any{"type": "integer", "description": "Timeout in milliseconds (default 15000)"},
				"render": map[string]any{
					"type":        "string",
					"description": "auto | always | never (default auto)",
					"enum":        []string{"auto", "always", "never"},
				},
			},
			"required": []string{"url"},
		},
	}
}

func (h *WebFetch) currentConfig(ctx context.Context) domain.SearchConfig {
	if h.ConfigFunc == nil {
		return domain.SearchConfig{}
	}
	cfg, err := h.ConfigFunc(ctx)
	if err != nil {
		return domain.SearchConfig{}
	}
	return cfg
}

func (h *WebFetch) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	urlStr, _ := input["url"].(string)
	if urlStr == "" {
		return domain.ToolResult{}, fmt.Errorf("url is required")
	}
	urlStr = upgradeToHTTPS(urlStr)

	maxChars := 8000
	if m, ok := input["max_chars"].(float64); ok {
		maxChars = int(m)
	}
	if maxChars < 100 {
		maxChars = 100
	}
	timeoutMs := defaultTimeoutMs
	if t, ok := input["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}
	if timeoutMs < 1000 {
		timeoutMs = 1000
	}
	if timeoutMs > maxTimeoutMs {
		timeoutMs = maxTimeoutMs
	}

	renderMode := "auto"
	if r, ok := input["render"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case "always", "never", "auto":
			renderMode = strings.ToLower(strings.TrimSpace(r))
		}
	}

	cfg := h.currentConfig(ctx)
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// The browser render path must honor the same egress allowlist as the HTTP
	// path; only a session full-network grant (__sandbox_allow_network) relaxes it.
	browserEgress := h.Egress
	if v, ok := input["__sandbox_allow_network"].(bool); ok && v {
		browserEgress = nil
	}

	if renderMode == "always" {
		return h.fetchViaBrowser(ctx, urlStr, maxChars, timeout, cfg, 0, "", browserEgress)
	}

	opts := withToolEgress(clientOpts(cfg.Proxy, cfg.UserAgent, timeout, false), h.Egress, input)
	resp, err := fetchWithOpts(ctx, urlStr, opts)
	if err != nil {
		return domain.ToolResult{}, err
	}
	body, err := readResponseBody(resp, maxFetchBytes)
	if err != nil {
		return domain.ToolResult{}, err
	}
	contentType := resp.Header.Get("Content-Type")
	if isRejectedContentType(contentType) {
		return domain.ToolResult{}, fmt.Errorf("content type %s is not supported", contentType)
	}
	content := string(body)
	finalURL := resp.Request.URL.String()
	statusCode := resp.StatusCode

	extractor := "raw"
	title := ""
	message := ""
	rendered := false

	if strings.Contains(contentType, "application/json") {
		extractor = "json"
	} else if strings.Contains(contentType, "text/html") || looksLikeHTML(content) {
		extracted, ext, pageTitle, spaHint := extractPageContent(content, finalURL)
		content = extracted
		extractor = ext
		title = pageTitle
		if spaHint && renderMode == "auto" && h.browserAvailable() {
			br, err := h.fetchViaBrowser(ctx, urlStr, maxChars, timeout, cfg, statusCode, finalURL, browserEgress)
			if err == nil {
				return br, nil
			}
			message = "Page content looks like a JavaScript-rendered shell; browser render failed: " + err.Error()
		} else if spaHint {
			if renderMode == "never" || h.Browser == nil || !h.browserAvailable() {
				message = "Page content looks like a JavaScript-rendered shell; little readable text was extracted. Pass render=always when a headless browser is available."
			}
		}
	}

	return packFetchResult(urlStr, finalURL, statusCode, extractor, title, message, content, maxChars, rendered)
}

func (h *WebFetch) browserAvailable() bool {
	if h.Browser == nil {
		return false
	}
	return h.Browser.Status().Available
}

func (h *WebFetch) fetchViaBrowser(
	ctx context.Context,
	urlStr string,
	maxChars int,
	timeout time.Duration,
	cfg domain.SearchConfig,
	httpStatus int,
	hintFinal string,
	eg HostEgressChecker,
) (domain.ToolResult, error) {
	// Enforce SSRF + egress allowlist on the entry URL. Note the headless
	// browser still follows in-page redirects and loads sub-resources on its
	// own; those hops are not individually re-checked here.
	if err := assertPublicURLWithEgress(urlStr, eg); err != nil {
		return domain.ToolResult{}, err
	}
	if h.Browser == nil || !h.Browser.Status().Available {
		st := domain.BrowserStatus{}
		if h.Browser != nil {
			st = h.Browser.Status()
		}
		reason := st.DegradedReason
		if reason == "" {
			reason = "install Chrome/Edge/Chromium or set runtime.browser.cdp_url"
		}
		return domain.ToolResult{}, fmt.Errorf("browser render required but unavailable: %s", reason)
	}

	html, finalURL, err := h.Browser.RenderHTML(ctx, port.BrowserRenderOptions{
		URL:       urlStr,
		Timeout:   timeout,
		WaitUntil: "networkidle",
		Proxy:     cfg.Proxy,
		UserAgent: cfg.UserAgent,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}
	if finalURL == "" {
		finalURL = hintFinal
	}
	if finalURL == "" {
		finalURL = urlStr
	}

	content, extractor, title, _ := extractPageContent(html, finalURL)
	status := httpStatus
	if status == 0 {
		status = 200
	}
	return packFetchResult(urlStr, finalURL, status, extractor+"+browser", title, "", content, maxChars, true)
}

func packFetchResult(
	urlStr, finalURL string,
	status int,
	extractor, title, message, content string,
	maxChars int,
	rendered bool,
) (domain.ToolResult, error) {
	truncated := false
	if len(content) > maxChars {
		content = content[:maxChars]
		truncated = true
	}
	result := map[string]any{
		"url":       urlStr,
		"finalUrl":  finalURL,
		"status":    status,
		"extractor": extractor,
		"truncated": truncated,
		"length":    len(content),
		"content":   content,
		"rendered":  rendered,
	}
	if title != "" {
		result["title"] = title
	}
	if message != "" {
		result["message"] = message
	}
	b, _ := json.Marshal(result)
	return domain.ToolResult{Content: string(b)}, nil
}

func looksLikeHTML(content string) bool {
	trim := strings.ToLower(strings.TrimSpace(content))
	return strings.HasPrefix(trim, "<!doctype") || strings.HasPrefix(trim, "<html")
}

func isRejectedContentType(contentType string) bool {
	lower := strings.ToLower(contentType)
	prefixes := []string{
		"image/", "audio/", "video/", "application/octet-stream",
		"application/zip", "application/gzip", "application/x-tar",
		"application/pdf", "application/msword", "application/vnd.ms",
		"application/vnd.openxml",
	}
	for _, p := range prefixes {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func extractPageContent(html, pageURL string) (content, extractor, title string, spaHint bool) {
	if u, err := url.Parse(pageURL); err == nil && u.Scheme != "" {
		if article, err := readability.FromReader(strings.NewReader(html), u); err == nil {
			text := strings.TrimSpace(article.TextContent)
			if text != "" {
				spaHint = isSPAShell(html, text)
				return text, "readability", strings.TrimSpace(article.Title), spaHint
			}
		}
	}
	text := extractReadableText(html)
	spaHint = isSPAShell(html, text)
	return text, "html", "", spaHint
}

func isSPAShell(html, extracted string) bool {
	if len(strings.TrimSpace(extracted)) >= 200 {
		return false
	}
	lower := strings.ToLower(html)
	scriptCount := strings.Count(lower, "<script")
	hasRoot := strings.Contains(lower, `id="root"`) ||
		strings.Contains(lower, `id='root'`) ||
		strings.Contains(lower, `id="app"`) ||
		strings.Contains(lower, `id='app'`) ||
		strings.Contains(lower, `id="__next"`) ||
		strings.Contains(lower, `__next`)
	return scriptCount >= 3 || hasRoot
}

func extractReadableText(html string) string {
	replacements := []string{
		"</p>", "\n\n",
		"</div>", "\n\n",
		"</section>", "\n\n",
		"</article>", "\n\n",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
	}
	for i := 0; i < len(replacements); i += 2 {
		html = strings.ReplaceAll(html, replacements[i], replacements[i+1])
		html = strings.ReplaceAll(html, strings.ToUpper(replacements[i]), replacements[i+1])
	}
	text := normalizeText(html)
	lines := strings.Split(text, "\n")
	var nonEmpty []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}
