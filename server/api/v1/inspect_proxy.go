package v1

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type proxyTarget struct {
	Scheme string
	Host   string
	Path   string
}

func parseProxyTarget(raw string) (proxyTarget, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "/")
	if raw == "" {
		return proxyTarget{}, fmt.Errorf("target is required")
	}
	scheme := "http"
	rest := raw
	if strings.HasPrefix(rest, "https/") || rest == "https" {
		scheme = "https"
		rest = strings.TrimPrefix(rest, "https")
		rest = strings.TrimPrefix(rest, "/")
	} else if strings.HasPrefix(rest, "http/") || rest == "http" {
		scheme = "http"
		rest = strings.TrimPrefix(rest, "http")
		rest = strings.TrimPrefix(rest, "/")
	}
	if rest == "" {
		return proxyTarget{}, fmt.Errorf("host is required")
	}
	hostPart, pathPart, found := strings.Cut(rest, "/")
	host := strings.Replace(hostPart, "-", ":", 1)
	if host == "" || strings.Contains(host, "://") {
		return proxyTarget{}, fmt.Errorf("invalid host")
	}
	path := "/"
	if found {
		path = "/" + pathPart
	}
	if path == "" {
		path = "/"
	}
	return proxyTarget{Scheme: scheme, Host: host, Path: path}, nil
}

func (t proxyTarget) URL(rawQuery string) string {
	path := t.Path
	if path == "" {
		path = "/"
	}
	u := t.Scheme + "://" + t.Host + path
	if rawQuery != "" {
		u += "?" + rawQuery
	}
	return u
}

func injectBaseAndInspect(body []byte, baseTag string) []byte {
	lower := bytes.ToLower(body)
	if !bytes.Contains(lower, []byte("<base")) {
		idx := bytes.Index(lower, []byte("<head>"))
		if idx >= 0 {
			insertAt := idx + len("<head>")
			out := make([]byte, 0, len(body)+len(baseTag))
			out = append(out, body[:insertAt]...)
			out = append(out, []byte(baseTag)...)
			out = append(out, body[insertAt:]...)
			body = out
		}
	}
	return injectInspectHTML(body)
}

func injectInspectHTML(body []byte) []byte {
	script := []byte(dqInspectScript)
	if i := bytes.LastIndex(bytes.ToLower(body), []byte("</body>")); i >= 0 {
		out := make([]byte, 0, len(body)+len(script))
		out = append(out, body[:i]...)
		out = append(out, script...)
		out = append(out, body[i:]...)
		return out
	}
	return append(append([]byte{}, body...), script...)
}

func relaxFrameHeaders(h http.Header) {
	h.Del("X-Frame-Options")
	csp := h.Get("Content-Security-Policy")
	if csp == "" {
		return
	}
	directives := strings.Split(csp, ";")
	var kept []string
	for _, d := range directives {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		low := strings.ToLower(d)
		if strings.HasPrefix(low, "frame-ancestors") {
			kept = append(kept, "frame-ancestors *")
			continue
		}
		if strings.HasPrefix(low, "script-src") && !strings.Contains(low, "'unsafe-inline'") {
			kept = append(kept, d+" 'unsafe-inline'")
			continue
		}
		kept = append(kept, d)
	}
	h.Set("Content-Security-Policy", strings.Join(kept, "; "))
}

func copyProxyResponseHeaders(dst http.Header, src http.Header) {
	skip := map[string]bool{
		"Content-Length":    true,
		"Content-Encoding":  true,
		"Transfer-Encoding": true,
		"Connection":        true,
	}
	for k, vs := range src {
		if skip[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	relaxFrameHeaders(dst)
}

var proxyHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func doProxyGet(c *http.Request, targetURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if ua := c.Header.Get("User-Agent"); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if cookie := c.Header.Get("Cookie"); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if auth := c.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if al := c.Header.Get("Accept-Language"); al != "" {
		req.Header.Set("Accept-Language", al)
	}
	req.Header.Set("Accept", c.Header.Get("Accept"))
	return proxyHTTPClient.Do(req)
}

func readProxyBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
