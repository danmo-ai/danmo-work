package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/adapter/llm"
	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/store/turnlog"
)

type testConfigStore struct {
	cfg *domain.ConfigFile
}

func (s *testConfigStore) Load(_ context.Context) (*domain.ConfigFile, error) {
	return s.cfg, nil
}

func (s *testConfigStore) Save(_ context.Context, _ *domain.ConfigFile) error {
	return nil
}

var _ port.ConfigStore = (*testConfigStore)(nil)

func testCompactionConfig(enabled bool, turnInterval, subInterval, maxTokens, cutTokens int) *testConfigStore {
	return &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Compaction: domain.ConfigCompactionSection{
					Enabled:      enabled,
					TurnInterval: turnInterval,
					SubInterval:  subInterval,
					MaxTokens:    maxTokens,
					CutTokens:    cutTokens,
					TriggerRatio: 0.85,
					ToolTruncate: 2000,
				},
			},
		},
	}
}

// runtime.compaction.model overrides the session model for summaries.
func TestCompactionSummarizeUsesConfiguredModel(t *testing.T) {
	mock := llm.NewMock().Finish("summary text")
	stream := NewStreamEventManager(nil)
	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	store := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Compaction: domain.ConfigCompactionSection{
					Enabled: true, Model: "summary-model",
					CutTokens: 100, TriggerRatio: 0.85, ToolTruncate: 2000,
				},
			},
		},
	}
	mgr := NewCompactionManager(mock, stream, store, cpStore, nil)

	old := []Message{{Role: RoleUser, Content: "please do the thing"}}
	if !mgr.CompactToRetain(context.Background(), "s-model", "turn-1", old, old, 1, "session-model", "turn-1", 1, 100) {
		t.Fatal("CompactToRetain should succeed")
	}
	if len(mock.Requests) == 0 {
		t.Fatal("summarize should have called the LLM")
	}
	if got := mock.Requests[len(mock.Requests)-1].Model; got != "summary-model" {
		t.Fatalf("summarize model: want summary-model, got %q", got)
	}
}

func TestEstimateMessageTokensCountsImageParts(t *testing.T) {
	plain := estimateMessageTokens(Message{Role: RoleUser, Content: "hi"})
	withImage := estimateMessageTokens(Message{
		Role: RoleUser, Content: "hi",
		Parts: []ContentPart{{Type: "image", MimeType: "image/png", Data: "base64data"}},
	})
	if withImage-plain != imagePartTokenEstimate {
		t.Fatalf("image part should add %d tokens, got %d", imagePartTokenEstimate, withImage-plain)
	}
}

func TestCompactionShouldCompact(t *testing.T) {
	mock := llm.NewMock().Finish("done")
	stream := NewStreamEventManager(nil)

	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	store := testCompactionConfig(true, 3, 2, 128000, 16000)
	mgr := NewCompactionManager(mock, stream, store, cpStore, nil)

	if !mgr.ShouldCompact("session-1", 3, 0, 0, "") {
		t.Error("ShouldCompact should return true for turnCount=3 with interval=3")
	}
	if mgr.ShouldCompact("session-1", 2, 0, 0, "") {
		t.Error("ShouldCompact should return false for turnCount=2 with interval=3")
	}
}

func TestCompactionShouldCompactWithActualUsage(t *testing.T) {
	mock := llm.NewMock().Finish("done")
	stream := NewStreamEventManager(nil)

	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	// Config with model entry for deepseek-v4-flash: context=1M, maxOutput=384K
	store := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Compaction: domain.ConfigCompactionSection{
					Enabled:      true,
					TurnInterval: 100,
					SubInterval:  100,
					TriggerRatio: 0.85,
					ToolTruncate: 16000,
				},
			},
			LLM: domain.ConfigLLMSection{
				Models: []domain.ModelConfig{
					{Model: "deepseek-v4-flash", ContextWindow: 1_000_000, MaxOutput: 384_000},
				},
			},
		},
	}
	modelCfg := NewModelConfigRegistry()
	mgr := NewCompactionManager(mock, stream, store, cpStore, modelCfg)

	// deepseek/deepseek-v4-flash: context=1M, maxOutput=384K, usable=616K, trigger@0.85=523K
	model := "deepseek/deepseek-v4-flash"

	// Below threshold: should NOT compact
	if mgr.ShouldCompact("s1", 1, 0, 400_000, model) {
		t.Error("should NOT compact when actual tokens (400K) < trigger threshold (~523K)")
	}

	// Above threshold: should compact
	if !mgr.ShouldCompact("s1", 1, 0, 600_000, model) {
		t.Error("should compact when actual tokens (600K) > trigger threshold (~523K)")
	}

	// No actual usage (maxPromptTokens=0), falls back to estimation + turn count
	if mgr.ShouldCompact("s1", 1, 50000, 0, model) {
		t.Error("should NOT compact with estimation only when turn count < interval")
	}
}

