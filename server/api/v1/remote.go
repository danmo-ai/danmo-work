package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/remote/connector"

	"github.com/gin-gonic/gin"
)

func remoteStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Remote == nil {
			c.JSON(http.StatusOK, connector.Status{Enabled: false})
			return
		}
		c.JSON(http.StatusOK, h.Remote.GetStatus())
	}
}

type remoteConfigureRequest struct {
	Enabled     bool   `json:"enabled"`
	HubURL      string `json:"hubUrl"`
	LocalBase   string `json:"localBase"`
	TLSInsecure bool   `json:"tlsInsecure"`
}

func remoteConfigure(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Remote == nil || h.Config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "remote connector not available"})
			return
		}
		var req remoteConfigureRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.HubURL = strings.TrimSpace(req.HubURL)
		req.LocalBase = strings.TrimSpace(req.LocalBase)
		if req.Enabled && req.HubURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hubUrl required when enabled"})
			return
		}
		if req.LocalBase == "" {
			req.LocalBase = "http://127.0.0.1:7801"
		}

		section := domain.ConfigRemoteSection{
			Enabled:     req.Enabled,
			HubURL:      req.HubURL,
			LocalBase:   req.LocalBase,
			TLSInsecure: req.TLSInsecure,
		}
		if _, err := h.Config.Update(c, domain.UpdateConfigFileRequest{Remote: &section}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		cur := h.Remote.Config()
		h.Remote.Apply(connector.Config{
			Enabled:      section.Enabled,
			HubURL:       section.HubURL,
			LocalBase:    section.LocalBase,
			TLSInsecure:  section.TLSInsecure,
			AppVersion:   cur.AppVersion,
			IdentityPath: cur.IdentityPath,
		})
		c.JSON(http.StatusOK, h.Remote.GetStatus())
	}
}

func remotePairCode(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Remote == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "remote connector not started"})
			return
		}
		st := h.Remote.GetStatus()
		if !st.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "remote disabled"})
			return
		}
		if !st.Connected {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device_offline: connect to hub first"})
			return
		}
		code, expiresIn, err := h.Remote.RequestPairingCode(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": code, "expiresIn": expiresIn})
	}
}

func remoteRevoke(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Remote == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "remote connector not started"})
			return
		}
		if err := h.Remote.RevokeTokens(c.Request.Context()); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
