package qq

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"danmo-work/core/port"
)

var msgSeqCounter atomic.Int64

func nextMsgSeq() int64 {
	return msgSeqCounter.Add(1)
}

// DeliverOutbound maps OutboundMessage onto QQ C2C/group send APIs.
func (a *Adapter) DeliverOutbound(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	text := strings.TrimSpace(msg.Text)
	title := strings.TrimSpace(msg.Title)
	var actions []port.OutboundAction
	if msg.Card != nil {
		if title == "" {
			title = msg.Card.Title
		}
		if strings.TrimSpace(msg.Card.Body) != "" {
			text = msg.Card.Body
		}
		actions = msg.Card.Actions
	}
	body := text
	if title != "" && body != "" {
		body = "**" + title + "**\n\n" + body
	} else if title != "" {
		body = "**" + title + "**"
	}
	if body == "" {
		body = "（无文本回复）"
	}
	payload := map[string]any{
		"msg_type": 2,
		"markdown": map[string]any{"content": body},
		"msg_seq":  nextMsgSeq(),
	}
	if in != nil && strings.TrimSpace(in.MessageID) != "" {
		payload["msg_id"] = in.MessageID
	}
	if kb := BuildKeyboard(actions); kb != nil {
		payload["keyboard"] = kb
	}
	return a.sendByScene(ctx, in, payload)
}

func (a *Adapter) sendByScene(ctx context.Context, in *port.InboundMessage, payload map[string]any) error {
	if in == nil {
		return fmt.Errorf("qq send: inbound required")
	}
	scene := ""
	if in.Meta != nil {
		scene = in.Meta["receive_type"]
		if scene == "" {
			scene = in.Meta["scene"]
		}
	}
	switch scene {
	case "group":
		gid := in.ChatID
		if in.Meta != nil && in.Meta["group_openid"] != "" {
			gid = in.Meta["group_openid"]
		}
		_, err := a.SendGroupMessage(ctx, gid, payload)
		return err
	default:
		openid := in.PeerID
		if in.Meta != nil && in.Meta["openid"] != "" {
			openid = in.Meta["openid"]
		}
		_, err := a.SendC2CMessage(ctx, openid, payload)
		return err
	}
}

// StartC2CStream begins a native C2C stream_messages session.
func (a *Adapter) StartC2CStream(ctx context.Context, in *port.InboundMessage, content string) (streamID string, seq int64, err error) {
	if in == nil {
		return "", 0, fmt.Errorf("qq stream: inbound required")
	}
	openid := in.PeerID
	if in.Meta != nil && in.Meta["openid"] != "" {
		openid = in.Meta["openid"]
	}
	seq = nextMsgSeq()
	body := map[string]any{
		"input_mode":   "replace",
		"input_state":  1,
		"content_type": "markdown",
		"content_raw":  content,
		"msg_seq":      seq,
		"index":        0,
	}
	if in.MessageID != "" {
		body["msg_id"] = in.MessageID
	}
	res, err := a.StreamC2C(ctx, openid, body)
	if err != nil {
		return "", 0, err
	}
	return res.StreamID, seq, nil
}

// UpdateC2CStream replaces stream content (input_state=1 generating).
func (a *Adapter) UpdateC2CStream(ctx context.Context, in *port.InboundMessage, streamID string, seq int64, index int, content string, done bool) error {
	if in == nil || streamID == "" {
		return nil
	}
	openid := in.PeerID
	if in.Meta != nil && in.Meta["openid"] != "" {
		openid = in.Meta["openid"]
	}
	state := 1
	if done {
		state = 10
	}
	body := map[string]any{
		"input_mode":    "replace",
		"input_state":   state,
		"content_type":  "markdown",
		"content_raw":   content,
		"stream_msg_id": streamID,
		"msg_seq":       seq,
		"index":         index,
	}
	if in.MessageID != "" {
		body["msg_id"] = in.MessageID
	}
	_, err := a.StreamC2C(ctx, openid, body)
	return err
}