func TestModelConfigRegistryContextWindow(t *testing.T) {
	reg := NewModelConfigRegistry()
	reg.SetModels([]domain.ModelConfig{
		{Model: "gpt-4o", ContextWindow: 128_000},
		{Model: "gpt-4.1", ContextWindow: 1_047_576},
		{Model: "o3-mini", ContextWindow: 200_000},
		{Model: "deepseek-v4-flash", ContextWindow: 1_000_000},
		{Model: "deepseek-chat", ContextWindow: 64_000},
		{Model: "claude-sonnet-4-20250514", ContextWindow: 200_000},
		{Model: "gemini-2.5-pro", ContextWindow: 1_048_576},
		{Model: "qwen-long", ContextWindow: 10_000_000},
	})

	tests := []struct {
		modelID  string
		expected int
	}{
		{"openai/gpt-4o", 128_000},
		{"openai/gpt-4.1", 1_047_576},
		{"openai/o3-mini", 200_000},
		{"deepseek/deepseek-v4-flash", 1_000_000},
		{"deepseek/deepseek-chat", 64_000},
		{"anthropic/claude-sonnet-4-20250514", 200_000},
		{"google/gemini-2.5-pro", 1_048_576},
		{"qwen/qwen-long", 10_000_000},
		{"unknown/some-model", 128_000}, // fallback to default
	}
	for _, tt := range tests {
		got := reg.ContextWindow(tt.modelID)
		if got != tt.expected {
			t.Errorf("ContextWindow(%q) = %d, want %d", tt.modelID, got, tt.expected)
		}
	}
}

func TestModelConfigRegistryConfigOverride(t *testing.T) {
	reg := NewModelConfigRegistry()
	reg.SetModels([]domain.ModelConfig{
		{Model: "gpt-4o", ContextWindow: 128_000, MaxOutput: 16_384, Temperature: 0.5},
		{Model: "custom-model", Temperature: 0.8, TopP: 0.95},
		{Model: "deepseek-v4-flash", ContextWindow: 1_000_000, MaxOutput: 384_000},
	})

	// Config exact match returns gen params
	gp := reg.GenParams("openai/gpt-4o")
	if gp == nil || gp.Temperature != 0.5 {
		t.Errorf("config override gen params: got %v, want temperature=0.5", gp)
	}

	// Custom model from config
	gp2 := reg.GenParams("provider/custom-model")
	if gp2 == nil || gp2.Temperature != 0.8 || gp2.TopP != 0.95 {
		t.Errorf("custom model gen params: got %v, want temperature=0.8 top_p=0.95", gp2)
	}

	// Non-configured model returns nil
	gp3 := reg.GenParams("unknown/some-model")
	if gp3 != nil {
		t.Errorf("non-configured gen params: got %v, want nil", gp3)
	}

	// Context window from config
	if got := reg.ContextWindow("openai/gpt-4o"); got != 128_000 {
		t.Errorf("context window: got %d, want 128000", got)
	}
	if got := reg.ContextWindow("deepseek/deepseek-v4-flash"); got != 1_000_000 {
		t.Errorf("context window: got %d, want 1000000", got)
	}
	// Non-configured model falls back to default
	if got := reg.ContextWindow("unknown/some-model"); got != 128_000 {
		t.Errorf("non-configured context window: got %d, want 128000", got)
	}
}

