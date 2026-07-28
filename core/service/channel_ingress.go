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
	channelPermAckMsg    = "已处理授权。"
	maxProgressLines     = 6
	progressMinInterval  = 800 * time.Millisecond
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

	// In-memory pending ask_user / permission waits keyed by channel|account|peer.
	askMu        sync.Mutex
	pendingAsks  map[string]pendingAsk
	pendingPerms map[string]pendingPerm
}

type pendingAsk struct {
	AskID      string
	Options    []string
	FormFields []domain.AskUserFormField
}

type pendingPerm struct {
	ApprovalID string
}

func NewChannelIngress(sessions *SessionManager, projects *ProjectManager, peers port.ChannelPeerStore, defaults port.ChannelDefaultsSource) *ChannelIngressService {
	return &ChannelIngressService{
		sessions:     sessions,
		projects:     projects,
		peers:        peers,
		defaults:     defaults,
		endpoints:    make(map[port.ChannelType]port.ChannelEndpoint),
		pendingAsks:  make(map[string]pendingAsk),
		pendingPerms: make(map[string]pendingPerm),
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
	if strings.TrimSpace(msg.Text) == "" && len(msg.Media) == 0 {
		return "", nil
	}
	if msg.PeerID == "" || msg.AccountID == "" {
		return "", fmt.Errorf("accountId and peerId required")
	}
	msg.Text = FormatMediaUserText(msg.Text, msg.Media)
	if note := strings.TrimSpace(msg.Meta["policy_note"]); note != "" {
		msg.Text = strings.TrimSpace(msg.Text + "\n\n" + note)
	}
	if strings.TrimSpace(msg.Text) == "" {
		return "", nil
	}

	// Button-style tokens pasted as text.
	if ev, ok := InteractionFromCallback(msg, strings.TrimSpace(msg.Text)); ok {
		return "", ing.HandleInteraction(ctx, ev)
	}

	if ing.sessions != nil {
		if handled, err := ing.tryResolvePendingAsk(ctx, msg); handled || err != nil {
			return "", err
		}
		if handled, err := ing.tryResolvePendingPerm(ctx, msg); handled || err != nil {
			return "", err
		}
	}

	if isProjectCommand(msg.Text) {
		return ing.handleProjectCommand(ctx, msg)
	}

	projectID, bindMeta, err := ing.resolvePeerProject(ctx, msg)
	if err != nil {
		return "", err
	}
	channelLabel := channelDisplayName(msg.Type)
	if projectID == "" {
		return ing.deliverOrReturn(ctx, &msg, fmt.Sprintf("请先在 Teams 设置 → %s 中绑定默认项目，或发送 /project 选择项目。", channelLabel))
	}
	if _, err := ing.projects.Get(ctx, projectID); err != nil {
		return ing.deliverOrReturn(ctx, &msg, fmt.Sprintf("绑定的项目不存在或已删除，请发送 /project 重新选择，或在设置 → %s 中绑定。", channelLabel))
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
	if meta == nil {
		meta = bindMeta
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
			Attachments:   MediaToVisionAttachments(msg.Media),
		})
		if cerr != nil {
			return "", cerr
		}
		sessionID = s.ID
		bindMeta := map[string]string{}
		if meta != nil {
			for k, v := range meta {
				bindMeta[k] = v
			}
		}
		if contextToken != "" {
			bindMeta["context_token"] = contextToken
		}
		if projectID != "" {
			bindMeta["project_id"] = projectID
		}
		if uerr := ing.peers.UpsertBinding(ctx, msg.Type, msg.AccountID, msg.PeerID, sessionID, bindMeta); uerr != nil {
			return "", uerr
		}
		ch := ing.sessions.Subscribe(sessionID)
		defer ing.sessions.Unsubscribe(sessionID, ch)
		turnID := ing.waitLatestTurnID(sessionID, 2*time.Second)
		return ing.runTurnPipeline(ctx, &msg, sessionID, ch, turnID, defs.AutoApprove)
	}

	// Existing session bound to a different project → start a fresh session.
	if s, gerr := ing.sessions.Get(ctx, sessionID); gerr == nil && strings.TrimSpace(s.ProjectID) != "" && s.ProjectID != projectID {
		_ = ing.peers.UpsertBinding(ctx, msg.Type, msg.AccountID, msg.PeerID, "", mergeStringMap(meta, map[string]string{"project_id": projectID}))
		return ing.HandleInbound(ctx, msg)
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
		UserInput:   msg.Text,
		AgentID:     defs.AgentID,
		ModelID:     modelID,
		Attachments: MediaToVisionAttachments(msg.Media),
	})
	if serr != nil {
		return "", serr
	}
	return ing.runTurnPipeline(ctx, &msg, sessionID, ch, turnID, defs.AutoApprove)
}

