package netproxy

import (
	"testing"
)

func TestNormalizeDomains(t *testing.T) {
	got := NormalizeDomains([]string{" Example.COM ", "", "*.npmjs.org", "example.com"})
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "example.com" || got[1] != "*.npmjs.org" {
		t.Fatalf("got %v", got)
	}
}

func TestMatch(t *testing.T) {
	rules := []string{"github.com", "*.npmjs.org", "registry.npmjs.org"}
	cases := []struct {
		host string
		want bool
	}{
		{"github.com", true},
		{"api.github.com", false},
		{"registry.npmjs.org", true},
		{"foo.npmjs.org", true},
		{"npmjs.org", true}, // base of *.npmjs.org
		{"evil.com", false},
		{"", false},
		{"127.0.0.1", false},
		{"169.254.169.254", false},
		{"::1", false},
		{"localhost", false},
		{"foo.localhost", false},
	}
	for _, tc := range cases {
		if got := Match(tc.host, rules); got != tc.want {
			t.Errorf("Match(%q)=%v want %v", tc.host, got, tc.want)
		}
	}
}

func TestSplitHostPort(t *testing.T) {
	h, p, err := SplitHostPort("example.com:443")
	if err != nil || h != "example.com" || p != "443" {
		t.Fatalf("got %q %q %v", h, p, err)
	}
	h, p, err = SplitHostPort("example.com")
	if err != nil || h != "example.com" || p != "" {
		t.Fatalf("got %q %q %v", h, p, err)
	}
}