func TestCompactionWithConfigModels(t *testing.T) {
	mock := llm.NewMock().Finish("done")
	stream := NewStreamEventManager(nil)

	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })

	// Config with model generation param override
	store := &testConfigStore{
		cfg: &domain.ConfigFile{
			Runtime: domain.ConfigRuntimeSection{
				Compaction: domain.ConfigCompactionSection{
					Enabled:      true,
					TurnInterval: 100,
					SubInterval:  100,
					TriggerRatio: 0.85,
					ToolTruncate: 2000,
				},
			},
			LLM: domain.ConfigLLMSection{
				Models: []domain.ModelConfig{
					{Model: "test-model", Temperature: 0.7},
				},
			},
		},
	}
	mgr := NewCompactionManager(mock, stream, store, cpStore, nil)

	// test-model has no context window pattern rule, so falls back to default 128K
	// default 128K - 8192 maxOutput = 119808 usable, trigger@0.85 = ~101837
	model := "provider/test-model"

	// Below threshold
	if mgr.ShouldCompact("s1", 1, 0, 80_000, model) {
		t.Error("should NOT compact when tokens (80K) < threshold (~101K)")
	}

	// Above threshold
	if !mgr.ShouldCompact("s1", 1, 0, 110_000, model) {
		t.Error("should compact when tokens (110K) > threshold (~101K)")
	}
}

func TestCompactionCompactAndRecover(t *testing.T) {
	mock := llm.NewMock().Finish("done").Finish("done").
		AddText("summary: test compaction completed")
	stream := NewStreamEventManager(nil)
	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })

	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)

	messages := []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant"},
		{Role: RoleUser, Content: "Hello, this is a very long message that should exceed the token cut limit so compaction triggers"},
		{Role: RoleAssistant, Content: "I understand. " + string(make([]byte, 200))},
		{Role: RoleUser, Content: "Another long message to ensure enough tokens for the cut point"},
		{Role: RoleAssistant, Content: "Responding again with " + string(make([]byte, 200))},
	}

	cutIdx := mgr.Compact(context.Background(), "session-1", "turn-test", messages, 2, "mock/test")
	if cutIdx <= 0 {
		t.Error("Compact should return non-zero cut index")
	}

	cp := mgr.Recover(context.Background(), "session-1")
	if cp == nil {
		t.Fatal("Recover should return checkpoint after compaction")
	}
	if cp.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want session-1", cp.SessionID)
	}
	if cp.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", cp.TurnCount)
	}
	if cp.TurnID != "turn-test" {
		t.Errorf("TurnID = %q, want turn-test", cp.TurnID)
	}
	if cp.Summary == "" {
		t.Error("Summary should not be empty")
	}

	loaded, err := cpStore.Load("session-1")
	if err != nil {
		t.Fatalf("Load from store: %v", err)
	}
	if loaded == nil {
		t.Fatal("Checkpoint should be persisted to store")
	}
	if loaded.Summary != cp.Summary {
		t.Errorf("Loaded summary = %q, want %q", loaded.Summary, cp.Summary)
	}
}

func TestExtractLatestTodos(t *testing.T) {
	messages := []Message{
		{Role: RoleUser, Content: "plan it"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID:   "c1",
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []any{
					map[string]any{"content": "old", "status": "completed", "priority": "low"},
				},
			},
		}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "todowrite", Content: "Todo list:..."},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID:   "c2",
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []any{
					map[string]any{"content": "do A", "status": "in_progress", "priority": "high"},
					map[string]any{"content": "do B", "status": "pending", "priority": "medium"},
				},
			},
		}}},
	}

	got := extractLatestTodos(messages)
	if len(got) != 2 {
		t.Fatalf("len(todos)=%d, want 2", len(got))
	}
	if got[0].Content != "do A" || got[0].Status != "in_progress" || got[0].Priority != "high" {
		t.Errorf("todo[0]=%+v", got[0])
	}
	if got[1].Content != "do B" || got[1].Status != "pending" {
		t.Errorf("todo[1]=%+v", got[1])
	}
}

