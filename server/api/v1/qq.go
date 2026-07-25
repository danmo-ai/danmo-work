package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

type qqConfigureRequest struct {
	Enabled         bool     `json:"enabled"`
	DefaultAgentID  string   `json:"defaultAgentId"`
	DefaultModelID  string   `json:"defaultModelId,omitempty"`
	AutoApprove     *bool    `json:"autoApprove,omitempty"`
	AppID           string   `json:"appId,omitempty"`
	ClientSecret    string   `json:"clientSecret,omitempty"`
	ProjectID       string   `json:"projectId,omitempty"`
	GroupDenyTools  []string `json:"groupDenyTools,omitempty"`
	RequireMention  *bool    `json:"requireMention,omitempty"`
	NativeC2CStream *bool    `json:"nativeC2cStream,omitempty"`
}

func qqRequireMentionEnabled(qc domain.ConfigQQChannel) bool {
	if qc.Group.RequireMention == nil {
		return true
	}
	return *qc.Group.RequireMention
}

func qqStatusPayload(qc domain.ConfigQQChannel, running bool) gin.H {
	return gin.H{
		"enabled":         qc.Enabled,
		"running":         running,
		"defaultAgentId":  qc.DefaultAgentID,
		"defaultModelId":  qc.DefaultModelID,
		"autoApprove":     qc.AutoApprove,
		"appId":           qc.AppID,
		"projectId":       qc.ProjectID,
		"hasClientSecret": qc.ClientSecret != "",
		"groupDenyTools":  qc.Group.DenyTools,
		"requireMention":  qqRequireMentionEnabled(qc),
		"nativeC2cStream": qc.QQNativeC2CStreamEnabled(),
	}
}

func qqStatus(h *Handler) gin.HandlerFunc {
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
		qc := cfg.Channels.QQ
		running := false
		if h.QQ != nil {
			running = h.QQ.IsRunning()
		}
		c.JSON(http.StatusOK, qqStatusPayload(qc, running))
	}
}

func qqConfigure(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Config == nil || h.Channels == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel manager unavailable"})
			return
		}
		var req qqConfigureRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.Config.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		qc := cfg.Channels.QQ
		qc.Enabled = req.Enabled
		if req.DefaultAgentID != "" {
			qc.DefaultAgentID = req.DefaultAgentID
		}
		if req.DefaultModelID != "" {
			qc.DefaultModelID = req.DefaultModelID
		}
		if req.AutoApprove != nil {
			qc.AutoApprove = *req.AutoApprove
		} else if req.Enabled {
			// Prefer in-chat approval for new QQ setups.
			qc.AutoApprove = false
		}
		if req.AppID != "" {
			qc.AppID = req.AppID
		}
		if req.ClientSecret != "" {
			qc.ClientSecret = req.ClientSecret
		}
		if req.ProjectID != "" {
			qc.ProjectID = req.ProjectID
		}
		if req.GroupDenyTools != nil {
			qc.Group.DenyTools = req.GroupDenyTools
		}
		if req.RequireMention != nil {
			qc.Group.RequireMention = req.RequireMention
		}
		if req.NativeC2CStream != nil {
			qc.NativeC2CStream = req.NativeC2CStream
		}
		if req.Enabled {
			if qc.DefaultAgentID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId required when enabling qq"})
				return
			}
			if qc.DefaultModelID == "" || !strings.Contains(qc.DefaultModelID, "/") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultModelId required when enabling qq (provider/model)"})
				return
			}
			if strings.TrimSpace(qc.ProjectID) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "projectId required when enabling qq"})
				return
			}
			if qc.AppID == "" || qc.ClientSecret == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "appId and clientSecret required when enabling qq"})
				return
			}
		}
		sec := cfg.Channels
		sec.QQ = qc
		if _, err := h.Config.Update(c.Request.Context(), domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.QQ != nil {
			if err := h.QQ.SyncFromConfig(c.Request.Context()); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		running := h.QQ != nil && h.QQ.IsRunning()
		c.JSON(http.StatusOK, qqStatusPayload(qc, running))
	}
}
