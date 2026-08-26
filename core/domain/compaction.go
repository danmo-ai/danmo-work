package domain

type CompactionCheckpoint struct {
	SessionID string               `json:"sessionId"`
	TurnID    string               `json:"turnId"`
	Summary   string               `json:"summary"`
	Todos     []CompactionTodoItem `json:"todos,omitempty"`
	// FileChanges is the path-level aggregate of session file-tool mutations,
	// preserved across compaction like Todos.
	FileChanges []CompactionFileChange `json:"fileChanges,omitempty"`
	// FileChangeLogSeq is the max journal Seq already merged into FileChanges.
	FileChangeLogSeq int64 `json:"fileChangeLogSeq,omitempty"`
	TurnCount        int   `json:"turnCount"`
	TokenEstimate    int   `json:"tokenEstimate"`
	// RetainFromTurnID is the first turn id to replay after compaction.
	// Turns strictly before this id are replaced by Summary in the system prompt.
	RetainFromTurnID string `json:"retainFromTurnId,omitempty"`
	// RetainSkipMessages skips this many leading reconstructed messages inside
	// RetainFromTurnID (0 = from the start of that turn). Enables mid-turn cuts
	// at tool-pair / message-block boundaries.
	RetainSkipMessages int `json:"retainSkipMessages,omitempty"`
}

// CompactionTodoItem is a structured todowrite entry preserved across compaction.
type CompactionTodoItem struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// FileChangeOp is the kind of mutation performed by a file tool.
type FileChangeOp string

const (
	FileChangeCreate FileChangeOp = "create"
	FileChangeUpdate FileChangeOp = "update"
	FileChangeDelete FileChangeOp = "delete"
)

// FileChangeRecord is one successful file-tool mutation written to the session journal.
type FileChangeRecord struct {
	Seq    int64        `json:"seq"`
	TurnID string       `json:"turnId"`
	CallID string       `json:"callId"`
	Tool   string       `json:"tool"`
	Path   string       `json:"path"`
	Op     FileChangeOp `json:"op"`
	At     string       `json:"at,omitempty"` // RFC3339
	Diff   string       `json:"diff,omitempty"`
	Bytes  int          `json:"bytes,omitempty"`
}

// CompactionFileChange is a path-level aggregate preserved on the checkpoint.
type CompactionFileChange struct {
	Path    string       `json:"path"`
	Op      FileChangeOp `json:"op"`
	Tools   []string     `json:"tools,omitempty"`
	TurnIDs []string     `json:"turnIds,omitempty"`
}

type CompactionConfig struct {
	Enabled      bool    `json:"enabled"`
	Model        string  `json:"model"`
	TriggerRatio float64 `json:"triggerRatio"`
	CutTokens    int     `json:"cutTokens"`
	TurnInterval int     `json:"turnInterval"`
	SubInterval  int     `json:"subInterval"`
	ToolTruncate int     `json:"toolTruncate"`
}