func TestCompactionPreservesTodos(t *testing.T) {
	mock := llm.NewMock().AddText(`{"summary":"kept going"}`)
	stream := NewStreamEventManager(nil)
	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)

	pad := string(make([]byte, 200))
	messages := []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant"},
		{Role: RoleUser, Content: "Hello, this is a very long message that should exceed the token cut limit so compaction triggers"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID:   "todo-1",
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []any{
					map[string]any{"content": "ship feature", "status": "in_progress", "priority": "high"},
				},
			},
		}}},
		{Role: RoleTool, ToolCallID: "todo-1", Name: "todowrite", Content: "Todo list: ship feature"},
		{Role: RoleAssistant, Content: "Working on it. " + pad},
		{Role: RoleUser, Content: "Another long message to ensure enough tokens for the cut point " + pad},
		{Role: RoleAssistant, Content: "Responding again with " + pad},
	}

	cutIdx := mgr.Compact(context.Background(), "session-todo", "turn-todo", messages, 3, "mock/test")
	if cutIdx <= 0 {
		t.Fatal("Compact should return non-zero cut index")
	}
	cp := mgr.Recover(context.Background(), "session-todo")
	if cp == nil {
		t.Fatal("expected checkpoint")
	}
	if len(cp.Todos) != 1 || cp.Todos[0].Content != "ship feature" {
		t.Fatalf("Todos=%+v, want ship feature", cp.Todos)
	}
}

func TestCompactionInheritsPrevTodos(t *testing.T) {
	mock := llm.NewMock().
		AddText(`{"summary":"first"}`).
		AddText(`{"summary":"second"}`)
	stream := NewStreamEventManager(nil)
	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)

	pad := string(make([]byte, 200))
	withTodo := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "long user message that should exceed cut " + pad},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID:   "t1",
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []any{
					map[string]any{"content": "keep me", "status": "pending", "priority": "medium"},
				},
			},
		}}},
		{Role: RoleTool, ToolCallID: "t1", Name: "todowrite", Content: "ok"},
		{Role: RoleAssistant, Content: "done " + pad},
		{Role: RoleUser, Content: "more long content " + pad},
		{Role: RoleAssistant, Content: "reply " + pad},
	}
	if cut := mgr.Compact(context.Background(), "s-inherit", "turn-1", withTodo, 2, "mock/test"); cut <= 0 {
		t.Fatal("first compact failed")
	}

	noTodo := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "continue without rewriting todos " + pad},
		{Role: RoleAssistant, Content: "still going " + pad},
		{Role: RoleUser, Content: "another long user turn " + pad},
		{Role: RoleAssistant, Content: "final " + pad},
	}
	if cut := mgr.Compact(context.Background(), "s-inherit", "turn-2", noTodo, 4, "mock/test"); cut <= 0 {
		t.Fatal("second compact failed")
	}
	cp := mgr.Recover(context.Background(), "s-inherit")
	if cp == nil || len(cp.Todos) != 1 || cp.Todos[0].Content != "keep me" {
		t.Fatalf("expected inherited todo, got %+v", cp)
	}
}

func TestFormatActiveTodosAndSystemPrompt(t *testing.T) {
	todos := []domain.CompactionTodoItem{
		{Content: "A", Status: "in_progress", Priority: "high"},
		{Content: "B", Status: "pending", Priority: "low"},
	}
	block := formatActiveTodos(todos)
	if block == "" || !contains(block, "<active-todos>") || !contains(block, "A") {
		t.Fatalf("formatActiveTodos=%q", block)
	}

	sys := buildSystemPrompt("persona", nil, nil, false, `{"summary":"x"}`, block, "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if !contains(sys, "<compaction-checkpoint>") {
		t.Error("expected compaction-checkpoint")
	}
	if !contains(sys, "<active-todos>") || !contains(sys, "[in_progress] A (high)") {
		t.Errorf("expected active-todos in prompt, got %q", sys)
	}
	if !contains(sys, "<ask-user-policy>") || !contains(sys, "ask_user") {
		t.Errorf("expected ask-user-policy in prompt, got %q", sys)
	}
	if !contains(sys, "<knowledge-policy>") || !contains(sys, "search_kb") {
		t.Errorf("expected knowledge-policy in prompt, got %q", sys)
	}
	if !contains(sys, "<memory-policy>") || !contains(sys, "memory_update") {
		t.Errorf("expected memory-policy in prompt, got %q", sys)
	}
	if !contains(sys, "<table-store-policy>") || !contains(sys, "table_upsert") {
		t.Errorf("expected table-store-policy in prompt, got %q", sys)
	}
	if contains(sys, "<delegation-policy>") {
		t.Error("delegation-policy should be absent when canDelegate=false")
	}
}

