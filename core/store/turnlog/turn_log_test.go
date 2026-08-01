package turnlog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
)

func testProjector(root string) func(string) string {
	return func(projectID string) string {
		if projectID == "" {
			return root
		}
		return filepath.Join(root, projectID)
	}
}

func writeToolPair(s *TurnLogStore, turnID, callID, name string) {
	s.Append(turnID, "tool_call", map[string]any{
		"call_id": callID, "name": name, "input": map[string]any{"x": 1},
	})
	s.Append(turnID, "tool_result", map[string]any{
		"call_id": callID, "name": name, "output": "ok",
	})
}

func countTypes(entries []map[string]any) (calls, results int) {
	for _, e := range entries {
		switch e["type"] {
		case "tool_call":
			calls++
		case "tool_result":
			results++
		}
	}
	return calls, results
}

func TestLoadForRecoveryDropsUnpairedTrailingToolCall(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	if err := s.Create("turn-1", "sess-1", "proj-a", "agent-1", "do stuff"); err != nil {
		t.Fatal(err)
	}
	writeToolPair(s, "turn-1", "c1", "read_file")
	writeToolPair(s, "turn-1", "c2", "exec_shell")
	// Interrupted after tool_call logged, before tool_result.
	s.Append("turn-1", "tool_call", map[string]any{
		"call_id": "c3", "name": "write_file", "input": map[string]any{"path": "x"},
	})
	s.EndTurn("turn-1", domain.TurnCancelled)

	goal, entries := s.LoadForRecovery("turn-1")
	if goal != "do stuff" {
		t.Fatalf("goal: want %q, got %q", "do stuff", goal)
	}
	calls, results := countTypes(entries)
	if calls != 2 || results != 2 {
		t.Fatalf("want 2 paired tool calls/results, got calls=%d results=%d entries=%d", calls, results, len(entries))
	}
	// Trailing unpaired call must not appear.
	for _, e := range entries {
		if e["type"] == "tool_call" {
			data, _ := e["data"].(map[string]any)
			if id, _ := data["call_id"].(string); id == "c3" {
				t.Fatal("unpaired trailing tool_call c3 should have been dropped")
			}
		}
	}
}

func TestLoadForRecoveryKeepsCompletePairs(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	if err := s.Create("turn-2", "sess-1", "proj-a", "agent-1", "goal"); err != nil {
		t.Fatal(err)
	}
	writeToolPair(s, "turn-2", "c1", "read_file")
	s.EndTurn("turn-2", domain.TurnCompleted)

	goal, entries := s.LoadForRecovery("turn-2")
	if goal != "goal" {
		t.Fatalf("goal: %q", goal)
	}
	calls, results := countTypes(entries)
	if calls != 1 || results != 1 {
		t.Fatalf("want 1 pair, got calls=%d results=%d", calls, results)
	}
}

