package turnlog

import (
	"context"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// CheckpointStore serves the latest compaction checkpoint per session from
// the control-plane database (compaction_checkpoints table), with an
// in-memory read cache. Legacy checkpoint_*.json files are imported once at
// bootstrap and are no longer authoritative.
type CheckpointStore struct {
	mu    sync.RWMutex
	repo  port.CheckpointRepo
	cache map[string]*domain.CompactionCheckpoint
}

func NewCheckpointStore(repo port.CheckpointRepo) *CheckpointStore {
	return &CheckpointStore{
		repo:  repo,
		cache: make(map[string]*domain.CompactionCheckpoint),
	}
}

func (s *CheckpointStore) Load(sessionID string) (*domain.CompactionCheckpoint, error) {
	s.mu.RLock()
	if cp, ok := s.cache[sessionID]; ok {
		s.mu.RUnlock()
		return cp, nil
	}
	s.mu.RUnlock()

	cp, err := s.repo.Get(context.Background(), sessionID)
	if err != nil || cp == nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[sessionID] = cp
	s.mu.Unlock()
	return cp, nil
}

func (s *CheckpointStore) Save(sessionID string, cp *domain.CompactionCheckpoint) error {
	if cp.SessionID == "" {
		cp.SessionID = sessionID
	}
	if err := s.repo.Save(context.Background(), *cp); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[sessionID] = cp
	s.mu.Unlock()
	return nil
}
