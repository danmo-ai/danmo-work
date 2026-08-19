package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type SessionManager struct {
	store  port.Repository
	engine port.Engine
	llm    port.LLMProvider
	mu     sync.Mutex
}

func NewSessionManager(store port.Repository, engine port.Engine, llm port.LLMProvider) *SessionManager {
	return &SessionManager{store: store, engine: engine, llm: llm}
}

func (m *SessionManager) SetEngine(engine port.Engine) {
	m.engine = engine
}

func (m *SessionManager) Create(ctx context.Context, req domain.CreateSessionRequest) (domain.Session, error) {
	atts, err := domain.NormalizeUserAttachments(req.Attachments)
	if err != nil {
		return domain.Session{}, err
	}
	if strings.TrimSpace(req.Content) == "" && len(atts) == 0 {
		return domain.Session{}, fmt.Errorf("content or attachments required")
	}
	if req.AgentID == "" {
		return domain.Session{}, fmt.Errorf("agentId required")
	}
	content := req.Content
	if strings.TrimSpace(content) == "" && len(atts) > 0 {
		content = "[Image attachment]"
	}
	now := time.Now().UTC()
	s := domain.Session{
		ID:        NewID("session"),
		Title:     strings.TrimSpace(req.Title),
		ProjectID: req.ProjectID,
		AgentID:   req.AgentID,
		ModelID:   req.ModelID,
		PlanMode:  req.PlanMode,
		Content:   content,
		Status:    domain.SessionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := m.store.Sessions().Create(ctx, s); err != nil {
		return domain.Session{}, fmt.Errorf("创建会话失败: %w", err)
	}
	m.engine.StartSession(ctx, s, atts)
	if !req.SkipAutoTitle && s.Title == "" {
		go m.generateTitle(s.ID, s.Content, s.ModelID)
	}
	return s, nil
}

func (m *SessionManager) generateTitle(sessionID, content, modelID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := m.llm.Chat(ctx, port.LLMChatRequest{
		Model: modelID,
		Messages: []port.ChatMessage{
			{Role: "system", Content: "You are a session title generator. Generate a concise title (max 8 words) for the given session description. Reply with ONLY the title, no extra text."},
			{Role: "user", Content: content},
		},
	})
	if err != nil {
		log.Printf("generate title for session %s: %v", sessionID, err)
		return
	}
	title := strings.TrimSpace(resp.Content)
	if title == "" {
		return
	}
	s, err := m.store.Sessions().Get(ctx, sessionID)
	if err != nil {
		log.Printf("generate title: get session %s: %v", sessionID, err)
		return
	}
	s.Title = title
	if err := m.store.Sessions().Update(ctx, s); err != nil {
		log.Printf("generate title: update session %s: %v", sessionID, err)
	}
}

func (m *SessionManager) StartTurn(ctx context.Context, sessionID string, req domain.SendMessageRequest) (string, error) {
	atts, err := domain.NormalizeUserAttachments(req.Attachments)
	if err != nil {
		return "", err
	}
	userInput := req.UserInput
	if strings.TrimSpace(userInput) == "" && len(atts) > 0 {
		userInput = "[Image attachment]"
	}
	if strings.TrimSpace(userInput) == "" {
		return "", fmt.Errorf("userInput or attachments required")
	}
	if len(req.SnapshotPaths) > 0 {
		ctx = WithSnapshotPaths(ctx, req.SnapshotPaths)
	}
	return m.engine.StartTurn(ctx, sessionID, userInput, req.AgentID, req.ModelID, atts)
}

func (m *SessionManager) CancelTurn(ctx context.Context, turnID string) {
	m.engine.CancelTurn(ctx, turnID)
}

func (m *SessionManager) ResumeTurn(ctx context.Context, sessionID, turnID string) error {
	return m.engine.ResumeTurn(ctx, sessionID, turnID)
}

func (m *SessionManager) ListTurns(sessionID string) []domain.TurnLog {
	if m == nil || m.engine == nil {
		return nil
	}
	return m.engine.ListTurns(sessionID)
}

// ActiveTurnID returns the in-flight turn for a session, if any.
func (m *SessionManager) ActiveTurnID(sessionID string) string {
	if m == nil || m.engine == nil {
		return ""
	}
	return m.engine.ActiveTurnID(sessionID)
}

func (m *SessionManager) Get(ctx context.Context, id string) (domain.Session, error) {
	return m.store.Sessions().Get(ctx, id)
}

func (m *SessionManager) List(ctx context.Context) ([]domain.Session, error) {
	return m.store.Sessions().List(ctx)
}

func (m *SessionManager) Update(ctx context.Context, id string, req domain.UpdateSessionRequest) (domain.Session, error) {
	s, err := m.store.Sessions().Get(ctx, id)
	if err != nil {
		return domain.Session{}, err
	}
	if req.Title != nil {
		s.Title = *req.Title
	}
	if req.ProjectID != nil {
		s.ProjectID = *req.ProjectID
	}
	if req.Status != nil {
		s.Status = *req.Status
	}
	if req.ModelID != nil {
		s.ModelID = *req.ModelID
	}
	if req.AgentID != nil {
		s.AgentID = *req.AgentID
	}
	if req.PlanMode != nil {
		s.PlanMode = *req.PlanMode
	}
	s.UpdatedAt = time.Now().UTC()
	if err := m.store.Sessions().Update(ctx, s); err != nil {
		return domain.Session{}, err
	}
	return s, nil
}

func (m *SessionManager) UpdateSession(ctx context.Context, s domain.Session) error {
	return m.store.Sessions().Update(ctx, s)
}

func (m *SessionManager) Delete(ctx context.Context, id string) error {
	if m.engine != nil {
		m.engine.RevokeSessionNetworkGrants(id)
	}
	_ = m.store.PendingMessages().DeleteBySession(ctx, id)
	// Cascade the session's history (turn rows, message entries, event
	// timeline). Best-effort: the session row is deleted first, so any
	// remainder becomes an orphan that startup retention finishes off.
	if err := m.store.Sessions().Delete(ctx, id); err != nil {
		return err
	}
	if err := m.store.TurnLogs().DeleteSessionHistory(ctx, id); err != nil {
		log.Printf("[sessions] delete %s: turn history cascade: %v", id, err)
	}
	if err := m.store.StreamEvents().DeleteBySession(ctx, id); err != nil {
		log.Printf("[sessions] delete %s: stream events cascade: %v", id, err)
	}
	return nil
}

func (m *SessionManager) StreamEvents(sessionID string, since int64) []domain.StreamEvent {
	return m.engine.StreamEvents(sessionID, since)
}

func (m *SessionManager) DecideApproval(ctx context.Context, id string, approved bool, scope string) error {
	a, err := m.store.Approvals().Get(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != "" && a.Status != "pending" {
		// Already decided — ignore repeat clicks from stale UI after reload.
		return nil
	}
	if scope == "" {
		scope = "once"
	}
	if approved {
		a.Status = "approved"
	} else {
		a.Status = "rejected"
	}
	if err := m.store.Approvals().Update(ctx, a); err != nil {
		return err
	}
	m.engine.ResolveApproval(id, approved, scope)
	m.engine.PublishPermissionDecided(a.SessionID, a.TurnID, id, approved, scope)
	return nil
}

func (m *SessionManager) Subscribe(sessionID string) chan domain.StreamEvent {
	return m.engine.Subscribe(sessionID)
}

func (m *SessionManager) Unsubscribe(sessionID string, ch chan domain.StreamEvent) {
	m.engine.Unsubscribe(sessionID, ch)
}

func (m *SessionManager) ResolveAskUser(askID, answer string) error {
	return m.engine.ResolveAskUser(askID, answer)
}

func (m *SessionManager) ListPending(ctx context.Context, sessionID string) ([]domain.PendingMessage, error) {
	return m.store.PendingMessages().ListBySession(ctx, sessionID)
}

func (m *SessionManager) EnqueuePending(ctx context.Context, sessionID string, req domain.EnqueuePendingRequest) (domain.PendingMessage, error) {
	atts, err := domain.NormalizeUserAttachments(req.Attachments)
	if err != nil {
		return domain.PendingMessage{}, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" && len(atts) == 0 {
		return domain.PendingMessage{}, fmt.Errorf("content or attachments required")
	}
	if content == "" && len(atts) > 0 {
		content = "[Image attachment]"
	}
	if _, err := m.store.Sessions().Get(ctx, sessionID); err != nil {
		return domain.PendingMessage{}, err
	}
	maxPos, err := m.store.PendingMessages().MaxPosition(ctx, sessionID)
	if err != nil {
		return domain.PendingMessage{}, err
	}
	now := time.Now().UTC()
	msg := domain.PendingMessage{
		ID:          NewID("pending"),
		SessionID:   sessionID,
		Content:     content,
		Attachments: atts,
		Position:    maxPos + 1,
		Status:      domain.PendingQueued,
		AgentID:     req.AgentID,
		ModelID:     req.ModelID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := m.store.PendingMessages().Create(ctx, msg); err != nil {
		return domain.PendingMessage{}, err
	}
	// If the session is already idle, start immediately; if busy, StartTurn
	// fails closed and the message stays queued for finishSessionTurn drain.
	go m.DrainPendingQueue(context.Background(), sessionID)
	return msg, nil
}

func (m *SessionManager) UpdatePending(ctx context.Context, sessionID, id string, req domain.UpdatePendingRequest) (domain.PendingMessage, error) {
	msg, err := m.store.PendingMessages().Get(ctx, id)
	if err != nil {
		return domain.PendingMessage{}, err
	}
	if msg.SessionID != sessionID {
		return domain.PendingMessage{}, fmt.Errorf("pending message not found")
	}
	if msg.Status != domain.PendingQueued {
		return domain.PendingMessage{}, fmt.Errorf("pending message is not editable")
	}
	if req.Content != nil {
		msg.Content = strings.TrimSpace(*req.Content)
	}
	if req.Attachments != nil {
		atts, err := domain.NormalizeUserAttachments(*req.Attachments)
		if err != nil {
			return domain.PendingMessage{}, err
		}
		msg.Attachments = atts
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.Attachments) == 0 {
		return domain.PendingMessage{}, fmt.Errorf("content or attachments required")
	}
	msg.UpdatedAt = time.Now().UTC()
	if err := m.store.PendingMessages().Update(ctx, msg); err != nil {
		return domain.PendingMessage{}, err
	}
	return msg, nil
}

func (m *SessionManager) DeletePending(ctx context.Context, sessionID, id string) error {
	msg, err := m.store.PendingMessages().Get(ctx, id)
	if err != nil {
		return err
	}
	if msg.SessionID != sessionID {
		return fmt.Errorf("pending message not found")
	}
	return m.store.PendingMessages().Delete(ctx, id)
}

func (m *SessionManager) ClearPending(ctx context.Context, sessionID string) error {
	return m.store.PendingMessages().DeleteBySession(ctx, sessionID)
}

func (m *SessionManager) ReorderPending(ctx context.Context, sessionID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return m.store.PendingMessages().Reorder(ctx, sessionID, ids)
}

// SteerPending marks a queued message for soft-steer into the active turn.
// The turn runner scans durable steering rows after each tool batch (before
// the next LLM call). If the session is idle, the message is sent as a new turn.
func (m *SessionManager) SteerPending(ctx context.Context, sessionID, id string) error {
	msg, err := m.store.PendingMessages().Get(ctx, id)
	if err != nil {
		return err
	}
	if msg.SessionID != sessionID {
		return fmt.Errorf("pending message not found")
	}
	if msg.Status != domain.PendingQueued && msg.Status != domain.PendingSteering {
		return fmt.Errorf("pending message is not steerable")
	}

	list, err := m.store.PendingMessages().ListBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(list))
	ids = append(ids, id)
	for _, item := range list {
		if item.ID == id {
			continue
		}
		ids = append(ids, item.ID)
	}
	if err := m.store.PendingMessages().Reorder(ctx, sessionID, ids); err != nil {
		return err
	}

	active := ""
	if m.engine != nil {
		active = m.engine.ActiveTurnID(sessionID)
	}
	if active == "" {
		// Idle: keep as queued and start a normal turn. Drain synchronously so
		// the steer HTTP response's pending list no longer includes this item.
		if msg.Status == domain.PendingSteering {
			msg.Status = domain.PendingQueued
			msg.UpdatedAt = time.Now().UTC()
			if err := m.store.PendingMessages().Update(ctx, msg); err != nil {
				return err
			}
		}
		m.DrainPendingQueue(ctx, sessionID)
		return nil
	}

	msg.Status = domain.PendingSteering
	msg.UpdatedAt = time.Now().UTC()
	return m.store.PendingMessages().Update(ctx, msg)
}

// ClaimSteering returns and deletes durable soft-steer messages for a session.
func (m *SessionManager) ClaimSteering(ctx context.Context, sessionID string) ([]domain.PendingMessage, error) {
	return m.store.PendingMessages().ClaimSteering(ctx, sessionID)
}

// DemoteSteering moves unclaimed soft-steer messages back to the next-turn queue.
func (m *SessionManager) DemoteSteering(ctx context.Context, sessionID string) error {
	return m.store.PendingMessages().DemoteSteering(ctx, sessionID)
}

// DrainPendingQueue starts the next queued turn if the session is idle.
func (m *SessionManager) DrainPendingQueue(ctx context.Context, sessionID string) {
	if m == nil || m.engine == nil || m.store == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok, err := m.store.PendingMessages().PopFront(ctx, sessionID)
	if err != nil {
		log.Printf("[pending] pop session %s: %v", sessionID, err)
		return
	}
	if !ok {
		return
	}
	turnID, err := m.engine.StartTurn(ctx, sessionID, msg.Content, msg.AgentID, msg.ModelID, msg.Attachments)
	if err != nil {
		log.Printf("[pending] start turn session %s: %v", sessionID, err)
		msg.Status = domain.PendingQueued
		msg.UpdatedAt = time.Now().UTC()
		if uerr := m.store.PendingMessages().Update(ctx, msg); uerr != nil {
			log.Printf("[pending] restore queued %s: %v", msg.ID, uerr)
		}
		return
	}
	if err := m.store.PendingMessages().Delete(ctx, msg.ID); err != nil {
		log.Printf("[pending] delete sent %s (turn %s): %v", msg.ID, turnID, err)
	}
}