func TestLoadForRecoveryStripsPartialAssistantBatch(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	if err := s.Create("turn-partial", "sess-1", "proj-a", "agent-1", "do"); err != nil {
		t.Fatal(err)
	}
	s.Append("turn-partial", "user", map[string]any{"content": "do"})
	// One assistant batch with two calls; only c1 completed before interrupt.
	s.Append("turn-partial", "assistant", map[string]any{
		"content": "reading",
		"tool_calls": []any{
			map[string]any{"id": "c1", "name": "read_file", "arguments": map[string]any{"p": "a"}},
			map[string]any{"id": "c2", "name": "read_file", "arguments": map[string]any{"p": "b"}},
		},
	})
	s.Append("turn-partial", "tool_result", map[string]any{
		"call_id": "c1", "name": "read_file", "output": "A",
	})
	s.EndTurn("turn-partial", domain.TurnCancelled)

	_, entries := s.LoadForRecovery("turn-partial")
	msgs := entriesToChatMessages(entries)
	if len(msgs) != 3 {
		t.Fatalf("want user+assistant+tool, got %d %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 || msgs[1].ToolCalls[0].ID != "c1" {
		t.Fatalf("expected only completed tool_call c1, got %+v", msgs[1])
	}
	if msgs[2].Role != "tool" || msgs[2].ToolCallID != "c1" {
		t.Fatalf("expected tool result c1, got %+v", msgs[2])
	}
}

func TestCreateReopensWithoutDuplicateStart(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	if err := s.Create("turn-3", "sess-1", "proj-a", "agent-1", "goal"); err != nil {
		t.Fatal(err)
	}
	writeToolPair(s, "turn-3", "c1", "read_file")
	s.EndTurn("turn-3", domain.TurnCancelled)

	// Resume: Create should reopen append, not write another start.
	if err := s.Create("turn-3", "sess-1", "proj-a", "agent-1", "goal"); err != nil {
		t.Fatal(err)
	}
	s.Append("turn-3", "tool_call", map[string]any{
		"call_id": "c2", "name": "exec_shell", "input": map[string]any{},
	})
	s.Append("turn-3", "tool_result", map[string]any{
		"call_id": "c2", "name": "exec_shell", "output": "done",
	})
	s.EndTurn("turn-3", domain.TurnCompleted)

	raw, err := s.LoadRawLog("turn-3")
	if err != nil {
		t.Fatal(err)
	}
	starts := 0
	dec := json.NewDecoder(bytes.NewReader(raw))
	for dec.More() {
		var e EntryJSON
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		if e.Type == "start" {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("want exactly 1 start entry after resume, got %d", starts)
	}
}

func TestLoadTurnLogZipFallsBackToStreamEvents(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	// No JSONL on disk — only stream events (historical turns / migration gaps).
	payload, _ := json.Marshal(domain.TurnStartedPayload{
		TurnID: "turn-old", AgentID: "default", Goal: "hello",
	})
	events := []domain.StreamEvent{
		{Seq: 1, Type: domain.EventTurnStarted, SessionID: "sess-1", TurnID: "turn-old", Payload: payload},
		{Seq: 2, Type: domain.EventUserMessage, SessionID: "sess-1", TurnID: "turn-old", Payload: []byte(`{"content":"hello"}`)},
		{Seq: 3, Type: domain.EventTurnEnded, SessionID: "sess-1", TurnID: "turn-old", Payload: []byte(`{"status":"completed"}`)},
	}

	zipBytes, err := s.LoadTurnLogZip("turn-old", events)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["manifest.json"] || !names["events.jsonl"] {
		t.Fatalf("zip contents: %v", names)
	}
	if names["turn-old.jsonl"] {
		t.Fatal("expected no jsonl when file missing")
	}

	var mf []TurnLogNode
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, _ := f.Open()
		defer rc.Close()
		if err := json.NewDecoder(rc).Decode(&mf); err != nil {
			t.Fatal(err)
		}
	}
	if len(mf) != 1 || mf[0].TurnID != "turn-old" || !mf[0].Missing {
		t.Fatalf("manifest: %+v", mf)
	}
	if mf[0].AgentID != "default" || mf[0].Goal != "hello" {
		t.Fatalf("meta from events: %+v", mf[0])
	}
}

func TestLoadTurnLogZipIncludesJSONLAndEvents(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))
	if err := s.Create("turn-z", "sess-1", "proj-a", "agent-1", "goal"); err != nil {
		t.Fatal(err)
	}
	writeToolPair(s, "turn-z", "c1", "read_file")
	s.EndTurn("turn-z", domain.TurnCompleted)

	payload, _ := json.Marshal(domain.TurnStartedPayload{
		TurnID: "turn-z", AgentID: "agent-1", Goal: "goal",
	})
	events := []domain.StreamEvent{
		{Seq: 1, Type: domain.EventTurnStarted, SessionID: "sess-1", TurnID: "turn-z", Payload: payload},
	}
	zipBytes, err := s.LoadTurnLogZip("turn-z", events)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["manifest.json"] || !names["events.jsonl"] || !names["turn-z.jsonl"] {
		t.Fatalf("zip contents: %v", names)
	}
}

