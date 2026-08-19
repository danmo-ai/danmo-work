package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ---- CheckpointRepo (control plane) ----

type checkpointRepo struct{ s *Store }

func (r *checkpointRepo) Save(ctx context.Context, cp domain.CompactionCheckpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	m := checkpointModel{SessionID: cp.SessionID, Data: string(data), UpdatedAt: time.Now().UTC()}
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "session_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
			}).
			Create(&m).Error
	})
}

func (r *checkpointRepo) Get(ctx context.Context, sessionID string) (*domain.CompactionCheckpoint, error) {
	var row checkpointModel
	err := r.s.db.WithContext(ctx).First(&row, "session_id = ?", sessionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cp domain.CompactionCheckpoint
	if err := json.Unmarshal([]byte(row.Data), &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func (r *checkpointRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&checkpointModel{}).Error
	})
}

// ---- FileChangeRepo (history plane) ----

type fileChangeRepo struct{ s *Store }

func (r *fileChangeRepo) Append(ctx context.Context, sessionID string, rec domain.FileChangeRecord) (int64, error) {
	err := r.s.withWrite(func(db *gorm.DB) error {
		seq := rec.Seq
		if seq == 0 {
			// SQLite has a single writer, so MAX+1 inside the write section is
			// race-free.
			var max int64
			if err := db.WithContext(ctx).Model(&fileChangeModel{}).
				Where("session_id = ?", sessionID).
				Select("COALESCE(MAX(seq), 0)").Scan(&max).Error; err != nil {
				return err
			}
			seq = max + 1
		}
		m := fileChangeModel{
			SessionID: sessionID, Seq: seq, TurnID: rec.TurnID, CallID: rec.CallID,
			Tool: rec.Tool, Path: rec.Path, Op: string(rec.Op), At: rec.At,
			Diff: rec.Diff, Bytes: rec.Bytes,
		}
		if err := db.WithContext(ctx).Create(&m).Error; err != nil {
			return err
		}
		rec.Seq = seq
		return nil
	})
	return rec.Seq, err
}

func (r *fileChangeRepo) ListAfter(ctx context.Context, sessionID string, afterSeq int64) ([]domain.FileChangeRecord, error) {
	var rows []fileChangeModel
	if err := r.s.db.WithContext(ctx).
		Where("session_id = ? AND seq > ?", sessionID, afterSeq).
		Order("seq asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.FileChangeRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, fileChangeToDomain(row))
	}
	return out, nil
}

func (r *fileChangeRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&fileChangeModel{}).Error
	})
}

var (
	_ port.CheckpointRepo = (*checkpointRepo)(nil)
	_ port.FileChangeRepo = (*fileChangeRepo)(nil)
)
