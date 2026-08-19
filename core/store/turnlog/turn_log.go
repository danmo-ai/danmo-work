// Package turnlog reconstructs LLM chat history from the DB-backed turn log
// (single source of truth: turns + turn_log_entries in work.db) and renders
// backward-compatible JSONL exports for debugging and zip download.
package turnlog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type TurnLogStore struct {
	repo port.TurnLogRepo
	mu   sync.Mutex
	// seq caches the last used entry seq per turn so the hot Append path does
	// not need a MAX(seq) query per message. DB rows remain the truth; the
	// cache is lazily rehydrated (resume, process restart).
	seq map[string]int
}

func NewTurnLogStore(repo port.TurnLogRepo) *TurnLogStore {
	return &TurnLogStore{repo: repo, seq: make(map[string]int)}
}

// Create registers a turn in the log. Idempotent: on an existing turn it only
// fills blank meta fields (resume after EndTurn or process restart) and never
// resets the persisted status — explicit status transitions belong to
// TurnRepo.UpdateStatus / EndTurn.
func (s *TurnLogStore) Create(turnID, sessionID, projectID, agentID, goal string) error {
	return s.create(turnID, sessionID, projectID, agentID, goal, false)
}

// CreateNested registers a nested tool-run turn (e.g. delegate_agent).
// Nested logs are for zip/debug only and are never replayed as parent
// session LLM history.
func (s *TurnLogStore) CreateNested(turnID, sessionID, projectID, agentID, goal string) error {
	return s.create(turnID, sessionID, projectID, agentID, goal, true)
}