func TestFindKeepStartRespectsToolPairs(t *testing.T) {
	// user + (assistant tools + 2 results) + user + text assistant
	msgs := []Message{
		{Role: RoleUser, Content: "goal"},
		{Role: RoleAssistant, Content: "reading", ToolCalls: []ToolCall{
			{ID: "c1", Name: "read_file", Arguments: map[string]any{"path": "a"}},
			{ID: "c2", Name: "read_file", Arguments: map[string]any{"path": "b"}},
		}},
		{Role: RoleTool, ToolCallID: "c1", Name: "read_file", Content: strings.Repeat("A", 400)},
		{Role: RoleTool, ToolCallID: "c2", Name: "read_file", Content: strings.Repeat("B", 400)},
		{Role: RoleUser, Content: "continue"},
		{Role: RoleAssistant, Content: "done"},
	}
	blocks := buildBlocks(msgs)
	if len(blocks) != 4 {
		t.Fatalf("want 4 blocks (user, tool-pair, user, assistant), got %d %+v", len(blocks), blocks)
	}
	if blocks[1].start != 1 || blocks[1].end != 4 {
		t.Fatalf("tool pair block should be [1,4), got %+v", blocks[1])
	}

	// Tiny budget: keep only the newest text assistant (+ ensure user)
	keep := findKeepStart(msgs, 20)
	if keep > 4 {
		t.Fatalf("keepStart=%d should include continue user (idx 4) or earlier", keep)
	}
	// Must not cut inside the tool pair [1,4)
	if keep > 1 && keep < 4 {
		t.Fatalf("keepStart=%d cuts inside tool pair", keep)
	}
}

func TestFindKeepStartSplitsOversizedTurn(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "start"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read_file", Arguments: map[string]any{"p": "x"}}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "read_file", Content: strings.Repeat("X", 800)},
		{Role: RoleUser, Content: "continue"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c2", Name: "read_file", Arguments: map[string]any{"p": "y"}}}},
		{Role: RoleTool, ToolCallID: "c2", Name: "read_file", Content: strings.Repeat("Y", 80)},
	}
	// Budget fits the last user + small pair; drops the huge first pair.
	keep := findKeepStart(msgs, 50)
	if keep != 3 {
		t.Fatalf("want keepStart=3 (continue user), got %d", keep)
	}
	if keep > 0 && keep < 3 {
		t.Fatalf("must not cut inside first tool pair, keepStart=%d", keep)
	}
}

func TestCompactToRetainStoresSkip(t *testing.T) {
	mock := llm.NewMock().AddText("handoff note")
	stream := NewStreamEventManager(nil)
	tmpDir := t.TempDir()
	cpStore := turnlog.NewCheckpointStore(func(pid string) string { return filepath.Join(tmpDir, pid) })
	mgr := NewCompactionManager(mock, stream, testCompactionConfig(true, 2, 2, 128000, 50), cpStore, nil)

	old := []Message{{Role: RoleUser, Content: "old work " + strings.Repeat("o", 200)}}
	ok := mgr.CompactToRetain(context.Background(), "s-skip", "turn-now", old, old, 3, "mock/test", "turn-big", 4, 999)
	if !ok {
		t.Fatal("CompactToRetain failed")
	}
	cp := mgr.Recover(context.Background(), "s-skip")
	if cp == nil {
		t.Fatal("nil checkpoint")
	}
	if cp.RetainFromTurnID != "turn-big" || cp.RetainSkipMessages != 4 {
		t.Fatalf("cursor: turn=%q skip=%d", cp.RetainFromTurnID, cp.RetainSkipMessages)
	}
}
