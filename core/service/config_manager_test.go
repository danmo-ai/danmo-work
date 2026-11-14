package service

import (
	"context"
	"testing"

	"danmo-work/core/domain"
)

// TestUpdatePreservesPermissionRules guards hand-written Soft Gate rules when a
// client (Settings) updates runtime without sending permissionRules.
func TestUpdatePreservesPermissionRules(t *testing.T) {
	rules := []domain.PermissionRule{{Pattern: "mcp_*", Action: domain.PermAsk}}
	store := &memConfigStore{cfg: &domain.ConfigFile{
		Runtime: domain.ConfigRuntimeSection{PermissionRules: rules},
	}}
	m := NewConfigManager(store)

	req := &domain.ConfigRuntimeSection{AutoApprove: true, PermissionMode: domain.PermModeAuto}
	if _, err := m.Update(context.Background(), domain.UpdateConfigFileRequest{Runtime: req}); err != nil {
		t.Fatal(err)
	}
	got := store.cfg.Runtime
	if !got.AutoApprove || got.PermissionMode != domain.PermModeAuto {
		t.Fatalf("runtime approval not applied: %+v", got)
	}
	if len(got.PermissionRules) != 1 || got.PermissionRules[0].Pattern != "mcp_*" {
		t.Fatalf("permission rules dropped: %+v", got.PermissionRules)
	}

	// Explicitly clearing rules still works.
	empty := []domain.PermissionRule{}
	req2 := &domain.ConfigRuntimeSection{PermissionRules: empty}
	if _, err := m.Update(context.Background(), domain.UpdateConfigFileRequest{Runtime: req2}); err != nil {
		t.Fatal(err)
	}
	if len(store.cfg.Runtime.PermissionRules) != 0 {
		t.Fatalf("explicit empty rules not honored: %+v", store.cfg.Runtime.PermissionRules)
	}
}