// HandleInteraction routes card/keyboard callbacks without starting a new turn.
func (ing *ChannelIngressService) HandleInteraction(ctx context.Context, ev port.InteractionEvent) error {
	if ev.PeerID == "" || ev.AccountID == "" {
		return fmt.Errorf("interaction: accountId and peerId required")
	}
	msg := port.InboundMessage{
		Type:      ev.Type,
		AccountID: ev.AccountID,
		PeerID:    ev.PeerID,
		ChatID:    ev.ChatID,
		MessageID: ev.MessageID,
		Meta:      ev.Meta,
	}
	switch ev.Kind {
	case port.InteractionAsk:
		if ev.TargetID == "" {
			return nil
		}
		answer := ev.Option
		if answer == "" || answer == "form" {
			key := peerKey(msg)
			ing.askMu.Lock()
			pending := ing.pendingAsks[key]
			ing.askMu.Unlock()
			if ev.Meta != nil && ev.Meta["form_json"] != "" && len(pending.FormFields) > 0 {
				var values map[string]any
				if json.Unmarshal([]byte(ev.Meta["form_json"]), &values) == nil {
					if formatted := formatFormAnswer(pending.FormFields, values); formatted != "" {
						answer = formatted
					}
				}
			}
		}
		if answer == "" || answer == "form" {
			answer = ev.Raw
		}
		if err := ing.sessions.ResolveAskUser(ev.TargetID, answer); err != nil {
			log.Printf("[channel] interaction ask %s: %v", ev.TargetID, err)
		}
		ing.clearPendingAsk(peerKey(msg), ev.TargetID)
		_, _ = ing.deliverOrReturn(ctx, &msg, channelAskAckMsg)
		return nil
	case port.InteractionPermission:
		if ev.TargetID == "" || ing.sessions == nil {
			return nil
		}
		approved := true
		scope := "once"
		switch strings.TrimSpace(ev.Option) {
		case "deny", "reject":
			approved = false
			scope = "once"
		case "session":
			approved = true
			scope = "session"
		case "once", "":
			approved = true
			scope = "once"
		default:
			approved = true
			scope = "once"
		}
		if err := ing.sessions.DecideApproval(ctx, ev.TargetID, approved, scope); err != nil {
			log.Printf("[channel] interaction perm %s: %v", ev.TargetID, err)
		}
		ing.clearPendingPerm(peerKey(msg), ev.TargetID)
		_, _ = ing.deliverOrReturn(ctx, &msg, channelPermAckMsg)
		return nil
	case port.InteractionProject:
		return ing.applyProjectSelection(ctx, msg, ev.TargetID)
	default:
		return nil
	}
}

func (ing *ChannelIngressService) resolvePeerProject(ctx context.Context, msg port.InboundMessage) (string, map[string]string, error) {
	_, meta, err := ing.peers.GetBinding(ctx, msg.Type, msg.AccountID, msg.PeerID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, err
	}
	if meta != nil {
		if pid := strings.TrimSpace(meta["project_id"]); pid != "" {
			return pid, meta, nil
		}
	}
	projectID, err := ing.peers.GetProjectID(ctx, msg.Type, msg.AccountID)
	if err != nil {
		return "", meta, err
	}
	return strings.TrimSpace(projectID), meta, nil
}

func (ing *ChannelIngressService) handleProjectCommand(ctx context.Context, msg port.InboundMessage) (string, error) {
	parts := strings.Fields(strings.TrimSpace(msg.Text))
	if len(parts) >= 2 {
		arg := strings.TrimSpace(parts[1])
		// Resolve by id or name.
		projects, err := ing.projects.List(ctx)
		if err != nil {
			return "", err
		}
		var match string
		for _, p := range projects {
			if p.ID == arg || strings.EqualFold(p.Name, arg) {
				match = p.ID
				break
			}
		}
		if match == "" {
			return ing.deliverOrReturn(ctx, &msg, "未找到项目："+arg+"\n发送 /project 查看列表。")
		}
		if err := ing.applyProjectSelection(ctx, msg, match); err != nil {
			return "", err
		}
		return "", nil
	}
	return ing.presentProjectPicker(ctx, msg)
}

