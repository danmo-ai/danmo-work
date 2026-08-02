package turnlog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type TurnLogEntry struct {
	Seq  int            `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type TurnLogStore struct {
	mu              sync.Mutex
	projector       func(projectID string) string
	logs            map[string]*turnFile
	sessionProjects map[string]string
}

type turnFile struct {
	log      *domain.TurnLog
	f        *os.File
	seq      int
	filePath string
}

func NewTurnLogStore(projector func(projectID string) string) *TurnLogStore {
	return &TurnLogStore{projector: projector, logs: make(map[string]*turnFile), sessionProjects: make(map[string]string)}
}

func (s *TurnLogStore) sessionsDir(projectID string) string {
	return filepath.Join(s.projector(projectID), "sessions")
}

// Create opens a turn log for writing under sessions/{sessionID}/{turnID}.jsonl.
// These session-level logs are the LLM history source for LoadSessionMessages.
//
// If the JSONL already exists (same-process resume after EndTurn, or process
// restart with the file still on disk), it reopens for append and does NOT
// write another "start" entry. Otherwise it creates a fresh file with start.
func (s *TurnLogStore) Create(turnID, sessionID, projectID, agentID, goal string) error {
	return s.create(turnID, sessionID, projectID, agentID, goal, "")
}

// CreateNested opens a debug log under sessions/{sessionID}/tool_runs/{turnID}.jsonl
// for a nested tool execution (e.g. delegate_agent). Nested logs are for zip/debug
// only and are never replayed as parent session LLM history.
func (s *TurnLogStore) CreateNested(turnID, sessionID, projectID, agentID, goal string) error {
	return s.create(turnID, sessionID, projectID, agentID, goal, "tool_runs")
}

func (s *TurnLogStore) create(turnID, sessionID, projectID, agentID, goal, subdir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectID == "" {
		projectID = "_default"
	}
	s.sessionProjects[sessionID] = projectID

	if tf, ok := s.loadTurnFileLocked(turnID); ok {
		if goal != "" && tf.log.Goal == "" {
			tf.log.Goal = goal
		}
		if agentID != "" && tf.log.AgentID == "" {
			tf.log.AgentID = agentID
		}
		tf.log.SessionID = sessionID
		tf.log.Status = domain.TurnRunning
		return s.reopenAppendLocked(tf)
	}

	dir := filepath.Join(s.sessionsDir(projectID), sessionID)
	if subdir != "" {
		dir = filepath.Join(dir, subdir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s.jsonl", turnID))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	tf := &turnFile{
		log:      &domain.TurnLog{ID: turnID, SessionID: sessionID, Status: domain.TurnRunning, AgentID: agentID, Goal: goal},
		f:        f,
		seq:      0,
		filePath: filePath,
	}
	s.logs[turnID] = tf

	tf.seq = 1
	tf.writeEntry(TurnLogEntry{Seq: tf.seq, Type: "start", Data: map[string]any{"agent_id": agentID, "goal": goal}})
	return nil
}

// Append writes a single entry to the turn's JSONL log.
//
// Allowed types for LLM reconstruction: user, assistant, tool_result,
// and legacy tool_call. start/end are written by Create / EndTurn.
//
// Do NOT call Append with diagnostic or audit types (e.g. "llm_error",
// "step", "permission_*"). Use Stream Events (port.EventStream) for those.
func (s *TurnLogStore) Append(turnID, typ string, data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, ok := s.logs[turnID]
	if !ok || tf.f == nil {
		return
	}
	tf.seq++
	tf.writeEntry(TurnLogEntry{Seq: tf.seq, Type: typ, Data: data})
}

func (s *TurnLogStore) EndTurn(turnID string, status domain.TurnStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, ok := s.logs[turnID]
	if !ok {
		return
	}
	tf.log.Status = status
	if tf.f != nil {
		tf.seq++
		mapStatus := map[string]any{"status": string(status)}
		tf.writeEntry(TurnLogEntry{Seq: tf.seq, Type: "end", Data: mapStatus})
		_ = tf.f.Close()
		tf.f = nil
	}
}

func (s *TurnLogStore) LastTurn(sessionID string) (turnID string, status domain.TurnStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.sessionDir(sessionID)
	entries, _ := os.ReadDir(dir)
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		if len(name) > 6 && name[len(name)-6:] == ".jsonl" {
			id := name[:len(name)-6]
			if tf, ok := s.logs[id]; ok {
				return tf.log.ID, tf.log.Status
			}
		}
	}
	return "", ""
}

func (s *TurnLogStore) ListTurns(sessionID string) []domain.TurnLog {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.listTurnIDsLocked(sessionID)
	out := make([]domain.TurnLog, 0, len(ids))
	for _, id := range ids {
		if _, ok := s.loadTurnFileLocked(id); ok {
			if tf, ok := s.logs[id]; ok {
				out = append(out, *tf.log)
			}
		}
	}
	return out
}

func (s *TurnLogStore) ListTurnIDs(sessionID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listTurnIDsLocked(sessionID)
}

func (s *TurnLogStore) listTurnIDsLocked(sessionID string) []string {
	dir := s.sessionDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, entry := range entries {
		// Only top-level *.jsonl — nested tool_runs/ logs are excluded from
		// session LLM history replay.
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 6 && name[len(name)-6:] == ".jsonl" {
			ids = append(ids, name[:len(name)-6])
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *TurnLogStore) LastStatus(sessionID string) domain.TurnStatus {
	_, status := s.LastTurn(sessionID)
	return status
}

func (s *TurnLogStore) sessionDir(sessionID string) string {
	if pid, ok := s.sessionProjects[sessionID]; ok {
		return filepath.Join(s.sessionsDir(pid), sessionID)
	}
	projectIDs, _ := os.ReadDir(s.projector(""))
	for _, e := range projectIDs {
		if !e.IsDir() {
			continue
		}
		sessionsRoot := filepath.Join(s.projector(e.Name()), "sessions")
		entries, err := os.ReadDir(sessionsRoot)
		if err != nil {
			continue
		}
		for _, se := range entries {
			if se.IsDir() && se.Name() == sessionID {
				s.sessionProjects[sessionID] = e.Name()
				return filepath.Join(sessionsRoot, sessionID)
			}
		}
	}
	dir := filepath.Join(s.sessionsDir("_default"), sessionID)
	os.MkdirAll(dir, 0755)
	return dir
}

type EntryJSON struct {
	Seq  int            `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

func (s *TurnLogStore) LoadForRecovery(turnID string) (goal string, entries []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return "", nil
	}
	goal = tf.log.Goal

	all := s.readEntriesLocked(tf.filePath)
	entries = trimIncompleteTurnEntries(all)
	return goal, entries
}

