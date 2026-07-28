package netproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestStartEmptyAllowlist(t *testing.T) {
	if _, err := Start(nil); err == nil {
		t.Fatal("expected error on empty allowlist")
	}
	if _, err := Start([]string{"  ", ""}); err == nil {
		t.Fatal("expected error on blank allowlist")
	}
}

func TestProxyHTTPAllowAndDeny(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-allow")
	}))
	defer upstream.Close()
	upURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address == "allowed.example:"+upURL.Port() {
			return net.DialTimeout(network, upURL.Host, timeout)
		}
		return nil, fmt.Errorf("unexpected dial %s", address)
	}
	proxy, err := StartWithDial([]string{"allowed.example"}, dial)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse("http://" + proxy.Addr())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			DisableKeepAlives: true,
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://allowed.example:" + upURL.Port() + "/path")
	if err != nil {
		t.Fatalf("allow request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "hello-allow" {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}

	resp2, err := client.Get("http://denied.example:" + upURL.Port() + "/")
	if err != nil {
		t.Fatalf("deny request err: %v", err)
	}
	_, _ = io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("deny status=%d", resp2.StatusCode)
	}

	// IP literals are never allowlisted (Match rejects them before dial).
	resp3, err := client.Get(upstream.URL + "/x")
	if err != nil {
		t.Fatalf("ip request err: %v", err)
	}
	_, _ = io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusForbidden {
		t.Fatalf("ip status=%d", resp3.StatusCode)
	}
}

func TestProxyCONNECTAllowAndDeny(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "via-connect")
	}))
	defer upstream.Close()
	upURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	dial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		if address == net.JoinHostPort("allowed.example", upURL.Port()) {
			return net.DialTimeout(network, upURL.Host, timeout)
		}
		return nil, fmt.Errorf("unexpected dial %s", address)
	}
	proxy, err := StartWithDial([]string{"allowed.example"}, dial)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()

	proxyURL, _ := url.Parse("http://" + proxy.Addr())
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			// Force CONNECT even for http by using a custom scheme dance:
			// http.Transport uses CONNECT only for HTTPS. Dial CONNECT manually.
		},
		Timeout: 5 * time.Second,
	}
	_ = client

	conn, err := net.DialTimeout("tcp", proxy.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "CONNECT allowed.example:%s HTTP/1.1\r\nHost: allowed.example:%s\r\n\r\n", upURL.Port(), upURL.Port())
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !containsHTTP200(string(buf[:n])) {
		t.Fatalf("CONNECT allow response: %q", buf[:n])
	}

	conn2, err := net.DialTimeout("tcp", proxy.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	_, _ = fmt.Fprintf(conn2, "CONNECT denied.example:%s HTTP/1.1\r\nHost: denied.example:%s\r\n\r\n", upURL.Port(), upURL.Port())
	n, err = conn2.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if containsHTTP200(string(buf[:n])) {
		t.Fatalf("CONNECT deny should not establish: %q", buf[:n])
	}
	if !containsStatus(string(buf[:n]), "403") {
		t.Fatalf("CONNECT deny response: %q", buf[:n])
	}
}

func containsHTTP200(s string) bool {
	return len(s) >= 12 && (s[:12] == "HTTP/1.1 200" || s[:12] == "HTTP/1.0 200")
}

func containsStatus(s, code string) bool {
	return len(s) >= 12 && s[9:12] == code
}
