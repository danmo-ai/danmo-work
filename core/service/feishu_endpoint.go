package service

import (
	"context"
	"strings"

	"danmo-work/core/adapter/feishu"
	"danmo-work/core/port"
)

// FeishuEndpoint adapts the Feishu Open API adapter to ChannelEndpoint +
// StreamSender + ChannelInteractor + ChannelApprover + ProgressUpdater.
type FeishuEndpoint struct {
	adapter *feishu.Adapter
}

func NewFeishuEndpoint(adapter *feishu.Adapter) *FeishuEndpoint {
	return &FeishuEndpoint{adapter: adapter}
}

func (e *FeishuEndpoint) Type() port.ChannelType { return port.ChannelFeishu }

func (e *FeishuEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream:  true,
		RichCards:          true,
		InteractiveAsk:     true,
		InteractiveApprove: true,
	}
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
	card := feishu.BuildProgressCard("执行中…", channelProcessingMsg, nil)
	res, err := e.adapter.SendInteractiveCard(ctx, in, card)
	if err != nil {
		// Degrade to text placeholder.
		tres, terr := e.adapter.SendTextMessage(ctx, in, channelProcessingMsg)
		if terr != nil {
			return "", err
		}
		return tres.MessageID, nil
	}
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
	headline := strings.TrimSpace(progress.Headline)
	if headline == "" {
		headline = "执行中…"
	}
	card := feishu.BuildProgressCard(headline, progress.TextBody, progress.Lines)
	if err := e.adapter.UpdateInteractiveCard(ctx, streamID, card); err == nil {
		return nil
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
	text := strings.TrimSpace(final.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	if strings.TrimSpace(streamID) != "" {
		card := feishu.BuildProgressCard("已完成", text, nil)
		if err := e.adapter.UpdateInteractiveCard(ctx, streamID, card); err == nil {
			return nil
		}
		if err := e.adapter.UpdateTextMessage(ctx, streamID, text); err == nil {
			return nil
		}
	}
	final.Text = text
	return e.adapter.DeliverOutbound(ctx, in, final)
}

func (e *FeishuEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
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
	text := formatPermissionText(ask)
	body := text
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "工具授权",
		Text:  text,
		Card: &port.OutboundCard{
			Title:   "工具授权",
			Body:    body,
			Actions: permissionActions(ask.ApprovalID),
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, err
	}
	return true, nil
}
