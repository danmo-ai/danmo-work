package service

import (
	"testing"

	"danmo-work/core/domain"
)

func TestLookupMatchesDeepSeekV4Variants(t *testing.T) {
	reg := NewModelConfigRegistry()
	reg.SetModels([]domain.ModelConfig{
		{
			Model:            "deepseek-v4-pro",
			ContextWindow:    1_000_000,
			AvailableEfforts: []string{"off", "low", "medium", "high", "max"},
		},
		{
			Model:            "deepseek-v4-flash",
			AvailableEfforts: []string{"off", "high"},
		},
	})

	cases := []struct {
		id   string
		want string
	}{
		{"deepseek/deepseek-v4-pro", "deepseek-v4-pro"},
		{"deepseek-v4-pro", "deepseek-v4-pro"},
		{"deepseek/deepseek-v4-pro/high", "deepseek-v4-pro"},
		{"deepseek/deepseek-v4", "deepseek-v4-pro"}, // short alias → shortest matching config (-pro)
		{"deepseek/deepseek-v4-pro-20260724", "deepseek-v4-pro"},
		{"deepseek/deepseek-v4-flash", "deepseek-v4-flash"},
	}
	for _, tc := range cases {
		got := reg.AvailableEfforts(tc.id)
		cfg := reg.lookup(tc.id)
		if cfg == nil {
			t.Fatalf("%s: lookup nil", tc.id)
		}
		if cfg.Model != tc.want {
			t.Fatalf("%s: model=%q want %q", tc.id, cfg.Model, tc.want)
		}
		if len(got) < 2 {
			t.Fatalf("%s: efforts=%v", tc.id, got)
		}
	}
}
