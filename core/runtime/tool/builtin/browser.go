package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func sessionIDFromInput(input map[string]any) string {
	s, _ := input["__session_id"].(string)
	return strings.TrimSpace(s)
}

func browserEgressFromInput(input map[string]any, egress HostEgressChecker) HostEgressChecker {
	if v, ok := input["__sandbox_allow_network"].(bool); ok && v {
		return nil
	}
	return egress
}

func formatBrowserSnapshot(title, url, snapshot string) string {
	var b strings.Builder
	if title != "" || url != "" {
		fmt.Fprintf(&b, "title: %s\nurl: %s\n\n", title, url)
	}
	b.WriteString(snapshot)
	return b.String()
}

func acquireBrowserPage(ctx context.Context, br port.Browser, input map[string]any) (port.BrowserPage, error) {
	if br == nil {
		return nil, fmt.Errorf("browser engine is not configured")
	}
	sid := sessionIDFromInput(input)
	if sid == "" {
		return nil, fmt.Errorf("session id is missing")
	}
	return br.AcquirePage(ctx, sid)
}

// BrowserNavigate opens or navigates the sticky session tab.
type BrowserNavigate struct {
	Browser port.Browser
	Egress  HostEgressChecker
}

func (h *BrowserNavigate) Name() string                { return "browser_navigate" }
func (h *BrowserNavigate) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *BrowserNavigate) Describe(args map[string]any) string {
	u, _ := args["url"].(string)
	if len(u) > 80 {
		u = u[:80] + "..."
	}
	if u == "" {
		return "browser_navigate"
	}
	return u
}
func (h *BrowserNavigate) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "browser_navigate",
		Description: "Navigate the sticky browser tab for this session to a URL and return an interactive snapshot with refs (e1, e2, …).\n\n" +
			"- Use for multi-step page interaction. For one-shot readable text extraction prefer web_fetch.\n" +
			"- url: HTTP/HTTPS URL (required).\n" +
			"- wait_until: load | domcontentloaded | networkidle (default load).\n" +
			"- After navigate, call browser_act with refs from the snapshot. Use browser_screenshot only when vision is needed.\n" +
			"- Private/local addresses are blocked (SSRF). Requires local Chrome/Edge/Chromium or runtime.browser.cdp_url.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "URL to open"},
				"wait_until": map[string]any{
					"type":        "string",
					"description": "load | domcontentloaded | networkidle (default load)",
					"enum":        []string{"load", "domcontentloaded", "networkidle"},
				},
			},
			"required": []string{"url"},
		},
	}
}

func (h *BrowserNavigate) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	urlStr, _ := input["url"].(string)
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return domain.ToolResult{}, fmt.Errorf("url is required")
	}
	urlStr = upgradeToHTTPS(urlStr)
	egress := browserEgressFromInput(input, h.Egress)
	if err := assertPublicURLWithEgress(urlStr, egress); err != nil {
		return domain.ToolResult{}, err
	}
	waitUntil := "load"
	if w, ok := input["wait_until"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(w)) {
		case "load", "domcontentloaded", "networkidle":
			waitUntil = strings.ToLower(strings.TrimSpace(w))
		}
	}
	page, err := acquireBrowserPage(ctx, h.Browser, input)
	if err != nil {
		return domain.ToolResult{}, err
	}
	res, err := page.Navigate(ctx, port.BrowserNavigateOptions{
		URL:       urlStr,
		Timeout:   45 * time.Second,
		WaitUntil: waitUntil,
	})
	if err != nil {
		return domain.ToolResult{}, err
	}
	content := formatBrowserSnapshot(res.Title, res.URL, res.Snapshot)
	return domain.ToolResult{
		Content: content,
		Meta: map[string]any{
			"op":    "browser_navigate",
			"url":   res.URL,
			"title": res.Title,
		},
	}, nil
}

// BrowserSnapshot returns the current page interactive snapshot.
type BrowserSnapshot struct {
	Browser port.Browser
}

func (h *BrowserSnapshot) Name() string                { return "browser_snapshot" }
func (h *BrowserSnapshot) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *BrowserSnapshot) Describe(args map[string]any) string {
	return "browser_snapshot"
}
func (h *BrowserSnapshot) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "browser_snapshot",
		Description: "Capture an interactive accessibility-oriented snapshot of the current sticky browser tab with refs (e1, e2, …).\n\n" +
			"- Call after navigate or when the page may have changed outside browser_act.\n" +
			"- Refs are only valid until the next snapshot/navigate/act.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (h *BrowserSnapshot) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	page, err := acquireBrowserPage(ctx, h.Browser, input)
	if err != nil {
		return domain.ToolResult{}, err
	}
	res, err := page.Snapshot(ctx)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: formatBrowserSnapshot(res.Title, res.URL, res.Snapshot),
		Meta: map[string]any{
			"op":    "browser_snapshot",
			"url":   res.URL,
			"title": res.Title,
		},
	}, nil
}

// BrowserAct performs a semantic action on the sticky page.
type BrowserAct struct {
	Browser port.Browser
}

