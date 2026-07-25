package service

import (
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// applyQQGroupPolicy annotates inbound group messages with deny_tools / policy_note.
// Returns false when the message should be dropped under require_mention rules.
func applyQQGroupPolicy(cfg domain.ConfigQQChannel, msg *port.InboundMessage) bool {
	if msg == nil {
		return false
	}
	scene := ""
	if msg.Meta != nil {
		scene = msg.Meta["receive_type"]
		if scene == "" {
			scene = msg.Meta["scene"]
		}
	}
	if scene != "group" {
		return true
	}
	policy := cfg.Group
	groupID := ""
	if msg.Meta != nil {
		groupID = msg.Meta["group_openid"]
	}
	if groupID != "" && policy.Groups != nil {
		if ov, ok := policy.Groups[groupID]; ok {
			if ov.RequireMention != nil {
				policy.RequireMention = ov.RequireMention
			}
			if len(ov.DenyTools) > 0 {
				policy.DenyTools = ov.DenyTools
			}
		}
	}
	// QQ only delivers GROUP_AT_MESSAGE_CREATE today; require_mention default true.
	requireMention := true
	if policy.RequireMention != nil {
		requireMention = *policy.RequireMention
	}
	if requireMention {
		// Already @-gated by event type; keep for future always-on group events.
		_ = requireMention
	}
	if msg.Meta == nil {
		msg.Meta = map[string]string{}
	}
	if len(policy.DenyTools) > 0 {
		msg.Meta["deny_tools"] = strings.Join(policy.DenyTools, ",")
		msg.Meta["policy_note"] = "[系统] 本群禁止需授权工具：" + strings.Join(policy.DenyTools, ", ")
	}
	return true
}
