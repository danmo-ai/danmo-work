package service

import (
	"fmt"
	"strings"

	"danmo-work/core/port"
)

// Compact callback tokens for Feishu card value / QQ keyboard action.data:
//
//	dw|<kind>|<id>|<opt>
//
// kind: a=ask, p=perm, j=project
const callbackPrefix = "dw"

// EncodeCallback builds a compact interactive callback token.
func EncodeCallback(kind port.InteractionKind, id, opt string) string {
	k := kindCode(kind)
	id = strings.ReplaceAll(strings.TrimSpace(id), "|", "_")
	opt = strings.ReplaceAll(strings.TrimSpace(opt), "|", "_")
	if opt == "" {
		opt = "-"
	}
	return fmt.Sprintf("%s|%s|%s|%s", callbackPrefix, k, id, opt)
}

// DecodeCallback parses a compact callback token.
func DecodeCallback(raw string) (kind port.InteractionKind, id, opt string, ok bool) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "|")
	if len(parts) != 4 || parts[0] != callbackPrefix {
		return "", "", "", false
	}
	kind, ok = kindFromCode(parts[1])
	if !ok {
		return "", "", "", false
	}
	id = parts[2]
	opt = parts[3]
	if opt == "-" {
		opt = ""
	}
	return kind, id, opt, id != ""
}

func kindCode(k port.InteractionKind) string {
	switch k {
	case port.InteractionAsk:
		return "a"
	case port.InteractionPermission:
		return "p"
	case port.InteractionProject:
		return "j"
	default:
		return string(k)
	}
}

func kindFromCode(c string) (port.InteractionKind, bool) {
	switch c {
	case "a", string(port.InteractionAsk):
		return port.InteractionAsk, true
	case "p", string(port.InteractionPermission):
		return port.InteractionPermission, true
	case "j", string(port.InteractionProject):
		return port.InteractionProject, true
	default:
		return "", false
	}
}

// InteractionFromCallback builds an InteractionEvent from a decoded token + peer context.
func InteractionFromCallback(base port.InboundMessage, raw string) (port.InteractionEvent, bool) {
	kind, id, opt, ok := DecodeCallback(raw)
	if !ok {
		return port.InteractionEvent{}, false
	}
	return port.InteractionEvent{
		Type:      base.Type,
		AccountID: base.AccountID,
		PeerID:    base.PeerID,
		ChatID:    base.ChatID,
		MessageID: base.MessageID,
		Kind:      kind,
		TargetID:  id,
		Option:    opt,
		Raw:       raw,
		Meta:      base.Meta,
	}, true
}
