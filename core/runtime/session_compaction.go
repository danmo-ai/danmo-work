package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

const (
	defaultMaxTokens     = 128000
	defaultTriggerRatio  = 0.85
	defaultCutTokens     = 16000
	defaultTurnInterval  = 6
	defaultSubInterval   = 4
	defaultToolTruncate  = 2000
	tokenEstimateDivisor = 4
)

type CompactionManager struct {
	mu          sync.RWMutex
	checkpoints map[string]*domain.CompactionCheckpoint
	llm         port.LLMProvider
	configStore port.ConfigStore
	stream      port.EventStream
	store       CompactionCheckpointStore
	fileChanges FileChangeJournal
	modelLimits *ModelConfigRegistry
}

type CompactionCheckpointStore interface {
	Load(sessionID string) (*domain.CompactionCheckpoint, error)
	Save(sessionID string, cp *domain.CompactionCheckpoint) error
}

type compactionCfg struct {
	enabled      bool
	maxTokens    int
	triggerRatio float64
	cutTokens    int
	turnInterval int
	subInterval  int
	toolTruncate int
}

func NewCompactionManager(llm port.LLMProvider, stream port.EventStream, configStore port.ConfigStore, store CompactionCheckpointStore, modelLimits *ModelConfigRegistry) *CompactionManager {
	if modelLimits == nil {
		modelLimits = NewModelConfigRegistry()
	}
	return &CompactionManager{
		checkpoints: make(map[string]*domain.CompactionCheckpoint),
		llm:         llm,
		configStore: configStore,
		stream:      stream,
		store:       store,
		modelLimits: modelLimits,
	}
}

// SetFileChangeJournal wires the session file-change log used during compaction.
func (m *CompactionManager) SetFileChangeJournal(journal FileChangeJournal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileChanges = journal
}

func (m *CompactionManager) loadCfg(ctx context.Context) compactionCfg {
	cfg := compactionCfg{
		maxTokens:    0, // 0 means “use model context window”
		triggerRatio: defaultTriggerRatio,
		cutTokens:    defaultCutTokens,
		turnInterval: defaultTurnInterval,
		subInterval:  defaultSubInterval,
		toolTruncate: defaultToolTruncate,
	}
	if m.configStore != nil {
		if c, err := m.configStore.Load(ctx); err == nil {
			rt := c.Runtime.Compaction
			cfg.enabled = rt.Enabled
			if rt.MaxTokens > 0 {
				cfg.maxTokens = rt.MaxTokens
			}
			if rt.TriggerRatio > 0 {
				cfg.triggerRatio = rt.TriggerRatio
			}
			if rt.CutTokens > 0 {
				cfg.cutTokens = rt.CutTokens
			}
			if rt.TurnInterval > 0 {
				cfg.turnInterval = rt.TurnInterval
			}
			if rt.SubInterval > 0 {
				cfg.subInterval = rt.SubInterval
			}
			if rt.ToolTruncate > 0 {
				cfg.toolTruncate = rt.ToolTruncate
			}
			// Reload model limits from config into the registry.
			m.modelLimits.SetModels(c.LLM.Models)
		}
	}
	return cfg
}

func (m *CompactionManager) ShouldCompact(sessionID string, turnCount int, tokenEstimate int, maxPromptTokens int, model string) bool {
	cfg := m.loadCfg(context.Background())
	if !cfg.enabled {
		return false
	}

	// Determine the effective context limit and actual token usage.
	// Priority: actual API usage > char-based estimation.
	contextLimit := cfg.maxTokens
	actualTokens := tokenEstimate

	// If we have actual prompt tokens from the LLM API, use them + model's real context window.
	if maxPromptTokens > 0 {
		actualTokens = maxPromptTokens
		if modelLimit := m.modelLimits.ContextWindow(model); modelLimit > 0 {
			// Reserve space for output tokens so we trigger before hitting the hard limit.
			maxOutput := m.modelLimits.MaxOutputTokens(model)
			usable := modelLimit - maxOutput
			if usable > 0 {
				// Use the smaller of config override and model-derived usable space.
				if contextLimit == 0 || usable < contextLimit {
					contextLimit = usable
				}
			}
		}
	}

	// Token-based trigger: actual usage exceeds triggerRatio of context limit.
	if contextLimit > 0 && actualTokens > int(float64(contextLimit)*cfg.triggerRatio) {
		return true
	}

	// If no context limit is known, try model context window as fallback.
	if contextLimit == 0 && model != "" {
		if modelLimit := m.modelLimits.ContextWindow(model); modelLimit > 0 {
			maxOutput := m.modelLimits.MaxOutputTokens(model)
			usable := modelLimit - maxOutput
			if usable > 0 && tokenEstimate > int(float64(usable)*cfg.triggerRatio) {
				return true
			}
		}
	}

	// Turn-count-based trigger (fallback when token data is unreliable).
	if cfg.turnInterval > 0 {
		cp := m.getCheckpoint(sessionID)
		if cp == nil {
			return turnCount >= cfg.turnInterval
		}
		interval := cfg.subInterval
		if interval <= 0 {
			interval = defaultSubInterval
		}
		return turnCount-cp.TurnCount >= interval
	}
	return false
}

