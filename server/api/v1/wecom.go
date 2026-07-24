package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

type wecomConfigureRequest struct {
	Enabled        bool   `json:"enabled"`
	DefaultAgentID string `json:"defaultAgentId"`
	DefaultModelID string `json:"defaultModelId,omitempty"`
	AutoApprove    *bool  `json:"autoApprove,omitempty"`
	BotID          string `json:"botId,omitempty"`
	Secret         string `json:"secret,omitempty"`
	WSURL          string `json:"wsUrl,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
}

func wecomStatusPayload(wc domain.ConfigWecomChannel, running bool) gin.H {
	return gin.H{
		"enabled":        wc.Enabled,
		"running":        running,
		"defaultAgentId": wc.DefaultAgentID,
		"defaultModelId": wc.DefaultModelID,
		"autoApprove":    wc.AutoApprove,
		"botId":          wc.BotID,
		"projectId":      wc.ProjectID,
		"wsUrl":          wc.WSURL,
		"hasSecret":      wc.Secret != "",
	}
}

func wecomStatus(h *Handler) gin.HandlerFunc {
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
		wc := cfg.Channels.Wecom
		running := false
		if h.Wecom != nil {
			running = h.Wecom.IsRunning()
		}
		c.JSON(http.StatusOK, wecomStatusPayload(wc, running))
	}
}

func wecomConfigure(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Config == nil || h.Channels == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel manager unavailable"})
			return
		}
		var req wecomConfigureRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.Config.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		wc := cfg.Channels.Wecom
		wc.Enabled = req.Enabled
		if req.DefaultAgentID != "" {
			wc.DefaultAgentID = req.DefaultAgentID
		}
		if req.DefaultModelID != "" {
			wc.DefaultModelID = req.DefaultModelID
		}
		if req.AutoApprove != nil {
			wc.AutoApprove = *req.AutoApprove
		} else if req.Enabled {
			wc.AutoApprove = true
		}
		if req.BotID != "" {
			wc.BotID = req.BotID
		}
		if req.Secret != "" {
			wc.Secret = req.Secret
		}
		if req.WSURL != "" {
			wc.WSURL = req.WSURL
		}
		if req.ProjectID != "" {
			wc.ProjectID = req.ProjectID
		}
		if req.Enabled {
			if wc.DefaultAgentID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId required when enabling wecom"})
				return
			}
			if wc.DefaultModelID == "" || !strings.Contains(wc.DefaultModelID, "/") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultModelId required when enabling wecom (provider/model)"})
				return
			}
			if strings.TrimSpace(wc.ProjectID) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "projectId required when enabling wecom"})
				return
			}
			if wc.BotID == "" || wc.Secret == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "botId and secret required when enabling wecom"})
				return
			}
		}
		sec := cfg.Channels
		sec.Wecom = wc
		if _, err := h.Config.Update(c.Request.Context(), domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.Wecom != nil {
			if err := h.Wecom.SyncFromConfig(c.Request.Context()); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		running := h.Wecom != nil && h.Wecom.IsRunning()
		c.JSON(http.StatusOK, wecomStatusPayload(wc, running))
	}
}
