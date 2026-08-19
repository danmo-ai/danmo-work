package service

import (
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type TurnLogManager struct {
	store port.TurnLogStore
}

func NewTurnLogManager(store port.TurnLogStore) *TurnLogManager {
	return &TurnLogManager{store: store}
}

func (m *TurnLogManager) Create(turnID, sessionID, projectID, agentID, goal string) error {
	return m.store.Create(turnID, sessionID, projectID, agentID, goal)
}

func (m *TurnLogManager) CreateNested(turnID, sessionID, projectID, agentID, goal string) error {
	return m.store.CreateNested(turnID, sessionID, projectID, agentID, goal)
}

// Append writes a turn-log entry used for LLM message reconstruction
// (user, assistant, tool_result, legacy tool_call). Do NOT write diagnostic
// entries — those belong in Stream Events.
func (m *TurnLogManager) Append(turnID, typ string, data map[string]any) {
	m.store.Append(turnID, typ, data)
}

func (m *TurnLogManager) EndTurn(turnID string, status domain.TurnStatus) {
	m.store.EndTurn(turnID, status)
}

func (m *TurnLogManager) ListTurnIDs(sessionID string) []string {
	return m.store.ListTurnIDs(sessionID)
}

func (m *TurnLogManager) LoadForRecovery(turnID string) (goal string, entries []map[string]any) {
	return m.store.LoadForRecovery(turnID)
}

func (m *TurnLogManager) ListIncompleteToolCalls(turnID string) []port.IncompleteToolCall {
	return m.store.ListIncompleteToolCalls(turnID)
}

func (m *TurnLogManager) LoadSessionMessages(sessionID, retainFromTurnID string, retainSkipMessages int) []port.ChatMessage {
	return m.store.LoadSessionMessages(sessionID, retainFromTurnID, retainSkipMessages)
}

func (m *TurnLogManager) LoadTurnMessages(turnID string) []port.ChatMessage {
	return m.store.LoadTurnMessages(turnID)
}

func (m *TurnLogManager) IsNestedToolRun(turnID string) bool {
	return m.store.IsNestedToolRun(turnID)
}

func (m *TurnLogManager) ListSessionEntries(sessionID string) []port.TurnLogEntryRecord {
	return m.store.ListSessionEntries(sessionID)
}

func (m *TurnLogManager) LoadRawLog(turnID string) ([]byte, error) {
	return m.store.LoadRawLog(turnID)
}

func (m *TurnLogManager) LoadTurnLogZip(turnID string, events []domain.StreamEvent) ([]byte, error) {
	return m.store.LoadTurnLogZip(turnID, events)
}
