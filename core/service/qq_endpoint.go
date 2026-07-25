package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"danmo-work/core/adapter/qq"
	"danmo-work/core/port"
)

type qqStreamState struct {
	id    string
	seq   int64
	index int
	c2c   bool
}

// QQEndpoint delivers via QQ Bot OpenAPI (markdown + keyboard + C2C stream).
type QQEndpoint struct {
	adapter *qq.Adapter

	mu      sync.Mutex
	streams map[string]*qqStreamState
}

func NewQQEndpoint(adapter *qq.Adapter) *QQEndpoint {
	return &QQEndpoint{
		adapter: adapter,
		streams: make(map[string]*qqStreamState),
	}
}

func (e *QQEndpoint) Type() port.ChannelType { return port.ChannelQQ }

func (e *QQEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream:  true,
		RichCards:          true,
		InteractiveAsk:     true,
		InteractiveApprove: true,
		NativeMedia:        true,
	}
}

func (e *QQEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	if e.adapter == nil {
		return nil
	}
	return e.adapter.DeliverOutbound(ctx, in, msg)
}

func (e *QQEndpoint) useNativeC2C(in *port.InboundMessage) bool {
	if e.adapter != nil && !e.adapter.NativeC2CStreamEnabled() {
		return false
	}
	if in == nil || in.Meta == nil {
		return true
	}
	scene := in.Meta["receive_type"]
	if scene == "" {
		scene = in.Meta["scene"]
	}
	return scene != "group"
}

func (e *QQEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	if e.adapter == nil {
		return "", nil
	}
	if e.useNativeC2C(in) {
		id, seq, err := e.adapter.StartC2CStream(ctx, in, channelProcessingMsg)
		if err == nil && id != "" {
			e.mu.Lock()
			e.streams[id] = &qqStreamState{id: id, seq: seq, index: 0, c2c: true}
			e.mu.Unlock()
			return id, nil
		}
	}
	// Fallback: send a markdown placeholder (no progressive id).
	_ = e.adapter.DeliverOutbound(ctx, in, port.OutboundMessage{
		Kind: port.OutboundKindMarkdown,
		Text: channelProcessingMsg,
	})
	return "", nil
}

func (e *QQEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	return e.UpdateProgress(ctx, in, streamID, port.ProgressSnapshot{
		Status:   "running",
		Headline: "执行中…",
		TextBody: fullContent,
	})
}

func (e *QQEndpoint) UpdateProgress(ctx context.Context, in *port.InboundMessage, streamID string, progress port.ProgressSnapshot) error {
	if e.adapter == nil || streamID == "" {
		return nil
	}
	e.mu.Lock()
	st := e.streams[streamID]
	e.mu.Unlock()
	if st == nil || !st.c2c {
		return nil
	}
	var b strings.Builder
	if h := strings.TrimSpace(progress.Headline); h != "" {
		b.WriteString("**")
		b.WriteString(h)
		b.WriteString("**\n\n")
	}
	if len(progress.Lines) > 0 {
		b.WriteString(strings.Join(progress.Lines, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString(strings.TrimSpace(progress.TextBody))
	content := strings.TrimSpace(b.String())
	if content == "" {
		content = channelProcessingMsg
	}
	e.mu.Lock()
	st.index++
	idx := st.index
	seq := st.seq
	e.mu.Unlock()
	return e.adapter.UpdateC2CStream(ctx, in, streamID, seq, idx, content, false)
}

func (e *QQEndpoint) FinishStream(ctx context.Context, in *port.InboundMessage, streamID string, final port.OutboundMessage) error {
	text := strings.TrimSpace(final.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	headline := strings.TrimSpace(final.Title)
	if final.Meta != nil {
		if h := strings.TrimSpace(final.Meta["headline"]); h != "" {
			headline = h
		}
	}
	if headline != "" && !strings.HasPrefix(text, "**"+headline) {
		text = "**" + headline + "**\n\n" + text
	}
	if streamID != "" {
		e.mu.Lock()
		st := e.streams[streamID]
		e.mu.Unlock()
		if st != nil && st.c2c {
			e.mu.Lock()
			st.index++
			idx := st.index
			seq := st.seq
			delete(e.streams, streamID)
			e.mu.Unlock()
			if err := e.adapter.UpdateC2CStream(ctx, in, streamID, seq, idx, text, true); err == nil {
				return nil
			}
		}
	}
	final.Text = text
	if final.Kind == "" {
		final.Kind = port.OutboundKindMarkdown
	}
	return e.Deliver(ctx, in, final)
}

func (e *QQEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	// QQ has no native multi-field form; formFields use markdown instructions + free-text reply.
	actions := askActions(ask)
	if len(ask.FormFields) > 0 {
		actions = nil
	}
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "需要你的确认",
		Text:  formatAskText(ask),
		Card: &port.OutboundCard{
			Title:   "需要你的确认",
			Body:    formatAskText(ask),
			Actions: actions,
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, err
	}
	return true, nil
}

func (e *QQEndpoint) PresentPermission(ctx context.Context, in *port.InboundMessage, ask port.PermissionPrompt) (bool, error) {
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "工具授权",
		Text:  formatPermissionText(ask),
		Card: &port.OutboundCard{
			Title:   "工具授权",
			Body:    formatPermissionText(ask),
			Actions: permissionActions(ask.ApprovalID),
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, fmt.Errorf("qq present permission: %w", err)
	}
	return true, nil
}
