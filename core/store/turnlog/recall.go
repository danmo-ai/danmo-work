package turnlog

import (
	"context"
	"strings"

	"danmo-work/core/port"
)

const ingestTruncateMarker = "\n...[truncated, "

// RecallToolResult returns the durable tool_result for callID in turnID.
// When multiple entries share callID (e.g. recovery rewrite), the last wins.
func (s *TurnLogStore) RecallToolResult(turnID, callID string) (port.RecalledToolResult, bool) {
	if turnID == "" || callID == "" {
		return port.RecalledToolResult{}, false
	}
	return recallFromEntries(turnID, s.entryMaps(context.Background(), turnID), callID)
}

// RecallToolResultInSession searches non-nested turns in sessionID, newest first.
func (s *TurnLogStore) RecallToolResultInSession(sessionID, callID string) (port.RecalledToolResult, bool) {
	if sessionID == "" || callID == "" {
		return port.RecalledToolResult{}, false
	}
	ids, err := s.repo.ListSessionTurnIDs(context.Background(), sessionID, false)
	if err != nil || len(ids) == 0 {
		return port.RecalledToolResult{}, false
	}
	ctx := context.Background()
	for i := len(ids) - 1; i >= 0; i-- {
		if r, ok := recallFromEntries(ids[i], s.entryMaps(ctx, ids[i]), callID); ok {
			return r, true
		}
	}
	return port.RecalledToolResult{}, false
}

func recallFromEntries(turnID string, entries []map[string]any, callID string) (port.RecalledToolResult, bool) {
	var last port.RecalledToolResult
	found := false
	for _, e := range entries {
		if e["type"] != "tool_result" {
			continue
		}
		data, _ := e["data"].(map[string]any)
		if stringField(data, "call_id") != callID {
			continue
		}
		output := stringField(data, "output")
		last = port.RecalledToolResult{
			TurnID:          turnID,
			CallID:          callID,
			ToolName:        stringField(data, "name"),
			Output:          output,
			IngestTruncated: isIngestTruncated(output),
		}
		found = true
	}
	return last, found
}

func isIngestTruncated(output string) bool {
	return strings.Contains(output, ingestTruncateMarker) && strings.Contains(output, " total chars]")
}
