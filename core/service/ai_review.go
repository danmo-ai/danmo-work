package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/store/turnlog"
	"danmo-work/core/textdiff"
)

// AIReviewManager handles pre-turn snapshots and post-turn AI Diff / revert.
type AIReviewManager struct {
	Projects  *ProjectManager
	Snapshots *turnlog.SnapshotStore
	Changes   *turnlog.FileChangeStore
}

func NewAIReviewManager(pm *ProjectManager, snaps *turnlog.SnapshotStore, changes *turnlog.FileChangeStore) *AIReviewManager {
	return &AIReviewManager{Projects: pm, Snapshots: snaps, Changes: changes}
}

var officeEditPathRe = regexp.MustCompile(`(?m)^path:\s*(.+)$`)

// PathsFromOfficeEdit extracts project-relative paths from an [office-edit] prompt.
func PathsFromOfficeEdit(userInput string) []string {
	if !strings.Contains(userInput, "[office-edit]") {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range officeEditPathRe.FindAllStringSubmatch(userInput, -1) {
		p := filepath.ToSlash(strings.TrimSpace(m[1]))
		p = strings.Trim(p, `"'`)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// SnapshotPaths reads each path and stores a pre-turn snapshot.
func (m *AIReviewManager) SnapshotPaths(ctx context.Context, projectID, sessionID, turnID string, paths []string) ([]turnlog.SnapshotMeta, error) {
	if m == nil || m.Snapshots == nil {
		return nil, fmt.Errorf("ai review not configured")
	}
	var out []turnlog.SnapshotMeta
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/"))
		if p == "" {
			continue
		}
		content, err := m.readProjectBytes(ctx, projectID, p)
		if err != nil {
			if os.IsNotExist(err) {
				// New file: empty snapshot so revert deletes / restores empty.
				meta, err := m.Snapshots.Save(projectID, sessionID, turnID, p, []byte{})
				if err != nil {
					return out, err
				}
				out = append(out, *meta)
				continue
			}
			return out, err
		}
		meta, err := m.Snapshots.Save(projectID, sessionID, turnID, p, content)
		if err != nil {
			return out, err
		}
		out = append(out, *meta)
	}
	return out, nil
}

func (m *AIReviewManager) readProjectBytes(ctx context.Context, projectID, relPath string) ([]byte, error) {
	fc, err := m.Projects.ReadFileContent(ctx, projectID, relPath)
	if err != nil {
		return nil, err
	}
	if fc.Binary {
		return nil, fmt.Errorf("binary file not supported for AI review snapshot")
	}
	return []byte(fc.Content), nil
}

// ReviewStatus compares snapshot to current disk content.
type ReviewStatus struct {
	TurnID       string `json:"turnId"`
	Path         string `json:"path"`
	Changed      bool   `json:"changed"`
	BaseHash     string `json:"baseHash"`
	CurrentHash  string `json:"currentHash,omitempty"`
	HasSnapshot  bool   `json:"hasSnapshot"`
	CanRevert    bool   `json:"canRevert"`
	HashOnly     bool   `json:"hashOnly,omitempty"`
	MissingFile  bool   `json:"missingFile,omitempty"`
}

func (m *AIReviewManager) Status(ctx context.Context, projectID, sessionID, turnID, relPath string) (*ReviewStatus, error) {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	meta, err := m.Snapshots.GetMeta(projectID, sessionID, turnID, relPath)
	if err != nil {
		if errors.Is(err, turnlog.ErrSnapshotNotFound) {
			return &ReviewStatus{TurnID: turnID, Path: relPath, HasSnapshot: false}, nil
		}
		return nil, err
	}
	st := &ReviewStatus{
		TurnID:      turnID,
		Path:        relPath,
		HasSnapshot: true,
		BaseHash:    meta.Hash,
		CanRevert:   meta.HasContent,
		HashOnly:    !meta.HasContent,
	}
	cur, err := m.readProjectBytes(ctx, projectID, relPath)
	if err != nil {
		if os.IsNotExist(err) {
			st.MissingFile = true
			st.Changed = meta.Bytes > 0 || meta.Hash != emptyHash()
			return st, nil
		}
		return nil, err
	}
	sum := sha256.Sum256(cur)
	st.CurrentHash = hex.EncodeToString(sum[:])
	st.Changed = st.CurrentHash != meta.Hash
	return st, nil
}

func emptyHash() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}

// DiffResult is snapshot vs current unified diff.
type DiffResult struct {
	Path      string `json:"path"`
	TurnID    string `json:"turnId"`
	Patch     string `json:"patch"`
	Changed   bool   `json:"changed"`
	CanRevert bool   `json:"canRevert"`
	HashOnly  bool   `json:"hashOnly,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (m *AIReviewManager) Diff(ctx context.Context, projectID, sessionID, turnID, relPath string) (*DiffResult, error) {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	old, meta, err := m.Snapshots.ReadContent(projectID, sessionID, turnID, relPath)
	if err != nil {
		if errors.Is(err, turnlog.ErrSnapshotNotFound) {
			return &DiffResult{Path: relPath, TurnID: turnID, Error: "snapshot_not_found"}, nil
		}
		if errors.Is(err, turnlog.ErrSnapshotNoContent) {
			st, stErr := m.Status(ctx, projectID, sessionID, turnID, relPath)
			if stErr != nil {
				return nil, stErr
			}
			return &DiffResult{
				Path: relPath, TurnID: turnID, Changed: st.Changed,
				HashOnly: true, CanRevert: false, Error: "hash_only",
			}, nil
		}
		return nil, err
	}
	cur, err := m.readProjectBytes(ctx, projectID, relPath)
	if err != nil {
		if os.IsNotExist(err) {
			cur = nil
		} else {
			return nil, err
		}
	}
	patch := textdiff.Unified(relPath, string(old), string(cur))
	return &DiffResult{
		Path:      relPath,
		TurnID:    turnID,
		Patch:     patch,
		Changed:   string(old) != string(cur),
		CanRevert: meta.HasContent,
	}, nil
}

// Revert restores snapshot content to the project file.
func (m *AIReviewManager) Revert(ctx context.Context, projectID, sessionID, turnID, relPath string) error {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	old, meta, err := m.Snapshots.ReadContent(projectID, sessionID, turnID, relPath)
	if err != nil {
		return err
	}
	if !meta.HasContent {
		return turnlog.ErrSnapshotNoContent
	}
	if len(old) == 0 {
		// Empty snapshot: delete file if it exists (was created this turn).
		root, err := m.Projects.resolveFilesRoot(ctx, projectID)
		if err != nil {
			return err
		}
		abs := filepath.Join(root, relPath)
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return m.Projects.WriteFileContent(ctx, projectID, relPath, string(old))
}

// ListFileChanges returns journal records, optionally filtered by turn.
func (m *AIReviewManager) ListFileChanges(sessionID, turnID string, afterSeq int64) ([]domain.FileChangeRecord, error) {
	if m.Changes == nil {
		return nil, nil
	}
	recs, err := m.Changes.LoadAfter(sessionID, afterSeq)
	if err != nil {
		return nil, err
	}
	if turnID == "" {
		return recs, nil
	}
	var out []domain.FileChangeRecord
	for _, r := range recs {
		if r.TurnID == turnID {
			out = append(out, r)
		}
	}
	return out, nil
}

// ApplyHunks applies selected unified-diff hunks from snapshot→desired onto current file.
// hunkIndexes are 0-based indexes into the snapshot-vs-current patch hunks.
// Accepted hunks take the "new" side; rejected hunks keep the "old" (snapshot) side —
// result is written as: start from snapshot, apply only accepted new hunks... Actually
// simpler approach: start from current content, for rejected hunks reverse them.
//
// Implementation: rebuild file as snapshot + only accepted hunks' new lines in order.
func (m *AIReviewManager) ApplyHunks(ctx context.Context, projectID, sessionID, turnID, relPath string, acceptAll bool, hunkIndexes []int) error {
	diff, err := m.Diff(ctx, projectID, sessionID, turnID, relPath)
	if err != nil {
		return err
	}
	if diff.HashOnly || diff.Error != "" {
		return fmt.Errorf("cannot apply hunks: %s", diff.Error)
	}
	old, _, err := m.Snapshots.ReadContent(projectID, sessionID, turnID, relPath)
	if err != nil {
		return err
	}
	cur, err := m.readProjectBytes(ctx, projectID, relPath)
	if err != nil {
		return err
	}
	result, err := applySelectedHunks(string(old), string(cur), diff.Patch, acceptAll, hunkIndexes)
	if err != nil {
		return err
	}
	return m.Projects.WriteFileContent(ctx, projectID, relPath, result)
}
