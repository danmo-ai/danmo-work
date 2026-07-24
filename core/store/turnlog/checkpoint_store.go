package turnlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"danqing-teams/core/domain"
)

type CheckpointStore struct {
	mu              sync.RWMutex
	projector       func(projectID string) string
	cache           map[string]*domain.CompactionCheckpoint
	sessionProjects map[string]string
}

func NewCheckpointStore(projector func(projectID string) string) *CheckpointStore {
	return &CheckpointStore{
		projector:       projector,
		cache:           make(map[string]*domain.CompactionCheckpoint),
		sessionProjects: make(map[string]string),
	}
}

func (s *CheckpointStore) sessionsDir(projectID string) string {
	return filepath.Join(s.projector(projectID), "sessions")
}

func (s *CheckpointStore) resolveSessionDir(sessionID string) string {
	if pid, ok := s.sessionProjects[sessionID]; ok {
		return filepath.Join(s.sessionsDir(pid), sessionID)
	}
	projectIDs, _ := os.ReadDir(s.projector(""))
	for _, e := range projectIDs {
		if !e.IsDir() {
			continue
		}
		sessionsRoot := filepath.Join(s.projector(e.Name()), "sessions")
		entries, err := os.ReadDir(sessionsRoot)
		if err != nil {
			continue
		}
		for _, se := range entries {
			if se.IsDir() && se.Name() == sessionID {
				s.sessionProjects[sessionID] = e.Name()
				return filepath.Join(sessionsRoot, sessionID)
			}
		}
	}
	dir := filepath.Join(s.sessionsDir("_default"), sessionID)
	return dir
}

func (s *CheckpointStore) RegisterSession(sessionID, projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionProjects[sessionID] = projectID
}

func (s *CheckpointStore) Load(sessionID string) (*domain.CompactionCheckpoint, error) {
	s.mu.RLock()
	if cp, ok := s.cache[sessionID]; ok {
		s.mu.RUnlock()
		return cp, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.resolveSessionDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var latest *domain.CompactionCheckpoint

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "checkpoint_") || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
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

		// TurnCount is monotonic for normal compaction. TurnID (nanosecond
		// suffix) deterministically selects the newest checkpoint on ties.
		if latest == nil ||
			cp.TurnCount > latest.TurnCount ||
			(cp.TurnCount == latest.TurnCount && cp.TurnID > latest.TurnID) {
			latest = &cp
		}
	}

	if latest != nil {
		s.cache[sessionID] = latest
	}
	return latest, nil
}

func (s *CheckpointStore) Save(sessionID string, cp *domain.CompactionCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[sessionID] = cp

	dir := s.resolveSessionDir(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("checkpoint_%s.json", cp.TurnID))
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
