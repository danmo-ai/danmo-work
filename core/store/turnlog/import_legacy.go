package turnlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// ImportLegacyJSONL performs the one-time migration of on-disk turn JSONL
// files (<project>/sessions/<sessionID>/[tool_runs/]<turnID>.jsonl) into the
// DB-backed turn log. Files are left in place as inert backups.
//
// Idempotent: a turn that already has entry rows in the DB is skipped, so a
// partially failed import can safely be retried on the next startup.
//
// Metadata merge rules follow TurnLogRepo.UpsertTurnMeta: existing turns rows
// (the engine dual-wrote them alongside JSONL) keep their status; only blank
// fields are filled and the nested flag is raised for tool_runs files. A file
// with no corresponding turns row is inserted with its parsed terminal status,
// or as failed when the file has no "end" entry — importing an ancient
// half-written turn as "running" would wrongly make it a recovery candidate.
func ImportLegacyJSONL(ctx context.Context, repo port.TurnLogRepo, projector func(projectID string) string) (imported int, err error) {
	root := projector("")
	projectEntries, readErr := os.ReadDir(root)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return 0, nil
		}
		return 0, readErr
	}

	var firstErr error
	for _, pe := range projectEntries {
		if !pe.IsDir() {
			continue
		}
		projectID := pe.Name()
		sessionsRoot := filepath.Join(projector(projectID), "sessions")
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
			for _, sub := range []struct {
				dir    string
				nested bool
			}{
				{sessionDir, false},
				{filepath.Join(sessionDir, "tool_runs"), true},
			} {
				files, err := os.ReadDir(sub.dir)
				if err != nil {
					continue
				}
				for _, fe := range files {
					name := fe.Name()
					if fe.IsDir() || !strings.HasSuffix(name, ".jsonl") {
						continue
					}
					turnID := strings.TrimSuffix(name, ".jsonl")
					ok, err := importTurnFile(ctx, repo, filepath.Join(sub.dir, name), turnID, sessionID, projectID, sub.nested)
					if err != nil && firstErr == nil {
						firstErr = fmt.Errorf("import %s: %w", filepath.Join(sub.dir, name), err)
					}
					if ok {
						imported++
					}
				}
			}
		}
	}
	return imported, firstErr
}

func importTurnFile(ctx context.Context, repo port.TurnLogRepo, filePath, turnID, sessionID, projectID string, nested bool) (bool, error) {
	maxSeq, err := repo.MaxSeq(ctx, turnID)
	if err != nil {
		return false, err
	}
	if maxSeq > 0 {
		return false, nil // already imported (or turn already lives in the DB era)
	}

	goal, agentID, status, entries, err := readLegacyFile(filePath)
	if err != nil {
		return false, err
	}

	if err := repo.UpsertTurnMeta(ctx, domain.TurnLog{
		ID: turnID, SessionID: sessionID, ProjectID: projectID,
		AgentID: agentID, Goal: goal, Status: status, Nested: nested,
	}); err != nil {
		return false, err
	}
	for _, e := range entries {
		if err := repo.AppendEntry(ctx, port.TurnLogEntryRecord{
			TurnID: turnID, Seq: e.Seq, Type: e.Type, Data: e.Data,
		}); err != nil {
			return false, err
		}
	}
	return true, nil
}

// readLegacyFile parses a legacy JSONL turn file. It tolerates a truncated
// trailing line (crash artifact) by keeping the well-formed prefix. Only
// whitelist message entries are returned; start/end map to metadata.
func readLegacyFile(filePath string) (goal, agentID string, status domain.TurnStatus, entries []EntryJSON, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", "", "", nil, err
	}
	defer f.Close()

	status = domain.TurnFailed // no "end" entry → treat as crashed, never a recovery candidate
	seq := 0
	dec := json.NewDecoder(f)
	for dec.More() {
		var e EntryJSON
		if decErr := dec.Decode(&e); decErr != nil {
			break // truncated/corrupt tail — keep the prefix
		}
		switch e.Type {
		case "start":
			if g := stringField(e.Data, "goal"); g != "" {
				goal = g
			}
			if a := stringField(e.Data, "agent_id"); a != "" {
				agentID = a
			}
		case "end":
			if st := stringField(e.Data, "status"); st != "" {
				status = domain.TurnStatus(st)
			}
		case "user", "assistant", "tool_call", "tool_result":
			// Legacy files always carry increasing seqs, but be defensive:
			// the DB enforces uniqueness of (turn_id, seq).
			if e.Seq <= seq {
				e.Seq = seq + 1
			}
			seq = e.Seq
			entries = append(entries, e)
		}
	}
	return goal, agentID, status, entries, nil
}