func TestLoadForRecoveryRehydratesFromDisk(t *testing.T) {
	root := t.TempDir()
	s1 := NewTurnLogStore(testProjector(root))
	if err := s1.Create("turn-4", "sess-1", "proj-a", "agent-1", "disk-goal"); err != nil {
		t.Fatal(err)
	}
	writeToolPair(s1, "turn-4", "c1", "read_file")
	s1.EndTurn("turn-4", domain.TurnCancelled)

	// Simulate process restart: new store, empty memory.
	s2 := NewTurnLogStore(testProjector(root))
	goal, entries := s2.LoadForRecovery("turn-4")
	if goal != "disk-goal" {
		t.Fatalf("goal from disk: want %q, got %q", "disk-goal", goal)
	}
	calls, results := countTypes(entries)
	if calls != 1 || results != 1 {
		t.Fatalf("disk recovery pairs: calls=%d results=%d", calls, results)
	}

	// Resume Create on fresh store should reopen existing file.
	if err := s2.Create("turn-4", "sess-1", "proj-a", "agent-1", "disk-goal"); err != nil {
		t.Fatal(err)
	}
	s2.Append("turn-4", "tool_call", map[string]any{
		"call_id": "c2", "name": "exec_shell", "input": map[string]any{},
	})
	s2.EndTurn("turn-4", domain.TurnCompleted)

	listed := s2.ListEntries("turn-4")
	starts := 0
	for _, e := range listed {
		if e.Type == "start" {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("after disk reopen want 1 start, got %d", starts)
	}
}

func TestLoadSessionMessagesRebuildsUserAssistantTools(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	if err := s.Create("turn-a", "sess-hist", "proj-a", "agent-1", "hello"); err != nil {
		t.Fatal(err)
	}
	s.Append("turn-a", "user", map[string]any{"content": "hello"})
	s.Append("turn-a", "assistant", map[string]any{"content": "hi there"})
	s.EndTurn("turn-a", domain.TurnCompleted)

	if err := s.Create("turn-b", "sess-hist", "proj-a", "agent-1", "weather"); err != nil {
		t.Fatal(err)
	}
	s.Append("turn-b", "user", map[string]any{"content": "weather"})
	s.Append("turn-b", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "c1", "name": "web_fetch", "arguments": map[string]any{"url": "https://x"}},
		},
	})
	s.Append("turn-b", "tool_result", map[string]any{"call_id": "c1", "name": "web_fetch", "output": "29C"})
	s.Append("turn-b", "assistant", map[string]any{"content": "It is 29C"})
	s.EndTurn("turn-b", domain.TurnCompleted)

	msgs := s.LoadSessionMessages("sess-hist", "", 0)
	if len(msgs) != 6 {
		t.Fatalf("want 6 messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Fatalf("msg0: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Fatalf("msg1: %+v", msgs[1])
	}
	if msgs[5].Role != "assistant" || msgs[5].Content != "It is 29C" {
		t.Fatalf("msg5: %+v", msgs[5])
	}
}

func TestLoadSessionMessagesIncludesFinalAssistant(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))
	_ = s.Create("turn-1", "sess-2", "proj-a", "a", "q")
	s.Append("turn-1", "user", map[string]any{"content": "q"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{map[string]any{"id": "c1", "name": "read_file", "arguments": map[string]any{"path": "x"}}},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "c1", "name": "read_file", "output": "data"})
	s.Append("turn-1", "assistant", map[string]any{"content": "done"})
	s.EndTurn("turn-1", domain.TurnCompleted)

	msgs := s.LoadSessionMessages("sess-2", "", 0)
	if len(msgs) != 4 {
		t.Fatalf("want 4 msgs, got %d %+v", len(msgs), msgs)
	}
	if msgs[3].Role != "assistant" || msgs[3].Content != "done" {
		t.Fatalf("final assistant: %+v", msgs[3])
	}
}

