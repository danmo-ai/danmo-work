package service

import (
	"fmt"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// formatAskText builds a numbered menu for channels without native cards.
func formatAskText(ask port.AskPrompt) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(ask.Question))
	if len(ask.FormFields) > 0 {
		b.WriteString("\n\n请按下列格式逐行回复（字段名可省略，按顺序填写也可）：")
		for _, f := range ask.FormFields {
			label := strings.TrimSpace(f.Label)
			if label == "" {
				label = f.Name
			}
			line := fmt.Sprintf("\n- %s", label)
			if f.Required {
				line += "（必填）"
			}
			if f.Type == "select" && len(f.Options) > 0 {
				line += " 选项：" + strings.Join(f.Options, " / ")
			}
			if f.Type == "boolean" {
				line += "（是/否）"
			}
			if ph := strings.TrimSpace(f.Placeholder); ph != "" {
				line += " 例：" + ph
			}
			b.WriteString(line)
		}
		b.WriteString("\n\n格式示例：\n字段A: 值1\n字段B: 值2")
		if len(ask.Options) > 0 {
			b.WriteString("\n\n或选择：")
			for i, opt := range ask.Options {
				b.WriteString(fmt.Sprintf("\n%d. %s", i+1, opt))
			}
		}
		return b.String()
	}
	if len(ask.Options) == 0 {
		b.WriteString("\n\n请直接回复答案。")
		return b.String()
	}
	b.WriteString("\n")
	for i, opt := range ask.Options {
		b.WriteString(fmt.Sprintf("\n%d. %s", i+1, opt))
	}
	if strings.TrimSpace(ask.DefaultOpt) != "" {
		b.WriteString(fmt.Sprintf("\n\n默认：%s", ask.DefaultOpt))
	}
	b.WriteString("\n\n请回复选项序号或原文。")
	return b.String()
}

// formatFormAnswer mirrors desktop AskUserBlock: "label: value" lines.
func formatFormAnswer(fields []domain.AskUserFormField, values map[string]any) string {
	if len(fields) == 0 || values == nil {
		return ""
	}
	var lines []string
	for _, f := range fields {
		raw, ok := values[f.Name]
		if !ok {
			continue
		}
		label := strings.TrimSpace(f.Label)
		if label == "" {
			label = f.Name
		}
		val := strings.TrimSpace(fmt.Sprint(raw))
		if f.Type == "boolean" {
			switch strings.ToLower(val) {
			case "true", "1", "yes", "y", "是":
				val = "是"
			case "false", "0", "no", "n", "否":
				val = "否"
			}
		}
		lines = append(lines, label+": "+val)
	}
	return strings.Join(lines, "\n")
}

// formatPermissionText builds a numbered menu for tool permission.
func formatPermissionText(ask port.PermissionPrompt) string {
	var b strings.Builder
	b.WriteString("需要授权工具执行\n")
	if strings.TrimSpace(ask.ToolName) != "" {
		b.WriteString(fmt.Sprintf("\n工具：%s", ask.ToolName))
	}
	if strings.TrimSpace(ask.Summary) != "" {
		b.WriteString(fmt.Sprintf("\n%s", strings.TrimSpace(ask.Summary)))
	}
	b.WriteString("\n\n1. 允许一次\n2. 本会话允许\n3. 拒绝")
	b.WriteString("\n\n请回复序号，或点击按钮。")
	return b.String()
}

// resolveAskAnswer maps a peer reply to an option label when it looks like an index.
func resolveAskAnswer(text string, options []string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	// Exact option match (case-sensitive, trimmed).
	for _, opt := range options {
		if text == strings.TrimSpace(opt) {
			return opt
		}
	}
	// Numeric index 1..N
	var n int
	if _, err := fmt.Sscanf(text, "%d", &n); err == nil && n >= 1 && n <= len(options) {
		return options[n-1]
	}
	return text
}

// resolvePermissionReply maps text to approved/scope.
func resolvePermissionReply(text string) (approved bool, scope string, ok bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	switch t {
	case "1", "once", "允许一次", "allow", "yes", "y", "允许":
		return true, "once", true
	case "2", "session", "本会话允许", "always", "本会话":
		return true, "session", true
	case "3", "deny", "拒绝", "no", "n":
		return false, "once", true
	default:
		return false, "", false
	}
}

// preferOutboundKind picks the richest kind the endpoint can deliver.
func preferOutboundKind(caps port.ChannelCapabilities, want port.OutboundKind) port.OutboundKind {
	if want == port.OutboundKindCard || want == port.OutboundKindMarkdown {
		if caps.RichCards {
			return want
		}
		return port.OutboundKindText
	}
	if want == "" {
		return port.OutboundKindText
	}
	return want
}

// finalOutboundFromParts joins collected agent message parts into an OutboundMessage.
func finalOutboundFromParts(parts []string) port.OutboundMessage {
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		text = "（无文本回复）"
	}
	return port.OutboundMessage{
		Kind: port.OutboundKindMarkdown,
		Text: text,
	}
}

func permissionActions(approvalID string) []port.OutboundAction {
	return []port.OutboundAction{
		{ID: EncodeCallback(port.InteractionPermission, approvalID, "once"), Label: "允许一次"},
		{ID: EncodeCallback(port.InteractionPermission, approvalID, "session"), Label: "本会话允许"},
		{ID: EncodeCallback(port.InteractionPermission, approvalID, "deny"), Label: "拒绝"},
	}
}

func askActions(ask port.AskPrompt) []port.OutboundAction {
	actions := make([]port.OutboundAction, 0, len(ask.Options))
	for _, opt := range ask.Options {
		actions = append(actions, port.OutboundAction{
			ID:    EncodeCallback(port.InteractionAsk, ask.AskID, opt),
			Label: opt,
		})
	}
	return actions
}

func isProjectCommand(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	lower := strings.ToLower(t)
	return lower == "/project" || lower == "/projects" || lower == "/bot-project" ||
		strings.HasPrefix(lower, "/project ") || strings.HasPrefix(lower, "/bot-project ")
}

// channelToolDenied checks Meta["deny_tools"] (comma-separated) against a tool name.
func channelToolDenied(msg *port.InboundMessage, toolName string) (bool, string) {
	if msg == nil || msg.Meta == nil {
		return false, ""
	}
	raw := strings.TrimSpace(msg.Meta["deny_tools"])
	if raw == "" {
		return false, ""
	}
	toolName = strings.TrimSpace(toolName)
	for _, part := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(part), toolName) {
			return true, fmt.Sprintf("群策略禁止工具「%s」，已自动拒绝。", toolName)
		}
	}
	return false, ""
}
