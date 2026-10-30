package container

import (
	"strings"
	"testing"
)

func TestReleaseDownloadURLLatest(t *testing.T) {
	u := ReleaseDownloadURL("dev", "amd64")
	if !strings.Contains(u, "/releases/latest/download/danmo-work-env-linux-amd64.tar.gz") {
		t.Fatalf("url=%s", u)
	}
}

func TestReleaseDownloadURLDesktop(t *testing.T) {
	u := ReleaseDownloadURLProfile("dev", "desktop", "amd64")
	if !strings.Contains(u, "/releases/latest/download/danmo-work-env-desktop-linux-amd64.tar.gz") {
		t.Fatalf("url=%s", u)
	}
	if AssetFileNameProfile("desktop", "arm64") != "danmo-work-env-desktop-linux-arm64.tar.gz" {
		t.Fatal(AssetFileNameProfile("desktop", "arm64"))
	}
}

func TestReleaseDownloadURLTagged(t *testing.T) {
	u := ReleaseDownloadURL("0.9.2", "arm64")
	if !strings.HasSuffix(u, "/releases/download/v0.9.2/danmo-work-env-linux-arm64.tar.gz") {
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
		if !strings.Contains(v.DownloadURL, "danmo-work-env-linux-"+v.Arch+".tar.gz") {
			t.Fatalf("url=%s", v.DownloadURL)
		}
	}
	if !seen["amd64"] || !seen["arm64"] {
		t.Fatalf("seen=%v", seen)
	}
}

func TestListTarVariantsDesktop(t *testing.T) {
	list := ListTarVariantsProfile("1.2.3", "desktop")
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	for _, v := range list {
		if v.Profile != "desktop" {
			t.Fatalf("profile=%s", v.Profile)
		}
		if !strings.Contains(v.AssetName, "desktop") {
			t.Fatalf("asset=%s", v.AssetName)
		}
	}
}

func TestFormatPortPublish(t *testing.T) {
	got := formatPortPublish(PortPublish{ContainerPort: 9222})
	if got != "127.0.0.1::9222/tcp" {
		t.Fatalf("got %s", got)
	}
	got = formatPortPublish(PortPublish{HostIP: "0.0.0.0", HostPort: 9333, ContainerPort: 6080, Proto: "tcp"})
	if got != "0.0.0.0:9333:6080/tcp" {
		t.Fatalf("got %s", got)
	}
}

func TestParseDockerPortsJSON(t *testing.T) {
	raw := `{"9222/tcp":[{"HostIp":"127.0.0.1","HostPort":"49152"}],"6080/tcp":[{"HostIp":"127.0.0.1","HostPort":"49153"}]}`
	ports, err := parseDockerPortsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ports[9222] != 49152 || ports[6080] != 49153 {
		t.Fatalf("%v", ports)
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