func (m *CompactionManager) Checkpoint(sessionID string) *domain.CompactionCheckpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkpoints[sessionID]
}

func (m *CompactionManager) Recover(ctx context.Context, sessionID string) *domain.CompactionCheckpoint {
	return m.getCheckpoint(sessionID)
}

func (m *CompactionManager) Compact(ctx context.Context, sessionID, turnID string, messages []Message, turnCount int, model string) int {
	cfg := m.loadCfg(ctx)
	if len(messages) <= 1 {
		return 0
	}

	tokensBefore := estimateTokenCount(messages)

	prevCP := m.getCheckpoint(sessionID)
	cutIdx := findKeepStart(messages, cfg.cutTokens)
	if cutIdx <= 0 {
		return 0
	}

	oldMessages := messages[:cutIdx]
	conversation := serializeConversation(oldMessages, cfg.toolTruncate)

	summary, err := m.summarize(ctx, conversation, prevCP, model)
	if err != nil || summary == "" {
		return 0
	}

	todos := extractLatestTodos(messages)
	if len(todos) == 0 && prevCP != nil {
		todos = prevCP.Todos
	}

	cp := &domain.CompactionCheckpoint{
		SessionID:     sessionID,
		TurnID:        turnID,
		Summary:       summary,
		Todos:         todos,
		TurnCount:     turnCount,
		TokenEstimate: tokensBefore,
	}
	applyFileChangesToCheckpoint(m.fileChanges, sessionID, prevCP, cp)
	m.setCheckpoint(sessionID, cp)

	if m.stream != nil {
		m.stream.Publish(ctx, sessionID, turnID, domain.EventContextCompacted, domain.ContextCompactedPayload{
			FilePath:       fmt.Sprintf("checkpoint_%s.json", turnID),
			TurnsCompacted: len(oldMessages),
			TokensBefore:   tokensBefore,
			TokensAfter:    estimateTokenCount(messages[cutIdx:]),
		})
	}

	return cutIdx
}

// CompactToRetain summarizes oldMessages and records retainFromTurnID /
// retainSkipMessages so later LoadSessionMessages can replay from a mid-turn cut.
// todoSource should be the full pre-cut history (old+keep) so latest todowrite is kept.
func (m *CompactionManager) CompactToRetain(ctx context.Context, sessionID, turnID string, oldMessages, todoSource []Message, turnCount int, model, retainFromTurnID string, retainSkipMessages, tokensBefore int) bool {
	if len(oldMessages) == 0 || retainFromTurnID == "" {
		return false
	}
	cfg := m.loadCfg(ctx)
	prevCP := m.getCheckpoint(sessionID)
	conversation := serializeConversation(oldMessages, cfg.toolTruncate)
	summary, err := m.summarize(ctx, conversation, prevCP, model)
	if err != nil || summary == "" {
		return false
	}
	src := todoSource
	if len(src) == 0 {
		src = oldMessages
	}
	todos := extractLatestTodos(src)
	if len(todos) == 0 && prevCP != nil {
		todos = prevCP.Todos
	}
	cp := &domain.CompactionCheckpoint{
		SessionID:          sessionID,
		TurnID:             turnID,
		Summary:            summary,
		Todos:              todos,
		TurnCount:          turnCount,
		TokenEstimate:      tokensBefore,
		RetainFromTurnID:   retainFromTurnID,
		RetainSkipMessages: retainSkipMessages,
	}
	applyFileChangesToCheckpoint(m.fileChanges, sessionID, prevCP, cp)
	m.setCheckpoint(sessionID, cp)
	if m.stream != nil {
		m.stream.Publish(ctx, sessionID, turnID, domain.EventContextCompacted, domain.ContextCompactedPayload{
			FilePath:       fmt.Sprintf("checkpoint_%s.json", turnID),
			TurnsCompacted: len(oldMessages),
			TokensBefore:   tokensBefore,
			TokensAfter:    0,
		})
	}
	return true
}

