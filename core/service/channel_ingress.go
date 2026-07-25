package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"gorm.io/gorm"
)

const (
	channelBusyReply     = "上一条消息还在处理中，请稍后再试。"
	channelProcessingMsg = "正在处理…"
	channelAskAckMsg     = "已收到。"
)

// ChannelIngressService orchestrates IM → Session turns.
// Platform differences live behind ChannelEndpoint (+ optional StreamSender / ChannelInteractor).
type ChannelIngressService struct {
	sessions *SessionManager
	projects *ProjectManager
	peers    port.ChannelPeerStore
	defaults port.ChannelDefaultsSource

	mu        sync.RWMutex
	endpoints map[port.ChannelType]port.ChannelEndpoint

	// In-memory pending ask_user waits keyed by channel|account|peer.
	// Kept here (not binding meta) so Weixin — which only stores context_token — works too.
	askMu       sync.Mutex
	pendingAsks map[string]pendingAsk
}

type pendingAsk struct {
	AskID   string
	Options []string
}

func NewChannelIngress(sessions *SessionManager, projects *ProjectManager, peers port.ChannelPeerStore, defaults port.ChannelDefaultsSource) *ChannelIngressService {
	return &ChannelIngressService{
		sessions:    sessions,
		projects:    projects,
		peers:       peers,
		defaults:    defaults,
		endpoints:   make(map[port.ChannelType]port.ChannelEndpoint),
		pendingAsks: make(map[string]pendingAsk),
	}
}

func (ing *ChannelIngressService) RegisterEndpoint(ep port.ChannelEndpoint) {
	if ep == nil {
		return
	}
	ing.mu.Lock()
	defer ing.mu.Unlock()
	ing.endpoints[ep.Type()] = ep
}

func (ing *ChannelIngressService) endpoint(t port.ChannelType) port.ChannelEndpoint {
	ing.mu.RLock()
	defer ing.mu.RUnlock()
	return ing.endpoints[t]
}

func peerKey(msg port.InboundMessage) string {
	return string(msg.Type) + "|" + msg.AccountID + "|" + msg.PeerID
}

