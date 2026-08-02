package sqlite

import (
	"encoding/json"
	"strings"
	"time"

	"danmo-work/core/domain"

	"gorm.io/gorm"
)

// ---- Agent ----

type agentModel struct {
	ID             string `gorm:"primaryKey"`
	Name           string
	Description    string
	Persona        string
	Mode           string
	SystemPrompt   string `gorm:"column:system_prompt"`
	Steps          int
	SkillIDsJSON   string `gorm:"column:skill_ids"`
	ToolsJSON      string `gorm:"column:tools"`
	MCPServersJSON string `gorm:"column:mcp_servers"`
	KnowledgeJSON  string `gorm:"column:knowledge_ids"`
	CanDelegate    bool   `gorm:"column:can_delegate"`
	InheritAmbient *bool  `gorm:"column:inherit_ambient"`
}

func (agentModel) TableName() string { return "agents" }

func (m *agentModel) BeforeSave(_ *gorm.DB) error {
	b, _ := json.Marshal(m.skillIDs())
	m.SkillIDsJSON = string(b)
	b, _ = json.Marshal(m.tools())
	m.ToolsJSON = string(b)
	b, _ = json.Marshal(m.mcpServers())
	m.MCPServersJSON = string(b)
	b, _ = json.Marshal(m.knowledgeIDs())
	m.KnowledgeJSON = string(b)
	return nil
}

func (m *agentModel) skillIDs() []string { return unmarshalSlice[string](m.SkillIDsJSON) }
func (m *agentModel) tools() []domain.ToolBinding {
	var v []domain.ToolBinding
	_ = json.Unmarshal([]byte(m.ToolsJSON), &v)
	if v == nil {
		return nil
	}
	return v
}
func (m *agentModel) mcpServers() []string   { return unmarshalSlice[string](m.MCPServersJSON) }
func (m *agentModel) knowledgeIDs() []string { return unmarshalSlice[string](m.KnowledgeJSON) }

func agentToDomain(m agentModel) domain.Agent {
	a := domain.Agent{
		ID: m.ID, Name: m.Name, Description: m.Description, Persona: m.Persona,
		Mode: domain.AgentMode(m.Mode), SystemPrompt: m.SystemPrompt, Steps: m.Steps,
		SkillIDs: m.skillIDs(), Tools: m.tools(), MCPServers: m.mcpServers(),
		KnowledgeIDs: m.knowledgeIDs(),
		CanDelegate:  m.CanDelegate, InheritAmbient: m.InheritAmbient,
	}
	domain.NormalizeAgentBindings(&a)
	return a
}

func agentFromDomain(a domain.Agent) agentModel {
	domain.NormalizeAgentBindings(&a)
	return agentModel{
		ID: a.ID, Name: a.Name, Description: a.Description, Persona: a.Persona,
		Mode: string(a.Mode), SystemPrompt: a.SystemPrompt, Steps: a.Steps,
		SkillIDsJSON: marshalJSON(a.SkillIDs), ToolsJSON: marshalJSON(a.Tools),
		MCPServersJSON: marshalJSON(a.MCPServers), KnowledgeJSON: marshalJSON(a.KnowledgeIDs),
		CanDelegate: a.CanDelegate, InheritAmbient: a.InheritAmbient,
	}
}

// ---- Skill ----

type skillModel struct {
	ID            string `gorm:"primaryKey"`
	Name          string
	Description   string
	License       string
	Compatibility string
	MetadataJSON  string `gorm:"column:metadata"`
	AllowedTools  string `gorm:"column:allowed_tools"`
	KeywordsJSON  string `gorm:"column:keywords"`
	ToolIDsJSON   string `gorm:"column:tool_ids"`
	SystemHint    string `gorm:"column:system_hint"`
	Body          string `gorm:"column:body"`
	SourcePath    string `gorm:"column:source_path"`
	Builtin       bool   `gorm:"column:builtin"`
}

func (skillModel) TableName() string { return "skills" }

func (m *skillModel) BeforeSave(_ *gorm.DB) error {
	m.KeywordsJSON = marshalJSON(m.keywords())
	m.ToolIDsJSON = marshalJSON(m.toolIDs())
	m.MetadataJSON = marshalJSONMap(m.metadata())
	return nil
}

func (m *skillModel) keywords() []string          { return unmarshalSlice[string](m.KeywordsJSON) }
func (m *skillModel) toolIDs() []string           { return unmarshalSlice[string](m.ToolIDsJSON) }
func (m *skillModel) metadata() map[string]string { return unmarshalMap(m.MetadataJSON) }

