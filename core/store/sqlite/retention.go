package sqlite

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// HistoryPruneStats summarizes one retention pass.
type HistoryPruneStats struct {
	OrphanSessions int // sessions with history but no session row
	AgedSessions   int // stale sessions whose history was age-pruned
	DeletedEvents  int64
	DeletedEntries int64
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

	// Candidate sessions referenced by history: turns (control plane) plus
	// stream events (history plane; sessions can have events without turns).
	candidates := map[string]bool{}
	var ids []string
	if err := s.db.WithContext(ctx).Model(&turnModel{}).Distinct("session_id").Pluck("session_id", &ids).Error; err != nil {
		return stats, err
	}
	for _, id := range ids {
		candidates[id] = true
	}
	ids = ids[:0]
	if err := hist.db.WithContext(ctx).Model(&streamEventModel{}).Distinct("session_id").Pluck("session_id", &ids).Error; err != nil {
		return stats, err
	}
	for _, id := range ids {
		candidates[id] = true
	}

	for sessionID := range candidates {
		if sessionID == "" || existing[sessionID] {
			continue
		}
		ev, en, err := s.deleteSessionHistoryRows(ctx, sessionID, true)
		if err != nil {
			return stats, err
		}
		stats.OrphanSessions++
		stats.DeletedEvents += ev
		stats.DeletedEntries += en
	}

	if maxAge > 0 {
		cutoff := time.Now().UTC().Add(-maxAge)
		var stale []string
		if err := s.db.WithContext(ctx).Model(&sessionModel{}).
			Where("updated_at < ?", cutoff).Pluck("id", &stale).Error; err != nil {
			return stats, err
		}
		for _, sessionID := range stale {
			ev, en, err := s.deleteSessionHistoryRows(ctx, sessionID, false)
			if err != nil {
				return stats, err
			}
			if ev > 0 || en > 0 {
				stats.AgedSessions++
				stats.DeletedEvents += ev
				stats.DeletedEntries += en
			}
		}
	}

	if stats.DeletedEvents > 0 || stats.DeletedEntries > 0 {
		if err := hist.incrementalVacuum(); err != nil {
			log.Printf("[sqlite] history incremental_vacuum: %v", err)
		}
	}
	return stats, nil
}

// deleteSessionHistoryRows removes a session's stream events and turn message
// entries. dropTurnRows additionally removes the turns metadata rows (orphan
// cleanup); age-based pruning keeps them so the UI turn list survives.
func (s *Store) deleteSessionHistoryRows(ctx context.Context, sessionID string, dropTurnRows bool) (events, entries int64, err error) {
	hist := s.historyOrSelf()

	var turnIDs []string
	if err := s.db.WithContext(ctx).Model(&turnModel{}).
		Where("session_id = ?", sessionID).Pluck("id", &turnIDs).Error; err != nil {
		return 0, 0, err
	}

	if len(turnIDs) > 0 {
		if err := hist.withWrite(func(db *gorm.DB) error {
			res := db.WithContext(ctx).Where("turn_id IN ?", turnIDs).Delete(&turnLogEntryModel{})
			entries = res.RowsAffected
			return res.Error
		}); err != nil {
			return events, entries, err
		}
	}
	if err := hist.withWrite(func(db *gorm.DB) error {
		res := db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&streamEventModel{})
		events = res.RowsAffected
		return res.Error
	}); err != nil {
		return events, entries, err
	}
	if dropTurnRows && len(turnIDs) > 0 {
		if err := s.withWrite(func(db *gorm.DB) error {
			return db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&turnModel{}).Error
		}); err != nil {
			return events, entries, err
		}
	}
	return events, entries, nil
}

func (s *Store) incrementalVacuum() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.Exec(`PRAGMA incremental_vacuum`)
	return err
}
