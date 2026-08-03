package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"danmo-work/core/port"
)

var _ port.LLMProvider = (*MockProvider)(nil)

type mockToolCall struct {
	Name string
	Args map[string]any
}

type callStep struct {
	ToolCalls []mockToolCall
	Text      string
	Reasoning string
	Delay     time.Duration
}

// ParallelCall is one tool_call inside a parallel batch step.
type ParallelCall struct {
	Name string
	Args map[string]any
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
	p.steps = append(p.steps, callStep{ToolCalls: []mockToolCall{{Name: tool, Args: args}}})
	return p
}

// AddParallelToolCalls queues one model step that returns multiple tool_calls
// in a single assistant message (LLM parallel tool-call contract).
func (p *MockProvider) AddParallelToolCalls(calls ...ParallelCall) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	step := callStep{ToolCalls: make([]mockToolCall, 0, len(calls))}
	for _, c := range calls {
		step.ToolCalls = append(step.ToolCalls, mockToolCall{Name: c.Name, Args: c.Args})
	}
	p.steps = append(p.steps, step)
	return p
}

func (p *MockProvider) AddToolCallWithReasoning(tool string, args map[string]any, reasoning string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{
		ToolCalls: []mockToolCall{{Name: tool, Args: args}},
		Reasoning: reasoning,
	})
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

// AddTextWithDelay queues a text step that waits for d (or context
// cancellation) before returning. Useful to keep a turn "in flight" so tests
// can cancel it deterministically.
func (p *MockProvider) AddTextWithDelay(content string, d time.Duration) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, callStep{Text: content, Delay: d})
	return p
}

func (p *MockProvider) Chat(ctx context.Context, req port.LLMChatRequest) (port.LLMChatResponse, error) {
	p.mu.Lock()

	p.Requests = append(p.Requests, req)

	if len(req.Messages) > 0 && req.Messages[0].Role == "system" &&
		strings.Contains(req.Messages[0].Content, "You are a session title generator") {
		p.mu.Unlock()
		return port.LLMChatResponse{Content: "Generated Title", Done: true}, nil
	}

	if p.cursor >= len(p.steps) {
		p.mu.Unlock()
		return port.LLMChatResponse{Content: "done", Done: true}, nil
	}
	step := p.steps[p.cursor]
	p.cursor++
	p.mu.Unlock()

	if step.Delay > 0 {
		// Sleep outside the lock so concurrent Chat calls (e.g. title
		// generation) are not blocked by a delayed step.
		select {
		case <-time.After(step.Delay):
		case <-ctx.Done():
			return port.LLMChatResponse{}, ctx.Err()
		}
	}

	if step.Text != "" {
		return port.LLMChatResponse{Content: step.Text, ReasoningContent: step.Reasoning, Done: true}, nil
	}
	tcs := make([]port.ChatToolCall, len(step.ToolCalls))
	for i, tc := range step.ToolCalls {
		id := tc.Name + "-id"
		if len(step.ToolCalls) > 1 {
			id = fmt.Sprintf("%s-id-%d", tc.Name, i)
		}
		tcs[i] = port.ChatToolCall{ID: id, Name: tc.Name, Arguments: tc.Args}
	}
	return port.LLMChatResponse{
		ReasoningContent: step.Reasoning,
		ToolCalls:        tcs,
	}, nil
}
