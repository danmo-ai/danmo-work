package ilink

const (
	DefaultBaseURL   = "https://ilinkai.weixin.qq.com"
	DefaultAppID     = "bot"
	DefaultBotType   = "3"
	DefaultChannelVer = "2.4.3"
	DefaultBotAgent  = "DanmoWork/1.0.0"
)

const (
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

const (
	MessageItemText  = 1
	MessageItemVoice = 3
)

const (
	MessageStateFinish = 2
)

type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
	BotAgent       string `json:"bot_agent,omitempty"`
}

type QRCodeResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

type QRStatusResponse struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type VoiceItem struct {
	Text string `json:"text,omitempty"`
}

type MessageItem struct {
	Type      int        `json:"type,omitempty"`
	TextItem  *TextItem  `json:"text_item,omitempty"`
	VoiceItem *VoiceItem `json:"voice_item,omitempty"`
}

type Message struct {
	MessageID    any           `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

type GetUpdatesResp struct {
	Ret                   int       `json:"ret"`
	ErrCode               int       `json:"errcode"`
	ErrMsg                string    `json:"errmsg,omitempty"`
	Msgs                  []Message `json:"msgs,omitempty"`
	GetUpdatesBuf         string    `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMs  int       `json:"longpolling_timeout_ms,omitempty"`
}

type SendMessageReq struct {
	Msg      Message  `json:"msg"`
	BaseInfo BaseInfo `json:"base_info"`
}

type Account struct {
	AccountID string
	Token     string
	BaseURL   string
	UserID    string
}
