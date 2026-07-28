package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"danmo-work/core/domain"
)

const (
	defaultHTTPRequestMaxChars = 8000
	minHTTPRequestMaxChars     = 100
	maxHTTPRequestMaxChars     = 100_000
)

// HTTPRequest is a constrained HTTP client for text/JSON APIs (not a full curl).
type HTTPRequest struct {
	ConfigFunc func(context.Context) (domain.SearchConfig, error)
	Egress     HostEgressChecker
}

func (h *HTTPRequest) Name() string                { return "http_request" }
func (h *HTTPRequest) RiskLevel() domain.RiskLevel { return domain.RiskMedium }

func (h *HTTPRequest) Describe(args map[string]any) string {
	method, _ := args["method"].(string)
	urlStr, _ := args["url"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	if len(urlStr) > 80 {
		urlStr = urlStr[:80] + "..."
	}
	return method + " " + urlStr
}

func (h *HTTPRequest) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "http_request",
		Description: "Call an external HTTP/HTTPS API and return status + text body.\n\n" +
			"- method: GET (default), HEAD, POST, PUT, PATCH, DELETE, OPTIONS.\n" +
			"- url: HTTP/HTTPS URL (required). Private/local addresses are blocked (SSRF).\n" +
			"- headers: optional string map (e.g. Authorization, Content-Type, Accept).\n" +
			"- body: optional UTF-8 text body (JSON/XML/form-urlencoded). Binary and multipart uploads are not supported.\n" +
			"- timeout_ms / max_chars: request timeout and response truncation.\n" +
			"- Prefer this over exec_shell curl for REST APIs. Prefer web_fetch for reading web pages.\n" +
			"- Non-text responses return Content-Type and byte length only (no base64).\n" +
			"- HTTP URLs on port 80 are automatically upgraded to HTTPS.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method (default GET)",
					"enum":        []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				},
				"url": map[string]any{"type": "string", "description": "HTTP/HTTPS URL"},
				"headers": map[string]any{
					"type":        "object",
					"description": "Optional request headers (string values)",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
				"body":       map[string]any{"type": "string", "description": "Optional UTF-8 request body"},
				"timeout_ms": map[string]any{"type": "integer", "description": "Timeout in milliseconds (default 15000)"},
				"max_chars":  map[string]any{"type": "integer", "description": "Maximum response body characters (default 8000)"},
			},
			"required": []string{"url"},
		},
	}
}

func (h *HTTPRequest) currentConfig(ctx context.Context) domain.SearchConfig {
	if h.ConfigFunc == nil {
		return domain.SearchConfig{}
	}
	cfg, err := h.ConfigFunc(ctx)
	if err != nil {
		return domain.SearchConfig{}
	}
	return cfg
}

