package turnlog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ImportLegacyArtifacts performs the one-time migration of on-disk session
// artifacts into the database:
//
//   - checkpoint_*.json → compaction_checkpoints (only the newest per session,
//     selected by TurnCount with TurnID as tiebreaker — same rule the old
//     file store used);
//   - file_changes.jsonl → file_changes (original Seq values preserved, so
//     checkpoint FileChangeLogSeq cursors stay valid).
//
// Files are left in place as inert backups. Idempotent: sessions that already
// have a checkpoint row / file-change rows are skipped, so a partially failed
// import can safely be retried on the next startup.
func ImportLegacyArtifacts(ctx context.Context, checkpoints port.CheckpointRepo, fileChanges port.FileChangeRepo, projector func(projectID string) string) (imported int, err error) {
	root := projector("")
	projectEntries, readErr := os.ReadDir(root)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return 0, nil
		}
		return 0, readErr
	}

	var firstErr error
	record := func(what, sessionID string, err error) {
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("import %s for session %s: %w", what, sessionID, err)
		}
	}
	for _, pe := range projectEntries {
		if !pe.IsDir() {
			continue
		}
		sessionsRoot := filepath.Join(projector(pe.Name()), "sessions")
		sessionEntries, err := os.ReadDir(sessionsRoot)
		if err != nil {
			continue
		}
		for _, se := range sessionEntries {
			if !se.IsDir() {
				continue
			}
			sessionID := se.Name()
			sessionDir := filepath.Join(sessionsRoot, sessionID)

			ok, err := importSessionCheckpoint(ctx, checkpoints, sessionDir, sessionID)
			record("checkpoint", sessionID, err)
			if ok {
				imported++
			}
			ok, err = importSessionFileChanges(ctx, fileChanges, sessionDir, sessionID)
			record("file changes", sessionID, err)
			if ok {
				imported++
			}
		}
	}
	return imported, firstErr
}

// importSessionCheckpoint picks the newest legacy checkpoint file and inserts
// it unless the session already has a checkpoint row.
func importSessionCheckpoint(ctx context.Context, repo port.CheckpointRepo, sessionDir, sessionID string) (bool, error) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return false, nil
	}
	var latest *domain.CompactionCheckpoint
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "checkpoint_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionDir, name))
		if err != nil {
			continue
		}
		var cp domain.CompactionCheckpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		if cp.SessionID != "" && cp.SessionID != sessionID {
			continue
		}
		if latest == nil ||
			cp.TurnCount > latest.TurnCount ||
			(cp.TurnCount == latest.TurnCount && cp.TurnID > latest.TurnID) {
			latest = &cp
		}
	}
	if latest == nil {
		return false, nil
	}
	existing, err := repo.Get(ctx, sessionID)
	if err != nil || existing != nil {
		return false, err
	}
	latest.SessionID = sessionID
	return true, repo.Save(ctx, *latest)
}

// importSessionFileChanges replays a legacy file_changes.jsonl into the DB
// with original Seq values, unless the session already has rows.
func importSessionFileChanges(ctx context.Context, repo port.FileChangeRepo, sessionDir, sessionID string) (bool, error) {
	f, err := os.Open(filepath.Join(sessionDir, "file_changes.jsonl"))
	if err != nil {
		return false, nil
	}
	defer f.Close()

	existing, err := repo.ListAfter(ctx, sessionID, 0)
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec domain.FileChangeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Seq == 0 {
			// Defensive: legacy seqs start at 1; a zero would trigger
			// auto-assignment and could collide with a preserved seq.
			continue
		}
		if _, err := repo.Append(ctx, sessionID, rec); err != nil {
			return n > 0, err
		}
		n++
	}
	return n > 0, sc.Err()
}
