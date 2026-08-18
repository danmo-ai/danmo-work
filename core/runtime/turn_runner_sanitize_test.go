package runtime

import "testing"

// TestSanitizeToolArgsStripsReservedKeys guards the egress permission gate:
// conditionally-injected grant keys must not be forgeable via model arguments.
func TestSanitizeToolArgsStripsReservedKeys(t *testing.T) {
	got := sanitizeToolArgs(map[string]any{
		"url":                     "https://evil.example",
		"__sandbox_allow_network": true,
		"__granted_domain":        "evil.example",
		"__work_dir":              "/",
	})
	if _, ok := got["__sandbox_allow_network"]; ok {
		t.Fatal("model-forged __sandbox_allow_network must be stripped")
	}
	if _, ok := got["__granted_domain"]; ok {
		t.Fatal("model-forged __granted_domain must be stripped")
	}
	if _, ok := got["__work_dir"]; ok {
		t.Fatal("reserved keys must be stripped before runtime injection")
	}
	if got["url"] != "https://evil.example" {
		t.Fatal("normal arguments must survive")
	}
	if out := sanitizeToolArgs(nil); out == nil {
		t.Fatal("nil args must become empty map")
	}
}
