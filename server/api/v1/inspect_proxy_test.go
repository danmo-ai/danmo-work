package v1

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestParseProxyTarget(t *testing.T) {
	cases := []struct {
		raw    string
		scheme string
		host   string
		path   string
	}{
		{raw: "localhost-3000/app", scheme: "http", host: "localhost:3000", path: "/app"},
		{raw: "https/localhost-5173/foo", scheme: "https", host: "localhost:5173", path: "/foo"},
		{raw: "http/127.0.0.1-8080/", scheme: "http", host: "127.0.0.1:8080", path: "/"},
		{raw: "localhost-3000", scheme: "http", host: "localhost:3000", path: "/"},
	}
	for _, tc := range cases {
		got, err := parseProxyTarget(tc.raw)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if got.Scheme != tc.scheme || got.Host != tc.host || got.Path != tc.path {
			t.Fatalf("%s: got %+v want scheme=%s host=%s path=%s", tc.raw, got, tc.scheme, tc.host, tc.path)
		}
		if !strings.HasPrefix(got.URL(""), tc.scheme+"://"+tc.host) {
			t.Fatalf("url %s", got.URL(""))
		}
	}
}

func TestRelaxFrameHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; frame-ancestors 'none'")
	relaxFrameHeaders(h)
	if h.Get("X-Frame-Options") != "" {
		t.Fatal("expected X-Frame-Options stripped")
	}
	csp := h.Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors *") || !strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("csp not relaxed: %s", csp)
	}
}

func TestInjectInspectHTML(t *testing.T) {
	in := []byte("<html><body><h1>hi</h1></body></html>")
	out := injectInspectHTML(in)
	if !bytes.Contains(out, []byte("__dqInspectInstalled")) {
		t.Fatal("inspect script not injected")
	}
	idxScript := bytes.Index(out, []byte("__dqInspectInstalled"))
	idxBody := bytes.Index(out, []byte("</body>"))
	if idxScript < 0 || idxBody < 0 || idxScript > idxBody {
		t.Fatal("script should appear before </body>")
	}
}

func TestProjectRawBaseTag(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{path: "/api/v1/projects/p1/raw/index.html", want: `<base href="/api/v1/projects/p1/raw/">`},
		{path: "/api/v1/projects/p1/raw/docs/demo.html", want: `<base href="/api/v1/projects/p1/raw/docs/">`},
		{path: "/api/v1/projects/p1/raw/a/b/c.html", want: `<base href="/api/v1/projects/p1/raw/a/b/">`},
	}
	for _, tc := range cases {
		if got := projectRawBaseTag(tc.path); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.path, got, tc.want)
		}
	}
}

func TestInjectBaseAndInspect(t *testing.T) {
	in := []byte("<html><head><title>t</title></head><body>hi</body></html>")
	out := injectBaseAndInspect(in, `<base href="/api/v1/projects/p/raw/docs/">`)
	if !bytes.Contains(out, []byte(`<base href="/api/v1/projects/p/raw/docs/">`)) {
		t.Fatal("base tag not injected")
	}
	if !bytes.Contains(out, []byte("__dqInspectInstalled")) {
		t.Fatal("inspect script not injected")
	}
	// Existing <base> must not be duplicated.
	withBase := []byte(`<html><head><base href="/other/"><title>t</title></head><body></body></html>`)
	out2 := injectBaseAndInspect(withBase, `<base href="/api/v1/projects/p/raw/docs/">`)
	if bytes.Count(bytes.ToLower(out2), []byte("<base")) != 1 {
		t.Fatalf("expected single base tag, got %s", out2)
	}
}
