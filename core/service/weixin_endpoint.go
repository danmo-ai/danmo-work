package service

import (
	"context"
	"fmt"
	"strings"

	"danmo-work/core/adapter/weixin/ilink"
	"danmo-work/core/port"
)

// WeixinEndpoint delivers plain text via iLink. Progressive stream is emulated
// as: send "正在处理…" then a second final message (iLink has no edit API here).
type WeixinEndpoint struct {
	bridge *WeixinBridge
}

func NewWeixinEndpoint(bridge *WeixinBridge) *WeixinEndpoint {
	return &WeixinEndpoint{bridge: bridge}
}

func (e *WeixinEndpoint) Type() port.ChannelType { return port.ChannelWeixin }

func (e *WeixinEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream:  true, // emulated: placeholder + final (UpdateStream no-op)
		RichCards:          false,
		InteractiveAsk:     true, // numbered text menu
		InteractiveApprove: true, // numbered text menu (1/2/3)
		NativeMedia:        true,
	}
}

func (e *WeixinEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	return e.sendText(ctx, in, msg.Text)
}

func (e *WeixinEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	if err := e.sendText(ctx, in, channelProcessingMsg); err != nil {
		return "", err
	}
	// Synthetic id — UpdateStream is a no-op; FinishStream sends a new message.
	return "weixin-stream", nil
}

func (e *WeixinEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	// iLink personal WeChat path has no message edit; skip mid-turn updates.
	return nil
}

func (e *WeixinEndpoint) UpdateProgress(ctx context.Context, in *port.InboundMessage, streamID string, progress port.ProgressSnapshot) error {
	// No mid-turn edit; tool lines are included on FinishStream via final.Meta/Text.
	return nil
}

func (e *WeixinEndpoint) FinishStream(ctx context.Context, in *port.InboundMessage, streamID string, final port.OutboundMessage) error {
	text := strings.TrimSpace(final.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	headline := strings.TrimSpace(final.Title)
	if final.Meta != nil {
		if mh := strings.TrimSpace(final.Meta["headline"]); mh != "" {
			headline = mh
		}
	}
	// Surface non-success end states; tool lines already live in final.Text.
	if headline != "" && headline != "已完成" && !strings.Contains(text, headline) {
		text = headline + "\n\n" + text
	}
	return e.sendText(ctx, in, text)
}

func (e *WeixinEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	if err := e.sendText(ctx, in, formatAskText(ask)); err != nil {
		return false, err
	}
	return true, nil
}

func (e *WeixinEndpoint) PresentPermission(ctx context.Context, in *port.InboundMessage, ask port.PermissionPrompt) (bool, error) {
	if err := e.sendText(ctx, in, formatPermissionText(ask)); err != nil {
		return false, err
	}
	return true, nil
}

func (e *WeixinEndpoint) sendText(ctx context.Context, in *port.InboundMessage, text string) error {
	text = strings.TrimSpace(text)
	if text == "" || e.bridge == nil {
		return nil
	}
	acc, err := e.bridge.store.WeixinAccounts().Get(ctx, in.AccountID)
	if err != nil {
		return err
	}
	ilAcc := ilink.Account{
		AccountID: acc.AccountID,
		Token:     acc.Token,
		BaseURL:   acc.BaseURL,
		UserID:    acc.UserID,
	}
	ctxTok := ""
	if in.Meta != nil {
		ctxTok = in.Meta["context_token"]
	}
	if ctxTok == "" {
		if binding, berr := e.bridge.store.WeixinBindings().GetByPeer(ctx, in.AccountID, in.PeerID); berr == nil {
			ctxTok = binding.ContextToken
		}
	}
	if err := e.bridge.client.SendText(ctx, ilAcc, in.PeerID, text, ctxTok, ""); err != nil {
		return fmt.Errorf("weixin send: %w", err)
	}
	return nil
}