// ListIncompleteToolCalls returns tool invocations present in the raw JSONL
// that do not yet have a matching tool_result. Recovery uses this as the
// authoritative open set before writing synthetic failures and resuming.
func (s *TurnLogStore) ListIncompleteToolCalls(turnID string) []port.IncompleteToolCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return nil
	}
	return listIncompleteToolCalls(s.readEntriesLocked(tf.filePath))
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

// LoadSessionMessages rebuilds full LLM chat history from session turn JSONL.
// If retainFromTurnID is non-empty, only that turn and later turns are included
// (compaction window). retainSkipMessages drops leading messages inside the
// first retained turn so mid-turn cuts can land on tool-pair boundaries.
func (s *TurnLogStore) LoadSessionMessages(sessionID, retainFromTurnID string, retainSkipMessages int) []port.ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.listTurnIDsLocked(sessionID)
	var out []port.ChatMessage
	for _, id := range ids {
		if retainFromTurnID != "" && id < retainFromTurnID {
			continue
		}
		msgs := s.loadTurnMessagesLocked(id)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadTurnMessagesLocked(turnID)
}

func (s *TurnLogStore) loadTurnMessagesLocked(turnID string) []port.ChatMessage {
	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return nil
	}
	entries := trimIncompleteTurnEntries(s.readEntriesLocked(tf.filePath))
	return entriesToChatMessages(entries)
}

