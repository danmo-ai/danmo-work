package domain

import (
	"time"
)

// ConfigChannelsSection holds external chat channel settings.
type ConfigChannelsSection struct {
	Weixin ConfigWeixinChannel `json:"weixin" mapstructure:"weixin" yaml:"weixin"`
	Feishu ConfigFeishuChannel `json:"feishu" mapstructure:"feishu" yaml:"feishu"`
	Wecom  ConfigWecomChannel  `json:"wecom" mapstructure:"wecom" yaml:"wecom"`
	QQ     ConfigQQChannel     `json:"qq" mapstructure:"qq" yaml:"qq"`
}

// ConfigQQChannel configures the QQ Bot channel via outbound WebSocket Gateway.
type ConfigQQChannel struct {
	Enabled        bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	DefaultAgentID string `json:"defaultAgentId" mapstructure:"default_agent_id" yaml:"default_agent_id"`
	DefaultModelID string `json:"defaultModelId" mapstructure:"default_model_id" yaml:"default_model_id"`
	AutoApprove    bool   `json:"autoApprove" mapstructure:"auto_approve" yaml:"auto_approve"`
	AppID          string `json:"appId" mapstructure:"app_id" yaml:"app_id"`
	ClientSecret   string `json:"clientSecret,omitempty" mapstructure:"client_secret" yaml:"client_secret,omitempty"`
	// ProjectID binds inbound QQ peers to one Teams project (overridable per-peer via /project).
	ProjectID string `json:"projectId,omitempty" mapstructure:"project_id" yaml:"project_id,omitempty"`
	// NativeC2CStream uses QQ C2C stream_messages API when true (default).
	NativeC2CStream *bool `json:"nativeC2cStream,omitempty" mapstructure:"native_c2c_stream" yaml:"native_c2c_stream,omitempty"`
	// Group configures group-chat policy (C2C unaffected).
	Group ConfigQQGroupPolicy `json:"group,omitempty" mapstructure:"group" yaml:"group,omitempty"`
}

// ConfigQQGroupPolicy controls QQ group chat behavior.
type ConfigQQGroupPolicy struct {
	// RequireMention defaults true: only @-mention group messages are accepted
	// (QQ already gates via GROUP_AT_MESSAGE_CREATE; kept for config clarity / future events).
	RequireMention *bool `json:"requireMention,omitempty" mapstructure:"require_mention" yaml:"require_mention,omitempty"`
	// DenyTools lists tool names rejected for group turns when they request approval
	// (e.g. exec_shell). Matching tools are auto-denied in-channel.
	DenyTools []string `json:"denyTools,omitempty" mapstructure:"deny_tools" yaml:"deny_tools,omitempty"`
	// Groups optional per-group_openid overrides.
	Groups map[string]ConfigQQGroupOverride `json:"groups,omitempty" mapstructure:"groups" yaml:"groups,omitempty"`
}

// ConfigQQGroupOverride overrides group policy for one group openid.
type ConfigQQGroupOverride struct {
	RequireMention *bool    `json:"requireMention,omitempty" mapstructure:"require_mention" yaml:"require_mention,omitempty"`
	DenyTools      []string `json:"denyTools,omitempty" mapstructure:"deny_tools" yaml:"deny_tools,omitempty"`
}

// ConfigWecomChannel configures WeCom (企业微信) AI Bot via outbound WebSocket.
type ConfigWecomChannel struct {
	Enabled        bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	DefaultAgentID string `json:"defaultAgentId" mapstructure:"default_agent_id" yaml:"default_agent_id"`
	DefaultModelID string `json:"defaultModelId" mapstructure:"default_model_id" yaml:"default_model_id"`
	AutoApprove    bool   `json:"autoApprove" mapstructure:"auto_approve" yaml:"auto_approve"`
	BotID          string `json:"botId" mapstructure:"bot_id" yaml:"bot_id"`
	Secret         string `json:"secret,omitempty" mapstructure:"secret" yaml:"secret,omitempty"`
	// WSURL overrides the default wss://openws.work.weixin.qq.com (private deploy).
	WSURL string `json:"wsUrl,omitempty" mapstructure:"ws_url" yaml:"ws_url,omitempty"`
	// ProjectID binds inbound WeCom peers to one Teams project.
	ProjectID string `json:"projectId,omitempty" mapstructure:"project_id" yaml:"project_id,omitempty"`
}

