package turnlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// MaxSnapshotBytes matches project text file read cap (~1MiB).
	MaxSnapshotBytes = 1 << 20
)

var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotTooLarge = errors.New("file too large to snapshot")
	ErrSnapshotNoContent = errors.New("snapshot has no content (hash-only)")
)

// SnapshotMeta is persisted beside optional content bytes.
type SnapshotMeta struct {
	TurnID    string    `json:"turnId"`
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
	Bytes     int       `json:"bytes"`
	HasContent bool     `json:"hasContent"`
	CreatedAt time.Time `json:"createdAt"`
}

// SnapshotStore persists pre-turn file snapshots under the session directory.
type SnapshotStore struct {
	mu        sync.Mutex
	projector func(projectID string) string
}

func NewSnapshotStore(projector func(projectID string) string) *SnapshotStore {
	return &SnapshotStore{projector: projector}
}

func (s *SnapshotStore) sessionDir(projectID, sessionID string) string {
	if projectID == "" {
		projectID = "_default"
	}
	return filepath.Join(s.projector(projectID), "sessions", sessionID)
}

func (s *SnapshotStore) snapDir(projectID, sessionID, turnID string) string {
	safeTurn := sanitizePathSegment(turnID)
	return filepath.Join(s.sessionDir(projectID, sessionID), "snapshots", safeTurn)
}

func sanitizePathSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "_empty"
	}
	return s
}

func encodePathKey(relPath string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(relPath)))
	return hex.EncodeToString(sum[:])
}

// Save writes snapshot content (when under limit) + meta for (session, turn, path).
func (s *SnapshotStore) Save(projectID, sessionID, turnID, relPath string, content []byte) (*SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	if relPath == "" || turnID == "" || sessionID == "" {
		return nil, fmt.Errorf("sessionId, turnId, and path are required")
	}

	hash := sha256.Sum256(content)
	meta := &SnapshotMeta{
		TurnID:     turnID,
		Path:       relPath,
		Hash:       hex.EncodeToString(hash[:]),
		Bytes:      len(content),
		HasContent: len(content) <= MaxSnapshotBytes,
		CreatedAt:  time.Now().UTC(),
	}

	dir := s.snapDir(projectID, sessionID, turnID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	key := encodePathKey(relPath)
	metaPath := filepath.Join(dir, key+".json")
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return nil, err
	}
	if meta.HasContent {
		if err := os.WriteFile(filepath.Join(dir, key+".bin"), content, 0644); err != nil {
			return nil, err
		}
	}
	return meta, nil
}

// GetMeta loads snapshot metadata.
func (s *SnapshotStore) GetMeta(projectID, sessionID, turnID, relPath string) (*SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getMetaLocked(projectID, sessionID, turnID, relPath)
}

func (s *SnapshotStore) getMetaLocked(projectID, sessionID, turnID, relPath string) (*SnapshotMeta, error) {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	key := encodePathKey(relPath)
	metaPath := filepath.Join(s.snapDir(projectID, sessionID, turnID), key+".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}
	var meta SnapshotMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ReadContent returns snapshot file bytes when available.
func (s *SnapshotStore) ReadContent(projectID, sessionID, turnID, relPath string) ([]byte, *SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.getMetaLocked(projectID, sessionID, turnID, relPath)
	if err != nil {
		return nil, nil, err
	}
	if !meta.HasContent {
		return nil, meta, ErrSnapshotNoContent
	}
	key := encodePathKey(filepath.ToSlash(strings.TrimPrefix(relPath, "/")))
	binPath := filepath.Join(s.snapDir(projectID, sessionID, turnID), key+".bin")
	data, err := os.ReadFile(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, meta, ErrSnapshotNoContent
		}
		return nil, meta, err
	}
	return data, meta, nil
}

// ListTurnPaths returns relative paths snapshotted for a turn.
func (s *SnapshotStore) ListTurnPaths(projectID, sessionID, turnID string) ([]SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.snapDir(projectID, sessionID, turnID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var meta SnapshotMeta
		if json.Unmarshal(data, &meta) == nil {
			out = append(out, meta)
		}
	}
	return out, nil
}
