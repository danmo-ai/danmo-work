package container

import (
	"strings"
	"testing"
)

func TestReleaseDownloadURLLatest(t *testing.T) {
	u := ReleaseDownloadURL("dev", "amd64")
	if !strings.Contains(u, "/releases/latest/download/danmo-work-env-linux-amd64.tar") {
		t.Fatalf("url=%s", u)
	}
}

func TestReleaseDownloadURLTagged(t *testing.T) {
	u := ReleaseDownloadURL("0.9.2", "arm64")
	if !strings.HasSuffix(u, "/releases/download/v0.9.2/danmo-work-env-linux-arm64.tar") {
		t.Fatalf("url=%s", u)
	}
	u2 := ReleaseDownloadURL("v1.0.0", "amd64")
	if !strings.Contains(u2, "/releases/download/v1.0.0/") {
		t.Fatalf("url=%s", u2)
	}
}

func TestRewriteProxyForApple(t *testing.T) {
	got := RewriteProxyForApple("127.0.0.1:1234")
	if got != "http://host.container.internal:1234" {
		t.Fatalf("got %q", got)
	}
	got = RewriteProxyForApple("http://localhost:9")
	if got != "http://host.container.internal:9" {
		t.Fatalf("got %q", got)
	}
}