func (h *BrowserAct) Name() string                { return "browser_act" }
func (h *BrowserAct) RiskLevel() domain.RiskLevel { return domain.RiskHigh }
func (h *BrowserAct) Describe(args map[string]any) string {
	action, _ := args["action"].(string)
	ref, _ := args["ref"].(string)
	if action == "" {
		return "browser_act"
	}
	if ref != "" {
		return action + " " + ref
	}
	return action
}
func (h *BrowserAct) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "browser_act",
		Description: "Perform a semantic action on the sticky browser tab, then return a fresh snapshot.\n\n" +
			"- action: click | type | press | scroll | select | hover.\n" +
			"- ref: required for click/type/select/hover (from the latest snapshot).\n" +
			"- text: for type (clears then types).\n" +
			"- key: for press (Enter, Tab, Escape, Backspace, ArrowUp, …).\n" +
			"- direction/amount: for scroll (up|down; amount in pixels, default 600).\n" +
			"- value: for select (option value or visible text).\n" +
			"- Prefer refs from the latest snapshot; do not guess selectors.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{"click", "type", "press", "scroll", "select", "hover"},
				},
				"ref":       map[string]any{"type": "string", "description": "Snapshot ref (e.g. e3)"},
				"text":      map[string]any{"type": "string", "description": "Text to type"},
				"key":       map[string]any{"type": "string", "description": "Key to press"},
				"direction": map[string]any{"type": "string", "enum": []string{"up", "down"}},
				"amount":    map[string]any{"type": "integer", "description": "Scroll pixels (default 600)"},
				"value":     map[string]any{"type": "string", "description": "Select option value/text"},
			},
			"required": []string{"action"},
		},
	}
}

func (h *BrowserAct) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	action, _ := input["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return domain.ToolResult{}, fmt.Errorf("action is required")
	}
	req := port.BrowserActRequest{
		Action:    action,
		Timeout:   30 * time.Second,
		Ref:       stringArg(input, "ref"),
		Text:      stringArg(input, "text"),
		Key:       stringArg(input, "key"),
		Direction: stringArg(input, "direction"),
		Value:     stringArg(input, "value"),
	}
	if a, ok := input["amount"].(float64); ok {
		req.Amount = int(a)
	}
	page, err := acquireBrowserPage(ctx, h.Browser, input)
	if err != nil {
		return domain.ToolResult{}, err
	}
	res, err := page.Act(ctx, req)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: formatBrowserSnapshot(res.Title, res.URL, res.Snapshot),
		Meta: map[string]any{
			"op":     "browser_act",
			"action": action,
			"url":    res.URL,
			"title":  res.Title,
		},
	}, nil
}

// BrowserScreenshot captures a PNG of the sticky tab.
type BrowserScreenshot struct {
	Browser       port.Browser
	SupportsImage func(modelID string) bool
}

func (h *BrowserScreenshot) Name() string                { return "browser_screenshot" }
func (h *BrowserScreenshot) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *BrowserScreenshot) Describe(args map[string]any) string {
	if v, ok := args["full_page"].(bool); ok && v {
		return "browser_screenshot full_page"
	}
	return "browser_screenshot"
}
func (h *BrowserScreenshot) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "browser_screenshot",
		Description: "Capture a PNG screenshot of the sticky browser tab and return it as an image for visual analysis.\n\n" +
			"- Prefer browser_snapshot for structured interaction; use this when layout/visual state matters.\n" +
			"- full_page: when true, capture the full scrollable page (default false = viewport).\n" +
			"- The current model must accept image input.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"full_page": map[string]any{"type": "boolean", "description": "Capture full page (default false)"},
			},
		},
	}
}

func (h *BrowserScreenshot) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.SupportsImage != nil {
		modelID, _ := input["__model_id"].(string)
		if modelID != "" && !h.SupportsImage(modelID) {
			return domain.ToolResult{}, fmt.Errorf("the current model (%s) does not accept image input — browser_screenshot is unavailable", modelID)
		}
	}
	fullPage := false
	if v, ok := input["full_page"].(bool); ok {
		fullPage = v
	}
	page, err := acquireBrowserPage(ctx, h.Browser, input)
	if err != nil {
		return domain.ToolResult{}, err
	}
	png, err := page.Screenshot(ctx, fullPage)
	if err != nil {
		return domain.ToolResult{}, err
	}
	desc := fmt.Sprintf("Browser screenshot (%d bytes, image/png)", len(png))
	if fullPage {
		desc = fmt.Sprintf("Browser full-page screenshot (%d bytes, image/png)", len(png))
	}
	return domain.ToolResult{
		Content: desc,
		Meta: map[string]any{
			"op":        "browser_screenshot",
			"mime_type": "image/png",
			"bytes":     len(png),
			"full_page": fullPage,
		},
		Parts: []domain.ToolResultPart{{
			Type:     "image",
			MimeType: "image/png",
			Data:     base64.StdEncoding.EncodeToString(png),
		}},
	}, nil
}

// BrowserClose closes the sticky browser tab for this session.
type BrowserClose struct {
	Browser port.Browser
}

func (h *BrowserClose) Name() string                { return "browser_close" }
func (h *BrowserClose) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *BrowserClose) Describe(args map[string]any) string {
	return "browser_close"
}
func (h *BrowserClose) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "browser_close",
		Description: "Close the sticky browser tab for this session (idempotent).\n\n" +
			"- Call when the interactive browsing goal is finished, unless the parent asked to keep the session.\n" +
			"- Idle tabs are also reclaimed automatically after ~15 minutes.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (h *BrowserClose) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Browser == nil {
		return domain.ToolResult{}, fmt.Errorf("browser engine is not configured")
	}
	sid := sessionIDFromInput(input)
	if sid == "" {
		return domain.ToolResult{}, fmt.Errorf("session id is missing")
	}
	if err := h.Browser.ClosePage(ctx, sid); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: "Browser tab closed",
		Meta:    map[string]any{"op": "browser_close"},
	}, nil
}
