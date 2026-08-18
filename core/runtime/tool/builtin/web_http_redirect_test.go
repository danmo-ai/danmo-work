package builtin

import (
	"net/http"
	"testing"
)

// TestRedirectGuardBlocksPrivateHops verifies that the HTTP client's
// CheckRedirect re-runs SSRF validation on redirect targets so a public page
// cannot bounce the fetcher into cloud-metadata or loopback addresses.
func TestRedirectGuardBlocksPrivateHops(t *testing.T) {
	if testSkipSSRF {
		t.Skip("SSRF guard disabled")
	}
	client := newWebClient(webClientOpts{})
	if client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to be set")
	}

	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:7801/internal",
		"http://10.0.0.5/",
	}
	for _, raw := range blocked {
		req, _ := http.NewRequest(http.MethodGet, raw, nil)
		if err := client.CheckRedirect(req, nil); err == nil {
			t.Fatalf("redirect to %s should be blocked", raw)
		}
	}

	// A public destination is still allowed.
	pub, _ := http.NewRequest(http.MethodGet, "http://1.1.1.1/", nil)
	if err := client.CheckRedirect(pub, nil); err != nil {
		t.Fatalf("public redirect should be allowed, got %v", err)
	}

	// Redirect cap still enforced.
	req, _ := http.NewRequest(http.MethodGet, "http://1.1.1.1/", nil)
	via := make([]*http.Request, 10)
	if err := client.CheckRedirect(req, via); err == nil {
		t.Fatal("expected redirect cap to trigger")
	}
}