func (ing *ChannelIngressService) presentProjectPicker(ctx context.Context, msg port.InboundMessage) (string, error) {
	projects, err := ing.projects.List(ctx)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return ing.deliverOrReturn(ctx, &msg, "还没有项目，请先在桌面端创建。")
	}
	cur, _, _ := ing.resolvePeerProject(ctx, msg)
	var b strings.Builder
	b.WriteString("选择项目（回复 /project <名称或ID>，或点击按钮）：\n")
	actions := make([]port.OutboundAction, 0, len(projects))
	for i, p := range projects {
		mark := ""
		if p.ID == cur {
			mark = " ← 当前"
		}
		b.WriteString(fmt.Sprintf("\n%d. %s (%s)%s", i+1, p.Name, p.ID, mark))
		actions = append(actions, port.OutboundAction{
			ID:    EncodeCallback(port.InteractionProject, p.ID, ""),
			Label: p.Name,
		})
	}
	out := port.OutboundMessage{
		Kind:  port.OutboundKindCard,
		Title: "切换项目",
		Text:  b.String(),
		Card: &port.OutboundCard{
			Title:   "切换项目",
			Body:    strings.TrimSpace(b.String()),
			Actions: actions,
		},
	}
	return ing.deliverOrReturn(ctx, &msg, out.Text, out)
}

func (ing *ChannelIngressService) applyProjectSelection(ctx context.Context, msg port.InboundMessage, projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil
	}
	if _, err := ing.projects.Get(ctx, projectID); err != nil {
		_, _ = ing.deliverOrReturn(ctx, &msg, "项目不存在："+projectID)
		return nil
	}
	_, meta, err := ing.peers.GetBinding(ctx, msg.Type, msg.AccountID, msg.PeerID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	meta = mergeStringMap(meta, map[string]string{"project_id": projectID})
	// Clear session so the next message opens a turn in the new project.
	if err := ing.peers.UpsertBinding(ctx, msg.Type, msg.AccountID, msg.PeerID, "", meta); err != nil {
		return err
	}
	p, _ := ing.projects.Get(ctx, projectID)
	name := p.Name
	if name == "" {
		name = projectID
	}
	_, _ = ing.deliverOrReturn(ctx, &msg, fmt.Sprintf("已切换到项目「%s」。下一条消息将在新会话中处理。", name))
	return nil
}

func mergeStringMap(base, patch map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}

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
		ing.clearPendingAsk(key, pending.AskID)
		return false, nil
	}
	ing.clearPendingAsk(key, pending.AskID)
	ep := ing.endpoint(msg.Type)
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

func (ing *ChannelIngressService) tryResolvePendingPerm(ctx context.Context, msg port.InboundMessage) (handled bool, err error) {
	key := peerKey(msg)
	ing.askMu.Lock()
	pending, ok := ing.pendingPerms[key]
	ing.askMu.Unlock()
	if !ok || pending.ApprovalID == "" {
		return false, nil
	}
	approved, scope, ok := resolvePermissionReply(msg.Text)
	if !ok {
		return false, nil
	}
	if rerr := ing.sessions.DecideApproval(ctx, pending.ApprovalID, approved, scope); rerr != nil {
		log.Printf("[channel] resolve permission %s: %v", pending.ApprovalID, rerr)
		ing.clearPendingPerm(key, pending.ApprovalID)
		return false, nil
	}
	ing.clearPendingPerm(key, pending.ApprovalID)
	_, _ = ing.deliverOrReturn(ctx, &msg, channelPermAckMsg)
	return true, nil
}

func (ing *ChannelIngressService) clearPendingAsk(key, askID string) {
	ing.askMu.Lock()
	defer ing.askMu.Unlock()
	if cur, still := ing.pendingAsks[key]; still && (askID == "" || cur.AskID == askID) {
		delete(ing.pendingAsks, key)
	}
}

