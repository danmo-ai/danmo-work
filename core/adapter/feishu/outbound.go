package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"danmo-work/core/port"
)

type sendResult struct {
	MessageID string
}

func (a *Adapter) resolveReceive(in *port.InboundMessage) (receiveID, receiveType string) {
	receiveType = "chat_id"
	if in.Meta != nil {
		receiveID = in.Meta["receive_id"]
		if t := in.Meta["receive_type"]; t != "" {
			receiveType = t
		}
	}
	if receiveID == "" {
		receiveID = in.ChatID
	}
	if receiveID == "" {
		receiveID = in.PeerID
		receiveType = "open_id"
	}
	return receiveID, receiveType
}

// SendTextMessage posts a text message and returns the Feishu message_id when available.
func (a *Adapter) SendTextMessage(ctx context.Context, in *port.InboundMessage, text string) (sendResult, error) {
	if strings.TrimSpace(text) == "" {
		return sendResult{}, nil
	}
	receiveID, receiveType := a.resolveReceive(in)
	return a.sendRaw(ctx, receiveID, receiveType, "text", map[string]string{"text": text})
}

// SendPostMessage sends a simple zh-CN post (rich text) built from title + markdown-ish body.
func (a *Adapter) SendPostMessage(ctx context.Context, in *port.InboundMessage, title, body string) (sendResult, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" && body == "" {
		return sendResult{}, nil
	}
	if title == "" {
		title = "Danmo Work"
	}
	content := map[string]any{
		"zh_cn": map[string]any{
			"title": title,
			"content": [][]map[string]any{
				{{"tag": "md", "text": body}},
			},
		},
	}
	receiveID, receiveType := a.resolveReceive(in)
	return a.sendRaw(ctx, receiveID, receiveType, "post", content)
}

// UpdateTextMessage patches an existing message to plain text (progressive stream emulation).
func (a *Adapter) UpdateTextMessage(ctx context.Context, messageID, text string) error {
	return a.patchMessage(ctx, messageID, "text", map[string]string{"text": text})
}

// SendInteractiveCard sends a schema 2.0 interactive card.
func (a *Adapter) SendInteractiveCard(ctx context.Context, in *port.InboundMessage, card map[string]any) (sendResult, error) {
	if card == nil {
		return sendResult{}, nil
	}
	receiveID, receiveType := a.resolveReceive(in)
	return a.sendRaw(ctx, receiveID, receiveType, "interactive", card)
}

// UpdateInteractiveCard patches an existing message to an interactive card.
func (a *Adapter) UpdateInteractiveCard(ctx context.Context, messageID string, card map[string]any) error {
	return a.patchMessage(ctx, messageID, "interactive", card)
}

func (a *Adapter) patchMessage(ctx context.Context, messageID, msgType string, content any) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return fmt.Errorf("feishu update: message_id required")
	}
	token, err := a.tenantToken(ctx)
	if err != nil {
		return err
	}
	contentStr := ""
	switch c := content.(type) {
	case string:
		contentStr = c
	default:
		contentStr = string(mustJSON(c))
	}
	payload, _ := json.Marshal(map[string]any{
		"msg_type": msgType,
		"content":  contentStr,
	})
	url := fmt.Sprintf("%s/im/v1/messages/%s", OpenAPIBase(a.config().Domain), messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("feishu update: HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.Code != 0 {
		return fmt.Errorf("feishu update: code=%d msg=%s", out.Code, out.Msg)
	}
	return nil
}

func (a *Adapter) sendRaw(ctx context.Context, receiveID, receiveType, msgType string, content any) (sendResult, error) {
	token, err := a.tenantToken(ctx)
	if err != nil {
		return sendResult{}, err
	}
	contentStr := ""
	switch c := content.(type) {
	case string:
		contentStr = c
	default:
		contentStr = string(mustJSON(c))
	}
	payload, _ := json.Marshal(map[string]any{
		"receive_id": receiveID,
		"msg_type":   msgType,
		"content":    contentStr,
	})
	url := fmt.Sprintf("%s/im/v1/messages?receive_id_type=%s", OpenAPIBase(a.config().Domain), receiveType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return sendResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := a.client.Do(req)
	if err != nil {
		return sendResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return sendResult{}, fmt.Errorf("feishu send: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Code != 0 {
		return sendResult{}, fmt.Errorf("feishu send: code=%d msg=%s", out.Code, out.Msg)
	}
	return sendResult{MessageID: out.Data.MessageID}, nil
}

// DeliverOutbound maps OutboundMessage kinds onto Feishu message types.
func (a *Adapter) DeliverOutbound(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	text := strings.TrimSpace(msg.Text)
	switch msg.Kind {
	case port.OutboundKindCard, port.OutboundKindMarkdown:
		title := strings.TrimSpace(msg.Title)
		body := text
		var actions []port.OutboundAction
		if msg.Card != nil {
			if title == "" {
				title = msg.Card.Title
			}
			if strings.TrimSpace(msg.Card.Body) != "" {
				body = msg.Card.Body
			}
			actions = msg.Card.Actions
		}
		// Prefer true interactive cards when buttons are present.
		if len(actions) > 0 {
			card := BuildInteractiveCard(title, body, actions)
			if _, err := a.SendInteractiveCard(ctx, in, card); err == nil {
				return nil
			}
			// Fall through to numbered post/text when interactive is rejected.
			var lines []string
			if body != "" {
				lines = append(lines, body, "")
			}
			for i, act := range actions {
				label := act.Label
				if label == "" {
					label = act.ID
				}
				lines = append(lines, fmt.Sprintf("%d. %s", i+1, label))
			}
			body = strings.Join(lines, "\n")
		}
		if title != "" || looksLikeMarkdown(body) {
			if _, err := a.SendPostMessage(ctx, in, title, body); err == nil {
				return nil
			}
			text = body
			if title != "" && body != "" {
				text = title + "\n\n" + body
			} else if title != "" {
				text = title
			}
		}
		fallthrough
	default:
		_, err := a.SendTextMessage(ctx, in, text)
		return err
	}
}

func looksLikeMarkdown(s string) bool {
	return strings.Contains(s, "\n") || strings.Contains(s, "**") || strings.Contains(s, "`") || strings.Contains(s, "# ")
}
