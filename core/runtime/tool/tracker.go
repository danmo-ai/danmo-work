package tool

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type FileSnapshot struct {
	Path    string
	Size    int64
	ModTime time.Time
}

type FileTracker struct {
	mu      sync.RWMutex
	files   map[string]FileSnapshot
	workDir string
}

func NewFileTracker(workDir string) *FileTracker {
	return &FileTracker{
		files:   make(map[string]FileSnapshot),
		workDir: workDir,
	}
}

func (t *FileTracker) NoteRead(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read_file: cannot stat %q: %w", path, err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.files[path] = FileSnapshot{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	return nil
}

func (t *FileTracker) RequireRead(path string) error {
	t.mu.RLock()
	snap, ok := t.files[path]
	t.mu.RUnlock()
	if !ok {
		return fmt.Errorf("file %q has not been read yet in this turn. Call read_file on this exact path before edit/write (reading a different file with a similar name does not count)", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() != snap.Size || !info.ModTime().Equal(snap.ModTime) {
		return fmt.Errorf("file %q has changed since it was last read (size/modTime mismatch). Another tool or process updated it — call read_file again on this path, then retry the edit with fresh content", path)
	}
	return nil
}

func (t *FileTracker) Snapshot(path string) (FileSnapshot, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	snap, ok := t.files[path]
	return snap, ok
}
