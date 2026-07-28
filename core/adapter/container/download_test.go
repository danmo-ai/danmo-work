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

func TestListTarVariants(t *testing.T) {
	list := ListTarVariants("1.2.3")
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	seen := map[string]bool{}
	for _, v := range list {
		seen[v.Arch] = true
		if !strings.Contains(v.DownloadURL, "danmo-work-env-linux-"+v.Arch+".tar") {
			t.Fatalf("url=%s", v.DownloadURL)
		}
	}
	if !seen["amd64"] || !seen["arm64"] {
		t.Fatalf("seen=%v", seen)
	}
}

func TestNormalizeArch(t *testing.T) {
	if NormalizeArch("x86_64") != "amd64" {
		t.Fatal("x86_64")
	}
	if NormalizeArch("aarch64") != "arm64" {
		t.Fatal("aarch64")
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
