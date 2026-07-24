package runtime

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"danmo-work/core/domain"
)

const (
	maxCompactionFileChanges = 100
	maxFileChangeTurnIDs     = 5
)

// FileChangeJournal is the compaction-side reader for the session file-change log.
type FileChangeJournal interface {
	LoadAfter(sessionID string, afterSeq int64) ([]domain.FileChangeRecord, error)
}

// FileChangeAppender is used by TurnRunner to persist successful file-tool mutations.
type FileChangeAppender interface {
	Append(sessionID, projectID string, rec domain.FileChangeRecord) (int64, error)
}

func isFileMutatingTool(name string) bool {
	switch name {
	case "write", "edit", "apply_patch":
		return true
	default:
		return false
	}
}

// fileChangeRecordsFromResult extracts journal records from a successful file-tool result.
func fileChangeRecordsFromResult(turnID, callID, toolName string, args map[string]any, result domain.ToolResult) []domain.FileChangeRecord {
	at := time.Now().UTC().Format(time.RFC3339)
	meta := result.Meta
	if meta == nil {
		meta = map[string]any{}
	}

	if raw, ok := meta["file_changes"]; ok {
		return recordsFromFileChangesMeta(turnID, callID, toolName, at, raw)
	}

	// Skip directory-only write.
	if writeType, _ := meta["write_type"].(string); writeType == "directory" {
		return nil
	}

	path, _ := meta["path"].(string)
	if path == "" {
		path, _ = args["path"].(string)
	}
	if path == "" {
		return nil
	}
	op := parseFileChangeOp(meta["op"])
	if op == "" {
		op = domain.FileChangeUpdate
	}
	diff, _ := meta["diff"].(string)
	return []domain.FileChangeRecord{{
		TurnID: turnID,
		CallID: callID,
		Tool:   toolName,
		Path:   path,
		Op:     op,
		At:     at,
		Diff:   diff,
		Bytes:  intFromAny(meta["bytes_written"]),
	}}
}

func recordsFromFileChangesMeta(turnID, callID, toolName, at string, raw any) []domain.FileChangeRecord {
	var items []map[string]any
	switch typed := raw.(type) {
	case []map[string]any:
		items = typed
	case []any:
		items = make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				items = append(items, m)
			}
		}
	default:
		return nil
	}
	out := make([]domain.FileChangeRecord, 0, len(items))
	for _, m := range items {
		path, _ := m["path"].(string)
		if path == "" {
			continue
		}
		op := parseFileChangeOp(m["op"])
		if op == "" {
			op = domain.FileChangeUpdate
		}
		diff, _ := m["diff"].(string)
		out = append(out, domain.FileChangeRecord{
			TurnID: turnID,
			CallID: callID,
			Tool:   toolName,
			Path:   path,
			Op:     op,
			At:     at,
			Diff:   diff,
			Bytes:  intFromAny(m["bytes_written"]),
		})
	}
	return out
}

func parseFileChangeOp(v any) domain.FileChangeOp {
	s, _ := v.(string)
	switch domain.FileChangeOp(s) {
	case domain.FileChangeCreate, domain.FileChangeUpdate, domain.FileChangeDelete:
		return domain.FileChangeOp(s)
	default:
		return ""
	}
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

type fileChangeEntry struct {
	change     domain.CompactionFileChange
	touchOrder int
	created    bool // true if we observed create in this merge stream
}

// mergeFileChanges merges prev checkpoint aggregates with newly journaled records.
// Last op wins per path. create→delete within the merged stream drops the path;
// otherwise delete markers are kept so the model knows the file was removed.
func mergeFileChanges(prev []domain.CompactionFileChange, delta []domain.FileChangeRecord) []domain.CompactionFileChange {
	byPath := make(map[string]*fileChangeEntry, len(prev)+len(delta))
	order := 0

	for _, p := range prev {
		if p.Path == "" {
			continue
		}
		byPath[p.Path] = &fileChangeEntry{
			change: domain.CompactionFileChange{
				Path:    p.Path,
				Op:      p.Op,
				Tools:   append([]string(nil), p.Tools...),
				TurnIDs: append([]string(nil), p.TurnIDs...),
			},
			touchOrder: order,
			created:    p.Op == domain.FileChangeCreate,
		}
		order++
	}

	for _, rec := range delta {
		if rec.Path == "" {
			continue
		}
		e, ok := byPath[rec.Path]
		if !ok {
			e = &fileChangeEntry{
				change: domain.CompactionFileChange{Path: rec.Path},
			}
			byPath[rec.Path] = e
		}
		e.touchOrder = order
		order++
		if rec.Op == domain.FileChangeCreate {
			e.created = true
		}
		e.change.Op = rec.Op
		e.change.Tools = appendUnique(e.change.Tools, rec.Tool)
		e.change.TurnIDs = appendUniqueCapped(e.change.TurnIDs, rec.TurnID, maxFileChangeTurnIDs)
		if rec.Op == domain.FileChangeDelete && e.created {
			delete(byPath, rec.Path)
		}
	}

	type ranked struct {
		path string
		e    *fileChangeEntry
	}
	list := make([]ranked, 0, len(byPath))
	for path, e := range byPath {
		list = append(list, ranked{path: path, e: e})
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].e.touchOrder < list[j].e.touchOrder
	})
	if len(list) > maxCompactionFileChanges {
		list = list[len(list)-maxCompactionFileChanges:]
	}

	out := make([]domain.CompactionFileChange, 0, len(list))
	for _, item := range list {
		out = append(out, item.e.change)
	}
	return out
}

func appendUnique(list []string, v string) []string {
	if v == "" || containsString(list, v) {
		return list
	}
	return append(list, v)
}

func appendUniqueCapped(list []string, v string, max int) []string {
	if v == "" {
		return list
	}
	if containsString(list, v) {
		return list
	}
	list = append(list, v)
	if max > 0 && len(list) > max {
		list = list[len(list)-max:]
	}
	return list
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// formatFileChanges renders structured file changes for system-prompt injection.
func formatFileChanges(changes []domain.CompactionFileChange) string {
	if len(changes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<session-file-changes>\n")
	for _, c := range changes {
		tools := strings.Join(c.Tools, ", ")
		if tools == "" {
			tools = "-"
		}
		turns := strings.Join(c.TurnIDs, ",")
		if turns == "" {
			turns = "-"
		}
		fmt.Fprintf(&b, "- %s %s (%s) turns=%s\n", c.Op, c.Path, tools, turns)
	}
	b.WriteString("</session-file-changes>")
	return b.String()
}

// applyFileChangesToCheckpoint loads journal delta since prev watermark and merges into cp.
func applyFileChangesToCheckpoint(journal FileChangeJournal, sessionID string, prev *domain.CompactionCheckpoint, cp *domain.CompactionCheckpoint) {
	if cp == nil {
		return
	}
	prevSeq := int64(0)
	var prevChanges []domain.CompactionFileChange
	if prev != nil {
		prevSeq = prev.FileChangeLogSeq
		prevChanges = prev.FileChanges
	}

	if journal == nil {
		cp.FileChanges = prevChanges
		cp.FileChangeLogSeq = prevSeq
		return
	}

	delta, err := journal.LoadAfter(sessionID, prevSeq)
	if err != nil || len(delta) == 0 {
		cp.FileChanges = prevChanges
		cp.FileChangeLogSeq = prevSeq
		return
	}
	cp.FileChanges = mergeFileChanges(prevChanges, delta)
	cp.FileChangeLogSeq = delta[len(delta)-1].Seq
}
