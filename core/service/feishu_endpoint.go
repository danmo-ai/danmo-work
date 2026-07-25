package service

import (
	"context"
	"strings"

	"danmo-work/core/adapter/feishu"
	"danmo-work/core/port"
)

// FeishuEndpoint adapts the Feishu Open API adapter to ChannelEndpoint +
// StreamSender + ChannelInteractor. Progressive stream is emulated by
// sending a placeholder text message and PATCHing it.
type FeishuEndpoint struct {
	adapter *feishu.Adapter
}

func NewFeishuEndpoint(adapter *feishu.Adapter) *FeishuEndpoint {
	return &FeishuEndpoint{adapter: adapter}
}

func (e *FeishuEndpoint) Type() port.ChannelType { return port.ChannelFeishu }

func (e *FeishuEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream: true,
		RichCards:         true,
		InteractiveAsk:    true,
	}
}

func (e *FeishuEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	if e.adapter == nil {
		return nil
	}
	return e.adapter.DeliverOutbound(ctx, in, msg)
}

func (e *FeishuEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	res, err := e.adapter.SendTextMessage(ctx, in, channelProcessingMsg)
	if err != nil {
		return "", err
	}
	return res.MessageID, nil
}

func (e *FeishuEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	if strings.TrimSpace(streamID) == "" {
		return nil
	}
	text := strings.TrimSpace(fullContent)
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
	// Prefer patching the placeholder for progressive UX; fall back to a new message.
	if strings.TrimSpace(streamID) != "" {
		if err := e.adapter.UpdateTextMessage(ctx, streamID, text); err == nil {
			return nil
		}
	}
	final.Text = text
	return e.adapter.DeliverOutbound(ctx, in, final)
}

func (e *FeishuEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	text := formatAskText(ask)
	actions := make([]port.OutboundAction, 0, len(ask.Options))
	for _, opt := range ask.Options {
		actions = append(actions, port.OutboundAction{Label: opt})
	}
	msg := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "需要你的确认",
		Text:  text,
		Card: &port.OutboundCard{
			Title:   "需要你的确认",
			Body:    strings.TrimSpace(ask.Question),
			Actions: actions,
		},
	}
	if err := e.Deliver(ctx, in, msg); err != nil {
		return false, err
	}
	return true, nil
}
