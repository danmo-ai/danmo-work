package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"

	"gopkg.in/yaml.v3"
)

func TestMergeLLMPresets_FillsEmptyBaseURLAndAppendsMissing(t *testing.T) {
	defaults := defaultLLMPresets()
	existing := []domain.LLMProviderPreset{
		{ID: "openai", Name: "OpenAI", Provider: domain.LLMProviderOpenAI, BaseURL: ""},
		{ID: "deepseek", Name: "DeepSeek", Provider: "", BaseURL: ""},
		{ID: "custom", Name: "My Gateway", Provider: domain.LLMProviderOpenAI, BaseURL: "https://gw.example/v1"},
	}
	out := mergeLLMPresets(existing, defaults, nil)

	byID := map[string]domain.LLMProviderPreset{}
	for _, p := range out {
		byID[p.ID] = p
	}
	if byID["openai"].BaseURL != "https://api.openai.com/v1" {
		t.Errorf("openai base: %q", byID["openai"].BaseURL)
	}
	if byID["deepseek"].BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("deepseek base: %q", byID["deepseek"].BaseURL)
	}
	if byID["deepseek"].Provider != domain.LLMProviderOpenAI {
		t.Errorf("deepseek provider: %q", byID["deepseek"].Provider)
	}
	if byID["custom"].BaseURL != "https://gw.example/v1" {
		t.Errorf("custom base overwritten: %q", byID["custom"].BaseURL)
	}
	if _, ok := byID["anthropic"]; !ok {
		t.Fatal("missing anthropic preset should be appended")
	}
	if byID["anthropic"].BaseURL != "https://api.anthropic.com/v1" {
		t.Errorf("anthropic base: %q", byID["anthropic"].BaseURL)
	}
}

func TestMergeLLMPresets_UsesLegacyBaseURL(t *testing.T) {
	existing := []domain.LLMProviderPreset{
		{ID: "mycorp", Name: "MyCorp", Provider: domain.LLMProviderOpenAI, BaseURL: ""},
	}
	legacy := map[string]string{"mycorp": "https://llm.mycorp.example/v1"}
	out := mergeLLMPresets(existing, nil, legacy)
	if out[0].BaseURL != "https://llm.mycorp.example/v1" {
		t.Errorf("legacy base: %q", out[0].BaseURL)
	}
}

func TestLLMProviderPreset_YAMLRoundTripBaseURL(t *testing.T) {
	in := []domain.LLMProviderPreset{
		{ID: "openai", Name: "OpenAI", Provider: domain.LLMProviderOpenAI, BaseURL: "https://api.openai.com/v1", Description: "GPT"},
	}
	b, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "base_url:") {
		t.Fatalf("expected base_url key, got:\n%s", s)
	}
	if strings.Contains(s, "baseurl:") {
		t.Fatalf("must not write baseurl key:\n%s", s)
	}
	var out []domain.LLMProviderPreset
	if err := yaml.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out[0].BaseURL != "https://api.openai.com/v1" {
		t.Errorf("round-trip base: %q", out[0].BaseURL)
	}
}

func TestLoader_MergesEmptyPresetBaseURLs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlText := `
llm:
  providers:
    - id: openai
      name: OpenAI
      provider: openai
      baseurl: ""
    - id: deepseek
      name: DeepSeek
      provider: openai
      baseurl: ""
`
	if err := os.WriteFile(path, []byte(yamlText), 0600); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(path)
	cfg, err := l.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.LLMProviderPreset{}
	for _, p := range cfg.LLM.Providers {
		byID[p.ID] = p
	}
	if byID["openai"].BaseURL != "https://api.openai.com/v1" {
		t.Errorf("openai: %q", byID["openai"].BaseURL)
	}
	if byID["deepseek"].BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("deepseek: %q", byID["deepseek"].BaseURL)
	}
}
