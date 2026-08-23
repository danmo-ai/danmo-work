package v1

import (
	"net/http"
	"strings"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

// validateChannelDefaultAgent rejects subagent ids for IM channel defaults.
func validateChannelDefaultAgent(h *Handler, c *gin.Context, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return true
	}
	if h.Agents == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent manager unavailable"})
		return false
	}
	agent, err := h.Agents.Get(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId not found"})
		return false
	}
	if agent.Mode == domain.AgentModeSubagent {
		c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAgentId must be a primary agent"})
		return false
	}
	return true
}
