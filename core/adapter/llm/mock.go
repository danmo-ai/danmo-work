package llm

import (
	"context"
	"strings"
	"sync"

	"danmo-work/core/port"
)

var _ port.LLMProvider = (*MockProvider)(nil)

type callStep struct {
	ToolCall  string
	Args      map[string]any
	Text      string
	Reasoning string
}

type MockProvider struct {
	mu       sync.Mutex
	steps    []callStep
	cursor   int
	Requests []port.LLMChatRequest
}

func NewMock() *MockProvider { return &MockProvider{} }

func (p *MockProvider) AddToolCall(tool string, args map[string]any) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{ToolCall: tool, Args: args})
	return p
}

func (p *MockProvider) AddToolCallWithReasoning(tool string, args map[string]any, reasoning string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{ToolCall: tool, Args: args, Reasoning: reasoning})
	return p
}

func (p *MockProvider) AddText(content string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{Text: content})
	return p
}

func (p *MockProvider) AddTextWithReasoning(content, reasoning string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{Text: content, Reasoning: reasoning})
	return p
}

func (p *MockProvider) Finish(summary string) *MockProvider {
	return p.AddText(summary)
}

func (p *MockProvider) Chat(_ context.Context, req port.LLMChatRequest) (port.LLMChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Requests = append(p.Requests, req)

	if len(req.Messages) > 0 && req.Messages[0].Role == "system" &&
		strings.Contains(req.Messages[0].Content, "You are a session title generator") {
		return port.LLMChatResponse{Content: "Generated Title", Done: true}, nil
	}

	if p.cursor >= len(p.steps) {
		return port.LLMChatResponse{Content: "done", Done: true}, nil
	}
	step := p.steps[p.cursor]
	p.cursor++

	if step.Text != "" {
		return port.LLMChatResponse{Content: step.Text, ReasoningContent: step.Reasoning, Done: true}, nil
	}
	return port.LLMChatResponse{
		ReasoningContent: step.Reasoning,
		ToolCalls: []port.ChatToolCall{{
			ID: step.ToolCall + "-id", Name: step.ToolCall, Arguments: step.Args,
		}},
	}, nil
}