func (s *TurnLogStore) create(turnID, sessionID, projectID, agentID, goal string, nested bool) error {
	if projectID == "" {
		projectID = "_default"
	}
	ctx := context.Background()
	if err := s.repo.UpsertTurnMeta(ctx, domain.TurnLog{
		ID: turnID, SessionID: sessionID, ProjectID: projectID,
		AgentID: agentID, Goal: goal, Status: domain.TurnRunning, Nested: nested,
	}); err != nil {
		return err
	}
	maxSeq, err := s.repo.MaxSeq(ctx, turnID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.seq[turnID] = maxSeq
	s.mu.Unlock()
	return nil
}

// Append persists a single turn-log entry.
//
// Allowed types for LLM reconstruction: user, assistant, tool_result,
// and legacy tool_call. Turn start/end live in the turns table.
//
// Do NOT call Append with diagnostic or audit types (e.g. "llm_error",
// "step", "permission_*"). Use Stream Events (port.EventStream) for those.
func (s *TurnLogStore) Append(turnID, typ string, data map[string]any) {
	ctx := context.Background()
	s.mu.Lock()
	seq, ok := s.seq[turnID]
	if !ok {
		max, err := s.repo.MaxSeq(ctx, turnID)
		if err != nil {
			s.mu.Unlock()
			log.Printf("[turnlog] append %s type=%s: max seq: %v", turnID, typ, err)
			return
		}
		seq = max
	}
	seq++
	s.seq[turnID] = seq
	s.mu.Unlock()

	if err := s.repo.AppendEntry(ctx, port.TurnLogEntryRecord{
		TurnID: turnID, Seq: seq, Type: typ, Data: data,
	}); err != nil {
		// A failed append means history silently diverges from what the LLM
		// saw; at minimum make the divergence visible in the log.
		log.Printf("[turnlog] append %s seq=%d type=%s: %v", turnID, seq, typ, err)
	}
}

// EndTurn records the terminal status. The repo never overwrites an existing
// terminal status (a concurrent CancelTurn wins over the finishing goroutine).
func (s *TurnLogStore) EndTurn(turnID string, status domain.TurnStatus) {
	if err := s.repo.EndTurn(context.Background(), turnID, status); err != nil {
		log.Printf("[turnlog] end turn %s status=%s: %v", turnID, status, err)
	}
	s.mu.Lock()
	delete(s.seq, turnID)
	s.mu.Unlock()
}

// ListTurnIDs returns the session's non-nested turn IDs in ascending order.
// Nested tool_runs logs are excluded from session LLM history replay.
func (s *TurnLogStore) ListTurnIDs(sessionID string) []string {
	ids, err := s.repo.ListSessionTurnIDs(context.Background(), sessionID, false)
	if err != nil {
		log.Printf("[turnlog] list turn ids session=%s: %v", sessionID, err)
		return nil
	}
	return ids
}

func (s *TurnLogStore) LoadForRecovery(turnID string) (goal string, entries []map[string]any) {
	ctx := context.Background()
	meta, ok, err := s.repo.GetTurnMeta(ctx, turnID)
	if err != nil {
		log.Printf("[turnlog] load for recovery %s: %v", turnID, err)
		return "", nil
	}
	if ok {
		goal = meta.Goal
	}
	all := s.entryMaps(ctx, turnID)
	return goal, trimIncompleteTurnEntries(all)
}

// ListIncompleteToolCalls returns tool invocations present in the raw log
// that do not yet have a matching tool_result. Recovery uses this as the
// authoritative open set before writing synthetic failures and resuming.
func (s *TurnLogStore) ListIncompleteToolCalls(turnID string) []port.IncompleteToolCall {
	return listIncompleteToolCalls(s.entryMaps(context.Background(), turnID))
}

func listIncompleteToolCalls(all []map[string]any) []port.IncompleteToolCall {
	haveResult := make(map[string]bool)
	for _, e := range all {
		if e["type"] != "tool_result" {
			continue
		}
		data, _ := e["data"].(map[string]any)
		if id := stringField(data, "call_id"); id != "" {
			haveResult[id] = true
		}
	}

	var out []port.IncompleteToolCall
	seen := make(map[string]bool)
	for _, e := range all {
		typ, _ := e["type"].(string)
		data, _ := e["data"].(map[string]any)
		switch typ {
		case "assistant":
			for _, tc := range toolCallsFromData(data) {
				if tc.ID == "" || haveResult[tc.ID] || seen[tc.ID] {
					continue
				}
				seen[tc.ID] = true
				out = append(out, port.IncompleteToolCall{
					CallID: tc.ID, Name: tc.Name, Input: tc.Arguments,
				})
			}
		case "tool_call":
			id := stringField(data, "call_id")
			if id == "" || haveResult[id] || seen[id] {
				continue
			}
			seen[id] = true
			input, _ := data["input"].(map[string]any)
			out = append(out, port.IncompleteToolCall{
				CallID: id, Name: stringField(data, "name"), Input: input,
			})
		}
	}
	return out
}

// LoadSessionMessages rebuilds full LLM chat history from the session's turn
// log. If retainFromTurnID is non-empty, only that turn and later turns are
// included (compaction window). retainSkipMessages drops leading messages
// inside the first retained turn so mid-turn cuts can land on tool-pair
// boundaries.
func (s *TurnLogStore) LoadSessionMessages(sessionID, retainFromTurnID string, retainSkipMessages int) []port.ChatMessage {
	ctx := context.Background()
	ids, err := s.repo.ListSessionTurnIDs(ctx, sessionID, false)
	if err != nil {
		log.Printf("[turnlog] load session messages %s: %v", sessionID, err)
		return nil
	}
	var out []port.ChatMessage
	for _, id := range ids {
		if retainFromTurnID != "" && id < retainFromTurnID {
			continue
		}
		msgs := s.loadTurnMessages(ctx, id)
		if id == retainFromTurnID && retainSkipMessages > 0 {
			if retainSkipMessages >= len(msgs) {
				continue
			}
			msgs = msgs[retainSkipMessages:]
		}
		// Checkpoint cursors should be written at message-block boundaries, but
		// repair the retained slice defensively so a stale/manual cursor can
		// never replay orphan calls or results to an LLM.
		msgs = keepCompleteChatToolPairs(msgs)
		out = append(out, msgs...)
	}
	return out
}

// keepCompleteChatToolPairs returns an API-safe chat history.
// OpenAI-compatible providers require assistant(tool_calls) to be followed
// immediately by matching tool messages. Corrupted turn logs (e.g. cross-session
// Log races) may leave results later in the stream; this rebuilds each pair as a
// contiguous block and drops orphan calls/results.
func keepCompleteChatToolPairs(messages []port.ChatMessage) []port.ChatMessage {
	results := make(map[string]port.ChatMessage, len(messages))
	for _, m := range messages {
		if m.Role == "tool" && m.ToolCallID != "" {
			if _, ok := results[m.ToolCallID]; !ok {
				results[m.ToolCallID] = m
			}
		}
	}

	used := make(map[string]bool, len(results))
	out := make([]port.ChatMessage, 0, len(messages))
	for _, m := range messages {
		switch {
		case m.Role == "assistant" && len(m.ToolCalls) > 0:
			calls := make([]port.ChatToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if _, ok := results[tc.ID]; ok && !used[tc.ID] {
					calls = append(calls, tc)
				}
			}
			if len(calls) == 0 {
				if m.Content != "" {
					cp := m
					cp.ToolCalls = nil
					out = append(out, cp)
				}
				continue
			}
			cp := m
			cp.ToolCalls = calls
			out = append(out, cp)
			for _, tc := range calls {
				out = append(out, results[tc.ID])
				used[tc.ID] = true
			}
		case m.Role == "tool":
			// Emitted with the owning assistant, or dropped as orphan.
			continue
		default:
			out = append(out, m)
		}
	}
	return out
}

func (s *TurnLogStore) LoadTurnMessages(turnID string) []port.ChatMessage {
	return s.loadTurnMessages(context.Background(), turnID)
}

func (s *TurnLogStore) loadTurnMessages(ctx context.Context, turnID string) []port.ChatMessage {
	entries := trimIncompleteTurnEntries(s.entryMaps(ctx, turnID))
	return entriesToChatMessages(entries)
}

// IsNestedToolRun reports whether this turn is a nested tool execution.
// Such turns are not parent session LLM history and must not be auto-resumed
// by RecoverRunning.
func (s *TurnLogStore) IsNestedToolRun(turnID string) bool {
	meta, ok, err := s.repo.GetTurnMeta(context.Background(), turnID)
	if err != nil {
		log.Printf("[turnlog] is nested tool run %s: %v", turnID, err)
		return false
	}
	return ok && meta.Nested
}

// ListSessionEntries returns every entry of every non-nested turn in the
// session, ordered by turn then seq (debug/inspection).
func (s *TurnLogStore) ListSessionEntries(sessionID string) []port.TurnLogEntryRecord {
	ctx := context.Background()
	ids, err := s.repo.ListSessionTurnIDs(ctx, sessionID, false)
	if err != nil {
		log.Printf("[turnlog] list session entries %s: %v", sessionID, err)
		return nil
	}
	var out []port.TurnLogEntryRecord
	for _, id := range ids {
		entries, err := s.repo.ListEntries(ctx, id)
		if err != nil {
			log.Printf("[turnlog] list entries %s: %v", id, err)
			continue
		}
		out = append(out, entries...)
	}
	return out
}

// entryMaps loads a turn's entries in the generic {seq,type,data} map shape
// consumed by the trim/reconstruction helpers.
func (s *TurnLogStore) entryMaps(ctx context.Context, turnID string) []map[string]any {
	entries, err := s.repo.ListEntries(ctx, turnID)
	if err != nil {
		log.Printf("[turnlog] list entries %s: %v", turnID, err)
		return nil
	}
	return recordsToMaps(entries)
}

func recordsToMaps(entries []port.TurnLogEntryRecord) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		m := map[string]any{"seq": e.Seq, "type": e.Type}
		if e.Data != nil {
			m["data"] = e.Data
		}
		out = append(out, m)
	}
	return out
}

