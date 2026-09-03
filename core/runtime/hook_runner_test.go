package runtime

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/service"
)

func TestHookMatcherMatches(t *testing.T) {
	cases := []struct {
		matcher, agent string
		want           bool
	}{
		{"", "novel", true},
		{"novel", "novel", true},
		{"novel", "translator", false},
		{"novel|translator", "translator", true},
		{"novel|translator", "other", false},
	}
	for _, c := range cases {
		if got := domain.HookMatcherMatches(c.matcher, c.agent); got != c.want {
			t.Errorf("HookMatcherMatches(%q,%q)=%v want %v", c.matcher, c.agent, got, c.want)
		}
	}
}

func TestHooksForAgent(t *testing.T) {
	hooks := []domain.ResolvedHook{
		{Event: domain.HookEventSubagentStart, Matcher: "novel", Command: "a"},
		{Event: domain.HookEventUserPromptSubmit, Matcher: "novel", Command: "b"},
		{Event: domain.HookEventSubagentStart, Matcher: "", Command: "c"},
	}
	got := service.HooksForAgent(hooks, domain.HookEventSubagentStart, "novel")
	if len(got) != 2 {
		t.Fatalf("want 2 hooks, got %d", len(got))
	}
	got = service.HooksForAgent(hooks, domain.HookEventSubagentStart, "other")
	if len(got) != 1 || got[0].Command != "c" {
		t.Fatalf("matcher-all hook expected, got %+v", got)
	}
}

func writeHookScript(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook runner tests use sh scripts")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunContextHookInjects(t *testing.T) {
	script := writeHookScript(t, `printf '{"additionalContext": "STYLE: 短句克制"}'`)
	h := domain.ResolvedHook{
		PluginName: "novel", PluginRoot: "/tmp/plugin", Event: domain.HookEventSubagentStart,
		Command: script,
	}
	got := runContextHook(context.Background(), h, "novel", "s1", "p1", t.TempDir(), "写第 3 章")
	if !strings.Contains(got, "STYLE: 短句克制") {
		t.Fatalf("missing additionalContext: %q", got)
	}
	if !strings.Contains(got, `source="novel/subagentStart"`) {
		t.Fatalf("missing provenance tag: %q", got)
	}
}

func TestRunContextHookPlaceholdersAndStdin(t *testing.T) {
	script := writeHookScript(t, `cat > /dev/null; printf '{"additionalContext": "ok"}'`)
	h := domain.ResolvedHook{
		PluginName: "novel", PluginRoot: "/nonexistent/plugin root", Event: domain.HookEventSubagentStart,
		Command: `test -d "${PLUGIN_DIR}" && exit 1; ` + script,
	}
	// ${PLUGIN_DIR} points to a missing dir → test -d fails → script runs.
	got := runContextHook(context.Background(), h, "novel", "", "", t.TempDir(), "goal")
	if !strings.Contains(got, "ok") {
		t.Fatalf("placeholder substitution broken: %q", got)
	}
}

func TestRunContextHookFailuresNeverBlock(t *testing.T) {
	cases := []domain.ResolvedHook{
		{PluginName: "x", Event: "subagentStart", Command: "exit 3"},                      // non-zero exit
		{PluginName: "x", Event: "subagentStart", Command: "printf 'not-json'"},           // invalid stdout
		{PluginName: "x", Event: "subagentStart", Command: "printf '{}'"},                 // empty context
		{PluginName: "x", Event: "subagentStart", Command: "sleep 5", TimeoutSec: 1},      // timeout
	}
	for i, h := range cases {
		start := time.Now()
		if got := runContextHook(context.Background(), h, "a", "", "", "", ""); got != "" {
			t.Fatalf("case %d: want empty, got %q", i, got)
		}
		if h.TimeoutSec == 1 && time.Since(start) > 4*time.Second {
			t.Fatalf("case %d: timeout not enforced", i)
		}
	}
}

func TestRunContextHookCapsLongOutput(t *testing.T) {
	long := strings.Repeat("长", domain.HookMaxAdditionalContextRunes+50)
	script := writeHookScript(t, `printf '{"additionalContext": "`+long+`"}'`)
	h := domain.ResolvedHook{PluginName: "x", Event: "subagentStart", Command: script}
	got := runContextHook(context.Background(), h, "a", "", "", "", "")
	// wrapper tags + capped content + ellipsis
	if len([]rune(got)) > domain.HookMaxAdditionalContextRunes+120 {
		t.Fatalf("output not capped: %d runes", len([]rune(got)))
	}
}

func TestJoinEphemeral(t *testing.T) {
	if got := joinEphemeral("", "a", "  ", "b"); got != "a\n\nb" {
		t.Fatalf("joinEphemeral = %q", got)
	}
	if got := joinEphemeral("", ""); got != "" {
		t.Fatalf("joinEphemeral empty = %q", got)
	}
}
