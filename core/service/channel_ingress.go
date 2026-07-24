package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

	"gorm.io/gorm"
)

const channelBusyReply = "上一条消息还在处理中，请稍后再试。"

// ChannelIngressService turns normalized inbound chat into Session turns.
type ChannelIngressService struct {
	sessions *SessionManager
	projects *ProjectManager
	peers    port.ChannelPeerStore
	defaults port.ChannelDefaultsSource
}

func NewChannelIngress(sessions *SessionManager, projects *ProjectManager, peers port.ChannelPeerStore, defaults port.ChannelDefaultsSource) *ChannelIngressService {
	return &ChannelIngressService{
		sessions: sessions,
		projects: projects,
		peers:    peers,
		defaults: defaults,
	}
}

func (ing *ChannelIngressService) HandleInbound(ctx context.Context, msg port.InboundMessage) (string, error) {
	if strings.TrimSpace(msg.Text) == "" {
		return "", nil
	}
	if msg.PeerID == "" || msg.AccountID == "" {
		return "", fmt.Errorf("accountId and peerId required")
	}

	projectID, err := ing.peers.GetProjectID(ctx, msg.Type, msg.AccountID)
	if err != nil {
		return "", err
	}
	projectID = strings.TrimSpace(projectID)
	channelLabel := channelDisplayName(msg.Type)
	if projectID == "" {
		return fmt.Sprintf("请先在 Teams 设置 → %s 中为该账号绑定一个项目。", channelLabel), nil
	}
	if _, err := ing.projects.Get(ctx, projectID); err != nil {
		return fmt.Sprintf("绑定的项目不存在或已删除，请在设置 → %s 中重新绑定项目。", channelLabel), nil
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
		return ing.collectReplyFrom(ctx, sessionID, ch, turnID, defs.AutoApprove, msg.Type), nil
	}

	if contextToken != "" {
		if meta == nil || meta["context_token"] != contextToken {
			_ = ing.peers.UpdateBindingMeta(ctx, msg.Type, msg.AccountID, msg.PeerID, map[string]string{"context_token": contextToken})
		}
	}
	if ing.sessionHasRunningTurn(sessionID) {
		return channelBusyReply, nil
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
	return ing.collectReplyFrom(ctx, sessionID, ch, turnID, defs.AutoApprove, msg.Type), nil
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

func (ing *ChannelIngressService) applyEvent(ev domain.StreamEvent, turnID string, autoApprove bool, channel port.ChannelType, parts *[]string) (done bool) {
	if turnID != "" && ev.TurnID != "" && ev.TurnID != turnID {
		return false
	}
	askStub := fmt.Sprintf("（%s通道暂不支持交互提问，请在桌面端继续）", channelDisplayName(channel))
	switch ev.Type {
	case domain.EventAgentMessage:
		var p domain.AgentMessagePayload
		if json.Unmarshal(ev.Payload, &p) == nil && strings.TrimSpace(p.Text) != "" {
			*parts = append(*parts, strings.TrimSpace(p.Text))
		}
	case domain.EventPermissionAsk:
		if autoApprove {
			var p domain.PermissionAskPayload
			if json.Unmarshal(ev.Payload, &p) == nil && p.ApprovalID != "" {
				_ = ing.sessions.DecideApproval(context.Background(), p.ApprovalID, true, "once")
			}
		}
	case domain.EventAskUserPending:
		var p domain.AskUserPayload
		if json.Unmarshal(ev.Payload, &p) == nil && p.AskID != "" {
			_ = ing.sessions.ResolveAskUser(p.AskID, askStub)
		}
	case domain.EventTurnEnded, domain.EventTurnFailed, domain.EventError, domain.EventSessionCompleted:
		return true
	}
	return false
}

func (ing *ChannelIngressService) collectReplyFrom(ctx context.Context, sessionID string, ch <-chan domain.StreamEvent, turnID string, autoApprove bool, channel port.ChannelType) string {
	var parts []string
	for _, ev := range ing.sessions.StreamEvents(sessionID, 0) {
		if ing.applyEvent(ev, turnID, autoApprove, channel, &parts) {
			return strings.Join(parts, "\n")
		}
	}
	deadline := time.After(10 * time.Minute)
	seen := make(map[int64]struct{})
	for _, ev := range ing.sessions.StreamEvents(sessionID, 0) {
		seen[ev.Seq] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return strings.Join(parts, "\n")
		case <-deadline:
			if len(parts) == 0 {
				return "处理超时，请稍后在桌面端查看。"
			}
			return strings.Join(parts, "\n")
		case ev, ok := <-ch:
			if !ok {
				return strings.Join(parts, "\n")
			}
			if _, dup := seen[ev.Seq]; dup {
				continue
			}
			seen[ev.Seq] = struct{}{}
			if ing.applyEvent(ev, turnID, autoApprove, channel, &parts) {
				return strings.Join(parts, "\n")
			}
		}
	}
}
