package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

func weixinStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		st, err := h.Weixin.Status(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, st)
	}
}

type weixinLoginStartRequest struct {
	ProjectID string `json:"projectId"`
}

func weixinLoginStart(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		var req weixinLoginStartRequest
		_ = c.ShouldBindJSON(&req)
		res, err := h.Weixin.StartLogin(c.Request.Context(), req.ProjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

type weixinLoginWaitRequest struct {
	SessionKey string `json:"sessionKey"`
	VerifyCode string `json:"verifyCode,omitempty"`
	TimeoutMs  int    `json:"timeoutMs,omitempty"`
}

func weixinLoginWait(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		var req weixinLoginWaitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.SessionKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionKey required"})
			return
		}
		res, err := h.Weixin.WaitLogin(c.Request.Context(), req.SessionKey, req.VerifyCode, req.TimeoutMs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

type weixinLogoutRequest struct {
	AccountID string `json:"accountId,omitempty"`
}

func weixinLogout(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		var req weixinLogoutRequest
		_ = c.ShouldBindJSON(&req)
		if err := h.Weixin.Logout(c.Request.Context(), req.AccountID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

type weixinAccountUpdateRequest struct {
	ProjectID *string `json:"projectId"`
}

func weixinUpdateAccount(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		accountID := c.Param("id")
		if accountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account id required"})
			return
		}
		var req weixinAccountUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.ProjectID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "projectId required (use empty string to unbind)"})
			return
		}
		acc, err := h.Weixin.SetAccountProject(c.Request.Context(), accountID, *req.ProjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, acc)
	}
}

func weixinBindings(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		list, err := h.Weixin.ListBindings(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if list == nil {
			list = []domain.WeixinBinding{}
		}
		c.JSON(http.StatusOK, list)
	}
}

type weixinEnableRequest struct {
	Enabled        bool   `json:"enabled"`
	DefaultAgentID string `json:"defaultAgentId"`
	DefaultModelID string `json:"defaultModelId,omitempty"`
	AutoApprove    *bool  `json:"autoApprove,omitempty"`
}

func weixinConfigure(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Weixin == nil || h.Config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "weixin bridge unavailable"})
			return
		}
		var req weixinEnableRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.Config.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		wx := cfg.Channels.Weixin
		wx.Enabled = req.Enabled
		wx.DefaultProjectID = "" // drop deprecated field on save
		if req.DefaultAgentID != "" {
			wx.DefaultAgentID = req.DefaultAgentID
		}
		if req.DefaultModelID != "" {
			wx.DefaultModelID = req.DefaultModelID
		}
		if req.AutoApprove != nil {
			wx.AutoApprove = *req.AutoApprove
		} else if req.Enabled {
			wx.AutoApprove = true
		}
		if req.Enabled {
			if wx.DefaultAgentID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId required when enabling weixin"})
				return
			}
			if wx.DefaultModelID == "" || !strings.Contains(wx.DefaultModelID, "/") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "defaultModelId required when enabling weixin (provider/model)"})
				return
			}
		}
		sec := cfg.Channels
		sec.Weixin = wx
		if _, err := h.Config.Update(c.Request.Context(), domain.UpdateConfigFileRequest{Channels: &sec}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := h.Weixin.SyncFromConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		st, _ := h.Weixin.Status(c.Request.Context())
		c.JSON(http.StatusOK, st)
	}
}
