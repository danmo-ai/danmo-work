package sqlite

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const historySplitMarker = "history_split_v1"

// MigrateHistoryTables performs the one-time move of the two bulk tables
// (stream_events, turn_log_entries) from work.db into the dedicated
// history.db. Batched so tens-of-GB installs migrate with bounded memory;
// idempotent via an app_meta marker in work.db — a partial run resumes at the
// next startup (copied rows are deleted from work.db per batch, so restarting
// re-copies only what is left).
//
// Freed work.db pages go to its freelist; the file shrinks only on VACUUM,
// which is intentionally not run here (it can take minutes and double disk
// usage on large installs). New data no longer grows work.db.
func MigrateHistoryTables(ctx context.Context, work, hist *Store) error {
	if work == hist || hist == nil {
		return nil
	}
	if _, ok, err := work.AppMeta().Get(ctx, historySplitMarker); err != nil {
		return err
	} else if ok {
		return nil
	}

	moved := 0
	n, err := moveHistoryRows(ctx, work, hist, "stream_events", func() any { return &[]streamEventModel{} })
	if err != nil {
		return fmt.Errorf("move stream_events: %w", err)
	}
	moved += n
	n, err = moveHistoryRows(ctx, work, hist, "turn_log_entries", func() any { return &[]turnLogEntryModel{} })
	if err != nil {
		return fmt.Errorf("move turn_log_entries: %w", err)
	}
	moved += n

	if moved > 0 {
		log.Printf("[sqlite] history split: moved %d rows from %s to %s", moved, work.dbPath, hist.dbPath)
	}
	return work.AppMeta().Set(ctx, historySplitMarker, "done")
}

// moveHistoryRows copies table rows work→hist in batches and deletes each
// copied batch from work. Rows keep their primary keys, so a batch that was
// inserted but not yet deleted when the process died is skipped on retry via
// ON CONFLICT DO NOTHING.
func moveHistoryRows(ctx context.Context, work, hist *Store, table string, newBatch func() any) (int, error) {
	if !work.db.Migrator().HasTable(table) {
		return 0, nil
	}
	const batchSize = 2000
	total := 0
	for {
		batch := newBatch()
		if err := work.db.WithContext(ctx).Table(table).Order("id asc").Limit(batchSize).Find(batch).Error; err != nil {
			return total, err
		}
		ids, count := collectIDs(batch)
		if count == 0 {
			return total, nil
		}
		if err := hist.withWrite(func(db *gorm.DB) error {
			// Ignore PK conflicts: a previous run may have inserted this batch
			// and crashed before deleting it from work.db.
			return db.WithContext(ctx).Table(table).
				Clauses(clause.OnConflict{DoNothing: true}).
				Create(batch).Error
		}); err != nil {
			return total, err
		}
		if err := work.withWrite(func(db *gorm.DB) error {
			return db.WithContext(ctx).Exec("DELETE FROM "+table+" WHERE id IN ?", ids).Error
		}); err != nil {
			return total, err
		}
		total += count
		if count < batchSize {
			return total, nil
		}
	}
}

func collectIDs(batch any) ([]any, int) {
	switch rows := batch.(type) {
	case *[]streamEventModel:
		ids := make([]any, 0, len(*rows))
		for _, r := range *rows {
			ids = append(ids, r.ID)
		}
		return ids, len(*rows)
	case *[]turnLogEntryModel:
		ids := make([]any, 0, len(*rows))
		for _, r := range *rows {
			ids = append(ids, r.ID)
		}
		return ids, len(*rows)
	default:
		return nil, 0
	}
}
