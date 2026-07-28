package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const webUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

const (
	defaultTimeoutMs      = 15000
	maxTimeoutMs          = 60000
	errorBodyPreviewBytes = 512
	maxFetchBytes         = 10 * 1024 * 1024
	maxHTTPRetries        = 3
)

type webClientOpts struct {
	Timeout    time.Duration
	Proxy      string
	UserAgent  string
	SkipSSRF   bool
	SkipEgress bool // session full-network grant under deny
	Egress     HostEgressChecker
}

func effectiveUserAgent(opts webClientOpts) string {
	if ua := strings.TrimSpace(opts.UserAgent); ua != "" {
		return ua
	}
	return webUserAgent
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed")
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

// upgradeToHTTPS upgrades http:// to https:// when the port is empty or 80.
func upgradeToHTTPS(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" {
		return raw
	}
	port := u.Port()
	if port != "" && port != "80" {
		return raw
	}
	u.Scheme = "https"
	u.Host = u.Hostname()
	return u.String()
}

// testSkipSSRF is set by unit tests that use httptest (loopback).
var testSkipSSRF bool

// HostEgressChecker is optional Hard egress policy (sandbox network allowlist/deny).
// Satisfied by port.EgressAuthority / port.Sandbox (CheckHost + ProxyURL).
type HostEgressChecker interface {
	CheckHost(host string) error
	ProxyURL() string
}

func assertPublicURL(raw string) error {
	return assertPublicURLWithEgress(raw, nil)
}

func assertPublicURLWithEgress(raw string, egress HostEgressChecker) error {
	if testSkipSSRF {
		return nil
	}
	if err := validateURL(raw); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if egress != nil {
		if err := egress.CheckHost(host); err != nil {
			return err
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("blocked: requests to private or local addresses are not allowed")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns lookup returned no addresses")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return fmt.Errorf("blocked: host resolves to a private or local address")
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	// Cloud metadata / common blocked ranges
	if ip4 := ip.To4(); ip4 != nil {
		if ip4.Equal(net.ParseIP("169.254.169.254")) {
			return false
		}
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return false
		}
	}
	return true
}

func newWebClient(opts webClientOpts) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	jar, _ := cookiejar.New(nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy := strings.TrimSpace(opts.Proxy); proxy != "" {
		if u, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{
		Timeout:   timeout,
		Jar:       jar,
		Transport: transport,
	}
}

func setBrowserGETHeaders(req *http.Request, opts webClientOpts) {
	req.Header.Set("User-Agent", effectiveUserAgent(opts))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

func doRequest(ctx context.Context, req *http.Request, opts webClientOpts) (*http.Response, error) {
	if !opts.SkipSSRF {
		var eg HostEgressChecker
		if !opts.SkipEgress {
			eg = opts.Egress
		}
		if err := assertPublicURLWithEgress(req.URL.String(), eg); err != nil {
			return nil, err
		}
	} else if err := validateURL(req.URL.String()); err != nil {
		return nil, err
	} else if opts.Egress != nil && !opts.SkipEgress && req.URL != nil {
		if err := opts.Egress.CheckHost(req.URL.Hostname()); err != nil {
			return nil, err
		}
	}

	// Prefer sandbox allowlist proxy when active (unless caller set an explicit proxy).
	if opts.Proxy == "" && opts.Egress != nil {
		if p := opts.Egress.ProxyURL(); p != "" {
			opts.Proxy = p
		}
	}

	client := newWebClient(opts)
	var lastErr error
	for attempt := 0; attempt < maxHTTPRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			// Rebuild body for POST retries
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				req.Body = body
			}
		}

		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = err
			if !isRetryableNetErr(err) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt+1 < maxHTTPRetries {
				if retryAfter > 0 {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(retryAfter):
					}
				}
				continue
			}
			return nil, lastErr
		}
		return resp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after retries")
}

func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "eof")
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		d := time.Duration(secs) * time.Second
		if d > 30*time.Second {
			d = 30 * time.Second
		}
		return d
	}
	return 0
}