// ConfigFeishuChannel configures the Feishu (Lark) channel via outbound WebSocket.
type ConfigFeishuChannel struct {
	Enabled        bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	DefaultAgentID string `json:"defaultAgentId" mapstructure:"default_agent_id" yaml:"default_agent_id"`
	DefaultModelID string `json:"defaultModelId" mapstructure:"default_model_id" yaml:"default_model_id"`
	AutoApprove    bool   `json:"autoApprove" mapstructure:"auto_approve" yaml:"auto_approve"`
	AppID          string `json:"appId" mapstructure:"app_id" yaml:"app_id"`
	AppSecret      string `json:"appSecret,omitempty" mapstructure:"app_secret" yaml:"app_secret,omitempty"`
	// Domain: "feishu" (default) or "lark" for international.
	Domain string `json:"domain,omitempty" mapstructure:"domain" yaml:"domain,omitempty"`
	// ProjectID binds inbound Feishu peers to one Teams project.
	ProjectID string `json:"projectId,omitempty" mapstructure:"project_id" yaml:"project_id,omitempty"`
}

// ConfigWeixinChannel configures the Weixin iLink bridge.
// Project binding lives on each WeixinAccount (one account → one project).
type ConfigWeixinChannel struct {
	Enabled        bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	DefaultAgentID string `json:"defaultAgentId" mapstructure:"default_agent_id" yaml:"default_agent_id"`
	DefaultModelID string `json:"defaultModelId" mapstructure:"default_model_id" yaml:"default_model_id"`
	AutoApprove    bool   `json:"autoApprove" mapstructure:"auto_approve" yaml:"auto_approve"`

	// DefaultProjectID is deprecated (migrated onto WeixinAccount.ProjectID).
	// Kept only so one-shot migration can read old YAML.
	DefaultProjectID string `json:"defaultProjectId,omitempty" mapstructure:"default_project_id" yaml:"default_project_id,omitempty"`
}

// WeixinAccount is a logged-in iLink bot account bound to one Teams project.
type WeixinAccount struct {
	AccountID string    `json:"accountId"`
	Token     string    `json:"token,omitempty"`
	BaseURL   string    `json:"baseUrl,omitempty"`
	UserID    string    `json:"userId,omitempty"`
	ProjectID string    `json:"projectId,omitempty"`
	SyncBuf   string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// WeixinBinding maps one Weixin peer to one Teams session (1:1).
type WeixinBinding struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountId"`
	PeerUserID   string    `json:"peerUserId"`
	SessionID    string    `json:"sessionId"`
	ContextToken string    `json:"contextToken,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type WeixinLoginStartResult struct {
	SessionKey string `json:"sessionKey"`
	QRCodeURL  string `json:"qrcodeUrl"`
	AccountID  string `json:"accountId,omitempty"`
}

type WeixinLoginWaitResult struct {
	Connected        bool   `json:"connected"`
	AlreadyConnected bool   `json:"alreadyConnected,omitempty"`
	AccountID        string `json:"accountId,omitempty"`
	UserID           string `json:"userId,omitempty"`
	ProjectID        string `json:"projectId,omitempty"`
	Message          string `json:"message,omitempty"`
	NeedsVerifyCode  bool   `json:"needsVerifyCode,omitempty"`
}

type WeixinStatus struct {
	Enabled        bool            `json:"enabled"`
	Running        bool            `json:"running"`
	DefaultAgentID string          `json:"defaultAgentId,omitempty"`
	DefaultModelID string          `json:"defaultModelId,omitempty"`
	AutoApprove    bool            `json:"autoApprove"`
	Accounts       []WeixinAccount `json:"accounts"`
	BindingCount   int             `json:"bindingCount"`
}

// ChannelBinding is a generic peer→session map for IM channels (Feishu, …).
type ChannelBinding struct {
	ID          string            `json:"id"`
	ChannelType string            `json:"channelType"`
	AccountID   string            `json:"accountId"`
	PeerID      string            `json:"peerId"`
	SessionID   string            `json:"sessionId"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
