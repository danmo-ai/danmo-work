package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

type feishuConfigureRequest struct {
	Enabled        bool   `json:"enabled"`
	DefaultAgentID string `json:"defaultAgentId"`
	DefaultModelID string `json:"defaultModelId,omitempty"`
	AutoApprove    *bool  `json:"autoApprove,omitempty"`
	Domain         string `json:"domain,omitempty"`
	AppID          string `json:"appId,omitempty"`
	AppSecret      string `json:"appSecret,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
	RichProgress   *bool  `json:"richProgress,omitempty"`
}

func feishuStatusPayload(fs domain.ConfigFeishuChannel, running bool) gin.H {
	domain := strings.ToLower(strings.TrimSpace(fs.Domain))
	if domain == "" {
		domain = "feishu"
	}
	return gin.H{
		"enabled":        fs.Enabled,
		"running":        running,
		"domain":         domain,
		"defaultAgentId": fs.DefaultAgentID,
		"defaultModelId": fs.DefaultModelID,
		"autoApprove":    fs.AutoApprove,
		"appId":          fs.AppID,
		"projectId":      fs.ProjectID,
		"hasAppSecret":   fs.AppSecret != "",
		"richProgress":   fs.FeishuRichProgressEnabled(),
	}
}

func feishuStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
			return
		}
		cfg, err := h.Config.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		fs := cfg.Channels.Feishu
		running := false
		if h.Feishu != nil {
			running = h.Feishu.IsRunning()
		}
		c.JSON(http.StatusOK, feishuStatusPayload(fs, running))
	}
}

func feishuConfigure(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Config == nil || h.Channels == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel manager unavailable"})
			return
		}
		var req feishuConfigureRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.Config.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		fs := cfg.Channels.Feishu
		fs.Enabled = req.Enabled
		if req.DefaultAgentID != "" {
			fs.DefaultAgentID = req.DefaultAgentID
		}
		if req.DefaultModelID != "" {
			fs.DefaultModelID = req.DefaultModelID
		}
		if req.AutoApprove != nil {
			fs.AutoApprove = *req.AutoApprove
		} else if req.Enabled {
			// Prefer in-chat card approval for new Feishu setups (Phase A).
			fs.AutoApprove = false
		}
		if req.Domain != "" {
			fs.Domain = req.Domain
		}
		if req.AppID != "" {
			fs.AppID = req.AppID
		}
		if req.AppSecret != "" {
			fs.AppSecret = req.AppSecret
		}
		if req.ProjectID != "" {
			fs.ProjectID = req.ProjectID
		}
		if req.RichProgress != nil {
			fs.RichProgress = req.RichProgress
		}
		if !validateChannelDefaultAgent(h, c, fs.DefaultAgentID) {
			return
		}
		if req.Enabled {
			if fs.DefaultAgentID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId required when enabling feishu"})
				return
			}
			if fs.DefaultModelID == "" || !strings.Contains(fs.DefaultModelID, "/") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultModelId required when enabling feishu (provider/model)"})
				return
			}
			if strings.TrimSpace(fs.ProjectID) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "projectId required when enabling feishu"})
				return
			}
			if fs.AppID == "" || fs.AppSecret == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "appId and appSecret required when enabling feishu"})
				return
			}
		}
		sec := cfg.Channels
		sec.Feishu = fs
		if _, err := h.Config.Update(c.Request.Context(), domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.Feishu != nil {
			if err := h.Feishu.SyncFromConfig(c.Request.Context()); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		running := h.Feishu != nil && h.Feishu.IsRunning()
		c.JSON(http.StatusOK, feishuStatusPayload(fs, running))
	}
}
