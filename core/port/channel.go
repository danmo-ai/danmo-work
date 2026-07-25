package port

import (
	"context"
)

// ChannelType identifies an external chat platform.
type ChannelType string

const (
	ChannelWeixin ChannelType = "weixin"
	ChannelFeishu ChannelType = "feishu"
	ChannelWecom  ChannelType = "wecom"
)

// InboundMessage is the normalized inbound chat message (WeKnora-style).
type InboundMessage struct {
	Type      ChannelType
	AccountID string
	PeerID    string
	ChatID    string
	ThreadID  string
	Text      string
	MessageID string
	Meta      map[string]string // e.g. context_token, req_id, stream_id, receive_id
}

// OutboundKind selects the richest delivery shape an endpoint may use.
// Endpoints MUST always honor Text as a universal fallback.
type OutboundKind string

const (
	OutboundKindText     OutboundKind = "text"
	OutboundKindMarkdown OutboundKind = "markdown"
	OutboundKindCard     OutboundKind = "card"
)

// OutboundMessage is a structured reply. Ingress builds these; endpoints degrade
// by Capabilities (e.g. card → numbered text on Weixin).
type OutboundMessage struct {
	Kind  OutboundKind
	Text  string // always set — plain-text fallback / stream body
	Title string // optional heading for markdown/card
	Card  *OutboundCard
	Meta  map[string]string
}

// OutboundCard is a lightweight interactive card model (Feishu-oriented).
type OutboundCard struct {
	Title   string
	Body    string
	Actions []OutboundAction
}

// OutboundAction is a button / option on a card (or text menu).
type OutboundAction struct {
	ID    string // stable id for callbacks; empty → use Label
	Label string
}

// OutboundReply is retained for adapters that still take a simple text reply.
// Prefer OutboundMessage for new code.
type OutboundReply struct {
	Content string
	Meta    map[string]string
}

// TextOutbound builds a plain-text OutboundMessage.
func TextOutbound(text string) OutboundMessage {
	return OutboundMessage{Kind: OutboundKindText, Text: text}
}

// ChannelCapabilities declares what a platform endpoint can do.
// Ingress orchestrates one turn pipeline and branches on these flags.
type ChannelCapabilities struct {
	// ProgressiveStream: mid-turn content updates (WeCom stream; others may emulate).
	ProgressiveStream bool
	// RichCards: markdown/card kinds beyond plain text.
	RichCards bool
	// InteractiveAsk: can present ask_user options and await a peer reply
	// (card buttons or text menu) instead of auto-stubbing.
	InteractiveAsk bool
}

// ChannelEndpoint is the platform-facing delivery surface registered with ingress.
// Bridges implement this (directly or via adapter wrappers). Optional behavior is
// composed through StreamSender / ChannelInteractor type assertions.
type ChannelEndpoint interface {
	Type() ChannelType
	Capabilities() ChannelCapabilities
	// Deliver sends a standalone or final message using the richest supported kind.
	Deliver(ctx context.Context, in *InboundMessage, msg OutboundMessage) error
}

// StreamSender is an optional ChannelEndpoint extension for progressive replies.
// Ingress calls Start → Update*(full replacement text) → Finish.
// Platforms that already emitted a placeholder (WeCom ~5s rule) may no-op Start
// and reuse Meta["stream_id"].
type StreamSender interface {
	StartStream(ctx context.Context, in *InboundMessage) (streamID string, err error)
	UpdateStream(ctx context.Context, in *InboundMessage, streamID, fullContent string) error
	FinishStream(ctx context.Context, in *InboundMessage, streamID string, final OutboundMessage) error
}

// AskPrompt is the normalized ask_user presentation for IM channels.
type AskPrompt struct {
	AskID      string
	Question   string
	Options    []string
	DefaultOpt string
}

// ChannelInteractor is an optional ChannelEndpoint extension for ask_user.
// PresentAsk should deliver the question (card/menu/text). Returning handled=true
// means ingress will wait for the next peer message to ResolveAskUser instead of
// auto-stubbing.
type ChannelInteractor interface {
	PresentAsk(ctx context.Context, in *InboundMessage, ask AskPrompt) (handled bool, err error)
}

// ChannelDefaults are channel-level agent/model/auto-approve settings.
type ChannelDefaults struct {
	AgentID     string
	ModelID     string
	AutoApprove bool
}

// ChannelStatus is a generic runtime status snapshot.
type ChannelStatus struct {
	Type    ChannelType `json:"type"`
	Enabled bool        `json:"enabled"`
	Running bool        `json:"running"`
}

// ChannelPeerStore resolves project binding and peer→session mappings.
// Weixin keeps its own tables; Feishu (and later channels) may share a generic table.
type ChannelPeerStore interface {
	GetProjectID(ctx context.Context, channel ChannelType, accountID string) (string, error)
	GetBinding(ctx context.Context, channel ChannelType, accountID, peerID string) (sessionID string, meta map[string]string, err error)
	UpsertBinding(ctx context.Context, channel ChannelType, accountID, peerID, sessionID string, meta map[string]string) error
	UpdateBindingMeta(ctx context.Context, channel ChannelType, accountID, peerID string, meta map[string]string) error
}

// ChannelDefaultsSource loads per-channel Agent/Model/AutoApprove.
type ChannelDefaultsSource interface {
	ChannelDefaults(ctx context.Context, channel ChannelType) (ChannelDefaults, error)
}

// ChannelRuntime manages long-lived connections (long-poll / WebSocket).
type ChannelRuntime interface {
	Type() ChannelType
	SyncFromConfig(ctx context.Context) error
	Stop()
	IsRunning() bool
}

// ChannelIngress turns an InboundMessage into a Teams Session turn and delivers
// the reply through the registered ChannelEndpoint for that channel type.
//
// Return value:
//   - When an endpoint is registered, ingress delivers itself and returns ("", nil)
//     on success (bridges must not send again).
//   - When no endpoint is registered, returns plain reply text for the bridge to send
//     (backward-compatible test / partial wiring path).
type ChannelIngress interface {
	RegisterEndpoint(ep ChannelEndpoint)
	HandleInbound(ctx context.Context, msg InboundMessage) (reply string, err error)
}
