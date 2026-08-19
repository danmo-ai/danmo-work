package sqlite

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// turnLogRepo implements port.TurnLogRepo across the two planes:
//   - turn metadata (goal, agent, status, nested) lives in the turns table of
//     the control-plane store (meta) — the same rows served by TurnRepo, so
//     there is exactly one fact per turn;
//   - message entries live in turn_log_entries of the history-plane store
//     (hist; same store as meta in single-database mode).
//
// Together they are the single source of truth for LLM history; JSONL is
// rendered from here on demand.
type turnLogRepo struct {
	meta *Store
	hist *Store
}

func (r *turnLogRepo) UpsertTurnMeta(ctx context.Context, t domain.TurnLog) error {
	return r.meta.withWrite(func(db *gorm.DB) error {
		var existing turnModel
		err := db.WithContext(ctx).First(&existing, "id = ?", t.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			m := turnFromDomain(t)
			return db.WithContext(ctx).Create(&m).Error
		}
		if err != nil {
			return err
		}
		updates := map[string]any{}
		if existing.SessionID == "" && t.SessionID != "" {
			updates["session_id"] = t.SessionID
		}
		if existing.ProjectID == "" && t.ProjectID != "" {
			updates["project_id"] = t.ProjectID
		}
		if existing.AgentID == "" && t.AgentID != "" {
			updates["agent_id"] = t.AgentID
		}
		if existing.Goal == "" && t.Goal != "" {
			updates["goal"] = t.Goal
		}
		// Nested is only ever raised, never cleared: a nested tool-run stays a
		// debug artifact even if a later writer omits the flag.
		if !existing.Nested && t.Nested {
			updates["nested"] = true
		}
		if len(updates) == 0 {
			return nil
		}
		return db.WithContext(ctx).Model(&turnModel{}).Where("id = ?", t.ID).Updates(updates).Error
	})
}

func (r *turnLogRepo) GetTurnMeta(ctx context.Context, turnID string) (domain.TurnLog, bool, error) {
	var row turnModel
	err := r.meta.db.WithContext(ctx).First(&row, "id = ?", turnID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.TurnLog{}, false, nil
	}
	if err != nil {
		return domain.TurnLog{}, false, err
	}
	return turnToDomain(row), true, nil
}

func (r *turnLogRepo) EndTurn(ctx context.Context, turnID string, status domain.TurnStatus) error {
	// Never overwrite an existing terminal status: CancelTurn persists
	// "cancelled" immediately and the late-finishing goroutine must not
	// resurrect the turn as completed/failed.
	return r.meta.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&turnModel{}).
			Where("id = ? AND (status = ? OR status = '')", turnID, string(domain.TurnRunning)).
			Update("status", string(status)).Error
	})
}

func (r *turnLogRepo) ListSessionTurnIDs(ctx context.Context, sessionID string, includeNested bool) ([]string, error) {
	q := r.meta.db.WithContext(ctx).Model(&turnModel{}).Where("session_id = ?", sessionID)
	if !includeNested {
		q = q.Where("nested = ?", false)
	}
	var ids []string
	if err := q.Order("id asc").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *turnLogRepo) AppendEntry(ctx context.Context, e port.TurnLogEntryRecord) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	m := turnLogEntryModel{TurnID: e.TurnID, Seq: e.Seq, Type: e.Type, Data: string(data)}
	return r.hist.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(&m).Error
	})
}

func (r *turnLogRepo) ListEntries(ctx context.Context, turnID string) ([]port.TurnLogEntryRecord, error) {
	var rows []turnLogEntryModel
	if err := r.hist.db.WithContext(ctx).Where("turn_id = ?", turnID).Order("seq asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]port.TurnLogEntryRecord, 0, len(rows))
	for _, row := range rows {
		var data map[string]any
		if row.Data != "" && row.Data != "null" {
			_ = json.Unmarshal([]byte(row.Data), &data)
		}
		out = append(out, port.TurnLogEntryRecord{TurnID: row.TurnID, Seq: row.Seq, Type: row.Type, Data: data})
	}
	return out, nil
}

func (r *turnLogRepo) MaxSeq(ctx context.Context, turnID string) (int, error) {
	var max int
	err := r.hist.db.WithContext(ctx).Model(&turnLogEntryModel{}).
		Where("turn_id = ?", turnID).
		Select("COALESCE(MAX(seq), 0)").Scan(&max).Error
	return max, err
}

// DeleteSessionHistory removes the session's turn rows (control plane) and
// their message entries (history plane). Best-effort ordering: entries first
// so a crash between the two deletes leaves discoverable turn rows whose
// entries are already gone — the startup orphan pruner finishes the job.
func (r *turnLogRepo) DeleteSessionHistory(ctx context.Context, sessionID string) error {
	ids, err := r.ListSessionTurnIDs(ctx, sessionID, true)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := r.hist.withWrite(func(db *gorm.DB) error {
			return db.WithContext(ctx).Where("turn_id IN ?", ids).Delete(&turnLogEntryModel{}).Error
		}); err != nil {
			return err
		}
	}
	return r.meta.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&turnModel{}).Error
	})
}