func TestLoadSessionMessagesSkipsNestedToolRunAndHonorsRetain(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	_ = s.Create("turn-1", "sess-3", "proj-a", "a", "one")
	s.Append("turn-1", "user", map[string]any{"content": "one"})
	s.Append("turn-1", "assistant", map[string]any{"content": "a1"})
	s.EndTurn("turn-1", domain.TurnCompleted)

	_ = s.CreateNested("turn-child", "sess-3", "proj-a", "worker", "sub")
	s.Append("turn-child", "user", map[string]any{"content": "sub"})
	s.Append("turn-child", "assistant", map[string]any{"content": "child-secret"})
	s.EndTurn("turn-child", domain.TurnCompleted)
	if !s.IsNestedToolRun("turn-child") {
		t.Fatal("expected nested tool-run log under tool_runs/")
	}

	_ = s.Create("turn-2", "sess-3", "proj-a", "a", "two")
	s.Append("turn-2", "user", map[string]any{"content": "two"})
	s.Append("turn-2", "assistant", map[string]any{"content": "a2"})
	s.EndTurn("turn-2", domain.TurnCompleted)

	all := s.LoadSessionMessages("sess-3", "", 0)
	for _, m := range all {
		if m.Content == "child-secret" || m.Content == "sub" {
			t.Fatalf("nested tool-run leaked into session history: %+v", all)
		}
	}
	if len(all) != 4 {
		t.Fatalf("want 4 parent msgs, got %d %+v", len(all), all)
	}

	retained := s.LoadSessionMessages("sess-3", "turn-2", 0)
	if len(retained) != 2 {
		t.Fatalf("retain from turn-2: want 2 msgs, got %d %+v", len(retained), retained)
	}
	if retained[0].Content != "two" || retained[1].Content != "a2" {
		t.Fatalf("retained: %+v", retained)
	}
}

func TestLoadSessionMessagesLegacyToolCall(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))
	_ = s.Create("turn-legacy", "sess-leg", "proj-a", "a", "g")
	s.Append("turn-legacy", "user", map[string]any{"content": "g"})
	writeToolPair(s, "turn-legacy", "c1", "read_file")
	s.Append("turn-legacy", "assistant", map[string]any{"content": "ok"})
	s.EndTurn("turn-legacy", domain.TurnCompleted)

	msgs := s.LoadSessionMessages("sess-leg", "", 0)
	if len(msgs) != 4 {
		t.Fatalf("want 4, got %d %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 {
		t.Fatalf("legacy tool_call -> assistant: %+v", msgs[1])
	}
	if msgs[2].Role != "tool" || msgs[2].Content != "ok" {
		t.Fatalf("tool result: %+v", msgs[2])
	}
}

