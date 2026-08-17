package runtime

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// BackfillUsageFromStreamEvents SUMs historical llm.usage events into rollups
// for sessions that do not yet have a session-grain row.
func BackfillUsageFromStreamEvents(ctx context.Context, usage port.UsageRepo, sessions port.SessionRepo, events port.StreamEventRepo) {
	if usage == nil || sessions == nil || events == nil {
		return
	}
	list, err := sessions.List(ctx)
	if err != nil {
		log.Printf("[UsageBackfill] list sessions: %v", err)
		return
	}
	for _, sess := range list {
		has, err := usage.HasGrain(ctx, domain.UsageGrainSession, sess.ID)
		if err != nil || has {
			continue
		}
		evs, err := events.ListBySession(ctx, sess.ID, 0)
		if err != nil {
			continue
		}
		type turnAgg struct {
			delta domain.UsageDelta
			at    time.Time
		}
		byTurn := map[string]*turnAgg{}
		var lastAt time.Time
		for _, ev := range evs {
			if ev.Type != domain.EventLLMUsage {
				continue
			}
			var p domain.LLMUsagePayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				continue
			}
			d := p.Delta()
			if d.Empty() {
				continue
			}
			at := ev.CreatedAt
			if at.IsZero() {
				at = time.Now().UTC()
			}
			if at.After(lastAt) {
				lastAt = at
			}
			turnID := ev.TurnID
			if turnID == "" {
				turnID = "_unknown"
			}
			agg := byTurn[turnID]
			if agg == nil {
				agg = &turnAgg{}
				byTurn[turnID] = agg
			}
			// Per-call model/agent: apply each event as its own AddDelta so model/agent grains split correctly.
			_ = usage.AddDelta(ctx, turnID, sess.ID, sess.ProjectID, d, at)
			agg.delta.PromptTokens += d.PromptTokens
			agg.delta.CompletionTokens += d.CompletionTokens
			agg.delta.TotalTokens += d.TotalTokens
			agg.at = at
		}
		if len(byTurn) == 0 {
			continue
		}
		log.Printf("[UsageBackfill] session %s: applied %d turn(s) from stream events (last=%s)", sess.ID, len(byTurn), lastAt.Format(time.RFC3339))
	}
}