func skillToDomain(m skillModel) domain.Skill {
	meta := m.metadata()
	marketSrc := ""
	if meta != nil {
		marketSrc = meta["market.source"]
	}
	return domain.Skill{
		ID: m.ID, Name: m.Name, Description: m.Description,
		License: m.License, Compatibility: m.Compatibility,
		Metadata: meta, AllowedTools: m.AllowedTools,
		Keywords: m.keywords(), ToolIDs: m.toolIDs(),
		SystemHint: m.SystemHint, Body: m.Body, SourcePath: m.SourcePath,
		Builtin: m.Builtin, MarketSource: marketSrc,
	}
}

func skillFromDomain(s domain.Skill) skillModel {
	meta := s.Metadata
	if s.MarketSource != "" {
		if meta == nil {
			meta = map[string]string{}
		}
		if meta["market.source"] == "" {
			meta["market.source"] = s.MarketSource
		}
	}
	return skillModel{
		ID: s.ID, Name: s.Name, Description: s.Description,
		License: s.License, Compatibility: s.Compatibility,
		MetadataJSON: marshalJSONMap(meta), AllowedTools: s.AllowedTools,
		KeywordsJSON: marshalJSON(s.Keywords), ToolIDsJSON: marshalJSON(s.ToolIDs),
		SystemHint: s.SystemHint, Body: s.Body, SourcePath: s.SourcePath,
		Builtin: s.Builtin,
	}
}

// ---- SkillFile ----

type skillFileModel struct {
	ID      string `gorm:"primaryKey"`
	SkillID string `gorm:"column:skill_id;index"`
	Path    string
	Content []byte
	Size    int64
}

func (skillFileModel) TableName() string { return "skill_files" }

func skillFileToDomain(m skillFileModel) domain.SkillFile {
	return domain.SkillFile{
		ID: m.ID, SkillID: m.SkillID, Path: m.Path, Content: m.Content, Size: m.Size,
	}
}

func skillFileFromDomain(f domain.SkillFile) skillFileModel {
	if f.ID == "" {
		f.ID = f.SkillID + ":" + f.Path
	}
	return skillFileModel{
		ID: f.ID, SkillID: f.SkillID, Path: f.Path, Content: f.Content, Size: f.Size,
	}
}

// ---- Session ----

