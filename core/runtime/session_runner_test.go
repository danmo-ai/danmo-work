package runtime

import (
	"errors"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/tool/builtin"
)

func TestReserveSessionTurnPreventsConcurrentStartAndResume(t *testing.T) {
	engine := &Engine{}
	if err := engine.reserveSessionTurn("session-1", "turn-1"); err != nil {
		t.Fatalf("reserve first turn: %v", err)
	}
	if err := engine.reserveSessionTurn("session-1", "turn-2"); !errors.Is(err, port.ErrSessionTurnRunning) {
		t.Fatalf("second turn should conflict, got %v", err)
	}

	// A stale goroutine must not release another turn's reservation.
	engine.releaseSessionTurn("session-1", "turn-2")
	if err := engine.reserveSessionTurn("session-1", "turn-3"); !errors.Is(err, port.ErrSessionTurnRunning) {
		t.Fatalf("stale release cleared active turn: %v", err)
	}

	engine.releaseSessionTurn("session-1", "turn-1")
	if err := engine.reserveSessionTurn("session-1", "turn-3"); err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
	if err := engine.reserveSessionTurn("session-2", "turn-4"); err != nil {
		t.Fatalf("different session should run independently: %v", err)
	}
}

func TestBuildTurnMessages_IncludesPreviousTurnMessages(t *testing.T) {
	engine := &Engine{
		turnRunner:   &TurnRunner{},
		knowledge:    builtin.NewKnowledge(),
		turnMessages: make(map[string][]Message),
	}
	agent := domain.Agent{
		ID:            "test-agent",
		SystemPrompt:   "You are a test assistant.",
		KnowledgeIDs:  []string{},
	}

	sessionID := "session-1"

	msgs1 := engine.buildTurnMessages(sessionID, agent, "hello turn 1", "")
	if len(msgs1) < 2 {
		t.Fatalf("turn 1: expected at least 2 messages (system + user), got %d", len(msgs1))
	}
	if msgs1[0].Role != RoleSystem {
		t.Errorf("turn 1: first message should be system, got %s", msgs1[0].Role)
	}
	if msgs1[len(msgs1)-1].Role != RoleUser || msgs1[len(msgs1)-1].Content != "hello turn 1" {
		t.Errorf("turn 1: last message should be user with goal, got role=%s content=%q",
			msgs1[len(msgs1)-1].Role, msgs1[len(msgs1)-1].Content)
	}

	engine.mu.Lock()
	engine.turnMessages[sessionID] = append(engine.turnMessages[sessionID], msgs1...)
	engine.mu.Unlock()

	msgs2 := engine.buildTurnMessages(sessionID, agent, "hello turn 2", "")

	turn1UserFound := false
	turn1SystemFound := false
	for _, msg := range msgs2 {
		if msg.Role == RoleUser && msg.Content == "hello turn 1" {
			turn1UserFound = true
		}
		if msg.Role == RoleSystem && msg.Content == msgs1[0].Content {
			turn1SystemFound = true
		}
	}
	if !turn1UserFound {
		t.Error("turn 2 messages do NOT contain turn 1's user message (cross-turn context lost)")
	}
	if !turn1SystemFound {
		t.Error("turn 2 messages do NOT contain turn 1's system prompt (cross-turn context lost)")
	}
	if msgs2[len(msgs2)-1].Role != RoleUser || msgs2[len(msgs2)-1].Content != "hello turn 2" {
		t.Errorf("turn 2: last message should be user with goal, got role=%s content=%q",
			msgs2[len(msgs2)-1].Role, msgs2[len(msgs2)-1].Content)
	}
	if len(msgs2) < len(msgs1)+1 {
		t.Errorf("turn 2: expected at least %d messages, got %d", len(msgs1)+1, len(msgs2))
	}
}

func TestBuildTurnMessages_EmptyPreviousMessages(t *testing.T) {
	engine := &Engine{
		turnRunner:   &TurnRunner{},
		knowledge:    builtin.NewKnowledge(),
		turnMessages: make(map[string][]Message),
	}

	agent := domain.Agent{
		ID:           "test-agent",
		SystemPrompt:  "You are a test assistant.",
		KnowledgeIDs: []string{},
	}

	msgs := engine.buildTurnMessages("session-1", agent, "hello", "")

	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != RoleSystem {
		t.Errorf("first message should be system, got %s", msgs[0].Role)
	}
	if msgs[len(msgs)-1].Role != RoleUser {
		t.Errorf("last message should be user, got %s", msgs[len(msgs)-1].Role)
	}
}

func TestBuildTurnMessages_CheckpointTextInSystemPrompt(t *testing.T) {
	engine := &Engine{
		turnRunner:   &TurnRunner{},
		knowledge:    builtin.NewKnowledge(),
		turnMessages: make(map[string][]Message),
	}

	agent := domain.Agent{
		ID:           "test-agent",
		SystemPrompt:  "You are a test assistant.",
		KnowledgeIDs: []string{},
	}

	checkpoint := "Previous summary: completed task A"
	msgs := engine.buildTurnMessages("session-1", agent, "continue", checkpoint)

	if len(msgs) == 0 {
		t.Fatal("expected at least 1 message")
	}
	if msgs[0].Role != RoleSystem {
		t.Fatal("first message should be system prompt")
	}
	if !contains(msgs[0].Content, checkpoint) {
		t.Errorf("system prompt should contain checkpoint text %q, got %q", checkpoint, msgs[0].Content)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildTurnMessages_MessageOrder(t *testing.T) {
	engine := &Engine{
		turnRunner:   &TurnRunner{},
		knowledge:    builtin.NewKnowledge(),
		turnMessages: make(map[string][]Message),
	}

	sessionID := "session-1"
	agent := domain.Agent{
		ID:           "test-agent",
		SystemPrompt:  "You are a test assistant.",
		KnowledgeIDs: []string{},
	}

	turn1Msgs := engine.buildTurnMessages(sessionID, agent, "TURN-1-GOAL", "")
	engine.mu.Lock()
	engine.turnMessages[sessionID] = append(engine.turnMessages[sessionID], turn1Msgs...)
	engine.mu.Unlock()

	turn2Msgs := engine.buildTurnMessages(sessionID, agent, "TURN-2-GOAL", "")

	lastUserIdx := -1
	for i, msg := range turn2Msgs {
		if msg.Role == RoleUser {
			lastUserIdx = i
		}
	}
	if lastUserIdx < 0 {
		t.Fatal("no user message found in turn 2")
	}
	if turn2Msgs[lastUserIdx].Content != "TURN-2-GOAL" {
		t.Errorf("last user message should be turn 2 goal, got %q", turn2Msgs[lastUserIdx].Content)
	}

	turn1UserIdx := -1
	for i, msg := range turn2Msgs {
		if msg.Role == RoleUser && msg.Content == "TURN-1-GOAL" {
			turn1UserIdx = i
		}
	}
	if turn1UserIdx >= 0 && turn1UserIdx > lastUserIdx {
		t.Error("turn 1's user message should appear BEFORE turn 2's user message")
	}
}