func (ing *ChannelIngressService) HandleInbound(ctx context.Context, msg port.InboundMessage) (string, error) {
	if strings.TrimSpace(msg.Text) == "" {
		return "", nil
	}
	if msg.PeerID == "" || msg.AccountID == "" {
		return "", fmt.Errorf("accountId and peerId required")
	}

	// Pending ask_user answer takes priority over a new turn.
	if ing.sessions != nil {
		if handled, err := ing.tryResolvePendingAsk(ctx, msg); handled || err != nil {
			return "", err
		}
	}

	projectID, err := ing.peers.GetProjectID(ctx, msg.Type, msg.AccountID)
	if err != nil {
		return "", err
	}
	projectID = strings.TrimSpace(projectID)
	channelLabel := channelDisplayName(msg.Type)
	if projectID == "" {
		return ing.deliverOrReturn(ctx, &msg, fmt.Sprintf("请先在 Teams 设置 → %s 中为该账号绑定一个项目。", channelLabel))
	}
	if _, err := ing.projects.Get(ctx, projectID); err != nil {
		return ing.deliverOrReturn(ctx, &msg, fmt.Sprintf("绑定的项目不存在或已删除，请在设置 → %s 中重新绑定项目。", channelLabel))
	}

	defs, err := ing.defaults.ChannelDefaults(ctx, msg.Type)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(defs.AgentID) == "" {
		return "", fmt.Errorf("未配置默认 Agent，请在设置中选择")
	}
	modelID := strings.TrimSpace(defs.ModelID)
	if modelID == "" || !strings.Contains(modelID, "/") {
		return "", fmt.Errorf("未配置默认模型，请在设置 → %s 中选择模型（格式 provider/model）", channelLabel)
	}

	sessionID, meta, err := ing.peers.GetBinding(ctx, msg.Type, msg.AccountID, msg.PeerID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	newSession := errors.Is(err, gorm.ErrRecordNotFound) || sessionID == ""

	contextToken := ""
	if msg.Meta != nil {
		contextToken = msg.Meta["context_token"]
	}

	if newSession {
		s, cerr := ing.sessions.Create(ctx, domain.CreateSessionRequest{
			Content:       msg.Text,
			AgentID:       defs.AgentID,
			ProjectID:     projectID,
			ModelID:       modelID,
			Title:         channelSessionTitle(msg.Type, msg.Text),
			SkipAutoTitle: true,
		})
		if cerr != nil {
			return "", cerr
		}
		sessionID = s.ID
		bindMeta := map[string]string{}
		if contextToken != "" {
			bindMeta["context_token"] = contextToken
		}
		if uerr := ing.peers.UpsertBinding(ctx, msg.Type, msg.AccountID, msg.PeerID, sessionID, bindMeta); uerr != nil {
			return "", uerr
		}
		ch := ing.sessions.Subscribe(sessionID)
		defer ing.sessions.Unsubscribe(sessionID, ch)
		turnID := ing.waitLatestTurnID(sessionID, 2*time.Second)
		return ing.runTurnPipeline(ctx, &msg, sessionID, ch, turnID, defs.AutoApprove)
	}

	if contextToken != "" {
		if meta == nil || meta["context_token"] != contextToken {
			_ = ing.peers.UpdateBindingMeta(ctx, msg.Type, msg.AccountID, msg.PeerID, map[string]string{"context_token": contextToken})
		}
	}
	if ing.sessionHasRunningTurn(sessionID) {
		return ing.deliverOrReturn(ctx, &msg, channelBusyReply)
	}
	if s, gerr := ing.sessions.Get(ctx, sessionID); gerr == nil && strings.TrimSpace(s.ModelID) == "" {
		_, _ = ing.sessions.Update(ctx, sessionID, domain.UpdateSessionRequest{ModelID: &modelID})
	}
	ch := ing.sessions.Subscribe(sessionID)
	defer ing.sessions.Unsubscribe(sessionID, ch)
	turnID, serr := ing.sessions.StartTurn(ctx, sessionID, domain.SendMessageRequest{
		UserInput: msg.Text,
		AgentID:   defs.AgentID,
		ModelID:   modelID,
	})
	if serr != nil {
		return "", serr
	}
	return ing.runTurnPipeline(ctx, &msg, sessionID, ch, turnID, defs.AutoApprove)
}

// tryResolvePendingAsk routes a peer reply to a waiting ask_user.
func (ing *ChannelIngressService) tryResolvePendingAsk(ctx context.Context, msg port.InboundMessage) (handled bool, err error) {
	key := peerKey(msg)
	ing.askMu.Lock()
	pending, ok := ing.pendingAsks[key]
	ing.askMu.Unlock()
	if !ok || pending.AskID == "" {
		return false, nil
	}
	answer := resolveAskAnswer(msg.Text, pending.Options)
	if rerr := ing.sessions.ResolveAskUser(pending.AskID, answer); rerr != nil {
		log.Printf("[channel] resolve ask_user %s: %v", pending.AskID, rerr)
		// Drop stale pending so the peer is not stuck; fall through to a new turn.
		ing.askMu.Lock()
		if cur, still := ing.pendingAsks[key]; still && cur.AskID == pending.AskID {
			delete(ing.pendingAsks, key)
		}
		ing.askMu.Unlock()
		return false, nil
	}
	ing.askMu.Lock()
	if cur, still := ing.pendingAsks[key]; still && cur.AskID == pending.AskID {
		delete(ing.pendingAsks, key)
	}
	ing.askMu.Unlock()
	ep := ing.endpoint(msg.Type)
	// WeCom already opened a stream for this inbound — finish it with a short ack.
	if sender, ok := ep.(port.StreamSender); ok && ep != nil {
		caps := ep.Capabilities()
		if caps.ProgressiveStream {
			streamID := ""
			if msg.Meta != nil {
				streamID = msg.Meta["stream_id"]
			}
			if streamID == "" {
				if sid, serr := sender.StartStream(ctx, &msg); serr == nil {
					streamID = sid
				}
			}
			if streamID != "" {
				_ = sender.FinishStream(ctx, &msg, streamID, port.TextOutbound(channelAskAckMsg))
				return true, nil
			}
		}
	}
	_, _ = ing.deliverOrReturn(ctx, &msg, channelAskAckMsg)
	return true, nil
}

// runTurnPipeline watches session stream events, optionally progressive-streams,
// handles ask_user / permissions, and delivers the final outbound via the endpoint.
func (ing *ChannelIngressService) runTurnPipeline(ctx context.Context, msg *port.InboundMessage, sessionID string, ch <-chan domain.StreamEvent, turnID string, autoApprove bool) (string, error) {
	ep := ing.endpoint(msg.Type)
	caps := port.ChannelCapabilities{}
	if ep != nil {
		caps = ep.Capabilities()
	}

	var sender port.StreamSender
	streamID := ""
	if s, ok := ep.(port.StreamSender); ok && caps.ProgressiveStream {
		sender = s
		var err error
		streamID, err = sender.StartStream(ctx, msg)
		if err != nil {
			log.Printf("[channel] StartStream %s: %v", msg.Type, err)
			sender = nil
			streamID = ""
		}
	}

	final := ing.collectReplyFrom(ctx, collectParams{
		sessionID:   sessionID,
		ch:          ch,
		turnID:      turnID,
		autoApprove: autoApprove,
		msg:         msg,
		ep:          ep,
		caps:        caps,
		sender:      sender,
		streamID:    streamID,
	})

	if sender != nil && streamID != "" {
		if err := sender.FinishStream(ctx, msg, streamID, final); err != nil {
			log.Printf("[channel] FinishStream %s: %v", msg.Type, err)
			// Fall back to Deliver so the user still gets a reply.
			return ing.deliverOrReturn(ctx, msg, final.Text)
		}
		if ep != nil {
			return "", nil
		}
		return final.Text, nil
	}
	return ing.deliverOrReturn(ctx, msg, final.Text, final)
}

// deliverOrReturn sends via endpoint when registered; otherwise returns text for the bridge.
func (ing *ChannelIngressService) deliverOrReturn(ctx context.Context, msg *port.InboundMessage, text string, rich ...port.OutboundMessage) (string, error) {
	ep := ing.endpoint(msg.Type)
	out := port.TextOutbound(text)
	if len(rich) > 0 {
		out = rich[0]
		if strings.TrimSpace(out.Text) == "" {
			out.Text = text
		}
	}
	if ep == nil {
		return out.Text, nil
	}
	out.Kind = preferOutboundKind(ep.Capabilities(), out.Kind)
	if err := ep.Deliver(ctx, msg, out); err != nil {
		return "", err
	}
	return "", nil
}

func channelDisplayName(t port.ChannelType) string {
	switch t {
	case port.ChannelWeixin:
		return "微信"
	case port.ChannelFeishu:
		return "飞书"
	case port.ChannelWecom:
		return "企业微信"
	default:
		return string(t)
	}
}

func channelSessionTitle(t port.ChannelType, text string) string {
	title := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if title == "" {
		return channelDisplayName(t) + "会话"
	}
	runes := []rune(title)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	return title
}

func (ing *ChannelIngressService) sessionHasRunningTurn(sessionID string) bool {
	for _, t := range ing.sessions.ListTurns(sessionID) {
		if t.Status == domain.TurnRunning {
			return true
		}
	}
	return false
}

func (ing *ChannelIngressService) waitLatestTurnID(sessionID string, wait time.Duration) string {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		turns := ing.sessions.ListTurns(sessionID)
		if len(turns) > 0 {
			return turns[len(turns)-1].ID
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

type collectParams struct {
	sessionID   string
	ch          <-chan domain.StreamEvent
	turnID      string
	autoApprove bool
	msg         *port.InboundMessage
	ep          port.ChannelEndpoint
	caps        port.ChannelCapabilities
	sender      port.StreamSender
	streamID    string
}

func (ing *ChannelIngressService) applyEvent(ev domain.StreamEvent, p collectParams, parts *[]string) (done bool) {
	if p.turnID != "" && ev.TurnID != "" && ev.TurnID != p.turnID {
		return false
	}
	switch ev.Type {
	case domain.EventAgentMessage:
		var payload domain.AgentMessagePayload
		if json.Unmarshal(ev.Payload, &payload) == nil && strings.TrimSpace(payload.Text) != "" {
			*parts = append(*parts, strings.TrimSpace(payload.Text))
			if p.sender != nil && p.streamID != "" {
				full := strings.Join(*parts, "\n")
				if err := p.sender.UpdateStream(context.Background(), p.msg, p.streamID, full); err != nil {
					log.Printf("[channel] UpdateStream %s: %v", p.msg.Type, err)
				}
			}
		}
	case domain.EventPermissionAsk:
		if p.autoApprove {
			var payload domain.PermissionAskPayload
			if json.Unmarshal(ev.Payload, &payload) == nil && payload.ApprovalID != "" {
				_ = ing.sessions.DecideApproval(context.Background(), payload.ApprovalID, true, "once")
			}
		}
	case domain.EventAskUserPending:
		ing.handleAskUserPending(ev, p)
	case domain.EventTurnEnded, domain.EventTurnFailed, domain.EventError, domain.EventSessionCompleted:
		return true
	}
	return false
}

func (ing *ChannelIngressService) handleAskUserPending(ev domain.StreamEvent, p collectParams) {
	var payload domain.AskUserPayload
	if json.Unmarshal(ev.Payload, &payload) != nil || payload.AskID == "" {
		return
	}
	ask := port.AskPrompt{
		AskID:      payload.AskID,
		Question:   payload.Question,
		Options:    payload.Options,
		DefaultOpt: payload.DefaultOpt,
	}

	handled := false
	if p.caps.InteractiveAsk && p.ep != nil {
		if interactor, ok := p.ep.(port.ChannelInteractor); ok {
			okHandled, err := interactor.PresentAsk(context.Background(), p.msg, ask)
			if err != nil {
				log.Printf("[channel] PresentAsk %s: %v", p.msg.Type, err)
			} else {
				handled = okHandled
			}
		}
	}
	if !handled && p.caps.InteractiveAsk {
		// Generic text-menu fallback when endpoint claims InteractiveAsk but has no interactor.
		_, _ = ing.deliverOrReturn(context.Background(), p.msg, formatAskText(ask))
		handled = true
	}
	if handled {
		ing.askMu.Lock()
		ing.pendingAsks[peerKey(*p.msg)] = pendingAsk{AskID: ask.AskID, Options: append([]string(nil), ask.Options...)}
		ing.askMu.Unlock()
		return
	}
	// Channels without interactive ask: stub so the turn can continue on desktop later.
	stub := fmt.Sprintf("（%s通道暂不支持交互提问，请在桌面端继续）", channelDisplayName(p.msg.Type))
	_ = ing.sessions.ResolveAskUser(ask.AskID, stub)
}

func (ing *ChannelIngressService) collectReplyFrom(ctx context.Context, p collectParams) port.OutboundMessage {
	var parts []string
	for _, ev := range ing.sessions.StreamEvents(p.sessionID, 0) {
		if ing.applyEvent(ev, p, &parts) {
			return finalOutboundFromParts(parts)
		}
	}
	deadline := time.After(10 * time.Minute)
	seen := make(map[int64]struct{})
	for _, ev := range ing.sessions.StreamEvents(p.sessionID, 0) {
		seen[ev.Seq] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return finalOutboundFromParts(parts)
		case <-deadline:
			if len(parts) == 0 {
				return port.TextOutbound("处理超时，请稍后在桌面端查看。")
			}
			return finalOutboundFromParts(parts)
		case ev, ok := <-p.ch:
			if !ok {
				return finalOutboundFromParts(parts)
			}
			if _, dup := seen[ev.Seq]; dup {
				continue
			}
			seen[ev.Seq] = struct{}{}
			if ing.applyEvent(ev, p, &parts) {
				return finalOutboundFromParts(parts)
			}
		}
	}
}