type sessionModel struct {
	ID        string `gorm:"primaryKey"`
	Title     string
	ProjectID string `gorm:"column:project_id"`
	AgentID   string `gorm:"column:agent_id"`
	ModelID   string `gorm:"column:model_id"`
	Content   string
	Status    string
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (sessionModel) TableName() string { return "sessions" }

func sessionToDomain(m sessionModel) domain.Session {
	return domain.Session{
		ID: m.ID, Title: m.Title, ProjectID: m.ProjectID, AgentID: m.AgentID,
		ModelID: m.ModelID, Content: m.Content,
		Status: domain.SessionStatus(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func sessionFromDomain(s domain.Session) sessionModel {
	return sessionModel{
		ID: s.ID, Title: s.Title, ProjectID: s.ProjectID, AgentID: s.AgentID,
		ModelID: s.ModelID, Content: s.Content,
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
	AgentID   string `gorm:"column:agent_id"`
	Status    string
	Goal      string
}

func (turnModel) TableName() string { return "turns" }

func turnToDomain(m turnModel) domain.TurnLog {
	return domain.TurnLog{
		ID: m.ID, SessionID: m.SessionID, AgentID: m.AgentID,
		Status: domain.TurnStatus(m.Status), Goal: m.Goal,
	}
}

func turnFromDomain(t domain.TurnLog) turnModel {
	return turnModel{
		ID: t.ID, SessionID: t.SessionID, AgentID: t.AgentID,
		Status: string(t.Status), Goal: t.Goal,
	}
}

// ---- MCPServer ----

type mcpServerModel struct {
	ID                   string `gorm:"primaryKey"`
	Name                 string
	Description          string
	Transport            string
	Command              string
	Args                 string
	URL                  string
	Env                  string
	HeadersJSON          string `gorm:"column:headers"`
	Auth                 string `gorm:"column:auth"`
	SecretHeadersRefJSON string `gorm:"column:secret_headers_ref"`
	OAuthClientID        string `gorm:"column:oauth_client_id"`
	OAuthAuthorizeURL    string `gorm:"column:oauth_authorize_url"`
	OAuthTokenURL        string `gorm:"column:oauth_token_url"`
	OAuthScopes          string `gorm:"column:oauth_scopes"`
	OAuthStatus          string `gorm:"column:oauth_status"`
	CatalogID            string `gorm:"column:catalog_id"`
	MarketSource         string `gorm:"column:market_source"`
	EnabledToolsJSON     string `gorm:"column:enabled_tools"`
	DiscoveredToolsJSON  string `gorm:"column:discovered_tools"`
	ToolTimeout          int    `gorm:"column:tool_timeout"`
	Status               string
	Enabled              bool
	// No DB default: GORM skips false zero-values when tag has default:true, which
	// made bound-only connectors (ambient_mount=0) persist as ambient on.
	AmbientMount bool   `gorm:"column:ambient_mount"`
	Network      string `gorm:"column:network"` // inherit | deny | allow | allowlist
}

func (mcpServerModel) TableName() string { return "mcp_servers" }

func (m *mcpServerModel) BeforeSave(_ *gorm.DB) error {
	if m.HeadersJSON == "" {
		m.HeadersJSON = "{}"
	}
	if m.EnabledToolsJSON == "" {
		m.EnabledToolsJSON = `["*"]`
	}
	return nil
}

func mcpServerToDomain(m mcpServerModel) domain.MCPServer {
	var headers map[string]string
	_ = json.Unmarshal([]byte(m.HeadersJSON), &headers)
	var secretRefs map[string]string
	_ = json.Unmarshal([]byte(m.SecretHeadersRefJSON), &secretRefs)
	var enabledTools []string
	_ = json.Unmarshal([]byte(m.EnabledToolsJSON), &enabledTools)
	if len(enabledTools) == 0 {
		enabledTools = []string{"*"}
	}
	var discovered []domain.MCPToolDef
	_ = json.Unmarshal([]byte(m.DiscoveredToolsJSON), &discovered)
	auth := domain.MCPAuthMode(m.Auth)
	if auth == "" {
		auth = domain.MCPAuthNone
	}
	return domain.MCPServer{
		ID:                m.ID,
		Name:              m.Name,
		Description:       m.Description,
		Transport:         m.Transport,
		Command:           m.Command,
		Args:              m.Args,
		URL:               m.URL,
		Env:               m.Env,
		Headers:           headers,
		Auth:              auth,
		SecretHeadersRef:  secretRefs,
		OAuthClientID:     m.OAuthClientID,
		OAuthAuthorizeURL: m.OAuthAuthorizeURL,
		OAuthTokenURL:     m.OAuthTokenURL,
		OAuthScopes:       m.OAuthScopes,
		OAuthStatus:       m.OAuthStatus,
		CatalogID:         m.CatalogID,
		MarketSource:      m.MarketSource,
		EnabledTools:      enabledTools,
		DiscoveredTools:   discovered,
		ToolTimeout:       m.ToolTimeout,
		Status:            m.Status,
		Enabled:           m.Enabled,
		AmbientMount:      m.AmbientMount,
		Network:           m.Network,
	}
}

func mcpServerFromDomain(s domain.MCPServer) mcpServerModel {
	headersJSON := "{}"
	if len(s.Headers) > 0 {
		b, _ := json.Marshal(s.Headers)
		headersJSON = string(b)
	}
	secretRefsJSON := "{}"
	if len(s.SecretHeadersRef) > 0 {
		b, _ := json.Marshal(s.SecretHeadersRef)
		secretRefsJSON = string(b)
	}
	enabledToolsJSON := `["*"]`
	if len(s.EnabledTools) > 0 {
		b, _ := json.Marshal(s.EnabledTools)
		enabledToolsJSON = string(b)
	}
	discoveredJSON := "[]"
	if len(s.DiscoveredTools) > 0 {
		b, _ := json.Marshal(s.DiscoveredTools)
		discoveredJSON = string(b)
	}
	timeout := s.ToolTimeout
	if timeout <= 0 {
		timeout = 300
	}
	status := s.Status
	if status == "" {
		status = "disconnected"
	}
	auth := string(s.Auth)
	if auth == "" {
		auth = string(domain.MCPAuthNone)
	}
	return mcpServerModel{
		ID:                   s.ID,
		Name:                 s.Name,
		Description:          s.Description,
		Transport:            s.Transport,
		Command:              s.Command,
		Args:                 s.Args,
		URL:                  s.URL,
		Env:                  s.Env,
		HeadersJSON:          headersJSON,
		Auth:                 auth,
		SecretHeadersRefJSON: secretRefsJSON,
		OAuthClientID:        s.OAuthClientID,
		OAuthAuthorizeURL:    s.OAuthAuthorizeURL,
		OAuthTokenURL:        s.OAuthTokenURL,
		OAuthScopes:          s.OAuthScopes,
		OAuthStatus:          s.OAuthStatus,
		CatalogID:            s.CatalogID,
		MarketSource:         s.MarketSource,
		EnabledToolsJSON:     enabledToolsJSON,
		DiscoveredToolsJSON:  discoveredJSON,
		ToolTimeout:          timeout,
		Status:               status,
		Enabled:              s.Enabled,
		AmbientMount:         s.AmbientMount,
		Network:              s.Network,
	}
}

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
