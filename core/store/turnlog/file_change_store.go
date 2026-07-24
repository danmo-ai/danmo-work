package turnlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"danmo-work/core/domain"
)

const maxFileChangeDiffBytes = 4096

// FileChangeStore appends session file-tool mutations to file_changes.jsonl.
type FileChangeStore struct {
	mu              sync.Mutex
	projector       func(projectID string) string
	sessionProjects map[string]string
	lastSeq         map[string]int64
}

func NewFileChangeStore(projector func(projectID string) string) *FileChangeStore {
	return &FileChangeStore{
		projector:       projector,
		sessionProjects: make(map[string]string),
		lastSeq:         make(map[string]int64),
	}
}

func (s *FileChangeStore) sessionsDir(projectID string) string {
	return filepath.Join(s.projector(projectID), "sessions")
}

func (s *FileChangeStore) RegisterSession(sessionID, projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if projectID == "" {
		projectID = "_default"
	}
	s.sessionProjects[sessionID] = projectID
}

func (s *FileChangeStore) resolveSessionDir(sessionID string) string {
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
	return filepath.Join(s.sessionsDir("_default"), sessionID)
}

func (s *FileChangeStore) journalPath(sessionID string) string {
	return filepath.Join(s.resolveSessionDir(sessionID), "file_changes.jsonl")
}

// Append assigns a monotonic Seq, truncates Diff, and writes one JSONL line.
func (s *FileChangeStore) Append(sessionID, projectID string, rec domain.FileChangeRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectID == "" {
		projectID = "_default"
	}
	s.sessionProjects[sessionID] = projectID

	seq, err := s.nextSeqLocked(sessionID)
	if err != nil {
		return 0, err
	}
	rec.Seq = seq
	if rec.At == "" {
		rec.At = time.Now().UTC().Format(time.RFC3339)
	}
	if len(rec.Diff) > maxFileChangeDiffBytes {
		rec.Diff = rec.Diff[:maxFileChangeDiffBytes] + "\n...[diff truncated]"
	}

	dir := s.resolveSessionDir(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, err
	}
	path := s.journalPath(sessionID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return 0, err
	}
	s.lastSeq[sessionID] = seq
	return seq, nil
}

// LoadAfter returns journal records with Seq > afterSeq (ascending).
func (s *FileChangeStore) LoadAfter(sessionID string, afterSeq int64) ([]domain.FileChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.journalPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []domain.FileChangeRecord
	var maxSeq int64
	sc := bufio.NewScanner(f)
	// Allow larger-than-default lines for truncated diffs.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec domain.FileChangeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Seq > maxSeq {
			maxSeq = rec.Seq
		}
		if rec.Seq > afterSeq {
			out = append(out, rec)
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if maxSeq > s.lastSeq[sessionID] {
		s.lastSeq[sessionID] = maxSeq
	}
	return out, nil
}

func (s *FileChangeStore) nextSeqLocked(sessionID string) (int64, error) {
	if seq, ok := s.lastSeq[sessionID]; ok && seq > 0 {
		return seq + 1, nil
	}
	maxSeq, err := s.scanMaxSeqLocked(sessionID)
	if err != nil {
		return 0, err
	}
	s.lastSeq[sessionID] = maxSeq
	return maxSeq + 1, nil
}

func (s *FileChangeStore) scanMaxSeqLocked(sessionID string) (int64, error) {
	path := s.journalPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	var maxSeq int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec domain.FileChangeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Seq > maxSeq {
			maxSeq = rec.Seq
		}
	}
	return maxSeq, sc.Err()
}
