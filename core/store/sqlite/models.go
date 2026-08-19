package sqlite

import (
	"encoding/json"
	"strings"
	"time"

	"danmo-work/core/domain"
)

// ---- Session ----

type sessionModel struct {
	ID        string `gorm:"primaryKey"`
	Title     string
	ProjectID string    `gorm:"column:project_id"`
	AgentID   string    `gorm:"column:agent_id"`
	ModelID   string    `gorm:"column:model_id"`
	PlanMode  bool      `gorm:"column:plan_mode"`
	Content   string
	Status    string
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (sessionModel) TableName() string { return "sessions" }

func sessionToDomain(m sessionModel) domain.Session {
	return domain.Session{
		ID: m.ID, Title: m.Title, ProjectID: m.ProjectID, AgentID: m.AgentID,
		ModelID: m.ModelID, PlanMode: m.PlanMode, Content: m.Content,
		Status: domain.SessionStatus(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func sessionFromDomain(s domain.Session) sessionModel {
	return sessionModel{
		ID: s.ID, Title: s.Title, ProjectID: s.ProjectID, AgentID: s.AgentID,
		ModelID: s.ModelID, PlanMode: s.PlanMode, Content: s.Content,
		Status: string(s.Status), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// ---- Project ----

type projectModel struct {
	ID        string `gorm:"primaryKey"`
	Name      string
	Directory string
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (projectModel) TableName() string { return "projects" }

func projectToDomain(m projectModel) domain.Project {
	return domain.Project{ID: m.ID, Name: m.Name, Directory: m.Directory, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func projectFromDomain(p domain.Project) projectModel {
	return projectModel{ID: p.ID, Name: p.Name, Directory: p.Directory, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

// ---- Approval ----

type approvalModel struct {
	ID        string `gorm:"primaryKey"`
	SessionID string `gorm:"column:session_id"`
	TurnID    string `gorm:"column:turn_id"`
	ToolName  string `gorm:"column:tool_name"`
	Summary   string
	Status    string
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (approvalModel) TableName() string { return "approvals" }

func approvalToDomain(m approvalModel) domain.Approval {
	return domain.Approval{ID: m.ID, SessionID: m.SessionID, TurnID: m.TurnID, ToolName: m.ToolName, Summary: m.Summary, Status: m.Status, CreatedAt: m.CreatedAt}
}

func approvalFromDomain(a domain.Approval) approvalModel {
	return approvalModel{ID: a.ID, SessionID: a.SessionID, TurnID: a.TurnID, ToolName: a.ToolName, Summary: a.Summary, Status: a.Status, CreatedAt: a.CreatedAt}
}

// ---- PendingMessage ----

type pendingMessageModel struct {
	ID              string    `gorm:"primaryKey"`
	SessionID       string    `gorm:"column:session_id;index"`
	Content         string    `gorm:"column:content"`
	AttachmentsJSON string    `gorm:"column:attachments_json"`
	Position        int       `gorm:"column:position;index"`
	Status          string    `gorm:"column:status;index"`
	AgentID         string    `gorm:"column:agent_id"`
	ModelID         string    `gorm:"column:model_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (pendingMessageModel) TableName() string { return "pending_messages" }

func pendingMessageToDomain(m pendingMessageModel) domain.PendingMessage {
	var atts []domain.UserAttachment
	if strings.TrimSpace(m.AttachmentsJSON) != "" {
		_ = json.Unmarshal([]byte(m.AttachmentsJSON), &atts)
	}
	return domain.PendingMessage{
		ID: m.ID, SessionID: m.SessionID, Content: m.Content, Attachments: atts,
		Position: m.Position, Status: domain.PendingMessageStatus(m.Status),
		AgentID: m.AgentID, ModelID: m.ModelID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func pendingMessageFromDomain(m domain.PendingMessage) pendingMessageModel {
	attsJSON := "[]"
	if len(m.Attachments) > 0 {
		if b, err := json.Marshal(m.Attachments); err == nil {
			attsJSON = string(b)
		}
	}
	return pendingMessageModel{
		ID: m.ID, SessionID: m.SessionID, Content: m.Content, AttachmentsJSON: attsJSON,
		Position: m.Position, Status: string(m.Status),
		AgentID: m.AgentID, ModelID: m.ModelID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ---- StreamEvent ----

type streamEventModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	SessionID string `gorm:"column:session_id;index"`
	TurnID    string `gorm:"column:turn_id"`
	Seq       int64  `gorm:"index"`
	Type      string
	Payload   string
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (streamEventModel) TableName() string { return "stream_events" }

func streamEventToDomain(m streamEventModel) domain.StreamEvent {
	return domain.StreamEvent{
		Seq: m.Seq, Type: m.Type, SessionID: m.SessionID,
		TurnID: m.TurnID, Payload: json.RawMessage(m.Payload), CreatedAt: m.CreatedAt,
	}
}

// ---- Turn ----

type turnModel struct {
	ID        string `gorm:"primaryKey"`
	SessionID string `gorm:"column:session_id;index"`
	ProjectID string `gorm:"column:project_id"`
	AgentID   string `gorm:"column:agent_id"`
	Status    string
	Goal      string
	Nested    bool `gorm:"column:nested"`
}

func (turnModel) TableName() string { return "turns" }

func turnToDomain(m turnModel) domain.TurnLog {
	return domain.TurnLog{
		ID: m.ID, SessionID: m.SessionID, ProjectID: m.ProjectID, AgentID: m.AgentID,
		Status: domain.TurnStatus(m.Status), Goal: m.Goal, Nested: m.Nested,
	}
}

func turnFromDomain(t domain.TurnLog) turnModel {
	return turnModel{
		ID: t.ID, SessionID: t.SessionID, ProjectID: t.ProjectID, AgentID: t.AgentID,
		Status: string(t.Status), Goal: t.Goal, Nested: t.Nested,
	}
}

// ---- Turn log entries (LLM history source of truth) ----

type turnLogEntryModel struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	TurnID    string `gorm:"column:turn_id;uniqueIndex:idx_turn_log_entries_turn_seq,priority:1"`
	Seq       int    `gorm:"column:seq;uniqueIndex:idx_turn_log_entries_turn_seq,priority:2"`
	Type      string `gorm:"column:type"`
	Data      string `gorm:"column:data"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (turnLogEntryModel) TableName() string { return "turn_log_entries" }

// ---- Helpers ----

func unmarshalSlice[T any](raw string) []T {
	var v []T
	if raw == "" || raw == "null" {
		return nil
	}
	_ = json.Unmarshal([]byte(raw), &v)
	return v
}

func marshalJSON(v any) string {
	if v == nil {
		return "[]"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func marshalJSONMap(m map[string]string) string {
	if m == nil {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func unmarshalMap(raw string) map[string]string {
	if raw == "" || raw == "null" {
		return nil
	}
	var v map[string]string
	_ = json.Unmarshal([]byte(raw), &v)
	return v
}

// ---- Weixin ----

type weixinAccountModel struct {
	AccountID string    `gorm:"column:account_id;primaryKey"`
	Token     string    `gorm:"column:token"`
	BaseURL   string    `gorm:"column:base_url"`
	UserID    string    `gorm:"column:user_id"`
	ProjectID string    `gorm:"column:project_id;index"`
	SyncBuf   string    `gorm:"column:sync_buf;type:text"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (weixinAccountModel) TableName() string { return "weixin_accounts" }

func weixinAccountToDomain(m weixinAccountModel) domain.WeixinAccount {
	return domain.WeixinAccount{
		AccountID: m.AccountID,
		Token:     m.Token,
		BaseURL:   m.BaseURL,
		UserID:    m.UserID,
		ProjectID: m.ProjectID,
		SyncBuf:   m.SyncBuf,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

type weixinBindingModel struct {
	ID           string    `gorm:"primaryKey"`
	AccountID    string    `gorm:"column:account_id;uniqueIndex:idx_weixin_peer"`
	PeerUserID   string    `gorm:"column:peer_user_id;uniqueIndex:idx_weixin_peer"`
	SessionID    string    `gorm:"column:session_id;index"`
	ContextToken string    `gorm:"column:context_token"`
	MetaJSON     string    `gorm:"column:meta_json;type:text"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (weixinBindingModel) TableName() string { return "weixin_bindings" }

func weixinBindingToDomain(m weixinBindingModel) domain.WeixinBinding {
	return domain.WeixinBinding{
		ID:           m.ID,
		AccountID:    m.AccountID,
		PeerUserID:   m.PeerUserID,
		SessionID:    m.SessionID,
		ContextToken: m.ContextToken,
		Meta:         unmarshalMap(m.MetaJSON),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ---- Generic channel bindings (Feishu, …) ----

type channelBindingModel struct {
	ID          string    `gorm:"primaryKey"`
	ChannelType string    `gorm:"column:channel_type;uniqueIndex:idx_channel_peer"`
	AccountID   string    `gorm:"column:account_id;uniqueIndex:idx_channel_peer"`
	PeerID      string    `gorm:"column:peer_id;uniqueIndex:idx_channel_peer"`
	SessionID   string    `gorm:"column:session_id;index"`
	MetaJSON    string    `gorm:"column:meta_json;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (channelBindingModel) TableName() string { return "channel_bindings" }

func channelBindingToDomain(m channelBindingModel) domain.ChannelBinding {
	return domain.ChannelBinding{
		ID:          m.ID,
		ChannelType: m.ChannelType,
		AccountID:   m.AccountID,
		PeerID:      m.PeerID,
		SessionID:   m.SessionID,
		Meta:        unmarshalMap(m.MetaJSON),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
