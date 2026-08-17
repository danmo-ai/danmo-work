package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/permission"
	"danmo-work/core/runtime/tool"
)

const (
	turnToolTextMaxChars       = 2000
	defaultKeepRecentToolSteps = 3
	turnHugeResultThreshold    = 60000
	defaultMaxToolOutputChars  = 50000
	turnTokenEstimateDivisor   = 4
	doomPatternWindow          = 8
	doomDescribeMaxLen         = 200
	toolErrorHint              = "\n[Analyze the error above and try a different approach.]"
)

const maxStepsPrompt = `<system-reminder>
CRITICAL - MAXIMUM STEPS REACHED

This agent has reached its maximum step limit. Tools are NO LONGER available.

STRICT REQUIREMENTS:
1. Do NOT attempt any more tool calls
2. MUST provide a text-only response summarizing what was accomplished
3. List any remaining tasks that were NOT completed
4. Recommend what the user should do next

This is your FINAL response for this turn.
</system-reminder>`

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ContentPart is a multimodal block on a message (vision images).
type ContentPart struct {
	Type     string `json:"type"` // "image"
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // raw base64
	Name     string `json:"name,omitempty"`
}

type Message struct {
	Role             Role          `json:"role"`
	Content          string        `json:"content,omitempty"`
	Parts            []ContentPart `json:"parts,omitempty"`
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
	Name             string        `json:"name,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"function_name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func toPortMessages(msgs []Message) []port.ChatMessage {
	out := make([]port.ChatMessage, len(msgs))
	for i, m := range msgs {
		var parts []port.ChatContentPart
		if len(m.Parts) > 0 {
			parts = make([]port.ChatContentPart, len(m.Parts))
			for j, p := range m.Parts {
				parts[j] = port.ChatContentPart{
					Type: p.Type, MimeType: p.MimeType, Data: p.Data, Name: p.Name,
				}
			}
		}
		out[i] = port.ChatMessage{
			Role: string(m.Role), Content: m.Content, Parts: parts,
			ToolCalls:  toPortToolCalls(m.ToolCalls),
			ToolCallID: m.ToolCallID, Name: m.Name,
			ReasoningContent: m.ReasoningContent,
		}
	}
	return out
}

func userMessageFromAttachments(goal string, atts []domain.UserAttachment) Message {
	msg := Message{Role: RoleUser, Content: goal}
	if len(atts) == 0 {
		return msg
	}
	parts := make([]ContentPart, 0, len(atts))
	for _, a := range atts {
		if a.Type != "image" || a.Data == "" {
			continue
		}
		parts = append(parts, ContentPart{
			Type: "image", MimeType: a.MimeType, Data: a.Data, Name: a.Name,
		})
	}
	msg.Parts = parts
	return msg
}

func userMessagePayload(goal string, atts []domain.UserAttachment) domain.UserMessagePayload {
	p := domain.UserMessagePayload{Content: goal}
	if len(atts) == 0 {
		return p
	}
	p.Attachments = make([]domain.UserMessageAttachment, 0, len(atts))
	for _, a := range atts {
		if a.Type != "image" || a.Data == "" {
			continue
		}
		mime := a.MimeType
		if mime == "" {
			mime = "image/png"
		}
		p.Attachments = append(p.Attachments, domain.UserMessageAttachment{
			Type: "image", Name: a.Name, MimeType: mime,
			DataURL: "data:" + mime + ";base64," + a.Data,
		})
	}
	return p
}

func toPortToolCalls(calls []ToolCall) []port.ChatToolCall {
	out := make([]port.ChatToolCall, len(calls))
	for i, c := range calls {
		out[i] = port.ChatToolCall{ID: c.ID, Name: c.Name, Arguments: c.Arguments}
	}
	return out
}

type TurnContext struct {
	SessionID string
	TurnID    string
	Agent     domain.Agent
	Model     string
	MaxSteps  int
	WorkDir   string
	ProjectID string
	// PlanMode restricts the agent to read-only built-in tools and injects a
	// plan-mode system prompt. Inherited by sub-turns.
	PlanMode bool
	// Path is the root→current turn chain (lead first). Used for delegation
	// depth and cycle checks; turn IDs alone do not encode ancestry.
	Path     []domain.TurnPathEntry
	OnReport func(domain.Report)
	Messages []Message
	// EphemeralContext is call-time-only user content (todos, file-changes,
	// knowledge hits) appended after <turn-context>. Not persisted.
	EphemeralContext string
	// ClaimSteers loads durable soft-steer messages (status=steering) for this
	// session. Called after parallel tools finish, before the next LLM call
	// (and when the model stops so a steer can keep the turn alive).
	ClaimSteers func() []Message
}

type approvalGate interface {
	WaitApproval(ctx context.Context, approvalID string) (ApprovalOutcome, error)
	CreateApproval(sessionID, turnID, toolName, description, reason, domain string) string
}

type TurnRunner struct {
	LLM                 port.LLMProvider
	Stream              port.EventStream
	Perm                *permission.Gate
	Registry            *tool.Registry
	SkillList           []domain.Skill
	ToolBindings        []domain.ToolBinding
	Approval            approvalGate
	ConfigStore         port.ConfigStore
	Log                 func(typ string, data map[string]any)
	FileTracker         *tool.FileTracker
	FileChanges         FileChangeAppender
	SandboxStatus       func() domain.SandboxStatus
	EffectiveIsolation  func() domain.EffectiveIsolation
	SessionAllowNetwork func(sessionID string) bool
	SessionAllowDomains func(sessionID string) []string
	GrantSessionDomains func(sessionID string, domains []string)
	GrantTurnDomains    func(turnID string, domains []string)
	ClearTurnDomains    func(turnID string)
	mu                  sync.Mutex
	doomState           map[string]*doomTurnState
}

// doomTurnState tracks consecutive identical tool signatures (mainstream-style).
type doomTurnState struct {
	lastKey  string
	streak   int
	patterns []string // recent signatures for A-B-A-B detection
}

type turnRunCfg struct {
	autoApprove            bool
	permissionMode         domain.PermissionMode
	doomLoopThreshold      int
	maxStepsDefault        int
	maxLLMFailures         int
	maxToolOutputChars     int
	compactionEnabled      bool
	compactionMaxTokens    int
	compactionTriggerRatio float64
	compactionCutTokens    int
	toolTruncateChars      int
	keepRecentToolSteps    int
}

func NewTurnRunner(llm port.LLMProvider, stream port.EventStream, perm *permission.Gate, reg *tool.Registry, configStore port.ConfigStore) *TurnRunner {
	return &TurnRunner{
		LLM: llm, Stream: stream, Perm: perm, Registry: reg,
		ConfigStore: configStore,
		doomState:   make(map[string]*doomTurnState),
	}
}

func (p *TurnRunner) loadRunCfg(ctx context.Context) turnRunCfg {
	cfg := turnRunCfg{
		doomLoopThreshold:      10,
		maxStepsDefault:        200,
		maxLLMFailures:         3,
		maxToolOutputChars:     defaultMaxToolOutputChars,
		compactionMaxTokens:    128000,
		compactionTriggerRatio: 0.85,
		compactionCutTokens:    16000,
		toolTruncateChars:      turnToolTextMaxChars,
		keepRecentToolSteps:    defaultKeepRecentToolSteps,
	}
	if p.ConfigStore != nil {
		if c, err := p.ConfigStore.Load(ctx); err == nil {
			rt := c.Runtime
			cfg.autoApprove = rt.AutoApprove
			cfg.permissionMode = rt.PermissionMode
			if cfg.permissionMode == "" {
				cfg.permissionMode = domain.PermModeInteractive
			}
			if p.Perm != nil {
				p.Perm = p.Perm.WithRules(rt.PermissionRules).WithMode(cfg.permissionMode)
			}
			if rt.Turn.DoomLoopThreshold > 0 {
				cfg.doomLoopThreshold = rt.Turn.DoomLoopThreshold
			}
			if rt.Turn.MaxStepsDefault > 0 {
				cfg.maxStepsDefault = rt.Turn.MaxStepsDefault
			}
			if rt.Turn.MaxLLMFailures > 0 {
				cfg.maxLLMFailures = rt.Turn.MaxLLMFailures
			}
			if rt.Tools.MaxOutputChars > 0 {
				cfg.maxToolOutputChars = rt.Tools.MaxOutputChars
			}
			cfg.compactionEnabled = rt.Compaction.Enabled
			cfg.compactionMaxTokens = rt.Compaction.MaxTokens
			cfg.compactionTriggerRatio = rt.Compaction.TriggerRatio
			if rt.Compaction.CutTokens > 0 {
				cfg.compactionCutTokens = rt.Compaction.CutTokens
			}
			if rt.Compaction.ToolTruncate > 0 {
				cfg.toolTruncateChars = rt.Compaction.ToolTruncate
			}
			if rt.Compaction.KeepRecentToolSteps > 0 {
				cfg.keepRecentToolSteps = rt.Compaction.KeepRecentToolSteps
			}
		}
	}
	return cfg
}

func (p *TurnRunner) Run(ctx context.Context, tctx TurnContext) (domain.Report, []Message, error) {
	cfg := p.loadRunCfg(ctx)

	if p.ClearTurnDomains != nil && tctx.TurnID != "" {
		defer p.ClearTurnDomains(tctx.TurnID)
	}

	p.FileTracker = tool.NewFileTracker(tctx.WorkDir)

	if tctx.MaxSteps <= 0 {
		tctx.MaxSteps = cfg.maxStepsDefault
	}

	messages := tctx.Messages

	// Call-time user tail (turn-context + todos/file-changes/kb): appended only
	// for LLM calls, NOT persisted (KV cache: static system + history stay a
	// stable prefix across steps and uncompressed turns).

	tools := p.Registry.Schemas()
	skillTools := skillToolSchemas(p.SkillList, p.ToolBindings)
	if tctx.PlanMode {
		// In plan mode only built-in read-only tools are exposed; skill tools
		// and any non-allowed registry schemas are removed.
		tools = filterSchemasByAllowed(tools, domain.PlanModeAllowedToolIDs)
		skillTools = filterSchemasByAllowed(skillTools, domain.PlanModeAllowedToolIDs)
	}
	if len(skillTools) > 0 {
		tools = mergeSchemas(tools, skillTools)
		for _, sk := range p.SkillList {
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventCapabilityActive, domain.CapabilityActivatedPayload{
				Name: sk.Name,
				Kind: "skill",
			})
		}
	}

	var finalReport domain.Report
	reportCaptured := false
	maxPromptTokens := 0 // track actual max prompt tokens from LLM API
	consecutiveLLMFailures := 0

	for step := 1; step <= tctx.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			// Return context error so afterTurn can distinguish cancel from normal completion.
			// Do NOT publish EventTurnFailed here — afterTurn handles it.
			return domain.Report{}, messages, ctx.Err()
		default:
		}

		if cfg.compactionEnabled && step > 1 {
			messages = p.compactMessages(messages, cfg)
		}

		isLastStep := step == tctx.MaxSteps

		if isLastStep {
			messages = append(messages, Message{Role: RoleUser, Content: maxStepsPrompt})
			p.logUserMessage(maxStepsPrompt)
		}

		p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepStarted, domain.StepPayload{Step: step})
		llmReq := port.LLMChatRequest{
			Model:    tctx.Model,
			Messages: appendCallTimeContext(toPortMessages(messages), tctx.WorkDir, tctx.Model, tctx.EphemeralContext),
			Tools:    tools,
		}
		if isLastStep {
			llmReq.ToolChoice = "none"
		}
		resp, err := p.LLM.Chat(ctx, llmReq)
		if err != nil {
			consecutiveLLMFailures++
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventError, domain.ErrorPayload{Message: err.Error(), Kind: "llm"})
			if consecutiveLLMFailures >= cfg.maxLLMFailures {
				finalReport = domain.Report{
					Status:          domain.ReportFailed,
					Summary:         fmt.Sprintf("LLM call failed %d times in a row: %s", consecutiveLLMFailures, err.Error()),
					Confidence:      0.2,
					StepsUsed:       step,
					MaxPromptTokens: maxPromptTokens,
				}
				reportCaptured = true
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
				break
			}
			// Transient-ish failures: feed error back and retry within the failure budget.
			retryMsg := "[System: LLM call failed — " + err.Error() + ". Please retry or respond in text.]"
			messages = append(messages, Message{Role: RoleUser, Content: retryMsg})
			p.logUserMessage(retryMsg)
			continue
		}
		consecutiveLLMFailures = 0
		if resp.Usage != nil {
			if resp.Usage.PromptTokens > maxPromptTokens {
				maxPromptTokens = resp.Usage.PromptTokens
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventLLMUsage, domain.LLMUsagePayload{
				PromptTokens:        resp.Usage.PromptTokens,
				CompletionTokens:    resp.Usage.CompletionTokens,
				TotalTokens:         resp.Usage.TotalTokens,
				CacheReadTokens:     resp.Usage.CacheReadTokens,
				CacheCreationTokens: resp.Usage.CacheCreationTokens,
				Model:               tctx.Model,
				AgentID:             tctx.Agent.ID,
			})
		}

		if len(resp.ToolCalls) == 0 {
			if resp.ReasoningContent != "" {
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentThinking, domain.AgentThinkingPayload{Text: resp.ReasoningContent})
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentMessage, domain.AgentMessagePayload{Text: resp.Content})
			messages = append(messages, Message{Role: RoleAssistant, Content: resp.Content})
			p.logAssistantMessage(Message{Role: RoleAssistant, Content: resp.Content})
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})

			// Model wanted to stop — scan durable steers so a late guidance
			// message can keep the same turn alive.
			before := len(messages)
			messages = p.applySoftSteers(ctx, tctx, messages)
			if len(messages) > before && !isLastStep {
				continue
			}

			finalReport = domain.Report{
				Status: domain.ReportDone, Summary: resp.Content,
				Confidence: 0.8, StepsUsed: step,
				MaxPromptTokens: maxPromptTokens,
			}
			if tctx.OnReport != nil {
				tctx.OnReport(finalReport)
			}
			reportCaptured = true
			break
		}

		if resp.ReasoningContent != "" {
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentThinking, domain.AgentThinkingPayload{Text: resp.ReasoningContent})
		}

		// IMPORTANT: Append the assistant message with tool_calls BEFORE any
		// tool result messages. OpenAI-compatible APIs require the message
		// order: assistant(tool_calls) → tool(result) → tool(result) → ...
		assistantToolCalls := make([]ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			assistantToolCalls[i] = ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
		}
		assistantMsg := Message{
			Role:             RoleAssistant,
			ToolCalls:        assistantToolCalls,
			ReasoningContent: resp.ReasoningContent,
		}
		messages = append(messages, assistantMsg)
		p.logAssistantMessage(assistantMsg)

		// Parallelism is Execute-only. Gate/permission stay serial; stream
		// status + messages are committed in call order after all Executes finish.
		batchMsgs, doomSummary, err := p.runToolCallBatch(ctx, tctx, cfg, resp.ToolCalls, assistantToolCalls)
		messages = append(messages, batchMsgs...)
		if err != nil {
			return domain.Report{}, messages, err
		}
		if doomSummary != "" {
			finalReport = domain.Report{
				Status: domain.ReportFailed, Summary: doomSummary, MaxPromptTokens: maxPromptTokens,
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventTurnFailed, domain.TurnEndedPayload{
				TurnID: tctx.TurnID, Status: string(domain.TurnFailed), Summary: doomSummary,
			})
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventError, domain.ErrorPayload{
				Message: doomSummary, Kind: "doom_loop",
			})
			reportCaptured = true
			break
		}

		// Safe soft-steer boundary: parallel tools finished → scan durable
		// steering messages before the next LLM call.
		messages = p.applySoftSteers(ctx, tctx, messages)
		p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
	}

	if !reportCaptured {
		finalReport = domain.Report{Status: domain.ReportFailed, Summary: "max steps reached", Confidence: 0.3, MaxPromptTokens: maxPromptTokens}
	}

	// Publish the full report including Summary and MaxPromptTokens.
	// Multiple consumers (CLI, frontend, tests) read the summary from this event;
	// stripping it left them with an empty report. EventAgentMessage carries the
	// streamed text for UI display, but EventReport is the structured final report.
	p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventReport, finalReport)
	p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventTurnEnded, domain.TurnEndedPayload{
		TurnID: tctx.TurnID, Status: string(finalReport.Status), Summary: finalReport.Summary,
	})
	return finalReport, messages, nil
}

type toolCallSlot struct {
	call     port.ChatToolCall
	handler  tool.Handler
	describe string
	args     map[string]any
	exec     bool // passed gate; needs Execute
	content  string
	errLabel string
	result   domain.ToolResult
	done     bool
	doom     bool
}

// interactiveToolNames must finish before any parallel Execute. They block on
// humans; running them alongside side-effect tools races the user's answer.
var interactiveToolNames = map[string]struct{}{
	"ask_user": {},
}

func isInteractiveTool(name string) bool {
	_, ok := interactiveToolNames[name]
	return ok
}

// runToolCallBatch implements the LLM parallel tool-call contract:
//  1. serial gate (doom / unknown / permission / approval)
//  2. serial start + Execute for interactive tools (ask_user)
//  3. serial start for remaining tools, then parallel Execute
//  4. serial completed|error + messages + Turn Log in call order
func (p *TurnRunner) runToolCallBatch(
	ctx context.Context,
	tctx TurnContext,
	cfg turnRunCfg,
	calls []port.ChatToolCall,
	assistantToolCalls []ToolCall,
) (msgs []Message, doomSummary string, err error) {
	slots := make([]toolCallSlot, len(calls))

	// --- 1) serial gate (includes permission approval waits) ---
	for i, call := range calls {
		select {
		case <-ctx.Done():
			msgs = p.closeUnfinishedToolCalls(tctx, msgs, assistantToolCalls)
			return msgs, "", ctx.Err()
		default:
		}

		slots[i].call = call
		handler, ok := p.Registry.Get(call.Name)
		describe := call.Name
		if ok {
			describe = handler.Describe(call.Arguments)
			slots[i].handler = handler
		}
		slots[i].describe = describe

		if p.trackDoom(tctx.TurnID, call.Name, describe, cfg.doomLoopThreshold) >= cfg.doomLoopThreshold {
			slots[i].content = "doom loop detected"
			slots[i].done = true
			slots[i].doom = true
			doomSummary = "doom loop for " + call.Name
			for j := i + 1; j < len(calls); j++ {
				slots[j].call = calls[j]
				slots[j].describe = calls[j].Name
				if h, ok := p.Registry.Get(calls[j].Name); ok {
					slots[j].describe = h.Describe(calls[j].Arguments)
				}
				slots[j].content = "cancelled"
				slots[j].errLabel = "cancelled"
				slots[j].done = true
			}
			break
		}
		if !ok {
			slots[i].content = "unknown tool: " + call.Name + toolErrorHint
			slots[i].done = true
			continue
		}

		args, content, errLabel, exec, gateErr := p.gateToolCall(ctx, tctx, cfg, call, handler, describe)
		if gateErr != nil {
			slots[i].content = "cancelled"
			slots[i].errLabel = "cancelled"
			slots[i].done = true
			msgs = p.commitToolResults(ctx, tctx, slots[:i+1])
			msgs = p.closeUnfinishedToolCalls(tctx, msgs, assistantToolCalls)
			return msgs, "", gateErr
		}
		if !exec {
			slots[i].content = content
			slots[i].errLabel = errLabel
			slots[i].done = true
			continue
		}
		slots[i].args = args
		slots[i].exec = true
	}

	// --- 2) interactive tools: serial start + Execute (ask_user 前置) ---
	for i := range slots {
		slot := &slots[i]
		if !slot.exec || !isInteractiveTool(slot.call.Name) {
			continue
		}
		p.publishToolStart(ctx, tctx, slot)
		if err := p.executeToolSlot(ctx, cfg, slot); err != nil {
			msgs = p.commitToolResults(ctx, tctx, slots)
			msgs = p.closeUnfinishedToolCalls(tctx, msgs, assistantToolCalls)
			return msgs, "", err
		}
	}

	// --- 3) non-interactive: serial start, then parallel Execute ---
	for i := range slots {
		slot := &slots[i]
		if !slot.exec || slot.done || isInteractiveTool(slot.call.Name) {
			continue
		}
		p.publishToolStart(ctx, tctx, slot)
	}
	var wg sync.WaitGroup
	for i := range slots {
		slot := &slots[i]
		if !slot.exec || slot.done || isInteractiveTool(slot.call.Name) {
			continue
		}
		wg.Add(1)
		go func(slot *toolCallSlot) {
			defer wg.Done()
			_ = p.executeToolSlot(ctx, cfg, slot)
		}(&slots[i])
	}
	wg.Wait()

	// --- 4) serial result commit: completed|error + messages + Turn Log ---
	msgs = p.commitToolResults(ctx, tctx, slots)
	if ctx.Err() != nil {
		msgs = p.closeUnfinishedToolCalls(tctx, msgs, assistantToolCalls)
		return msgs, "", ctx.Err()
	}
	return msgs, doomSummary, nil
}

func (p *TurnRunner) executeToolSlot(ctx context.Context, cfg turnRunCfg, slot *toolCallSlot) (err error) {
	// A panicking tool handler must degrade to a normal tool failure instead of
	// killing the process — parallel Execute goroutines would otherwise crash
	// the whole server and force the RecoverRunning restart path.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[tool] %s panicked: %v\n%s", slot.call.Name, r, debug.Stack())
			slot.content = limitToolOutput(fmt.Sprintf("tool panicked: %v", r)+toolErrorHint, cfg.maxToolOutputChars)
			slot.errLabel = fmt.Sprintf("panic: %v", r)
			slot.done = true
			err = nil
		}
	}()
	result, execErr := slot.handler.Execute(ctx, slot.args)
	if execErr != nil {
		errContent := limitToolOutput(execErr.Error()+toolErrorHint, cfg.maxToolOutputChars)
		errLabel := execErr.Error()
		if errors.Is(execErr, context.Canceled) || ctx.Err() != nil {
			errLabel = "cancelled"
			errContent = "cancelled" + toolErrorHint
			slot.content = errContent
			slot.errLabel = errLabel
			slot.done = true
			return ctx.Err()
		}
		slot.content = errContent
		slot.errLabel = errLabel
		slot.done = true
		return nil
	}
	result.Content = limitToolOutput(result.Content, cfg.maxToolOutputChars)
	slot.result = result
	slot.content = result.Content
	slot.done = true
	return nil
}

func (p *TurnRunner) publishToolStart(ctx context.Context, tctx TurnContext, slot *toolCallSlot) {
	p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolPending, domain.ToolPart{
		CallID: slot.call.ID, Name: slot.call.Name, Description: slot.describe,
		Status: domain.ToolPending, Input: slot.call.Arguments,
	})
	p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolRunning, domain.ToolPart{
		CallID: slot.call.ID, Name: slot.call.Name, Description: slot.describe,
		Status: domain.ToolRunning, Input: slot.call.Arguments,
	})
}

// commitToolResults publishes completed|error and appends tool messages / Turn Log
// in call order. Start events must already have been published serially.
func (p *TurnRunner) commitToolResults(ctx context.Context, tctx TurnContext, slots []toolCallSlot) []Message {
	var msgs []Message
	for i := range slots {
		slot := &slots[i]
		if !slot.done {
			continue
		}
		if slot.errLabel != "" {
			pubCtx := ctx
			if slot.errLabel == "cancelled" || (ctx != nil && ctx.Err() != nil) {
				pubCtx = context.Background()
			}
			p.Stream.Publish(pubCtx, tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
				CallID: slot.call.ID, Name: slot.call.Name, Description: slot.describe,
				Status: domain.ToolError, Error: slot.errLabel,
			})
		} else if slot.exec && !slot.doom {
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolCompleted, domain.ToolPart{
				CallID: slot.call.ID, Name: slot.call.Name, Description: slot.describe,
				Status: domain.ToolCompleted, Output: slot.content,
			})
			p.recordFileChanges(tctx, slot.call.ID, slot.call.Name, slot.call.Arguments, slot.result)
		}
		msgs = append(msgs, Message{
			Role:       RoleTool,
			ToolCallID: slot.call.ID,
			Name:       slot.call.Name,
			Content:    slot.content,
			Parts:      toolResultParts(slot.result),
		})
		p.logToolResult(slot.call.ID, slot.call.Name, slot.content)
	}
	return msgs
}

// toolResultParts converts tool-result multimodal parts into message parts.
func toolResultParts(result domain.ToolResult) []ContentPart {
	if len(result.Parts) == 0 {
		return nil
	}
	parts := make([]ContentPart, 0, len(result.Parts))
	for _, p := range result.Parts {
		if p.Type != "image" || p.Data == "" {
			continue
		}
		parts = append(parts, ContentPart{
			Type:     p.Type,
			MimeType: p.MimeType,
			Data:     p.Data,
		})
	}
	return parts
}

// gateToolCall runs permission / approval for one call. On success with exec=true,
// args are ready for Execute. Soft denials return exec=false with tool content.
func (p *TurnRunner) gateToolCall(
	ctx context.Context,
	tctx TurnContext,
	cfg turnRunCfg,
	call port.ChatToolCall,
	handler tool.Handler,
	describe string,
) (args map[string]any, content, errLabel string, exec bool, err error) {
	cmdStr, _ := call.Arguments["command"].(string)
	urlStr, _ := call.Arguments["url"].(string)
	sbStatus := domain.SandboxStatus{}
	if p.SandboxStatus != nil {
		sbStatus = p.SandboxStatus()
	}
	isolation := domain.ComputeEffectiveIsolation(sbStatus, domain.EnvironmentStatus{})
	if p.EffectiveIsolation != nil {
		isolation = p.EffectiveIsolation()
	}
	allowNet := false
	if p.SessionAllowNetwork != nil {
		allowNet = p.SessionAllowNetwork(tctx.SessionID)
	}
	var sessionDomains []string
	if p.SessionAllowDomains != nil {
		sessionDomains = p.SessionAllowDomains(tctx.SessionID)
	}
	searchProvider, searchBaseURL := "", ""
	if call.Name == "web_search" && p.ConfigStore != nil {
		if c, err := p.ConfigStore.Load(ctx); err == nil {
			searchProvider = string(c.Search.Provider)
			searchBaseURL = c.Search.BaseURL
		}
	}
	risk := handler.RiskLevel()
	if call.Name == "http_request" {
		method, _ := call.Arguments["method"].(string)
		risk = permission.EffectiveHTTPRequestRisk(risk, method, permission.ParseHTTPHeadersFromArgs(call.Arguments))
	}
	if call.Name == "file_op" {
		action, _ := call.Arguments["action"].(string)
		risk = permission.EffectiveFileOpRisk(risk, action)
	}
	permResult := p.Perm.CheckRequest(permission.Request{
		ToolName:            call.Name,
		Risk:                risk,
		Command:             cmdStr,
		URL:                 urlStr,
		SearchProvider:      searchProvider,
		SearchBaseURL:       searchBaseURL,
		Sandbox:             sbStatus,
		Isolation:           isolation,
		SessionAllowNetwork: allowNet,
		SessionAllowDomains: sessionDomains,
		Mode:                cfg.permissionMode,
	})
	decision := permResult.Decision
	if decision == permission.DecisionDeny {
		return nil, "permission denied" + toolErrorHint, "permission denied", false, nil
	}

	allowNetworkForRun := allowNet
	grantedDomain := ""
	grantScope := "once"
	if decision == permission.DecisionAsk {
		canAuto := cfg.autoApprove && permission.AutoApprovable(permResult.Reason)
		if !canAuto && p.Approval != nil {
			approvalID := p.Approval.CreateApproval(tctx.SessionID, tctx.TurnID, call.Name, describe, permResult.Reason, permResult.Domain)
			scopeOpts := []string{"once"}
			// Full-network session grant only in deny mode (ReasonNetwork).
			// Domain grants use ReasonNetworkDomain with session scope.
			if permResult.Reason == permission.ReasonNetwork || permResult.Reason == permission.ReasonNetworkDomain {
				scopeOpts = append(scopeOpts, "session")
			}
			desc := describe
			if permResult.Reason == permission.ReasonNetworkDomain && permResult.Domain != "" {
				desc = "allow domain " + permResult.Domain + " — " + describe
			}
			if permResult.Reason == permission.ReasonNetwork {
				desc = "full outbound network (deny mode escape) — " + describe
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventPermissionAsk, domain.PermissionAskPayload{
				ApprovalID: approvalID, CallID: call.ID, Tool: call.Name, Description: desc,
				Reason: permResult.Reason, Domain: permResult.Domain, ScopeOptions: scopeOpts,
			})
			outcome, waitErr := p.Approval.WaitApproval(ctx, approvalID)
			if waitErr != nil {
				if errors.Is(waitErr, context.Canceled) || ctx.Err() != nil {
					return nil, "", "", false, ctx.Err()
				}
				return nil, "approval wait failed: " + waitErr.Error() + toolErrorHint, "approval wait failed", false, nil
			}
			if !outcome.Approved {
				return nil, "User rejected this tool call. Do not retry the same command; choose a safer alternative or ask the user." + toolErrorHint, "approval rejected", false, nil
			}
			if outcome.Scope != "" {
				grantScope = outcome.Scope
			}
		} else if !canAuto {
			return nil, "permission ask required but approval gate unavailable" + toolErrorHint, "approval unavailable", false, nil
		}
		switch permResult.Reason {
		case permission.ReasonNetwork:
			allowNetworkForRun = true
		case permission.ReasonNetworkDomain:
			grantedDomain = permResult.Domain
		}
	}

	args = cloneMap(call.Arguments)
	if args == nil {
		args = map[string]any{}
	}
	args["__session_id"] = tctx.SessionID
	args["__turn_id"] = tctx.TurnID
	args["__agent_id"] = tctx.Agent.ID
	args["__project_id"] = tctx.ProjectID
	args["__model_id"] = tctx.Model
	args["__work_dir"] = tctx.WorkDir
	args["__call_id"] = call.ID
	args["__file_tracker"] = p.FileTracker
	args["__turn_path"] = effectiveTurnPath(tctx)
	if allowNetworkForRun {
		args["__sandbox_allow_network"] = true
	}
	if grantedDomain != "" {
		if grantScope == "session" {
			if p.GrantSessionDomains != nil {
				p.GrantSessionDomains(tctx.SessionID, []string{grantedDomain})
			}
		} else if p.GrantTurnDomains != nil {
			p.GrantTurnDomains(tctx.TurnID, []string{grantedDomain})
		}
		args["__granted_domain"] = grantedDomain
	}
	return args, "", "", true, nil
}

// closeUnfinishedToolCalls appends cancelled tool results for any call in the
// batch that still lacks a tool message. All tools are treated the same.
func (p *TurnRunner) closeUnfinishedToolCalls(tctx TurnContext, messages []Message, calls []ToolCall) []Message {
	haveResult := make(map[string]bool)
	for _, m := range messages {
		if m.Role == RoleTool && m.ToolCallID != "" {
			haveResult[m.ToolCallID] = true
		}
	}
	for _, call := range calls {
		if haveResult[call.ID] {
			continue
		}
		describe := call.Name
		if p.Registry != nil {
			if h, ok := p.Registry.Get(call.Name); ok {
				describe = h.Describe(call.Arguments)
			}
		}
		p.Stream.Publish(context.Background(), tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
			CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: "cancelled",
		})
		messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "cancelled"})
		p.logToolResult(call.ID, call.Name, "cancelled")
	}
	return messages
}

func (p *TurnRunner) logUserMessage(content string) {
	if p.Log == nil {
		return
	}
	p.Log("user", map[string]any{"content": content})
}

func (p *TurnRunner) applySoftSteers(ctx context.Context, tctx TurnContext, messages []Message) []Message {
	if tctx.ClaimSteers == nil {
		return messages
	}
	steers := tctx.ClaimSteers()
	if len(steers) == 0 {
		return messages
	}
	for _, m := range steers {
		if m.Role == "" {
			m.Role = RoleUser
		}
		messages = append(messages, m)
		p.logUserMessage(m.Content)
		if p.Stream != nil {
			atts := make([]domain.UserAttachment, 0, len(m.Parts))
			for _, part := range m.Parts {
				if part.Type != "image" || part.Data == "" {
					continue
				}
				atts = append(atts, domain.UserAttachment{
					Type: "image", Name: part.Name, MimeType: part.MimeType, Data: part.Data,
				})
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventUserMessage, userMessagePayload(m.Content, atts))
		}
	}
	return messages
}

func (p *TurnRunner) logAssistantMessage(msg Message) {
	if p.Log == nil {
		return
	}
	data := map[string]any{}
	if msg.Content != "" {
		data["content"] = msg.Content
	}
	if len(msg.ToolCalls) > 0 {
		tcs := make([]map[string]any, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			tcs = append(tcs, map[string]any{
				"id":        tc.ID,
				"name":      tc.Name,
				"arguments": tc.Arguments,
			})
		}
		data["tool_calls"] = tcs
	}
	p.Log("assistant", data)
}

func (p *TurnRunner) logToolResult(callID, name, output string) {
	if p.Log == nil {
		return
	}
	p.Log("tool_result", map[string]any{"call_id": callID, "name": name, "output": output})
}

func (p *TurnRunner) recordFileChanges(tctx TurnContext, callID, toolName string, args map[string]any, result domain.ToolResult) {
	if p.FileChanges == nil || !isFileMutatingTool(toolName) {
		return
	}
	for _, rec := range fileChangeRecordsFromResult(tctx.TurnID, callID, toolName, args, result) {
		if _, err := p.FileChanges.Append(tctx.SessionID, tctx.ProjectID, rec); err != nil {
			// Best-effort: do not fail the turn if the journal write fails.
			continue
		}
	}
}

func (p *TurnRunner) trackDoom(turnID, tool, describe string, threshold int) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.doomState[turnID] == nil {
		p.doomState[turnID] = &doomTurnState{}
	}
	st := p.doomState[turnID]
	if len(describe) > doomDescribeMaxLen {
		describe = describe[:doomDescribeMaxLen]
	}
	key := tool + "\x00" + describe

	// Consecutive identical signatures only; any different tool resets the streak.
	if key == st.lastKey {
		st.streak++
	} else {
		st.lastKey = key
		st.streak = 1
	}

	st.patterns = append(st.patterns, key)
	if len(st.patterns) > doomPatternWindow {
		st.patterns = st.patterns[1:]
	}

	if detectAlternatingLoop(st.patterns, threshold) {
		return threshold
	}
	return st.streak
}

// detectAlternatingLoop catches A-B-A-B… streaks from the end (consecutive ping-pong).
func detectAlternatingLoop(patterns []string, threshold int) bool {
	if threshold < 2 || len(patterns) < threshold*2 {
		return false
	}
	a, b := patterns[len(patterns)-1], patterns[len(patterns)-2]
	if a == b {
		return false
	}
	altCount := 0
	for i := len(patterns) - 1; i >= 0; i-- {
		expect := a
		if (len(patterns)-1-i)%2 == 1 {
			expect = b
		}
		if patterns[i] != expect {
			break
		}
		altCount++
	}
	need := threshold * 2
	if need < 8 {
		need = 8 // at least 4 A-B pairs; avoids false positives like todowrite↔write
	}
	return altCount >= need
}

func (p *TurnRunner) compactMessages(messages []Message, cfg turnRunCfg) []Message {
	if len(messages) <= 1 {
		return messages
	}
	messages = p.dedupToolResults(messages)
	messages = truncateToolResults(messages, cfg.toolTruncateChars, cfg.keepRecentToolSteps)
	messages = p.enforceToolPairing(messages)
	high := highWaterTokens(cfg)
	if high > 0 && estimateTurnTokens(messages) > high {
		messages = p.snipHead(messages, lowWaterTokens(cfg))
	}
	return messages
}

func highWaterTokens(cfg turnRunCfg) int {
	if cfg.compactionMaxTokens > 0 {
		return int(float64(cfg.compactionMaxTokens) * cfg.compactionTriggerRatio)
	}
	return 0
}

func lowWaterTokens(cfg turnRunCfg) int {
	high := highWaterTokens(cfg)
	if high <= 0 {
		return 0
	}
	if cfg.compactionCutTokens <= 0 || cfg.compactionCutTokens >= high {
		return high
	}
	return cfg.compactionCutTokens
}

func (p *TurnRunner) dedupToolResults(messages []Message) []Message {
	keyToIDs := make(map[string][]string)
	for _, m := range messages {
		if m.Role == RoleAssistant {
			for _, tc := range m.ToolCalls {
				key := tc.Name + "|" + ToolInputKey(tc.Arguments)
				keyToIDs[key] = append(keyToIDs[key], tc.ID)
			}
		}
	}

	dupIDs := make(map[string]bool)
	for _, ids := range keyToIDs {
		if len(ids) <= 1 {
			continue
		}
		for i := 0; i < len(ids)-1; i++ {
			dupIDs[ids[i]] = true
		}
	}

	if len(dupIDs) == 0 {
		return messages
	}

	result := make([]Message, len(messages))
	copy(result, messages)
	for i := range result {
		if result[i].Role == RoleTool && dupIDs[result[i].ToolCallID] {
			result[i].Content = fmt.Sprintf("[dedup] %s: 重复调用，同输入，参见最新结果", result[i].Name)
		}
	}
	return result
}

// recentToolCallIDs returns tool_call IDs belonging to the last keepSteps
// assistant messages that issued tools (one LLM step each).
func recentToolCallIDs(messages []Message, keepSteps int) map[string]struct{} {
	if keepSteps <= 0 {
		return nil
	}
	protected := make(map[string]struct{})
	found := 0
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != RoleAssistant || len(m.ToolCalls) == 0 {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.ID != "" {
				protected[tc.ID] = struct{}{}
			}
		}
		found++
		if found >= keepSteps {
			break
		}
	}
	return protected
}

// truncateToolResults caps older tool results; the latest keepRecentSteps LLM
// tool-call batches keep full content (still subject to max_output_chars at execute).
func truncateToolResults(messages []Message, maxChars, keepRecentSteps int) []Message {
	if maxChars <= 0 {
		maxChars = turnToolTextMaxChars
	}
	protected := recentToolCallIDs(messages, keepRecentSteps)
	result := make([]Message, len(messages))
	copy(result, messages)
	for i := range result {
		if result[i].Role != RoleTool {
			continue
		}
		if _, ok := protected[result[i].ToolCallID]; ok {
			continue
		}
		content := result[i].Content
		limit := maxChars
		if isHugeResult(content) {
			limit = turnHugeResultThreshold
		}
		result[i].Content = limitToolOutput(content, limit)
	}
	return result
}

func isHugeResult(content string) bool {
	return len(content) > turnHugeResultThreshold
}

// limitToolOutput hard-caps tool result text at maxChars bytes, backing up to
// a valid UTF-8 boundary so truncated content stays well-formed.
func limitToolOutput(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	cut := maxChars
	for cut > 0 && !utf8.ValidString(content[:cut]) {
		cut--
	}
	if cut <= 0 {
		cut = maxChars
	}
	return content[:cut] + fmt.Sprintf("\n...[truncated, %d total chars]", len(content))
}

func (p *TurnRunner) enforceToolPairing(messages []Message) []Message {
	return keepCompleteToolPairs(messages)
}

// salvagePairedTurnDelta keeps this-turn messages that form complete tool pairs.
// Used when a turn is cancelled/failed so the next turn still sees finished work
// (e.g. read_file/glob results) without unpaired assistant tool_calls.
func salvagePairedTurnDelta(delta []Message) []Message {
	return keepCompleteToolPairs(delta)
}

// keepCompleteToolPairs returns an API-safe message sequence. Each assistant
// tool_call is kept only when its result exists, and results are re-emitted
// immediately after their assistant (repairing interleaved/corrupt history).
// Text content survives even if all tool calls are removed.
func keepCompleteToolPairs(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}
	results := make(map[string]Message, len(messages))
	for _, m := range messages {
		if m.Role == RoleTool && m.ToolCallID != "" {
			if _, ok := results[m.ToolCallID]; !ok {
				results[m.ToolCallID] = m
			}
		}
	}
	used := make(map[string]bool, len(results))
	out := make([]Message, 0, len(messages))
	for _, m := range messages {
		switch {
		case m.Role == RoleAssistant && len(m.ToolCalls) > 0:
			kept := make([]ToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if _, ok := results[tc.ID]; ok && !used[tc.ID] {
					kept = append(kept, tc)
				}
			}
			if len(kept) == 0 {
				if m.Content != "" {
					cp := m
					cp.ToolCalls = nil
					out = append(out, cp)
				}
				continue
			}
			cp := m
			cp.ToolCalls = kept
			out = append(out, cp)
			for _, tc := range kept {
				out = append(out, results[tc.ID])
				used[tc.ID] = true
			}
		case m.Role == RoleTool:
			continue
		default:
			out = append(out, m)
		}
	}
	return out
}

func (p *TurnRunner) snipHead(messages []Message, budget int) []Message {
	systemCount := 0
	for _, m := range messages {
		if m.Role == RoleSystem {
			systemCount++
		} else {
			break
		}
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	// Protect the last user message — it is the current turn's goal.
	// Removing it would make the turn meaningless.
	lastUserIdx := -1
	for i := len(result) - 1; i >= systemCount; i-- {
		if result[i].Role == RoleUser {
			lastUserIdx = i
			break
		}
	}

	i := systemCount
	for i < len(result) {
		cur := estimateTurnTokens(result)
		if cur <= budget {
			break
		}

		// Stop if the next message to remove is the protected last user message
		// or beyond it (everything after the user message is the current turn's work).
		if lastUserIdx >= 0 && i >= lastUserIdx {
			break
		}

		m := result[i]
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			ids := make(map[string]bool)
			for _, tc := range m.ToolCalls {
				ids[tc.ID] = true
			}
			result = removeAt(result, i)
			if lastUserIdx > i {
				lastUserIdx--
			}
			for j := i; j < len(result); {
				rj := result[j]
				if rj.Role == RoleTool && ids[rj.ToolCallID] {
					result = removeAt(result, j)
					if lastUserIdx > j {
						lastUserIdx--
					}
				} else {
					j++
				}
			}
		} else {
			result = removeAt(result, i)
			if lastUserIdx > i {
				lastUserIdx--
			}
		}
	}
	return result
}

func removeAt(msgs []Message, idx int) []Message {
	return append(msgs[:idx], msgs[idx+1:]...)
}

func estimateTurnTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += turnEstimateMessageTokens(m)
	}
	return total
}

func turnEstimateMessageTokens(m Message) int {
	n := 0
	n += len(m.Role) / turnTokenEstimateDivisor
	n += len(m.Content) / turnTokenEstimateDivisor
	n += len(m.Name) / turnTokenEstimateDivisor
	n += len(m.ToolCallID) / turnTokenEstimateDivisor
	for _, p := range m.Parts {
		if p.Type == "image" && p.Data != "" {
			n += imagePartTokenEstimate
		}
	}
	for _, tc := range m.ToolCalls {
		n += len(tc.ID) / turnTokenEstimateDivisor
		n += len(tc.Name) / turnTokenEstimateDivisor
		raw, _ := json.Marshal(tc.Arguments)
		n += len(raw) / turnTokenEstimateDivisor
	}
	return n
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// effectiveTurnPath returns the delegation path for tool injection. When Path
// was omitted (e.g. tests), synthesize the current frame from TurnID/Agent.
func effectiveTurnPath(tctx TurnContext) []domain.TurnPathEntry {
	if len(tctx.Path) > 0 {
		return tctx.Path
	}
	if tctx.TurnID == "" {
		return nil
	}
	return []domain.TurnPathEntry{{TurnID: tctx.TurnID, AgentID: tctx.Agent.ID}}
}

func mergeSchemas(base, extra []domain.ToolSchema) []domain.ToolSchema {
	seen := map[string]struct{}{}
	var out []domain.ToolSchema
	for _, s := range base {
		if _, ok := seen[s.Name]; !ok {
			seen[s.Name] = struct{}{}
			out = append(out, s)
		}
	}
	for _, s := range extra {
		if _, ok := seen[s.Name]; !ok {
			seen[s.Name] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

func filterSchemasByAllowed(schemas []domain.ToolSchema, allowed map[string]struct{}) []domain.ToolSchema {
	out := make([]domain.ToolSchema, 0, len(schemas))
	for _, s := range schemas {
		if _, ok := allowed[s.Name]; ok {
			out = append(out, s)
		}
	}
	return out
}

func formatTurnContext(workDir, model string) string {
	now := time.Now()
	return "<turn-context>\n" +
		"Current time: " + now.Format("2006-01-02T15:04:05Z07:00") + " (" + now.Weekday().String() + ")\n" +
		"Working directory: " + workDir + "\n" +
		"Model: " + model + "\n" +
		"</turn-context>"
}

// buildTurnContextMessage creates a user message with dynamic per-call context.
// NOT persisted in messages — only appended temporarily for LLM calls.
func buildTurnContextMessage(workDir, model string) Message {
	return Message{Role: RoleUser, Content: formatTurnContext(workDir, model)}
}

// appendCallTimeContext appends turn-context plus optional ephemeral blocks as a
// trailing user message. Temporary — the original messages slice is not modified.
func appendCallTimeContext(msgs []port.ChatMessage, workDir, model, extra string) []port.ChatMessage {
	content := formatTurnContext(workDir, model)
	if extra = strings.TrimSpace(extra); extra != "" {
		content += "\n\n" + extra
	}
	out := make([]port.ChatMessage, len(msgs)+1)
	copy(out, msgs)
	out[len(msgs)] = port.ChatMessage{Role: "user", Content: content}
	return out
}