func (h *HTTPRequest) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	urlStr, _ := input["url"].(string)
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return domain.ToolResult{}, fmt.Errorf("url is required")
	}
	urlStr = upgradeToHTTPS(urlStr)

	method, _ := input["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	if !isAllowedHTTPMethod(method) {
		return domain.ToolResult{}, fmt.Errorf("unsupported method %q (allowed: GET HEAD POST PUT PATCH DELETE OPTIONS)", method)
	}

	body, _ := input["body"].(string)
	if body != "" {
		if !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
			return domain.ToolResult{}, fmt.Errorf("body must be UTF-8 text without null bytes; binary payloads are not supported")
		}
		if method == http.MethodGet || method == http.MethodHead {
			return domain.ToolResult{}, fmt.Errorf("body is not allowed for %s", method)
		}
	}

	headers, err := parseStringHeaders(input["headers"])
	if err != nil {
		return domain.ToolResult{}, err
	}

	cfg := h.currentConfig(ctx)
	timeoutMs := intFromArg(input["timeout_ms"], cfg.TimeoutMs)
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}
	if timeoutMs > maxTimeoutMs {
		timeoutMs = maxTimeoutMs
	}

	maxChars := intFromArg(input["max_chars"], defaultHTTPRequestMaxChars)
	if maxChars < minHTTPRequestMaxChars {
		maxChars = minHTTPRequestMaxChars
	}
	if maxChars > maxHTTPRequestMaxChars {
		maxChars = maxHTTPRequestMaxChars
	}

	var bodyReader io.Reader
	var getBody func() (io.ReadCloser, error)
	if body != "" {
		bodyBytes := []byte(body)
		bodyReader = bytes.NewReader(bodyBytes)
		getBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return domain.ToolResult{}, err
	}
	req.GetBody = getBody

	req.Header.Set("User-Agent", effectiveUserAgent(webClientOpts{UserAgent: cfg.UserAgent}))
	req.Header.Set("Accept", "application/json, text/plain, text/*, application/xml, */*;q=0.1")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		trimmed := strings.TrimSpace(body)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		}
	}

	opts := withToolEgress(clientOpts(cfg.Proxy, cfg.UserAgent, time.Duration(timeoutMs)*time.Millisecond, false), h.Egress, input)
	resp, err := doRequest(ctx, req, opts)
	if err != nil {
		return domain.ToolResult{}, err
	}

	raw, err := readResponseBody(resp, maxFetchBytes)
	if err != nil {
		return domain.ToolResult{}, err
	}

	ct := resp.Header.Get("Content-Type")
	out := map[string]any{
		"status":       resp.StatusCode,
		"status_text":  resp.Status,
		"url":          resp.Request.URL.String(),
		"content_type": ct,
		"bytes":        len(raw),
	}

	if isNonTextHTTPBody(ct, raw) {
		out["body"] = ""
		out["truncated"] = true
		out["message"] = fmt.Sprintf("non-text response (%s, %d bytes); binary bodies are not returned", emptyDefault(ct, "unknown"), len(raw))
	} else {
		text := string(raw)
		if !utf8.ValidString(text) {
			out["body"] = ""
			out["truncated"] = true
			out["message"] = fmt.Sprintf("response is not valid UTF-8 (%d bytes); binary bodies are not returned", len(raw))
		} else {
			truncated := false
			if len(text) > maxChars {
				text = text[:maxChars]
				truncated = true
			}
			out["body"] = text
			out["truncated"] = truncated
		}
	}

	// Surface a few useful response headers (never dump Set-Cookie values fully).
	if loc := resp.Header.Get("Location"); loc != "" {
		out["location"] = loc
	}

	b, err := json.Marshal(out)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: string(b)}, nil
}

func isAllowedHTTPMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func parseStringHeaders(v any) (map[string]string, error) {
	if v == nil {
		return nil, nil
	}
	switch h := v.(type) {
	case map[string]string:
		return h, nil
	case map[string]any:
		out := make(map[string]string, len(h))
		for k, raw := range h {
			s, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("headers.%s must be a string", k)
			}
			out[k] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("headers must be an object of string values")
	}
}

func intFromArg(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return fallback
}

func isNonTextHTTPBody(contentType string, body []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case ct == "":
		// fall through to byte inspection
	case strings.HasPrefix(ct, "text/"),
		strings.HasPrefix(ct, "application/json"),
		strings.HasPrefix(ct, "application/ld+json"),
		strings.HasPrefix(ct, "application/xml"),
		strings.HasPrefix(ct, "application/xhtml+xml"),
		strings.HasPrefix(ct, "application/javascript"),
		strings.HasPrefix(ct, "application/x-www-form-urlencoded"),
		strings.HasPrefix(ct, "application/graphql"),
		strings.HasPrefix(ct, "application/problem+json"),
		strings.HasSuffix(ct, "+json"),
		strings.HasSuffix(ct, "+xml"):
		return false
	case strings.HasPrefix(ct, "image/"),
		strings.HasPrefix(ct, "audio/"),
		strings.HasPrefix(ct, "video/"),
		strings.HasPrefix(ct, "multipart/"),
		ct == "application/octet-stream",
		ct == "application/pdf",
		ct == "application/zip",
		ct == "application/gzip",
		ct == "application/x-tar",
		ct == "application/protobuf",
		ct == "application/x-protobuf":
		return true
	default:
		if strings.HasPrefix(ct, "application/") {
			// Unknown application/* — treat as non-text unless it looks like UTF-8 text.
			return !looksLikeUTF8Text(body)
		}
	}
	return !looksLikeUTF8Text(body)
}

func looksLikeUTF8Text(body []byte) bool {
	if len(body) == 0 {
		return true
	}
	if bytes.IndexByte(body, 0) >= 0 {
		return false
	}
	if !utf8.Valid(body) {
		return false
	}
	// Reject if too many non-printable control chars.
	sample := body
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	bad := 0
	for _, b := range sample {
		if b < 0x09 || (b > 0x0d && b < 0x20) {
			bad++
		}
	}
	return bad*20 <= len(sample) // <=5% control bytes
}

func emptyDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