func (m *CompactionManager) summarize(ctx context.Context, conversation string, prev *domain.CompactionCheckpoint, model string) (string, error) {
	var prompt string
	if prev != nil && prev.Summary != "" {
		prompt = fmt.Sprintf(compactionIncrementalPrompt, prev.Summary, conversation)
	} else {
		prompt = fmt.Sprintf(compactionPrompt, conversation)
	}

	resp, err := m.llm.Chat(ctx, port.LLMChatRequest{
		Model: model,
		Messages: []port.ChatMessage{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (m *CompactionManager) getCheckpoint(sessionID string) *domain.CompactionCheckpoint {
	m.mu.RLock()
	if cp, ok := m.checkpoints[sessionID]; ok {
		m.mu.RUnlock()
		return cp
	}
	m.mu.RUnlock()

	if m.store != nil {
		cp, err := m.store.Load(sessionID)
		if err == nil && cp != nil {
			m.mu.Lock()
			m.checkpoints[sessionID] = cp
			m.mu.Unlock()
			return cp
		}
	}
	return nil
}

func (m *CompactionManager) setCheckpoint(sessionID string, cp *domain.CompactionCheckpoint) {
	m.mu.Lock()
	m.checkpoints[sessionID] = cp
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.Save(sessionID, cp)
	}
}

func findCutPoint(messages []Message, cutTokens int) int {
	return findKeepStart(messages, cutTokens)
}

// msgBlock is a contiguous [start, end) range that must be kept or dropped together.
type msgBlock struct {
	start, end int
}

// buildBlocks groups messages into atomic units:
//   - assistant(content? + tool_calls) + following matching tool_results
//   - otherwise a single message (user / text-only assistant / orphan tool)
func buildBlocks(messages []Message) []msgBlock {
	var blocks []msgBlock
	for i := 0; i < len(messages); {
		m := messages[i]
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			ids := make(map[string]bool, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				ids[tc.ID] = true
			}
			start := i
			i++
			for i < len(messages) && messages[i].Role == RoleTool && ids[messages[i].ToolCallID] {
				i++
			}
			blocks = append(blocks, msgBlock{start: start, end: i})
			continue
		}
		blocks = append(blocks, msgBlock{start: i, end: i + 1})
		i++
	}
	return blocks
}

func blockTokens(messages []Message, b msgBlock) int {
	n := 0
	for i := b.start; i < b.end; i++ {
		n += estimateMessageTokens(messages[i])
	}
	return n
}

// findKeepStart returns the first index of the retain window under a hard
// cutTokens budget, walking newest→oldest by complete blocks. A single newest
// block that alone exceeds the budget is still kept (caller should truncate
// tool results). Prefer retaining at least one user message when present.
func findKeepStart(messages []Message, cutTokens int) int {
	if len(messages) == 0 {
		return 0
	}
	if cutTokens <= 0 {
		return 0
	}
	blocks := buildBlocks(messages)
	if len(blocks) == 0 {
		return 0
	}

	acc := 0
	keepStart := len(messages)
	for i := len(blocks) - 1; i >= 0; i-- {
		bt := blockTokens(messages, blocks[i])
		if acc > 0 && acc+bt > cutTokens {
			break
		}
		acc += bt
		keepStart = blocks[i].start
		if acc >= cutTokens {
			break
		}
	}
	if keepStart >= len(messages) {
		keepStart = blocks[len(blocks)-1].start
	}

	// Ensure the retain window includes a user message when one exists earlier.
	hasUser := false
	for i := keepStart; i < len(messages); i++ {
		if messages[i].Role == RoleUser {
			hasUser = true
			break
		}
	}
	if !hasUser {
		for i := keepStart - 1; i >= 0; i-- {
			if messages[i].Role != RoleUser {
				continue
			}
			for _, b := range blocks {
				if i >= b.start && i < b.end {
					keepStart = b.start
					break
				}
			}
			break
		}
	}
	return keepStart
}

// extractLatestTodos walks messages newest-first and returns the last todowrite list.
func extractLatestTodos(messages []Message) []domain.CompactionTodoItem {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != RoleAssistant {
			continue
		}
		for j := len(m.ToolCalls) - 1; j >= 0; j-- {
			tc := m.ToolCalls[j]
			if tc.Name != "todowrite" {
				continue
			}
			if items := parseTodoArgs(tc.Arguments); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func parseTodoArgs(args map[string]any) []domain.CompactionTodoItem {
	if args == nil {
		return nil
	}
	raw, ok := args["todos"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]domain.CompactionTodoItem, 0, len(list))
	for _, t := range list {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		content, _ := m["content"].(string)
		if content == "" {
			continue
		}
		status, _ := m["status"].(string)
		if status == "" {
			status = "pending"
		}
		priority, _ := m["priority"].(string)
		if priority == "" {
			priority = "medium"
		}
		out = append(out, domain.CompactionTodoItem{
			Content:  content,
			Status:   status,
			Priority: priority,
		})
	}
	return out
}

// formatActiveTodos renders structured todos for system-prompt injection.
func formatActiveTodos(todos []domain.CompactionTodoItem) string {
	if len(todos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<active-todos>\n")
	for i, t := range todos {
		b.WriteString(fmt.Sprintf("%d. [%s] %s (%s)\n", i+1, t.Status, t.Content, t.Priority))
	}
	b.WriteString("</active-todos>")
	return b.String()
}

func serializeConversation(messages []Message, truncateLen int) string {
	var b strings.Builder
	for _, m := range messages {
		switch m.Role {
		case RoleUser:
			b.WriteString("[User]: ")
			b.WriteString(m.Content)
			b.WriteString("\n\n")
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					argsStr, _ := json.Marshal(tc.Arguments)
					b.WriteString(fmt.Sprintf("[Assistant → tool_call: %s(%s)]\n", tc.Name, string(argsStr)))
				}
			} else {
				b.WriteString("[Assistant]: ")
				b.WriteString(m.Content)
				b.WriteString("\n\n")
			}
		case RoleTool:
			content := m.Content
			if truncateLen > 0 && len(content) > truncateLen {
				content = content[:truncateLen] + "...(truncated)"
			}
			b.WriteString(fmt.Sprintf("[Tool result: %s]: %s\n\n", m.Name, content))
		}
	}
	return b.String()
}

func estimateTokenCount(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += estimateMessageTokens(m)
	}
	return total
}

func estimateMessageTokens(m Message) int {
	n := 0
	n += len(m.Role) / tokenEstimateDivisor
	n += len(m.Content) / tokenEstimateDivisor
	n += len(m.Name) / tokenEstimateDivisor
	n += len(m.ToolCallID) / tokenEstimateDivisor
	for _, tc := range m.ToolCalls {
		n += len(tc.ID) / tokenEstimateDivisor
		n += len(tc.Name) / tokenEstimateDivisor
		// Avoid json.Marshal in the hot path — it dominates loadRetainedHistory
		// when transcripts contain many large tool_call argument maps.
		n += estimateArgsBytes(tc.Arguments) / tokenEstimateDivisor
	}
	return n
}

// estimateArgsBytes approximates serialized argument size without JSON encoding.
func estimateArgsBytes(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		return len(x)
	case []byte:
		return len(x)
	case map[string]any:
		n := 2
		for k, val := range x {
			n += len(k) + 2 + estimateArgsBytes(val)
		}
		return n
	case []any:
		n := 2
		for _, val := range x {
			n += estimateArgsBytes(val) + 1
		}
		return n
	case bool, int, int32, int64, float32, float64:
		return 8
	default:
		return 32
	}
}

