package service

import (
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func TestLoadHooksConfigValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hooks.json")
	body := `{
  "version": 1,
  "hooks": {
    "subagentStart": [
      {"matcher": "novel", "hooks": [{"type": "command", "command": "python3 ${PLUGIN_DIR}/scripts/x.py", "timeoutSec": 5}]}
    ]
  }
}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadHooksConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	hooks := resolveHooks("novel", dir, cfg)
	if len(hooks) != 1 {
		t.Fatalf("want 1 resolved hook, got %d", len(hooks))
	}
	h := hooks[0]
	if h.Event != domain.HookEventSubagentStart || h.Matcher != "novel" || h.TimeoutSec != 5 || h.PluginRoot != dir {
		t.Fatalf("bad resolved hook: %+v", h)
	}
}

func TestLoadHooksConfigRejects(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"bad event":    `{"hooks": {"preToolUse": [{"hooks": [{"type": "command", "command": "x"}]}]}}`,
		"bad handler":  `{"hooks": {"subagentStart": [{"hooks": [{"type": "prompt"}]}]}}`,
		"empty command": `{"hooks": {"subagentStart": [{"hooks": [{"type": "command", "command": "  "}]}]}}`,
		"bad json":     `{not json`,
	}
	for name, body := range cases {
		p := filepath.Join(dir, name+".json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadHooksConfig(p); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}
