package service

import (
	"fmt"
	"strings"

	"danmo-work/core/port"
)

// formatAskText builds a numbered menu for channels without native cards.
func formatAskText(ask port.AskPrompt) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(ask.Question))
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