func TestLoadSessionMessagesHonorsRetainSkip(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	_ = s.Create("turn-1", "sess-skip", "proj-a", "a", "g")
	s.Append("turn-1", "user", map[string]any{"content": "u1"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "c1", "name": "read_file", "arguments": map[string]any{"p": "a"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "c1", "name": "read_file", "output": "OLD"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "c2", "name": "read_file", "arguments": map[string]any{"p": "b"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "c2", "name": "read_file", "output": "NEW"})
	s.EndTurn("turn-1", domain.TurnCompleted)

	// Skip first 3 messages (user + first tool pair), keep second pair.
	msgs := s.LoadSessionMessages("sess-skip", "turn-1", 3)
	if len(msgs) != 2 {
		t.Fatalf("want 2 msgs after skip, got %d %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "assistant" || len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].ID != "c2" {
		t.Fatalf("expected second assistant tool_calls, got %+v", msgs[0])
	}
	if msgs[1].Role != "tool" || msgs[1].Content != "NEW" {
		t.Fatalf("expected NEW tool result, got %+v", msgs[1])
	}
}

func TestLoadSessionMessagesReordersLateToolResults(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	// Mirrors a corrupted Weixin turn log: http_request result arrived after an
	// intervening foreign tool pair (cross-session Log race).
	_ = s.Create("turn-1", "sess-late", "proj-a", "a", "g")
	s.Append("turn-1", "user", map[string]any{"content": "天气"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "http1", "name": "http_request", "arguments": map[string]any{"url": "x"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "foreign", "name": "delegate_agent", "output": "leak"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "read1", "name": "read_file", "arguments": map[string]any{"path": "y"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "read1", "name": "read_file", "output": "file"})
	s.Append("turn-1", "assistant", map[string]any{"content": "office done"})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "http1", "name": "http_request", "output": "weather-json"})
	s.Append("turn-1", "assistant", map[string]any{"content": "预报"})
	s.EndTurn("turn-1", domain.TurnCompleted)

	msgs := s.LoadSessionMessages("sess-late", "", 0)
	if len(msgs) != 7 {
		t.Fatalf("want 7 msgs (orphan foreign dropped, http result reordered), got %d %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 || msgs[1].ToolCalls[0].ID != "http1" {
		t.Fatalf("msg1 assistant http1: %+v", msgs[1])
	}
	if msgs[2].Role != "tool" || msgs[2].ToolCallID != "http1" || msgs[2].Content != "weather-json" {
		t.Fatalf("msg2 must be immediate http1 result, got %+v", msgs[2])
	}
	if msgs[3].Role != "assistant" || len(msgs[3].ToolCalls) != 1 || msgs[3].ToolCalls[0].ID != "read1" {
		t.Fatalf("msg3 assistant read1: %+v", msgs[3])
	}
	if msgs[4].Role != "tool" || msgs[4].ToolCallID != "read1" {
		t.Fatalf("msg4 read1 result: %+v", msgs[4])
	}
	if msgs[5].Role != "assistant" || msgs[5].Content != "office done" {
		t.Fatalf("msg5 office: %+v", msgs[5])
	}
	if msgs[6].Role != "assistant" || msgs[6].Content != "预报" {
		t.Fatalf("msg6 forecast: %+v", msgs[6])
	}
}

func TestLoadSessionMessagesRepairsRetainSkipInsideToolPair(t *testing.T) {
	root := t.TempDir()
	s := NewTurnLogStore(testProjector(root))

	_ = s.Create("turn-1", "sess-mid-pair", "proj-a", "a", "g")
	s.Append("turn-1", "user", map[string]any{"content": "u1"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "c1", "name": "read_file", "arguments": map[string]any{"p": "a"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "c1", "name": "read_file", "output": "ORPHAN"})
	s.Append("turn-1", "assistant", map[string]any{
		"tool_calls": []any{
			map[string]any{"id": "c2", "name": "read_file", "arguments": map[string]any{"p": "b"}},
		},
	})
	s.Append("turn-1", "tool_result", map[string]any{"call_id": "c2", "name": "read_file", "output": "PAIRED"})
	s.EndTurn("turn-1", domain.TurnCompleted)

	// A malformed/stale checkpoint skips user + c1 assistant but leaves c1's
	// result at the head. Recovery must drop that orphan and keep the next pair.
	msgs := s.LoadSessionMessages("sess-mid-pair", "turn-1", 2)
	if len(msgs) != 2 {
		t.Fatalf("want only the complete c2 pair, got %d %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "assistant" || len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].ID != "c2" {
		t.Fatalf("expected c2 assistant, got %+v", msgs[0])
	}
	if msgs[1].Role != "tool" || msgs[1].ToolCallID != "c2" || msgs[1].Content != "PAIRED" {
		t.Fatalf("expected c2 result, got %+v", msgs[1])
	}
}
