package turnlog

import (
	"context"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

const maxFileChangeDiffBytes = 4096

// FileChangeStore records session file-tool mutations in the history-plane
// database (file_changes table). Legacy file_changes.jsonl files are imported
// once at bootstrap and are no longer authoritative.
type FileChangeStore struct {
	repo port.FileChangeRepo
}

func NewFileChangeStore(repo port.FileChangeRepo) *FileChangeStore {
	return &FileChangeStore{repo: repo}
}

// Append assigns a monotonic per-session Seq, truncates Diff, and persists
// the record. projectID is unused (kept for the FileChangeAppender contract;
// location no longer depends on the project).
func (s *FileChangeStore) Append(sessionID, projectID string, rec domain.FileChangeRecord) (int64, error) {
	_ = projectID
	if rec.At == "" {
		rec.At = time.Now().UTC().Format(time.RFC3339)
	}
	if len(rec.Diff) > maxFileChangeDiffBytes {
		rec.Diff = rec.Diff[:maxFileChangeDiffBytes] + "\n...[diff truncated]"
	}
	rec.Seq = 0 // repo assigns the next per-session seq
	return s.repo.Append(context.Background(), sessionID, rec)
}

// LoadAfter returns journal records with Seq > afterSeq (ascending).
func (s *FileChangeStore) LoadAfter(sessionID string, afterSeq int64) ([]domain.FileChangeRecord, error) {
	return s.repo.ListAfter(context.Background(), sessionID, afterSeq)
}
