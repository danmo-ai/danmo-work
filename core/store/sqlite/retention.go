package sqlite

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// HistoryPruneStats summarizes one retention pass.
type HistoryPruneStats struct {
	OrphanSessions     int // sessions with history but no session row
	AgedSessions       int // stale sessions whose history was age-pruned
	DeletedEvents      int64
	DeletedEntries     int64
	DeletedFileChanges int64
}

// PruneHistory bounds history growth:
//
//  1. Orphan cleanup (always): history whose session row no longer exists —
//     turn rows, message entries, and stream events — is deleted. This also
//     finishes interrupted cascade deletes.
//  2. Age-based pruning (opt-in, maxAge > 0): sessions untouched for longer
//     than maxAge lose their message entries and stream events. Session and
//     turn metadata plus memories are kept, so the session list stays intact;
//     only timeline and LLM replay context are dropped.
//
// Freed pages are returned to the OS via incremental_vacuum on the history
// store (enabled at creation by NewHistory).
func (s *Store) PruneHistory(ctx context.Context, maxAge time.Duration) (HistoryPruneStats, error) {
	var stats HistoryPruneStats
	hist := s.historyOrSelf()

	existing := map[string]bool{}
	var sessionIDs []string
	if err := s.db.WithContext(ctx).Model(&sessionModel{}).Pluck("id", &sessionIDs).Error; err != nil {
		return stats, err
	}
	for _, id := range sessionIDs {
		existing[id] = true
	}

	// Candidate sessions referenced by history: turns and checkpoints
	// (control plane) plus stream events and file changes (history plane;
	// sessions can have events without turns).
	candidates := map[string]bool{}
	collect := func(db *gorm.DB, model any) error {
		var ids []string
		if err := db.WithContext(ctx).Model(model).Distinct("session_id").Pluck("session_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			candidates[id] = true
		}
		return nil
	}
	if err := collect(s.db, &turnModel{}); err != nil {
		return stats, err
	}
	if err := collect(s.db, &checkpointModel{}); err != nil {
		return stats, err
	}
	if err := collect(hist.db, &streamEventModel{}); err != nil {
		return stats, err
	}
	if err := collect(hist.db, &fileChangeModel{}); err != nil {
		return stats, err
	}

	for sessionID := range candidates {
		if sessionID == "" || existing[sessionID] {
			continue
		}
		ev, en, fc, err := s.deleteSessionHistoryRows(ctx, sessionID, true)
		if err != nil {
			return stats, err
		}
		stats.OrphanSessions++
		stats.DeletedEvents += ev
		stats.DeletedEntries += en
		stats.DeletedFileChanges += fc
	}

	if maxAge > 0 {
		cutoff := time.Now().UTC().Add(-maxAge)
		var stale []string
		if err := s.db.WithContext(ctx).Model(&sessionModel{}).
			Where("updated_at < ?", cutoff).Pluck("id", &stale).Error; err != nil {
			return stats, err
		}
		for _, sessionID := range stale {
			ev, en, fc, err := s.deleteSessionHistoryRows(ctx, sessionID, false)
			if err != nil {
				return stats, err
			}
			if ev > 0 || en > 0 || fc > 0 {
				stats.AgedSessions++
				stats.DeletedEvents += ev
				stats.DeletedEntries += en
				stats.DeletedFileChanges += fc
			}
		}
	}

	if stats.DeletedEvents > 0 || stats.DeletedEntries > 0 || stats.DeletedFileChanges > 0 {
		if err := hist.incrementalVacuum(); err != nil {
			log.Printf("[sqlite] history incremental_vacuum: %v", err)
		}
	}
	return stats, nil
}

// deleteSessionHistoryRows removes a session's stream events, turn message
// entries, and file-change journal. dropTurnRows additionally removes the
// turns metadata rows and the compaction checkpoint (orphan cleanup);
// age-based pruning keeps both so the UI turn list survives and the summary
// remains available if the session is ever resumed.
func (s *Store) deleteSessionHistoryRows(ctx context.Context, sessionID string, dropTurnRows bool) (events, entries, fileChanges int64, err error) {
	hist := s.historyOrSelf()

	var turnIDs []string
	if err := s.db.WithContext(ctx).Model(&turnModel{}).
		Where("session_id = ?", sessionID).Pluck("id", &turnIDs).Error; err != nil {
		return 0, 0, 0, err
	}

	if len(turnIDs) > 0 {
		if err := hist.withWrite(func(db *gorm.DB) error {
			res := db.WithContext(ctx).Where("turn_id IN ?", turnIDs).Delete(&turnLogEntryModel{})
			entries = res.RowsAffected
			return res.Error
		}); err != nil {
			return events, entries, fileChanges, err
		}
	}
	if err := hist.withWrite(func(db *gorm.DB) error {
		res := db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&streamEventModel{})
		events = res.RowsAffected
		return res.Error
	}); err != nil {
		return events, entries, fileChanges, err
	}
	if err := hist.withWrite(func(db *gorm.DB) error {
		res := db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&fileChangeModel{})
		fileChanges = res.RowsAffected
		return res.Error
	}); err != nil {
		return events, entries, fileChanges, err
	}
	if dropTurnRows {
		if err := s.withWrite(func(db *gorm.DB) error {
			if len(turnIDs) > 0 {
				if err := db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&turnModel{}).Error; err != nil {
					return err
				}
			}
			return db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&checkpointModel{}).Error
		}); err != nil {
			return events, entries, fileChanges, err
		}
	}
	return events, entries, fileChanges, nil
}

func (s *Store) incrementalVacuum() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.Exec(`PRAGMA incremental_vacuum`)
	return err
}