func fetchWithOpts(ctx context.Context, urlStr string, opts webClientOpts) (*http.Response, error) {
	if err := validateURL(urlStr); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	setBrowserGETHeaders(req, opts)
	return doRequest(ctx, req, opts)
}

func postJSON(ctx context.Context, urlStr string, payload any, headers map[string]string, opts webClientOpts) (*http.Response, error) {
	if err := validateURL(urlStr); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", effectiveUserAgent(opts))
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return doRequest(ctx, req, opts)
}

func readResponseBody(resp *http.Response, limit int64) ([]byte, error) {
	defer resp.Body.Close()
	if limit > 0 {
		return io.ReadAll(io.LimitReader(resp.Body, limit))
	}
	return io.ReadAll(resp.Body)
}

func truncateErrorBody(body string) string {
	stripped := sanitizeErrorBody(body)
	if len(stripped) <= errorBodyPreviewBytes {
		return stripped
	}
	return stripped[:errorBodyPreviewBytes] + "..."
}

func sanitizeErrorBody(body string) string {
	visible := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, stripHTMLTags(body))
	re := regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	return re.ReplaceAllString(visible, "Bearer [REDACTED]")
}

func stripHTMLTags(s string) string {
	var out strings.Builder
	inTag := false
	for i := 0; i < len(s); {
		if i+7 < len(s) && strings.ToLower(s[i:i+7]) == "<script" {
			if end := strings.Index(strings.ToLower(s[i:]), "</script>"); end >= 0 {
				i += end + 9
				continue
			}
		}
		c := s[i]
		if c == '<' {
			inTag = true
			i++
			continue
		}
		if c == '>' {
			inTag = false
			i++
			continue
		}
		if !inTag {
			out.WriteByte(c)
		}
		i++
	}
	return out.String()
}

func normalizeText(s string) string {
	s = htmlUnescape(s)
	s = stripHTMLTags(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func htmlUnescape(s string) string {
	replacements := map[string]string{
		"\u0026amp;":  "\u0026",
		"\u0026lt;":   "\u003c",
		"\u0026gt;":   "\u003e",
		"\u0026quot;": `"`,
		"\u0026#39;":  "'",
		"\u0026nbsp;": " ",
	}
	for old, new := range replacements {
		s = strings.ReplaceAll(s, old, new)
	}
	return s
}

func decodeHTMLEntities(s string) string {
	re := regexp.MustCompile(`&(?:#(\d+)|#x([0-9A-Fa-f]+)|([a-zA-Z]+));`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		groups := re.FindStringSubmatch(match)
		if groups[1] != "" {
			var n int
			_, _ = fmt.Sscanf(groups[1], "%d", &n)
			if n > 0 {
				return string(rune(n))
			}
		}
		if groups[2] != "" {
			var n int
			_, _ = fmt.Sscanf(groups[2], "%x", &n)
			if n > 0 {
				return string(rune(n))
			}
		}
		switch groups[3] {
		case "amp":
			return "&"
		case "lt":
			return "<"
		case "gt":
			return ">"
		case "quot":
			return `"`
		case "apos":
			return "'"
		case "nbsp":
			return " "
		case "copy":
			return "\u00A9"
		case "reg":
			return "\u00AE"
		case "mdash":
			return "\u2014"
		case "ndash":
			return "\u2013"
		case "lsquo":
			return "\u2018"
		case "rsquo":
			return "\u2019"
		case "ldquo":
			return "\u201C"
		case "rdquo":
			return "\u201D"
		case "hellip":
			return "\u2026"
		}
		return match
	})
}

func firstString(m map[string]any, keys []string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func withToolEgress(opts webClientOpts, egress HostEgressChecker, input map[string]any) webClientOpts {
	opts.Egress = egress
	if input != nil {
		if v, ok := input["__sandbox_allow_network"].(bool); ok && v {
			opts.SkipEgress = true
		}
	}
	return opts
}

func clientOpts(proxy, userAgent string, timeout time.Duration, skipSSRF bool) webClientOpts {
	return webClientOpts{
		Timeout:   timeout,
		Proxy:     proxy,
		UserAgent: userAgent,
		SkipSSRF:  skipSSRF,
	}
}
