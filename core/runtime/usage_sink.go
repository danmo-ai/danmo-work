package runtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// UsageSink accumulates llm.usage stream events into turn/session/project/model/agent rollups.
// Wired onto StreamEventManager — does not live inside TurnRunner.
type UsageSink struct {
	usage     port.UsageRepo
	sessions  port.SessionRepo
	mu        sync.Mutex
	projCache map[string]string // sessionID -> projectID
}

func NewUsageSink(usage port.UsageRepo, sessions port.SessionRepo) *UsageSink {
	return &UsageSink{
		usage:     usage,
		sessions:  sessions,
		projCache: make(map[string]string),
	}
}

func (s *UsageSink) OnEvent(ctx context.Context, ev domain.StreamEvent) {
	if s == nil || s.usage == nil || ev.Type != domain.EventLLMUsage {
		return
	}
	var p domain.LLMUsagePayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return
	}
	delta := p.Delta()
	if delta.Empty() {
		return
	}
	projectID := s.resolveProject(ctx, ev.SessionID)
	at := ev.CreatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if err := s.usage.AddDelta(ctx, ev.TurnID, ev.SessionID, projectID, delta, at); err != nil {
		log.Printf("[UsageSink] AddDelta session=%s turn=%s: %v", ev.SessionID, ev.TurnID, err)
	}
}

func (s *UsageSink) resolveProject(ctx context.Context, sessionID string) string {
	if sessionID == "" || s.sessions == nil {
		return ""
	}
	s.mu.Lock()
	if pid, ok := s.projCache[sessionID]; ok {
		s.mu.Unlock()
		return pid
	}
	s.mu.Unlock()

	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return ""
	}
	s.mu.Lock()
	s.projCache[sessionID] = sess.ProjectID
	s.mu.Unlock()
	return sess.ProjectID
}
