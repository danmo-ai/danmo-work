package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

var _ port.EventStream = (*StreamEventManager)(nil)

type StreamEventManager struct {
	mu    sync.RWMutex
	seq   atomic.Int64
	subs  map[string][]chan domain.StreamEvent
	store port.StreamEventRepo
}

func NewStreamEventManager(repo port.StreamEventRepo) *StreamEventManager {
	m := &StreamEventManager{
		subs:  make(map[string][]chan domain.StreamEvent),
		store: repo,
	}
	if repo != nil {
		m.seq.Store(repo.MaxSeq())
	}
	return m
}

func (m *StreamEventManager) Publish(ctx context.Context, sessionID, turnID, typ string, payload any) domain.StreamEvent {
	raw, _ := json.Marshal(payload)
	seq := m.seq.Add(1)
	ev := domain.StreamEvent{
		Seq: seq, Type: typ, SessionID: sessionID, TurnID: turnID,
		Payload: raw, CreatedAt: time.Now().UTC(),
	}

	if m.store != nil {
		// Persist even when the turn ctx is already cancelled — otherwise
		// tool.error / similar events can vanish from history (seq gap, stuck "running").
		saveCtx := ctx
		if ctx == nil || ctx.Err() != nil {
			saveCtx = context.Background()
		}
		_ = m.store.Save(saveCtx, ev)
	}

	m.mu.RLock()
	for _, ch := range m.subs[sessionID] {
		select {
		case ch <- ev:
		default:
			// Slow SSE consumer: drop rather than block the agent loop.
			// Clients heal via since_seq backfill + events/poll gap fill.
		}
	}
	m.mu.RUnlock()

	return ev
}

func (m *StreamEventManager) Subscribe(sessionID string) chan domain.StreamEvent {
	// Larger buffer reduces drops during SSE backfill / UI stalls; still non-blocking.
	ch := make(chan domain.StreamEvent, 256)
	m.mu.Lock()
	m.subs[sessionID] = append(m.subs[sessionID], ch)
	m.mu.Unlock()
	return ch
}

func (m *StreamEventManager) Unsubscribe(sessionID string, ch chan domain.StreamEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subs := m.subs[sessionID]
	out := subs[:0]
	for _, c := range subs {
		if c != ch {
			out = append(out, c)
		}
	}
	m.subs[sessionID] = out
	close(ch)
}

func (m *StreamEventManager) ListSince(sessionID string, since int64) []domain.StreamEvent {
	events, err := m.store.ListBySession(context.Background(), sessionID, since)
	if err != nil {
		return nil
	}
	return events
}
