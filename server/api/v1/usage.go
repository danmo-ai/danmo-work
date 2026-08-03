package v1

import (
	"net/http"
	"time"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

func getSessionUsage(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		bd, err := h.Store.Usage().SummarizeSession(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, bd)
	}
}

func getProjectUsage(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		bd, err := h.Store.Usage().SummarizeProject(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, bd)
	}
}

func getUsageSummary(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		sum, err := h.Store.Usage().SummarizeScope(c, c.Query("project_id"), c.Query("model"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sum)
	}
}

func getUsageSeries(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		filter := domain.UsageSeriesFilter{
			Period:    domain.UsagePeriod(c.DefaultQuery("period", "day")),
			ProjectID: c.Query("project_id"),
			Model:     c.Query("model"),
			AgentID:   c.Query("agent_id"),
			Grain:     domain.UsageGrain(c.DefaultQuery("grain", "session")),
		}
		if v := c.Query("from"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.From = t
			}
		}
		if v := c.Query("to"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.To = t
			}
		}
		points, err := h.Store.Usage().Series(c, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"points": points})
	}
}

func listUsageModels(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		projectID := c.Query("project_id")
		rows, err := h.Store.Usage().ListByProject(c, projectID, domain.UsageGrainModel)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": rows})
	}
}

func listUsageAgents(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store unavailable"})
			return
		}
		projectID := c.Query("project_id")
		rows, err := h.Store.Usage().ListByProject(c, projectID, domain.UsageGrainAgent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"agents": rows})
	}
}
