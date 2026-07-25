package service

import (
	"context"
	"strings"
	"sync"

	"danmo-work/core/adapter/feishu"
	"danmo-work/core/port"
)

// FeishuEndpoint adapts the Feishu Open API adapter to ChannelEndpoint +
// StreamSender + ChannelInteractor + ChannelApprover + ProgressUpdater.
type FeishuEndpoint struct {
	adapter *feishu.Adapter

	mu       sync.Mutex
	progress map[string]port.ProgressSnapshot // streamID → last snapshot
}

func NewFeishuEndpoint(adapter *feishu.Adapter) *FeishuEndpoint {
	return &FeishuEndpoint{
		adapter:  adapter,
		progress: make(map[string]port.ProgressSnapshot),
	}
}

func (e *FeishuEndpoint) Type() port.ChannelType { return port.ChannelFeishu }

func (e *FeishuEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream:  true,
		RichCards:          true,
		InteractiveAsk:     true,
		InteractiveApprove: true,
		NativeMedia:        true,
	}
}

func (e *FeishuEndpoint) richProgress() bool {
	if e.adapter == nil {
		return true
	}
	return e.adapter.RichProgressEnabled()
}

func (e *FeishuEndpoint) rememberProgress(streamID string, snap port.ProgressSnapshot) {
	if strings.TrimSpace(streamID) == "" {
		return
	}
	e.mu.Lock()
	e.progress[streamID] = snap
	e.mu.Unlock()
}

func (e *FeishuEndpoint) takeProgress(streamID string) (port.ProgressSnapshot, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	snap, ok := e.progress[streamID]
	return snap, ok
}

func (e *FeishuEndpoint) clearProgress(streamID string) {
	e.mu.Lock()
	delete(e.progress, streamID)
	e.mu.Unlock()
}

func (e *FeishuEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	if e.adapter == nil {
		return nil
	}
	return e.adapter.DeliverOutbound(ctx, in, msg)
}

func (e *FeishuEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	if e.adapter == nil {
		return "", nil
	}
	if !e.richProgress() {
		tres, err := e.adapter.SendTextMessage(ctx, in, channelProcessingMsg)
		if err != nil {
			return "", err
		}
		return tres.MessageID, nil
	}
	card := feishu.BuildProgressCard("执行中…", channelProcessingMsg, nil, nil)
	res, err := e.adapter.SendInteractiveCard(ctx, in, card)
	if err != nil {
		// Degrade to text placeholder.
		tres, terr := e.adapter.SendTextMessage(ctx, in, channelProcessingMsg)
		if terr != nil {
			return "", err
		}
		return tres.MessageID, nil
	}
	e.rememberProgress(res.MessageID, port.ProgressSnapshot{
		Status:   "running",
		Headline: "执行中…",
		TextBody: channelProcessingMsg,
	})
	return res.MessageID, nil
}

func (e *FeishuEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	return e.UpdateProgress(ctx, in, streamID, port.ProgressSnapshot{
		Status:   "running",
		Headline: "执行中…",
		TextBody: fullContent,
	})
}

func (e *FeishuEndpoint) UpdateProgress(ctx context.Context, in *port.InboundMessage, streamID string, progress port.ProgressSnapshot) error {
	if e.adapter == nil || strings.TrimSpace(streamID) == "" {
		return nil
	}
	e.rememberProgress(streamID, progress)
	headline := strings.TrimSpace(progress.Headline)
	if headline == "" {
		headline = "执行中…"
	}
	if e.richProgress() {
		card := feishu.BuildProgressCard(headline, progress.TextBody, progress.Lines, nil)
		if err := e.adapter.UpdateInteractiveCard(ctx, streamID, card); err == nil {
			return nil
		}
	}
	// Degrade to text patch.
	text := strings.TrimSpace(progress.TextBody)
	if len(progress.Lines) > 0 {
		joined := strings.Join(progress.Lines, "\n")
		if text != "" {
			text = joined + "\n\n" + text
		} else {
			text = joined
		}
	}
	if text == "" {
		text = channelProcessingMsg
	}
	return e.adapter.UpdateTextMessage(ctx, streamID, text)
}

func (e *FeishuEndpoint) FinishStream(ctx context.Context, in *port.InboundMessage, streamID string, final port.OutboundMessage) error {
	defer e.clearProgress(streamID)
	text := strings.TrimSpace(final.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	headline := "已完成"
	agentText := text
	var toolLines []string
	if final.Meta != nil {
		if h := strings.TrimSpace(final.Meta["headline"]); h != "" {
			headline = h
		}
		if a := strings.TrimSpace(final.Meta["agent_text"]); a != "" {
			agentText = a
		}
		if raw := strings.TrimSpace(final.Meta["tool_lines"]); raw != "" {
			toolLines = strings.Split(raw, "\n")
		}
	}
	if strings.TrimSpace(final.Title) != "" && headline == "已完成" {
		headline = strings.TrimSpace(final.Title)
	}
	if strings.TrimSpace(streamID) != "" {
		if e.richProgress() {
			card := feishu.BuildProgressCard(headline, agentText, toolLines, nil)
			if err := e.adapter.UpdateInteractiveCard(ctx, streamID, card); err == nil {
				return nil
			}
		}
		if err := e.adapter.UpdateTextMessage(ctx, streamID, text); err == nil {
			return nil
		}
	}
	final.Text = text
	return e.adapter.DeliverOutbound(ctx, in, final)
}

func (e *FeishuEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	if len(ask.FormFields) > 0 && e.adapter != nil {
		token := EncodeCallback(port.InteractionAsk, ask.AskID, "form")
		card := feishu.BuildAskFormCard("需要你的确认", ask.Question, ask.FormFields, token)
		if _, err := e.adapter.SendInteractiveCard(ctx, in, card); err == nil {
			return true, nil
		}
		// Fall through to text/options card.
	}
	text := formatAskText(ask)
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "需要你的确认",
		Text:  text,
		Card: &port.OutboundCard{
			Title:   "需要你的确认",
			Body:    strings.TrimSpace(ask.Question),
			Actions: askActions(ask),
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, err
	}
	return true, nil
}

func (e *FeishuEndpoint) PresentPermission(ctx context.Context, in *port.InboundMessage, ask port.PermissionPrompt) (bool, error) {
	actions := permissionActions(ask.ApprovalID)
	text := formatPermissionText(ask)
	// Prefer same progress card when StreamID is available (Phase A §4.3).
	if e.adapter != nil && e.richProgress() && strings.TrimSpace(ask.StreamID) != "" {
		snap, ok := e.takeProgress(ask.StreamID)
		headline := "等待授权"
		body := text
		var lines []string
		if ok {
			if strings.TrimSpace(snap.TextBody) != "" {
				body = strings.TrimSpace(snap.TextBody) + "\n\n" + text
			}
			lines = snap.Lines
		}
		card := feishu.BuildProgressCard(headline, body, lines, actions)
		if err := e.adapter.UpdateInteractiveCard(ctx, ask.StreamID, card); err == nil {
			e.rememberProgress(ask.StreamID, port.ProgressSnapshot{
				Status:   "awaiting_perm",
				Headline: headline,
				Lines:    lines,
				TextBody: body,
			})
			return true, nil
		}
	}
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "工具授权",
		Text:  text,
		Card: &port.OutboundCard{
			Title:   "工具授权",
			Body:    text,
			Actions: actions,
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, err
	}
	return true, nil
}