// IsNestedToolRun reports whether this turn log lives under tool_runs/
// (nested tool execution). Such turns are not parent session LLM history and
// must not be auto-resumed by RecoverRunning.
func (s *TurnLogStore) IsNestedToolRun(turnID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return false
	}
	return strings.Contains(filepath.ToSlash(tf.filePath), "/tool_runs/")
}

func (s *TurnLogStore) readEntriesLocked(filePath string) []map[string]any {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var all []map[string]any
	for dec.More() {
		var e map[string]any
		if err := dec.Decode(&e); err == nil {
			all = append(all, e)
		}
	}
	return all
}

// trimIncompleteTurnEntries keeps reconstructable whitelist entries and drops
// unpaired tool calls. Trailing assistants/tool_calls without any results are
// removed; within a batch, tool_calls that lack a matching tool_result are
// stripped (aligned with salvagePairedTurnDelta) so resume stays API-valid.
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

func (s *TurnLogStore) LoadRawLog(turnID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return nil, fmt.Errorf("turn log not found: %s", turnID)
	}

	return os.ReadFile(tf.filePath)
}

// TurnLogNode represents a turn and its delegated children for zip packaging.
type TurnLogNode struct {
	TurnID   string         `json:"turnId"`
	AgentID  string         `json:"agentId,omitempty"`
	Goal     string         `json:"goal,omitempty"`
	File     string         `json:"file,omitempty"`
	Missing  bool           `json:"missing,omitempty"` // JSONL absent; events may still be present
	Children []*TurnLogNode `json:"children,omitempty"`
}

