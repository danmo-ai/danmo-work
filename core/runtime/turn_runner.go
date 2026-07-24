package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/runtime/permission"
	"danmo-work/core/runtime/tool"
)

const (
	turnToolTextMaxChars     = 2000
	turnHugeResultThreshold  = 60000
	turnTokenEstimateDivisor = 4
	doomPatternWindow        = 8
	doomDescribeMaxLen       = 200
	toolErrorHint            = "\n[Analyze the error above and try a different approach.]"
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
	Role       Role          `json:"role"`
	Content    string        `json:"content,omitempty"`
	Parts      []ContentPart `json:"parts,omitempty"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
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
	OnReport  func(domain.Report)
	Messages  []Message
}

type approvalGate interface {
	WaitApproval(ctx context.Context, approvalID string) (ApprovalOutcome, error)
	CreateApproval(sessionID, turnID, toolName, description, reason string) string
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
	SessionAllowNetwork func(sessionID string) bool
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
	doomLoopThreshold      int
	maxStepsDefault        int
	maxLLMFailures         int
	compactionEnabled      bool
	compactionMaxTokens    int
	compactionTriggerRatio float64
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
		compactionMaxTokens:    128000,
		compactionTriggerRatio: 0.85,
	}
	if p.ConfigStore != nil {
		if c, err := p.ConfigStore.Load(ctx); err == nil {
			rt := c.Runtime
			cfg.autoApprove = rt.AutoApprove
			if rt.Turn.DoomLoopThreshold > 0 {
				cfg.doomLoopThreshold = rt.Turn.DoomLoopThreshold
			}
			if rt.Turn.MaxStepsDefault > 0 {
				cfg.maxStepsDefault = rt.Turn.MaxStepsDefault
			}
			if rt.Turn.MaxLLMFailures > 0 {
				cfg.maxLLMFailures = rt.Turn.MaxLLMFailures
			}
			cfg.compactionEnabled = rt.Compaction.Enabled
			cfg.compactionMaxTokens = rt.Compaction.MaxTokens
			cfg.compactionTriggerRatio = rt.Compaction.TriggerRatio
		}
	}
	return cfg
}

func (p *TurnRunner) Run(ctx context.Context, tctx TurnContext) (domain.Report, []Message, error) {
	cfg := p.loadRunCfg(ctx)

	p.FileTracker = tool.NewFileTracker(tctx.WorkDir)

	if tctx.MaxSteps <= 0 {
		tctx.MaxSteps = cfg.maxStepsDefault
	}

	messages := tctx.Messages

	// Turn context: appended only for LLM calls, NOT persisted in messages
	// (KV cache friendly: static system prompt prefix stays identical across turns).
	turnCtxMsg := buildTurnContextMessage(tctx.WorkDir, tctx.Model)

	tools := p.Registry.Schemas()
	skillTools := skillToolSchemas(p.SkillList, p.ToolBindings)
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
			Messages: appendTurnContext(toPortMessages(messages), turnCtxMsg),
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
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			})
		}

		if len(resp.ToolCalls) == 0 {
			finalReport = domain.Report{
				Status: domain.ReportDone, Summary: resp.Content,
				Confidence: 0.8, StepsUsed: step,
				MaxPromptTokens: maxPromptTokens,
			}
			if tctx.OnReport != nil {
				tctx.OnReport(finalReport)
			}
			if resp.ReasoningContent != "" {
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentThinking, domain.AgentThinkingPayload{Text: resp.ReasoningContent})
			}
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentMessage, domain.AgentMessagePayload{Text: resp.Content})
			messages = append(messages, Message{Role: RoleAssistant, Content: resp.Content})
			p.logAssistantMessage(Message{Role: RoleAssistant, Content: resp.Content})
			reportCaptured = true
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
			break
		}

		if resp.ReasoningContent != "" {
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventAgentThinking, domain.AgentThinkingPayload{Text: resp.ReasoningContent})
		}

		processedCalls := make([]ToolCall, 0, len(resp.ToolCalls))

		// IMPORTANT: Append the assistant message with tool_calls BEFORE any
		// tool result messages. OpenAI-compatible APIs require the message
		// order: assistant(tool_calls) → tool(result) → tool(result) → ...
		assistantToolCalls := make([]ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			assistantToolCalls[i] = ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
		}
		assistantMsg := Message{Role: RoleAssistant, ToolCalls: assistantToolCalls}
		messages = append(messages, assistantMsg)
		p.logAssistantMessage(assistantMsg)

		for _, call := range resp.ToolCalls {
			select {
			case <-ctx.Done():
				messages = p.closeUnfinishedToolCalls(tctx, messages, assistantToolCalls)
				return domain.Report{}, messages, ctx.Err()
			default:
			}

			handler, ok := p.Registry.Get(call.Name)
			describe := call.Name
			if ok {
				describe = handler.Describe(call.Arguments)
			}

			if p.trackDoom(tctx.TurnID, call.Name, describe, cfg.doomLoopThreshold) >= cfg.doomLoopThreshold {
				finalReport = domain.Report{Status: domain.ReportFailed, Summary: "doom loop for " + call.Name, MaxPromptTokens: maxPromptTokens}
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventTurnFailed, domain.TurnEndedPayload{
					TurnID: tctx.TurnID, Status: "failed", Summary: "doom loop for " + call.Name,
				})
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventError, domain.ErrorPayload{
					Message: "doom loop for " + call.Name, Kind: "doom_loop",
				})
				// Synthetic tool result to maintain assistant(tool_calls) ↔ tool pairing.
				messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "doom loop detected"})
				p.logToolResult(call.ID, call.Name, "doom loop detected")
				reportCaptured = true
				break
			}

			if !ok {
				messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "unknown tool: " + call.Name + toolErrorHint})
				p.logToolResult(call.ID, call.Name, "unknown tool: "+call.Name+toolErrorHint)
				processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
				continue
			}

			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolPending, domain.ToolPart{
				CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolPending, Input: call.Arguments,
			})

			cmdStr, _ := call.Arguments["command"].(string)
			sbStatus := domain.SandboxStatus{}
			if p.SandboxStatus != nil {
				sbStatus = p.SandboxStatus()
			}
			allowNet := false
			if p.SessionAllowNetwork != nil {
				allowNet = p.SessionAllowNetwork(tctx.SessionID)
			}
			risk := handler.RiskLevel()
			if call.Name == "http_request" {
				method, _ := call.Arguments["method"].(string)
				risk = permission.EffectiveHTTPRequestRisk(risk, method, permission.ParseHTTPHeadersFromArgs(call.Arguments))
			}
			permResult := p.Perm.CheckRequest(permission.Request{
				ToolName:            call.Name,
				Risk:                risk,
				Command:             cmdStr,
				Sandbox:             sbStatus,
				SessionAllowNetwork: allowNet,
			})
			decision := permResult.Decision
			if decision == permission.DecisionDeny {
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
					CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: "permission denied",
				})
				messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "permission denied" + toolErrorHint})
				p.logToolResult(call.ID, call.Name, "permission denied"+toolErrorHint)
				processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
				continue
			}

			allowNetworkForRun := allowNet
			if decision == permission.DecisionAsk && !cfg.autoApprove && p.Approval != nil {
				description := describe
				approvalID := p.Approval.CreateApproval(tctx.SessionID, tctx.TurnID, call.Name, description, permResult.Reason)
				scopeOpts := []string{"once"}
				if permResult.Reason == permission.ReasonNetwork {
					scopeOpts = append(scopeOpts, "session")
				}
				p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventPermissionAsk, domain.PermissionAskPayload{
					ApprovalID: approvalID, CallID: call.ID, Tool: call.Name, Description: description,
					Reason: permResult.Reason, ScopeOptions: scopeOpts,
				})
				outcome, err := p.Approval.WaitApproval(ctx, approvalID)
				if err != nil {
					// Turn cancel during approval is a hard stop (not a soft deny).
					if errors.Is(err, context.Canceled) || ctx.Err() != nil {
						p.Stream.Publish(context.Background(), tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
							CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: "cancelled",
						})
						messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "cancelled"})
						p.logToolResult(call.ID, call.Name, "cancelled")
						messages = p.closeUnfinishedToolCalls(tctx, messages, assistantToolCalls)
						return domain.Report{}, messages, ctx.Err()
					}
					msg := "approval wait failed: " + err.Error() + toolErrorHint
					p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
						CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: "approval wait failed",
					})
					messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: msg})
					p.logToolResult(call.ID, call.Name, msg)
					processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
					continue
				}
				if !outcome.Approved {
					// Soft deny: return a tool error and let the agent continue
					// (mainstream UX — rejection must not kill the whole turn).
					msg := "User rejected this tool call. Do not retry the same command; choose a safer alternative or ask the user." + toolErrorHint
					p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
						CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: "approval rejected",
					})
					messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: msg})
					p.logToolResult(call.ID, call.Name, msg)
					processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
					continue
				}
				if permResult.Reason == permission.ReasonNetwork {
					allowNetworkForRun = true
				}
			} else if decision == permission.DecisionAsk && cfg.autoApprove && permResult.Reason == permission.ReasonNetwork {
				allowNetworkForRun = true
			}

			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolRunning, domain.ToolPart{
				CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolRunning, Input: call.Arguments,
			})

			args := cloneMap(call.Arguments)
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
			if allowNetworkForRun {
				args["__sandbox_allow_network"] = true
			}

			result, err := handler.Execute(ctx, args)
			if err != nil {
				errContent := err.Error() + toolErrorHint
				errLabel := err.Error()
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					errLabel = "cancelled"
					errContent = "cancelled" + toolErrorHint
				}
				// Use Background so tool.error survives a cancelled turn ctx.
				p.Stream.Publish(context.Background(), tctx.SessionID, tctx.TurnID, domain.EventToolError, domain.ToolPart{
					CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolError, Error: errLabel,
				})
				messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: errContent})
				processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
				p.logToolResult(call.ID, call.Name, errContent)
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					messages = p.closeUnfinishedToolCalls(tctx, messages, assistantToolCalls)
					return domain.Report{}, messages, ctx.Err()
				}
				continue
			}

			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventToolCompleted, domain.ToolPart{
				CallID: call.ID, Name: call.Name, Description: describe, Status: domain.ToolCompleted, Output: result.Content,
			})
			messages = append(messages, Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: result.Content})
			processedCalls = append(processedCalls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
			p.logToolResult(call.ID, call.Name, result.Content)
			p.recordFileChanges(tctx, call.ID, call.Name, call.Arguments, result)
		}
		if reportCaptured {
			break
		}
		if !reportCaptured {
			p.Stream.Publish(ctx, tctx.SessionID, tctx.TurnID, domain.EventStepEnded, domain.StepPayload{Step: step})
		}
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
	messages = p.truncateToolResults(messages)
	messages = p.enforceToolPairing(messages)
	budget := budgetTokens(cfg)
	if estimateTurnTokens(messages) > budget && budget > 0 {
		messages = p.snipHead(messages, budget)
	}
	return messages
}

func budgetTokens(cfg turnRunCfg) int {
	if cfg.compactionMaxTokens > 0 {
		return int(float64(cfg.compactionMaxTokens) * cfg.compactionTriggerRatio)
	}
	return 0
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

func (p *TurnRunner) truncateToolResults(messages []Message) []Message {
	result := make([]Message, len(messages))
	copy(result, messages)
	for i := range result {
		if result[i].Role != RoleTool {
			continue
		}
		content := result[i].Content
		limit := turnToolTextMaxChars
		if isHugeResult(content) {
			limit = turnHugeResultThreshold
		}
		if len(content) > limit {
			result[i].Content = content[:limit] + fmt.Sprintf("\n...[truncated, %d total chars]", len(content))
		}
	}
	return result
}

func isHugeResult(content string) bool {
	return len(content) > turnHugeResultThreshold
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

// keepCompleteToolPairs returns an API-safe message sequence. In a batched
// assistant response, each call is retained independently only when its result
// exists. Text content survives even if all tool calls are removed.
func keepCompleteToolPairs(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}
	haveResult := make(map[string]bool)
	for _, m := range messages {
		if m.Role == RoleTool && m.ToolCallID != "" {
			haveResult[m.ToolCallID] = true
		}
	}
	out := make([]Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			kept := make([]ToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if haveResult[tc.ID] {
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
			continue
		}
		out = append(out, m)
	}
	callIDs := make(map[string]bool)
	for _, m := range out {
		if m.Role == RoleAssistant {
			for _, tc := range m.ToolCalls {
				callIDs[tc.ID] = true
			}
		}
	}
	final := make([]Message, 0, len(out))
	for _, m := range out {
		if m.Role == RoleTool && m.ToolCallID != "" && !callIDs[m.ToolCallID] {
			continue
		}
		final = append(final, m)
	}
	return final
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
			for j := i; j < len(result); {
				rj := result[j]
				if rj.Role == RoleTool && ids[rj.ToolCallID] {
					result = removeAt(result, j)
				} else {
					j++
				}
			}
		} else {
			result = removeAt(result, i)
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

// buildTurnContextMessage creates a system message with dynamic per-turn context.
// NOT persisted in messages — only appended temporarily for LLM calls.
func buildTurnContextMessage(workDir, model string) Message {
	now := time.Now()
	content := "<turn-context>\n" +
		"Current time: " + now.Format("2006-01-02T15:04:05Z07:00") + " (" + now.Weekday().String() + ")\n" +
		"Working directory: " + workDir + "\n" +
		"Model: " + model + "\n" +
		"</turn-context>"
	return Message{Role: RoleSystem, Content: content}
}

// appendTurnContext appends the turn context message to the LLM request messages.
// This is a temporary append — the original messages slice is not modified.
func appendTurnContext(msgs []port.ChatMessage, tc Message) []port.ChatMessage {
	out := make([]port.ChatMessage, len(msgs)+1)
	copy(out, msgs)
	out[len(msgs)] = port.ChatMessage{Role: string(tc.Role), Content: tc.Content}
	return out
}