type EntryJSON struct {
	Seq  int            `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// ListEntries returns a turn's raw entries (without synthetic start/end).
func (s *TurnLogStore) ListEntries(turnID string) []EntryJSON {
	entries, err := s.repo.ListEntries(context.Background(), turnID)
	if err != nil {
		log.Printf("[turnlog] list entries %s: %v", turnID, err)
		return nil
	}
	out := make([]EntryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, EntryJSON{Seq: e.Seq, Type: e.Type, Data: e.Data})
	}
	return out
}

// LoadRawLog renders the turn's log as JSONL bytes in the legacy on-disk
// format (synthetic "start"/"end" lines from turn meta plus the entry rows).
func (s *TurnLogStore) LoadRawLog(turnID string) ([]byte, error) {
	ctx := context.Background()
	meta, hasMeta, err := s.repo.GetTurnMeta(ctx, turnID)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListEntries(ctx, turnID)
	if err != nil {
		return nil, err
	}
	if !hasMeta && len(entries) == 0 {
		return nil, fmt.Errorf("turn log not found: %s", turnID)
	}
	return renderTurnJSONL(meta, entries), nil
}

// renderTurnJSONL renders the legacy JSONL file format from DB rows:
// a "start" line (seq 0), the entries, and an "end" line when the turn
// reached a terminal status.
func renderTurnJSONL(meta domain.TurnLog, entries []port.TurnLogEntryRecord) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	lastSeq := 0
	_ = enc.Encode(EntryJSON{Seq: 0, Type: "start", Data: map[string]any{
		"agent_id": meta.AgentID, "goal": meta.Goal,
	}})
	for _, e := range entries {
		_ = enc.Encode(EntryJSON{Seq: e.Seq, Type: e.Type, Data: e.Data})
		if e.Seq > lastSeq {
			lastSeq = e.Seq
		}
	}
	if meta.Status != "" && meta.Status != domain.TurnRunning {
		_ = enc.Encode(EntryJSON{Seq: lastSeq + 1, Type: "end", Data: map[string]any{
			"status": string(meta.Status),
		}})
	}
	return buf.Bytes()
}

// TurnLogNode represents a turn and its delegated children for zip packaging.
type TurnLogNode struct {
	TurnID   string         `json:"turnId"`
	AgentID  string         `json:"agentId,omitempty"`
	Goal     string         `json:"goal,omitempty"`
	File     string         `json:"file,omitempty"`
	Missing  bool           `json:"missing,omitempty"` // no log rows; events may still be present
	Children []*TurnLogNode `json:"children,omitempty"`
}

func (s *TurnLogStore) LoadTurnLogZip(turnID string, events []domain.StreamEvent) ([]byte, error) {
	ctx := context.Background()

	// Build parent->children map and per-turn meta from stream events.
	childrenOf := make(map[string][]string)
	metaByTurn := make(map[string]struct{ agentID, goal string })
	turnsWithEvents := make(map[string]bool)
	seenChild := make(map[string]bool)

	for _, ev := range events {
		if ev.TurnID != "" {
			turnsWithEvents[ev.TurnID] = true
		}
		if ev.Type == domain.EventTurnStarted {
			var p domain.TurnStartedPayload
			if err := json.Unmarshal(ev.Payload, &p); err == nil {
				m := metaByTurn[ev.TurnID]
				if p.AgentID != "" {
					m.agentID = p.AgentID
				}
				if p.Goal != "" {
					m.goal = p.Goal
				}
				metaByTurn[ev.TurnID] = m
			}
		}
		if ev.Type == domain.EventDelegateStarted {
			var p domain.DelegateStartedPayload
			if err := json.Unmarshal(ev.Payload, &p); err == nil && p.ChildTurnID != "" {
				if !seenChild[p.ChildTurnID] {
					childrenOf[ev.TurnID] = append(childrenOf[ev.TurnID], p.ChildTurnID)
					seenChild[p.ChildTurnID] = true
				}
				m := metaByTurn[p.ChildTurnID]
				if p.AgentID != "" {
					m.agentID = p.AgentID
				}
				if p.Goal != "" {
					m.goal = p.Goal
				}
				metaByTurn[p.ChildTurnID] = m
			}
		}
	}

	loadTurn := func(id string) (domain.TurnLog, []port.TurnLogEntryRecord, bool) {
		meta, hasMeta, err := s.repo.GetTurnMeta(ctx, id)
		if err != nil {
			log.Printf("[turnlog] zip meta %s: %v", id, err)
			return domain.TurnLog{}, nil, false
		}
		entries, err := s.repo.ListEntries(ctx, id)
		if err != nil {
			log.Printf("[turnlog] zip entries %s: %v", id, err)
			return domain.TurnLog{}, nil, false
		}
		return meta, entries, hasMeta || len(entries) > 0
	}

	rootMeta, rootEntries, rootHasLog := loadTurn(turnID)
	if !rootHasLog && !turnsWithEvents[turnID] {
		return nil, fmt.Errorf("turn log not found: %s", turnID)
	}

	// Collect related turns: prefer log rows; fall back to stream-event presence.
	files := make(map[string][]byte)
	turnIDs := make(map[string]bool)
	var collect func(id string) *TurnLogNode
	collect = func(id string) *TurnLogNode {
		meta, entries, hasLog := loadTurn(id)
		if id == turnID {
			meta, entries, hasLog = rootMeta, rootEntries, rootHasLog
		}
		evMeta := metaByTurn[id]
		hasEvents := turnsWithEvents[id]
		if !hasLog && !hasEvents && id != turnID {
			return nil
		}
		turnIDs[id] = true
		node := &TurnLogNode{TurnID: id}
		if hasLog {
			files[id] = renderTurnJSONL(meta, entries)
			node.File = id + ".jsonl"
			node.AgentID = meta.AgentID
			node.Goal = meta.Goal
		} else {
			node.Missing = true
		}
		if node.AgentID == "" {
			node.AgentID = evMeta.agentID
		}
		if node.Goal == "" {
			node.Goal = evMeta.goal
		}
		for _, childID := range childrenOf[id] {
			if child := collect(childID); child != nil {
				node.Children = append(node.Children, child)
			}
		}
		return node
	}

	rootNode := collect(turnID)

	relatedEvents := make([]domain.StreamEvent, 0)
	for _, ev := range events {
		if turnIDs[ev.TurnID] {
			relatedEvents = append(relatedEvents, ev)
		}
	}

	var nodes []*TurnLogNode
	if rootNode != nil {
		nodes = append(nodes, rootNode)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	manifest, _ := json.MarshalIndent(nodes, "", "  ")
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write(manifest)

	if len(relatedEvents) > 0 {
		var evBuf bytes.Buffer
		enc := json.NewEncoder(&evBuf)
		for _, ev := range relatedEvents {
			_ = enc.Encode(ev)
		}
		w, _ = zw.Create("events.jsonl")
		_, _ = w.Write(evBuf.Bytes())
	}

	for id, data := range files {
		w, _ = zw.Create(id + ".jsonl")
		_, _ = w.Write(data)
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// trimIncompleteTurnEntries keeps reconstructable whitelist entries and drops
// unpaired tool calls. Trailing assistants/tool_calls without any results are
// removed; within a batch, tool_calls that lack a matching tool_result are
// stripped (aligned with runtime keepCompleteToolPairs) so resume stays API-valid.
func trimIncompleteTurnEntries(all []map[string]any) []map[string]any {
	// Collect whitelist indices first.
	var kept []map[string]any
	for _, e := range all {
		typ, _ := e["type"].(string)
		switch typ {
		case "user", "assistant", "tool_call", "tool_result":
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return nil
	}

	// Drop trailing unpaired tool call / assistant-with-tools without matching results.
	for len(kept) > 0 {
		last := kept[len(kept)-1]
		typ, _ := last["type"].(string)
		switch typ {
		case "tool_result", "user":
			return filterPartialToolPairs(kept)
		case "assistant":
			data, _ := last["data"].(map[string]any)
			if !assistantHasToolCalls(data) {
				return filterPartialToolPairs(kept) // text-only assistant is complete
			}
			// Assistant with tool_calls but no following tool_results — drop it.
			kept = kept[:len(kept)-1]
		case "tool_call":
			kept = kept[:len(kept)-1]
		default:
			kept = kept[:len(kept)-1]
		}
	}
	return nil
}

// filterPartialToolPairs drops tool_calls (and legacy tool_call entries) that
// have no matching tool_result, and drops orphan tool_results. Mutates entry
// data copies so assistant batches remain paired for LLM replay.
func filterPartialToolPairs(entries []map[string]any) []map[string]any {
	haveResult := make(map[string]bool)
	for _, e := range entries {
		if e["type"] != "tool_result" {
			continue
		}
		data, _ := e["data"].(map[string]any)
		if id := stringField(data, "call_id"); id != "" {
			haveResult[id] = true
		}
	}

	out := make([]map[string]any, 0, len(entries))
	keptCallIDs := make(map[string]bool)
	for _, e := range entries {
		typ, _ := e["type"].(string)
		switch typ {
		case "assistant":
			data, _ := e["data"].(map[string]any)
			if data == nil {
				out = append(out, e)
				continue
			}
			raw, ok := data["tool_calls"]
			if !ok || raw == nil {
				out = append(out, e)
				continue
			}
			arr, ok := raw.([]any)
			if !ok {
				out = append(out, e)
				continue
			}
			filtered := make([]any, 0, len(arr))
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id := stringField(m, "id")
				if id == "" || !haveResult[id] {
					continue
				}
				filtered = append(filtered, item)
				keptCallIDs[id] = true
			}
			if len(filtered) == len(arr) {
				out = append(out, e)
				continue
			}
			content := stringField(data, "content")
			if len(filtered) == 0 && content == "" {
				continue // nothing reconstructable
			}
			cpData := copyStringAnyMap(data)
			if len(filtered) == 0 {
				delete(cpData, "tool_calls")
			} else {
				cpData["tool_calls"] = filtered
			}
			cp := copyStringAnyMap(e)
			cp["data"] = cpData
			out = append(out, cp)
		case "tool_call":
			data, _ := e["data"].(map[string]any)
			id := stringField(data, "call_id")
			if id == "" || !haveResult[id] {
				continue
			}
			keptCallIDs[id] = true
			out = append(out, e)
		case "tool_result":
			// Deferred: only keep if call survives above. Collect for second pass.
			out = append(out, e)
		default:
			out = append(out, e)
		}
	}

	final := make([]map[string]any, 0, len(out))
	for _, e := range out {
		if e["type"] == "tool_result" {
			data, _ := e["data"].(map[string]any)
			id := stringField(data, "call_id")
			if id != "" && !keptCallIDs[id] {
				continue
			}
		}
		final = append(final, e)
	}
	return final
}

func copyStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func assistantHasToolCalls(data map[string]any) bool {
	if data == nil {
		return false
	}
	raw, ok := data["tool_calls"]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case []any:
		return len(v) > 0
	case []map[string]any:
		return len(v) > 0
	default:
		return false
	}
}

func entriesToChatMessages(entries []map[string]any) []port.ChatMessage {
	var out []port.ChatMessage
	for _, e := range entries {
		typ, _ := e["type"].(string)
		data, _ := e["data"].(map[string]any)
		if data == nil {
			data = map[string]any{}
		}
		switch typ {
		case "user":
			msg := port.ChatMessage{Role: "user", Content: stringField(data, "content")}
			if parts := partsFromData(data); len(parts) > 0 {
				msg.Parts = parts
			}
			out = append(out, msg)
		case "assistant":
			msg := port.ChatMessage{Role: "assistant", Content: stringField(data, "content")}
			msg.ToolCalls = toolCallsFromData(data)
			out = append(out, msg)
		case "tool_call":
			// Legacy: one assistant message per call.
			callID := stringField(data, "call_id")
			name := stringField(data, "name")
			args, _ := data["input"].(map[string]any)
			out = append(out, port.ChatMessage{
				Role: "assistant",
				ToolCalls: []port.ChatToolCall{{
					ID: callID, Name: name, Arguments: args,
				}},
			})
		case "tool_result":
			out = append(out, port.ChatMessage{
				Role:       "tool",
				ToolCallID: stringField(data, "call_id"),
				Name:       stringField(data, "name"),
				Content:    stringField(data, "output"),
			})
		}
	}
	return out
}

func stringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

func toolCallsFromData(data map[string]any) []port.ChatToolCall {
	raw, ok := data["tool_calls"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []port.ChatToolCall
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		args, _ := m["arguments"].(map[string]any)
		if args == nil {
			args, _ = m["input"].(map[string]any)
		}
		out = append(out, port.ChatToolCall{
			ID:        stringField(m, "id"),
			Name:      stringField(m, "name"),
			Arguments: args,
		})
	}
	return out
}

func partsFromData(data map[string]any) []port.ChatContentPart {
	raw, ok := data["parts"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []port.ChatContentPart
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		// v1: skip base64 blobs; only keep text metadata if present without data.
		if stringField(m, "data") != "" {
			continue
		}
		out = append(out, port.ChatContentPart{
			Type:     stringField(m, "type"),
			MimeType: stringField(m, "mimeType"),
			Name:     stringField(m, "name"),
			Text:     stringField(m, "text"),
		})
	}
	return out
}
