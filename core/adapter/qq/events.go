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

type qqAttachment struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
}

type c2cMessage struct {
	ID          string         `json:"id"`
	Content     string         `json:"content"`
	Timestamp   string         `json:"timestamp"`
	Attachments []qqAttachment `json:"attachments"`
	Author      struct {
		UserOpenID string `json:"user_openid"`
	} `json:"author"`
}

type groupAtMessage struct {
	ID           string         `json:"id"`
	Content      string         `json:"content"`
	GroupOpenID  string         `json:"group_openid"`
	Timestamp    string         `json:"timestamp"`
	Attachments  []qqAttachment `json:"attachments"`
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
		media := attachmentsToMedia(m.Attachments)
		if (text == "" && len(media) == 0) || m.Author.UserOpenID == "" {
			return nil, nil, ""
		}
		return &port.InboundMessage{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    m.Author.UserOpenID,
			ChatID:    m.Author.UserOpenID,
			Text:      text,
			MessageID: m.ID,
			Media:     media,
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
		media := attachmentsToMedia(m.Attachments)
		if (text == "" && len(media) == 0) || peer == "" {
			return nil, nil, ""
		}
		return &port.InboundMessage{
			Type:      port.ChannelQQ,
			AccountID: accountID,
			PeerID:    peer,
			ChatID:    m.GroupOpenID,
			Text:      text,
			MessageID: m.ID,
			Media:     media,
			Meta: map[string]string{
				"scene":        "group",
				"openid":       peer,
				"group_openid": m.GroupOpenID,
				"message_id":   m.ID,
				"receive_id":   m.GroupOpenID,
				"receive_type": "group",
				"mentioned":    "true", // GROUP_AT_MESSAGE_CREATE is always @-gated
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

func attachmentsToMedia(atts []qqAttachment) []port.InboundMedia {
	if len(atts) == 0 {
		return nil
	}
	out := make([]port.InboundMedia, 0, len(atts))
	for _, a := range atts {
		url := strings.TrimSpace(a.URL)
		if url == "" {
			continue
		}
		name := strings.TrimSpace(a.Filename)
		if name == "" {
			name = strings.TrimSpace(a.FileName)
		}
		mime := strings.TrimSpace(a.ContentType)
		kind := "file"
		lower := strings.ToLower(mime + " " + name)
		switch {
		case strings.Contains(lower, "image") || strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".webp"):
			kind = "image"
		case strings.Contains(lower, "audio") || strings.HasSuffix(lower, ".silk") || strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".wav"):
			kind = "audio"
		case strings.Contains(lower, "video") || strings.HasSuffix(lower, ".mp4"):
			kind = "video"
		}
		out = append(out, port.InboundMedia{
			Name:     name,
			URL:      url,
			MimeType: mime,
			Kind:     kind,
		})
	}
	return out
}
