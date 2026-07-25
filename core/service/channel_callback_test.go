package service

import (
	"testing"

	"danmo-work/core/port"
)

func TestEncodeDecodeCallback(t *testing.T) {
	tok := EncodeCallback(port.InteractionPermission, "apr_1", "once")
	kind, id, opt, ok := DecodeCallback(tok)
	if !ok || kind != port.InteractionPermission || id != "apr_1" || opt != "once" {
		t.Fatalf("got %v %q %q %v", kind, id, opt, ok)
	}
	tok2 := EncodeCallback(port.InteractionProject, "proj1", "")
	kind, id, opt, ok = DecodeCallback(tok2)
	if !ok || kind != port.InteractionProject || id != "proj1" || opt != "" {
		t.Fatalf("project: %v %q %q %v", kind, id, opt, ok)
	}
}

func TestResolvePermissionReply(t *testing.T) {
	okA, scope, ok := resolvePermissionReply("1")
	if !ok || !okA || scope != "once" {
		t.Fatal("once")
	}
	okA, scope, ok = resolvePermissionReply("拒绝")
	if !ok || okA {
		t.Fatal("deny")
	}
}

func TestIsProjectCommand(t *testing.T) {
	if !isProjectCommand("/project") || !isProjectCommand("/bot-project foo") {
		t.Fatal("expected project cmds")
	}
	if isProjectCommand("hello") {
		t.Fatal("not a project cmd")
	}
}