const compactionPrompt = `You are writing a compaction handoff for an AI agent that will continue this session.
The agent will not see the conversation below — only your note (plus later uncompacted turns).

Write a concise handoff so the next agent can resume correctly: honor constraints, avoid redoing finished work, and take the right next step.
Prefer concrete anchors (paths, errors, IDs, decisions) over narrative filler.
When relevant, cover: the user goal and success criteria; decisions and constraints still in force; what is done / in progress / blocked; open questions and the next concrete move.
Omit chatter, redundant tool output, and anything the agent can re-read from the repo.
Use any structure that helps (prose or light markdown). Do not force JSON.

<conversation>
%s
</conversation>`

const compactionIncrementalPrompt = `You are updating a compaction handoff for an AI agent that will continue this session.
The agent will not see the conversation below — only your updated note (plus later uncompacted turns).

Merge the previous handoff with the new transcript. Keep what still matters, refresh progress and status, drop stale or obsolete details.
Write a concise handoff so the next agent can resume correctly: honor constraints, avoid redoing finished work, and take the right next step.
Prefer concrete anchors (paths, errors, IDs, decisions) over narrative filler.
Omit chatter, redundant tool output, and anything the agent can re-read from the repo.
Use any structure that helps (prose or light markdown). Do not force JSON.

<previous-summary>
%s
</previous-summary>

<conversation>
%s
</conversation>`

func ToolInputKey(args map[string]any) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		if strings.HasPrefix(k, "__") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%v,", k, args[k])
	}
	return b.String()
}