func (s *TurnLogStore) LoadTurnLogZip(turnID string, events []domain.StreamEvent) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	rootTF, rootHasFile := s.loadTurnFileLocked(turnID)
	if !rootHasFile && !turnsWithEvents[turnID] {
		return nil, fmt.Errorf("turn log not found: %s", turnID)
	}

	// Collect related turns: prefer JSONL; fall back to stream-event presence.
	files := make(map[string]string)
	turnIDs := make(map[string]bool)
	var collect func(id string) *TurnLogNode
	collect = func(id string) *TurnLogNode {
		tf, hasFile := s.loadTurnFileLocked(id)
		meta := metaByTurn[id]
		hasEvents := turnsWithEvents[id]
		if !hasFile && !hasEvents && id != turnID {
			return nil
		}
		turnIDs[id] = true
		node := &TurnLogNode{TurnID: id}
		if hasFile {
			files[id] = tf.filePath
			node.File = id + ".jsonl"
			node.AgentID = tf.log.AgentID
			node.Goal = tf.log.Goal
		} else {
			node.Missing = true
		}
		if node.AgentID == "" {
			node.AgentID = meta.agentID
		}
		if node.Goal == "" {
			node.Goal = meta.goal
		}
		for _, childID := range childrenOf[id] {
			if child := collect(childID); child != nil {
				node.Children = append(node.Children, child)
			}
		}
		return node
	}

	rootNode := collect(turnID)
	if rootTF != nil && rootNode != nil {
		if rootNode.AgentID == "" {
			rootNode.AgentID = rootTF.log.AgentID
		}
		if rootNode.Goal == "" {
			rootNode.Goal = rootTF.log.Goal
		}
	}

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

	for id, fp := range files {
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		w, _ = zw.Create(id + ".jsonl")
		_, _ = w.Write(data)
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *TurnLogStore) ListEntries(turnID string) []EntryJSON {
	s.mu.Lock()
	defer s.mu.Unlock()

	tf, ok := s.loadTurnFileLocked(turnID)
	if !ok {
		return nil
	}

	f, err := os.Open(tf.filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	entries := make([]EntryJSON, 0)
	for dec.More() {
		var e EntryJSON
		if err := dec.Decode(&e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func LoadTurnLog(projector func(projectID string) string, projectID, sessionID string) ([]EntryJSON, error) {
	dir := filepath.Join(projector(projectID), "sessions", sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var all []EntryJSON
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 6 && name[len(name)-6:] == ".jsonl" {
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			dec := json.NewDecoder(f)
			for dec.More() {
				var e EntryJSON
				if err := dec.Decode(&e); err == nil {
					all = append(all, e)
				}
			}
			f.Close()
		}
	}
	return all, nil
}

func (tf *turnFile) writeEntry(e TurnLogEntry) {
	b, _ := json.Marshal(e)
	tf.f.Write(append(b, '\n'))
}

// loadTurnFileLocked returns an in-memory turnFile, rehydrating from disk when
// needed (e.g. after process restart). Caller must hold s.mu.
func (s *TurnLogStore) loadTurnFileLocked(turnID string) (*turnFile, bool) {
	if tf, ok := s.logs[turnID]; ok {
		return tf, true
	}
	filePath, sessionID, projectID := s.locateTurnFile(turnID)
	if filePath == "" {
		return nil, false
	}
	goal, agentID, status, lastSeq := readTurnMeta(filePath)
	tf := &turnFile{
		log: &domain.TurnLog{
			ID:        turnID,
			SessionID: sessionID,
			Status:    status,
			AgentID:   agentID,
			Goal:      goal,
		},
		f:        nil,
		seq:      lastSeq,
		filePath: filePath,
	}
	s.logs[turnID] = tf
	if sessionID != "" {
		s.sessionProjects[sessionID] = projectID
	}
	return tf, true
}

func (s *TurnLogStore) reopenAppendLocked(tf *turnFile) error {
	if tf.f != nil {
		return nil
	}
	f, err := os.OpenFile(tf.filePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	tf.f = f
	return nil
}

// locateTurnFile scans project data dirs for sessions/*/{turnID}.jsonl
// or sessions/*/tool_runs/{turnID}.jsonl.
func (s *TurnLogStore) locateTurnFile(turnID string) (filePath, sessionID, projectID string) {
	root := s.projector("")
	projectEntries, err := os.ReadDir(root)
	if err != nil {
		return "", "", ""
	}
	want := turnID + ".jsonl"
	for _, pe := range projectEntries {
		if !pe.IsDir() {
			continue
		}
		pid := pe.Name()
		sessionsRoot := filepath.Join(s.projector(pid), "sessions")
		sessionEntries, err := os.ReadDir(sessionsRoot)
		if err != nil {
			continue
		}
		for _, se := range sessionEntries {
			if !se.IsDir() {
				continue
			}
			sid := se.Name()
			for _, rel := range []string{
				filepath.Join(sessionsRoot, sid, want),
				filepath.Join(sessionsRoot, sid, "tool_runs", want),
			} {
				if st, err := os.Stat(rel); err == nil && !st.IsDir() {
					return rel, sid, pid
				}
			}
		}
	}
	return "", "", ""
}

func readTurnMeta(filePath string) (goal, agentID string, status domain.TurnStatus, lastSeq int) {
	status = domain.TurnRunning
	f, err := os.Open(filePath)
	if err != nil {
		return "", "", status, 0
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	for dec.More() {
		var e EntryJSON
		if err := dec.Decode(&e); err != nil {
			break
		}
		if e.Seq > lastSeq {
			lastSeq = e.Seq
		}
		switch e.Type {
		case "start":
			if e.Data != nil {
				if g, ok := e.Data["goal"].(string); ok && g != "" {
					goal = g
				}
				if a, ok := e.Data["agent_id"].(string); ok && a != "" {
					agentID = a
				}
			}
		case "end":
			if e.Data != nil {
				if st, ok := e.Data["status"].(string); ok && st != "" {
					status = domain.TurnStatus(st)
				}
			}
		}
	}
	return goal, agentID, status, lastSeq
}
