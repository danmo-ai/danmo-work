package qq

import (
	"encoding/json"
	"strings"

	"danmo-work/core/port"
)

// InboundHandler is called for each normalized inbound QQ message.
type InboundHandler func(msg port.InboundMessage) error

// InteractionHandler is called for keyboard button callbacks.
type InteractionHandler func(ev port.InteractionEvent, interactionID string) error

// DispatchPayload is a Gateway dispatch event envelope (op=0).
type DispatchPayload struct {
	Op int             `json:"op"`
	S  int64           `json:"s"`
	T  string          `json:"t"`
	D  json.RawMessage `json:"d"`
}

type c2cMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		UserOpenID string `json:"user_openid"`
	} `json:"author"`
}

type groupAtMessage struct {
	ID           string `json:"id"`
	Content      string `json:"content"`
	GroupOpenID  string `json:"group_openid"`
	Timestamp    string `json:"timestamp"`
	Author       struct {
		MemberOpenID string `json:"member_openid"`
	} `json:"author"`
}

type interactionEvent struct {
	ID               string `json:"id"`
	Type             int    `json:"type"`
	Scene            string `json:"scene"`
	ChatType         int    `json:"chat_type"`
	UserOpenID       string `json:"user_openid"`
	GroupOpenID      string `json:"group_openid"`
	GroupMemberOpenID string `json:"group_member_openid"`
	Data             struct {
		Resolved struct {
			ButtonData string `json:"button_data"`
			ButtonID   string `json:"button_id"`
			MessageID  string `json:"message_id"`
		} `json:"resolved"`
		Resoloved struct {
			ButtonData string `json:"button_data"`
			ButtonID   string `json:"button_id"`
			MessageID  string `json:"message_id"`
		} `json:"resoloved"`
	} `json:"data"`
}

// NormalizeDispatch converts a QQ Gateway dispatch into inbound or interaction.
func NormalizeDispatch(accountID string, t string, raw json.RawMessage) (msg *port.InboundMessage, interaction *port.InteractionEvent, interactionID string) {
	switch t {
	case "C2C_MESSAGE_CREATE":
		var m c2cMessage
		if json.Unmarshal(raw, &m) != nil {
			return nil, nil, ""
		}
		text := strings.TrimSpace(m.Content)
		if text == "" || m.Author.UserOpenID == "" {
			return nil, nil, ""
		}
		return &port.InboundMessage{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    m.Author.UserOpenID,
			ChatID:    m.Author.UserOpenID,
			Text:      text,
			MessageID: m.ID,
			Meta: map[string]string{
				"scene":        "c2c",
				"openid":       m.Author.UserOpenID,
				"message_id":   m.ID,
				"receive_id":   m.Author.UserOpenID,
				"receive_type": "c2c",
			},
		}, nil, ""
	case "GROUP_AT_MESSAGE_CREATE":
		var m groupAtMessage
		if json.Unmarshal(raw, &m) != nil {
			return nil, nil, ""
		}
		text := stripAtBot(strings.TrimSpace(m.Content))
		peer := m.Author.MemberOpenID
		if peer == "" {
			peer = m.GroupOpenID
		}
		if text == "" || peer == "" {
			return nil, nil, ""
		}
		return &port.InboundMessage{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    peer,
			ChatID:    m.GroupOpenID,
			Text:      text,
			MessageID: m.ID,
			Meta: map[string]string{
				"scene":        "group",
				"openid":       peer,
				"group_openid": m.GroupOpenID,
				"message_id":   m.ID,
				"receive_id":   m.GroupOpenID,
				"receive_type": "group",
			},
		}, nil, ""
	case "INTERACTION_CREATE":
		var m interactionEvent
		if json.Unmarshal(raw, &m) != nil {
			return nil, nil, ""
		}
		data := m.Data.Resolved.ButtonData
		btnID := m.Data.Resolved.ButtonID
		msgID := m.Data.Resolved.MessageID
		if data == "" {
			data = m.Data.Resoloved.ButtonData
		}
		if btnID == "" {
			btnID = m.Data.Resoloved.ButtonID
		}
		if msgID == "" {
			msgID = m.Data.Resoloved.MessageID
		}
		if data == "" {
			data = btnID
		}
		peer := m.UserOpenID
		if peer == "" {
			peer = m.GroupMemberOpenID
		}
		chat := m.UserOpenID
		if m.ChatType == 1 || m.Scene == "group" {
			chat = m.GroupOpenID
		}
		if peer == "" {
			peer = chat
		}
		base := port.InboundMessage{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    peer,
			ChatID:    chat,
			MessageID: msgID,
			Meta: map[string]string{
				"scene":        m.Scene,
				"message_id":   msgID,
				"receive_id":   chat,
				"openid":       peer,
				"group_openid": m.GroupOpenID,
			},
		}
		if m.ChatType == 1 || m.Scene == "group" {
			base.Meta["receive_type"] = "group"
		} else {
			base.Meta["receive_type"] = "c2c"
		}
		// Decode happens in service layer; here we only carry raw data as Text for InteractionFromCallback.
		ev := port.InteractionEvent{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    peer,
			ChatID:    chat,
			MessageID: msgID,
			Raw:       data,
			Meta:      base.Meta,
		}
		return nil, &ev, m.ID
	default:
		return nil, nil, ""
	}
}

func stripAtBot(s string) string {
	// QQ often prefixes "@botname ".
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "@") {
		if i := strings.IndexAny(s, " \t\n"); i > 0 {
			return strings.TrimSpace(s[i+1:])
		}
	}
	return s
}