func (ing *ChannelIngressService) clearPendingPerm(key, approvalID string) {
	ing.askMu.Lock()
	defer ing.askMu.Unlock()
	if cur, still := ing.pendingPerms[key]; still && (approvalID == "" || cur.ApprovalID == approvalID) {
		delete(ing.pendingPerms, key)
	}
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
	var progress port.ProgressUpdater
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
	if p, ok := ep.(port.ProgressUpdater); ok {
		progress = p
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
		progress:    progress,
		streamID:    streamID,
	})

	if sender != nil && streamID != "" {
		if err := sender.FinishStream(ctx, msg, streamID, final); err != nil {
			log.Printf("[channel] FinishStream %s: %v", msg.Type, err)
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
	case port.ChannelQQ:
		return "QQ"
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
	progress    port.ProgressUpdater
	streamID    string
}

func (ing *ChannelIngressService) applyEvent(ev domain.StreamEvent, p collectParams, parts *[]string, toolLines *[]string, lastProgress *time.Time, failed *bool) (done bool) {
	if p.turnID != "" && ev.TurnID != "" && ev.TurnID != p.turnID {
		return false
	}
	switch ev.Type {
	case domain.EventAgentMessage:
		var payload domain.AgentMessagePayload
		if json.Unmarshal(ev.Payload, &payload) == nil && strings.TrimSpace(payload.Text) != "" {
			*parts = append(*parts, strings.TrimSpace(payload.Text))
			ing.emitProgress(p, *parts, *toolLines, "running", "执行中…", lastProgress, true)
		}
	case domain.EventToolRunning, domain.EventToolPending:
		var tp domain.ToolPart
		if json.Unmarshal(ev.Payload, &tp) == nil {
			line := "⟳ " + strings.TrimSpace(tp.Name)
			if d := strings.TrimSpace(tp.Description); d != "" {
				line += " · " + truncateRunes(d, 80)
			}
			*toolLines = appendToolLine(*toolLines, line)
			ing.emitProgress(p, *parts, *toolLines, "tool", "执行中…", lastProgress, false)
		}
	case domain.EventToolCompleted:
		var tp domain.ToolPart
		if json.Unmarshal(ev.Payload, &tp) == nil {
			line := "✓ " + strings.TrimSpace(tp.Name)
			*toolLines = appendToolLine(*toolLines, line)
			ing.emitProgress(p, *parts, *toolLines, "tool", "执行中…", lastProgress, false)
		}
	case domain.EventToolError:
		var tp domain.ToolPart
		if json.Unmarshal(ev.Payload, &tp) == nil {
			line := "✗ " + strings.TrimSpace(tp.Name)
			if e := strings.TrimSpace(tp.Error); e != "" {
				line += " · " + truncateRunes(e, 60)
			}
			*toolLines = appendToolLine(*toolLines, line)
			ing.emitProgress(p, *parts, *toolLines, "error", "工具出错", lastProgress, true)
		}
	case domain.EventPermissionAsk:
		ing.handlePermissionAsk(ev, p)
	case domain.EventAskUserPending:
		ing.handleAskUserPending(ev, p)
	case domain.EventTurnFailed, domain.EventError:
		if failed != nil {
			*failed = true
		}
		return true
	case domain.EventTurnEnded, domain.EventSessionCompleted:
		return true
	}
	return false
}

func (ing *ChannelIngressService) emitProgress(p collectParams, parts, toolLines []string, status, headline string, last *time.Time, force bool) {
	if p.streamID == "" {
		return
	}
	// Keep approval / ask buttons stable on the progress card while waiting.
	if p.msg != nil && (ing.hasPendingPerm(peerKey(*p.msg)) || ing.hasPendingAsk(peerKey(*p.msg))) {
		return
	}
	now := time.Now()
	if !force && last != nil && !last.IsZero() && now.Sub(*last) < progressMinInterval {
		return
	}
	if last != nil {
		*last = now
	}
	full := strings.Join(parts, "\n")
	if p.progress != nil {
		if err := p.progress.UpdateProgress(context.Background(), p.msg, p.streamID, port.ProgressSnapshot{
			Status:   status,
			Headline: headline,
			Lines:    append([]string(nil), toolLines...),
			TextBody: full,
		}); err != nil {
			log.Printf("[channel] UpdateProgress %s: %v", p.msg.Type, err)
		}
		return
	}
	if p.sender != nil {
		body := full
		if len(toolLines) > 0 {
			progressText := strings.Join(toolLines, "\n")
			if body != "" {
				body = progressText + "\n\n" + body
			} else {
				body = progressText
			}
		}
		if strings.TrimSpace(body) == "" {
			body = channelProcessingMsg
		}
		if err := p.sender.UpdateStream(context.Background(), p.msg, p.streamID, body); err != nil {
			log.Printf("[channel] UpdateStream %s: %v", p.msg.Type, err)
		}
	}
}

func appendToolLine(lines []string, line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return lines
	}
	lines = append(lines, line)
	if len(lines) > maxProgressLines {
		lines = lines[len(lines)-maxProgressLines:]
	}
	return lines
}

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func (ing *ChannelIngressService) handlePermissionAsk(ev domain.StreamEvent, p collectParams) {
	var payload domain.PermissionAskPayload
	if json.Unmarshal(ev.Payload, &payload) != nil || payload.ApprovalID == "" {
		return
	}
	if denied, reason := channelToolDenied(p.msg, payload.Tool); denied {
		_ = ing.sessions.DecideApproval(context.Background(), payload.ApprovalID, false, "once")
		_, _ = ing.deliverOrReturn(context.Background(), p.msg, reason)
		return
	}
	if p.autoApprove && domain.AutoApprovableReason(payload.Reason) {
		_ = ing.sessions.DecideApproval(context.Background(), payload.ApprovalID, true, "once")
		return
	}
	ask := port.PermissionPrompt{
		ApprovalID: payload.ApprovalID,
		ToolName:   payload.Tool,
		Summary:    strings.TrimSpace(payload.Description),
		Scopes:     payload.ScopeOptions,
		StreamID:   p.streamID,
	}
	if ask.Summary == "" {
		ask.Summary = strings.TrimSpace(payload.Reason)
	}
	handled := false
	if p.caps.InteractiveApprove && p.ep != nil {
		if approver, ok := p.ep.(port.ChannelApprover); ok {
			okHandled, err := approver.PresentPermission(context.Background(), p.msg, ask)
			if err != nil {
				log.Printf("[channel] PresentPermission %s: %v", p.msg.Type, err)
			} else {
				handled = okHandled
			}
		}
	}
	if !handled && p.caps.InteractiveApprove {
		_, _ = ing.deliverOrReturn(context.Background(), p.msg, formatPermissionText(ask))
		handled = true
	}
	if handled {
		ing.askMu.Lock()
		ing.pendingPerms[peerKey(*p.msg)] = pendingPerm{ApprovalID: ask.ApprovalID}
		ing.askMu.Unlock()
		return
	}
	// No in-channel approve: leave pending for desktop (do not auto-stub deny).
	_, _ = ing.deliverOrReturn(context.Background(), p.msg, fmt.Sprintf("（%s通道需要桌面端授权工具「%s」，或在设置中开启自动批准）", channelDisplayName(p.msg.Type), ask.ToolName))
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
		FormFields: payload.FormFields,
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
		_, _ = ing.deliverOrReturn(context.Background(), p.msg, formatAskText(ask))
		handled = true
	}
	if handled {
		ing.askMu.Lock()
		ing.pendingAsks[peerKey(*p.msg)] = pendingAsk{
			AskID:      ask.AskID,
			Options:    append([]string(nil), ask.Options...),
			FormFields: append([]domain.AskUserFormField(nil), ask.FormFields...),
		}
		ing.askMu.Unlock()
		return
	}
	stub := fmt.Sprintf("（%s通道暂不支持交互提问，请在桌面端继续）", channelDisplayName(p.msg.Type))
	_ = ing.sessions.ResolveAskUser(ask.AskID, stub)
}

