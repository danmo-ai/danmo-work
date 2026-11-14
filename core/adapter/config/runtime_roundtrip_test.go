package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

// TestSaveLoadRuntimeApprovalRoundTrip guards against yaml.Marshal writing Go
// field names ("autoapprove") instead of the mapstructure keys used on Load
// ("auto_approve"). That mismatch silently discarded Settings changes, so the
// approval mode appeared impossible to save.
func TestSaveLoadRuntimeApprovalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	l := NewLoader(path)

	cfg, err := l.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Runtime.AutoApprove = true
	cfg.Runtime.PermissionMode = domain.PermModeAuto
	cfg.Runtime.Compaction.KeepRecentToolSteps = 7
	cfg.Runtime.Knowledge.SearchTopK = 9
	if err := l.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "auto_approve: true") || !strings.Contains(string(raw), "permission_mode: auto") {
		t.Fatalf("save did not write canonical runtime keys:\n%s", raw)
	}

	reloaded, err := NewLoader(path).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Runtime.AutoApprove {
		t.Fatal("auto_approve lost on reload")
	}
	if reloaded.Runtime.PermissionMode != domain.PermModeAuto {
		t.Fatalf("permission_mode = %q, want auto", reloaded.Runtime.PermissionMode)
	}
	if reloaded.Runtime.Compaction.KeepRecentToolSteps != 7 {
		t.Fatalf("keep_recent_tool_steps = %d, want 7", reloaded.Runtime.Compaction.KeepRecentToolSteps)
	}
	if reloaded.Runtime.Knowledge.SearchTopK != 9 {
		t.Fatalf("search_top_k = %d, want 9", reloaded.Runtime.Knowledge.SearchTopK)
	}
}

// TestLoadMigratesLegacyFieldNameKeys keeps configs already rewritten by the
// old Save path usable instead of dropping their multi-word settings.
func TestLoadMigratesLegacyFieldNameKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := `
runtime:
  autoapprove: true
  permissionmode: auto
  turn:
    doomloopthreshold: 22
  knowledge:
    searchtopk: 8
  compaction:
    keeprecenttoolsteps: 5
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader(path).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Runtime.AutoApprove || cfg.Runtime.PermissionMode != domain.PermModeAuto {
		t.Fatalf("approval not migrated: auto=%v mode=%q", cfg.Runtime.AutoApprove, cfg.Runtime.PermissionMode)
	}
	if cfg.Runtime.Turn.DoomLoopThreshold != 22 {
		t.Fatalf("doom_loop_threshold = %d, want 22", cfg.Runtime.Turn.DoomLoopThreshold)
	}
	if cfg.Runtime.Knowledge.SearchTopK != 8 {
		t.Fatalf("search_top_k = %d, want 8", cfg.Runtime.Knowledge.SearchTopK)
	}
	if cfg.Runtime.Compaction.KeepRecentToolSteps != 5 {
		t.Fatalf("keep_recent_tool_steps = %d, want 5", cfg.Runtime.Compaction.KeepRecentToolSteps)
	}

	// A reused loader must see values written by Save after the migration,
	// instead of a stale migrated override shadowing the file.
	l := NewLoader(path)
	if _, err := l.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg.Runtime.AutoApprove = false
	cfg.Runtime.PermissionMode = domain.PermModeInteractive
	if err := l.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	again, err := l.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Runtime.AutoApprove || again.Runtime.PermissionMode != domain.PermModeInteractive {
		t.Fatalf("reload shadowed saved approval: auto=%v mode=%q", again.Runtime.AutoApprove, again.Runtime.PermissionMode)
	}
}
