package v1

import (
	"net/http"
	"strconv"

	"danmo-work/core/domain"
	"danmo-work/core/service"

	"github.com/gin-gonic/gin"
)

type MarketHandler struct {
	Market *service.MarketManager
}

func listMarketSources(h *MarketHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sources, err := h.Market.ListSources(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sources)
	}
}

func listMarketCatalog(h *MarketHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		refresh, _ := strconv.ParseBool(c.Query("refresh"))
		items, warnings, err := h.Market.ListCatalog(c, refresh)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if items == nil {
			items = []domain.MarketListing{}
		}
		c.JSON(http.StatusOK, domain.MarketCatalogResponse{
			Items:    items,
			Warnings: warnings,
		})
	}
}

func installMarketItem(h *MarketHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.InstallMarketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := h.Market.Install(c, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func uninstallMarketItem(h *MarketHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UninstallMarketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.Market.Uninstall(c, req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
