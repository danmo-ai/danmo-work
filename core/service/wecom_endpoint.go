package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"danmo-work/core/adapter/wecom"
	"danmo-work/core/port"
)

// WecomEndpoint delivers via the active AI Bot WebSocket long connection.
// ProgressiveStream uses msgtype=stream (placeholder already sent by LongConn).
type WecomEndpoint struct {
	mu   sync.RWMutex
	conn *wecom.LongConn
}

func NewWecomEndpoint() *WecomEndpoint {
	return &WecomEndpoint{}
}

func (e *WecomEndpoint) SetConn(lc *wecom.LongConn) {
	e.mu.Lock()
	e.conn = lc
	e.mu.Unlock()
}

func (e *WecomEndpoint) ClearConn(lc *wecom.LongConn) {
	e.mu.Lock()
	if e.conn == lc {
		e.conn = nil
	}
	e.mu.Unlock()
}

func (e *WecomEndpoint) longConn() *wecom.LongConn {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.conn
}

func (e *WecomEndpoint) Type() port.ChannelType { return port.ChannelWecom }

func (e *WecomEndpoint) Capabilities() port.ChannelCapabilities {
	return port.ChannelCapabilities{
		ProgressiveStream: true,
		RichCards:         false,
		InteractiveAsk:    true, // text menu via stream update
	}
}

func (e *WecomEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	// WeCom AI Bot replies go through the stream API on the inbound req.
	reqID, streamID := streamMeta(in)
	if reqID == "" || streamID == "" {
		return fmt.Errorf("wecom deliver: missing req_id/stream_id")
	}
	lc := e.longConn()
	if lc == nil {
		return fmt.Errorf("wecom deliver: not connected")
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	return lc.ReplyStream(reqID, streamID, text, true)
}

func (e *WecomEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	// LongConn already sent the ~5s placeholder; reuse its stream_id.
	_, streamID := streamMeta(in)
	if streamID == "" {
		return "", fmt.Errorf("wecom start stream: missing stream_id")
	}
	return streamID, nil
}

func (e *WecomEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	reqID, _ := streamMeta(in)
	if reqID == "" || streamID == "" {
		return fmt.Errorf("wecom update stream: missing req_id/stream_id")
	}
	lc := e.longConn()
	if lc == nil {
		return fmt.Errorf("wecom update stream: not connected")
	}
	text := strings.TrimSpace(fullContent)
	if text == "" {
		text = channelProcessingMsg
	}
	return lc.ReplyStream(reqID, streamID, text, false)
}

func (e *WecomEndpoint) FinishStream(ctx context.Context, in *port.InboundMessage, streamID string, final port.OutboundMessage) error {
	reqID, _ := streamMeta(in)
	if reqID == "" || streamID == "" {
		return fmt.Errorf("wecom finish stream: missing req_id/stream_id")
	}
	lc := e.longConn()
	if lc == nil {
		return fmt.Errorf("wecom finish stream: not connected")
	}
	text := strings.TrimSpace(final.Text)
	if text == "" {
		text = "（无文本回复）"
	}
	return lc.ReplyStream(reqID, streamID, text, true)
}

func (e *WecomEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	// Keep the ask inside the same stream bubble (only reply channel available).
	reqID, streamID := streamMeta(in)
	if reqID == "" || streamID == "" {
		return false, fmt.Errorf("wecom present ask: missing req_id/stream_id")
	}
	lc := e.longConn()
	if lc == nil {
		return false, fmt.Errorf("wecom present ask: not connected")
	}
	if err := lc.ReplyStream(reqID, streamID, formatAskText(ask), false); err != nil {
		return false, err
	}
	return true, nil
}

func streamMeta(in *port.InboundMessage) (reqID, streamID string) {
	if in == nil || in.Meta == nil {
		return "", ""
	}
	return in.Meta["req_id"], in.Meta["stream_id"]
}