func (ing *ChannelIngressService) collectReplyFrom(ctx context.Context, p collectParams) port.OutboundMessage {
	var parts []string
	var toolLines []string
	var lastProgress time.Time
	var failed bool
	for _, ev := range ing.sessions.StreamEvents(p.sessionID, 0) {
		if ing.applyEvent(ev, p, &parts, &toolLines, &lastProgress, &failed) {
			return finalOutboundFromParts(parts, toolLines, failed)
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
			return finalOutboundFromParts(parts, toolLines, failed)
		case <-deadline:
			if len(parts) == 0 && len(toolLines) == 0 {
				return port.TextOutbound("处理超时，请稍后在桌面端查看。")
			}
			return finalOutboundFromParts(parts, toolLines, true)
		case ev, ok := <-p.ch:
			if !ok {
				return finalOutboundFromParts(parts, toolLines, failed)
			}
			if _, dup := seen[ev.Seq]; dup {
				continue
			}
			seen[ev.Seq] = struct{}{}
			if ing.applyEvent(ev, p, &parts, &toolLines, &lastProgress, &failed) {
				return finalOutboundFromParts(parts, toolLines, failed)
			}
		}
	}
}

func (ing *ChannelIngressService) hasPendingPerm(key string) bool {
	ing.askMu.Lock()
	defer ing.askMu.Unlock()
	_, ok := ing.pendingPerms[key]
	return ok
}

func (ing *ChannelIngressService) hasPendingAsk(key string) bool {
	ing.askMu.Lock()
	defer ing.askMu.Unlock()
	_, ok := ing.pendingAsks[key]
	return ok
}
