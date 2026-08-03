package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestLoadMigratesLegacyModelLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := `llm:
  providers:
    - id: openai
      name: OpenAI
      provider: openai
      base_url: "https://api.openai.com/v1"
  model_limits:
    - model: gpt-4o
      context_window: 128000
      max_output: 16384
    - model: claude-sonnet-4
      context_window: 200000
      max_output: 64000
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.LLM.Models) < 2 {
		t.Fatalf("migrated models len=%d, want >=2; models=%+v", len(cfg.LLM.Models), cfg.LLM.Models)
	}
	if cfg.LLM.Models[0].Model != "gpt-4o" || cfg.LLM.Models[0].ContextWindow != 128000 {
		t.Fatalf("first migrated model = %+v", cfg.LLM.Models[0])
	}
	if cfg.LLM.Models[1].Model != "claude-sonnet-4" || cfg.LLM.Models[1].MaxOutput != 64000 {
		t.Fatalf("second migrated model = %+v", cfg.LLM.Models[1])
	}
	foundK3 := false
	for _, m := range cfg.LLM.Models {
		if m.Model == "k3-256k" {
			foundK3 = true
			break
		}
	}
	if !foundK3 {
		t.Fatal("expected built-in k3-256k appended after migrate")
	}

	// Saving any section must persist llm.models and drop the legacy key.
	if err := l.Save(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "models:") {
		t.Fatalf("expected models key after save, got:\n%s", text)
	}
	if strings.Contains(text, "model_limits:") {
		t.Fatalf("expected model_limits removed after save, got:\n%s", text)
	}

	l2 := NewLoader(path)
	cfg2, err := l2.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.LLM.Models) < 2 {
		t.Fatalf("reload models len=%d, want >=2", len(cfg2.LLM.Models))
	}
}

func TestLoadSeedsDefaultModelsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  providers: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.LLM.Models) == 0 {
		t.Fatal("expected default model catalog when llm.models is empty")
	}
	found := false
	for _, m := range cfg.LLM.Models {
		if m.Model == "gpt-4o" && m.ContextWindow > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected gpt-4o in default catalog, got %d entries", len(cfg.LLM.Models))
	}
}

func TestLoadKeepsExplicitModels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `llm:
  models:
    - model: custom-only
      context_window: 42000
      max_output: 1000
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(path)
	cfg, err := l.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.LLM.Models) < 1 || cfg.LLM.Models[0].Model != "custom-only" {
		t.Fatalf("unexpected models: %+v", cfg.LLM.Models)
	}
	foundK3 := false
	for _, m := range cfg.LLM.Models {
		if m.Model == "k3-256k" && m.ReasoningDialect == "kimi_k3" {
			foundK3 = true
			break
		}
	}
	if !foundK3 {
		t.Fatal("expected built-in k3-256k merged into catalog")
	}
}

func TestRefreshModelConfigsOverlaysDialect(t *testing.T) {
	existing := []domain.ModelConfig{
		{Model: "k3", ContextWindow: 1, AvailableEfforts: []string{"off"}},
		{Model: "custom-only", ContextWindow: 42},
	}
	out := RefreshModelConfigs(existing, DefaultModelConfigs())
	if out[0].Model != "k3" || out[0].ReasoningDialect != "kimi_k3" {
		t.Fatalf("k3: %+v", out[0])
	}
	if out[0].ContextWindow != 1000000 {
		t.Fatalf("k3 window: %d", out[0].ContextWindow)
	}
	if len(out[0].AvailableEfforts) < 3 || out[0].AvailableEfforts[0] == "off" {
		t.Fatalf("k3 efforts: %v", out[0].AvailableEfforts)
	}
	if out[1].Model != "custom-only" || out[1].ContextWindow != 42 {
		t.Fatalf("custom: %+v", out[1])
	}
	found := false
	for _, m := range out {
		if m.Model == "k3-256k" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing k3-256k")
	}
}

