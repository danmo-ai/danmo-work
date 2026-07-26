package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type Handler struct {
	Sessions     *service.SessionManager
	Projects     *service.ProjectManager
	LLMConfig    *service.LLMConfigManager
	Config       *service.ConfigManager
	SearchConfig *service.SearchConfigManager
	Agents        *service.AgentManager
	Skills        *service.SkillManager
	SkillHandler  *SkillHandler
	MarketHandler *MarketHandler
	TurnLogs      *service.TurnLogManager
	MCPServers    *service.MCPManager
	Automations   *service.AutomationManager
	Weixin        *service.WeixinBridge
	Feishu        *service.FeishuBridge
	Wecom         *service.WecomBridge
	QQ            *service.QQBridge
	Channels      *service.ChannelManager
	Sandbox       port.Sandbox
	Browser       port.Browser
	Store         port.Repository
	TableStore    port.TableStoreRepo
}

type RouterConfig struct {
	FrontendDir string
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// Allow Tauri desktop (tauri://localhost), web dev (localhost:*), and same-origin
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func NewRouter(h *Handler, cfg RouterConfig) *gin.Engine {
	r := gin.New()
	r.Use(corsMiddleware())
	api := r.Group("/api/v1")
	api.POST("/sessions", createSession(h))
	api.GET("/sessions", listSessions(h))
	api.GET("/sessions/:id", getSession(h))
	api.PATCH("/sessions/:id", updateSession(h))
	api.DELETE("/sessions/:id", deleteSession(h))
	api.POST("/sessions/:id/turns", sendMessage(h))
	api.GET("/sessions/:id/turns", listTurns(h))
	api.POST("/sessions/:id/turns/:turnID/resume", resumeTurn(h))
	api.DELETE("/sessions/:id/turns/:turnID", cancelTurn(h))
	api.GET("/sessions/:id/turns/:turnID/log", downloadTurnLog(h))
	api.GET("/sessions/:id/events", streamEvents(h))
	api.GET("/sessions/:id/events/poll", pollEvents(h))
	api.POST("/projects", createProject(h))
	api.GET("/projects", listProjects(h))
	api.GET("/projects/:id", getProject(h))
	api.PATCH("/projects/:id", updateProject(h))
	api.DELETE("/projects/:id", deleteProject(h))
	api.GET("/projects/:id/sessions", listProjectSessions(h))
	api.GET("/projects/:id/files", listProjectFiles(h))
	api.GET("/projects/:id/files/content", readProjectFile(h))
	api.PUT("/projects/:id/files/content", writeProjectFile(h))
	api.GET("/projects/:id/raw/*filepath", serveProjectFile(h))
	api.GET("/proxy/*target", proxyDevServer(h))
	api.GET("/projects/:id/git-changes", getProjectGitChanges(h))
	api.GET("/projects/:id/git-branches", getProjectGitBranches(h))
	api.POST("/projects/:id/git-checkout", checkoutProjectGitBranch(h))
	api.GET("/projects/:id/terminal", projectTerminal(h))
	api.GET("/llm/configs", getLLMConfigs(h))
	api.POST("/llm/configs", createLLMConfig(h))
	api.PUT("/llm/configs/:id", updateLLMConfig(h))
	api.DELETE("/llm/configs/:id", deleteLLMConfig(h))
	api.POST("/llm/configs/fetch-models", fetchLLMModelsFromRequest(h))
	api.POST("/llm/configs/:id/refresh-models", refreshLLMModels(h))
	api.PATCH("/llm/configs/:id/models/:modelName", toggleLLMModel(h))
	api.GET("/llm/models", listLLMModels(h))
	api.GET("/llm/presets", getLLMPresets(h))
	api.POST("/approvals/:id/decide", decideApproval(h))
	api.POST("/asks/:id/resolve", resolveAskUser(h))
	api.GET("/config", getConfig(h))
	api.PUT("/config", updateConfig(h))
	api.GET("/channels/weixin/status", weixinStatus(h))
	api.PUT("/channels/weixin", weixinConfigure(h))
	api.POST("/channels/weixin/login/start", weixinLoginStart(h))
	api.POST("/channels/weixin/login/wait", weixinLoginWait(h))
	api.POST("/channels/weixin/logout", weixinLogout(h))
	api.PUT("/channels/weixin/accounts/:id", weixinUpdateAccount(h))
	api.GET("/channels/weixin/bindings", weixinBindings(h))

	api.GET("/channels/feishu/status", feishuStatus(h))
	api.PUT("/channels/feishu", feishuConfigure(h))
	api.GET("/channels/wecom/status", wecomStatus(h))
	api.PUT("/channels/wecom", wecomConfigure(h))
	api.GET("/channels/qq/status", qqStatus(h))
	api.PUT("/channels/qq", qqConfigure(h))
	api.GET("/sandbox/status", getSandboxStatus(h))
	api.GET("/browser/status", getBrowserStatus(h))
	api.GET("/model-configs", getModelConfigs(h))
	api.PUT("/model-configs", updateModelConfigs(h))
	api.GET("/search/config", getSearchConfig(h))
	api.PUT("/search/config", updateSearchConfig(h))
	api.GET("/agents", listAgents(h))
	api.POST("/agents", createAgent(h))
	api.GET("/agents/:id", getAgent(h))
	api.GET("/agents/:id/available-skills", listAgentAvailableSkills(h))
	api.PUT("/agents/:id", updateAgent(h))
	api.POST("/agents/:id/reset", resetAgent(h))
	api.DELETE("/agents/:id", deleteAgent(h))
	api.GET("/memories", listMemories(h))
	api.DELETE("/memories", deleteMemory(h))
	api.GET("/tables", listTables(h))
	api.GET("/tables/:table/rows", listTableRows(h))
	api.GET("/tables/:table/rows/:key", getTableRow(h))
	api.DELETE("/tables/:table/rows/:key", deleteTableRow(h))
	api.GET("/mcp/servers", listMCPServers(h))
	api.POST("/mcp/servers", createMCPServer(h))
	api.GET("/mcp/servers/:id", getMCPServer(h))
	api.PUT("/mcp/servers/:id", updateMCPServer(h))
	api.DELETE("/mcp/servers/:id", deleteMCPServer(h))
	api.POST("/mcp/servers/:id/refresh-tools", refreshMCPTools(h))
	api.PATCH("/mcp/servers/:id/tools/:toolName", toggleMCPTool(h))
	api.POST("/mcp/servers/:id/oauth/begin", beginMCPOAuth(h))
	api.POST("/mcp/servers/:id/oauth/complete", completeMCPOAuth(h))
	api.GET("/mcp/catalog", listConnectorCatalog(h))
	api.POST("/mcp/catalog/:id/install", installConnectorCatalog(h))

	api.GET("/automations", listAutomations(h))
	api.POST("/automations", createAutomation(h))
	api.GET("/automations/:id", getAutomation(h))
	api.PUT("/automations/:id", updateAutomation(h))
	api.DELETE("/automations/:id", deleteAutomation(h))
	api.POST("/automations/:id/toggle", toggleAutomation(h))
	api.POST("/automations/:id/run", runAutomation(h))
	api.POST("/hooks/*path", automationWebhook(h))

	api.GET("/skills", listSkills(h.SkillHandler))
	api.POST("/skills", createSkill(h.SkillHandler))
	api.GET("/skills/:id", getSkill(h.SkillHandler))
	api.PUT("/skills/:id", updateSkill(h.SkillHandler))
	api.DELETE("/skills/:id", deleteSkill(h.SkillHandler))
	api.POST("/skills/import", importSkillDir(h.SkillHandler))
	api.POST("/skills/:id/reset", resetSkill(h.SkillHandler))
	api.GET("/skills/:id/export", exportSkillMD(h.SkillHandler))
	api.GET("/skills/:id/files", listSkillFiles(h.SkillHandler))
	api.PUT("/skills/:id/files/*path", upsertSkillFile(h.SkillHandler))
	api.DELETE("/skills/:id/files/*path", deleteSkillFile(h.SkillHandler))
	api.GET("/skills/:id/files/*path", getSkillFile(h.SkillHandler))

	if h.MarketHandler != nil {
		api.GET("/market/sources", listMarketSources(h.MarketHandler))
		api.GET("/market/catalog", listMarketCatalog(h.MarketHandler))
		api.POST("/market/install", installMarketItem(h.MarketHandler))
		api.POST("/market/uninstall", uninstallMarketItem(h.MarketHandler))
	}

	api.GET("/version", getVersion())

	if cfg.FrontendDir != "" {
		r.Static("/app", cfg.FrontendDir)
		r.NoRoute(func(c *gin.Context) {
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/app" {
				c.File(cfg.FrontendDir + "/index.html")
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
	}

	return r
}

func createSession(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.CreateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if cfgs, _ := h.LLMConfig.GetAll(c); len(cfgs) > 0 {
			if _, _, err := h.LLMConfig.ResolveProvider(c, req.ModelID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		session, err := h.Sessions.Create(c, req)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "required") || strings.Contains(msg, "attachments[") || strings.Contains(msg, "unsupported") {
				c.JSON(http.StatusBadRequest, gin.H{"error": msg})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusCreated, session)
	}
}

func listSessions(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := h.Sessions.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sessions)
	}
}

func getSession(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, err := h.Sessions.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

func updateSession(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpdateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		session, err := h.Sessions.Update(c, c.Param("id"), req)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

func deleteSession(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.Sessions.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func listTurns(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		turns := h.Sessions.ListTurns(c.Param("id"))
		c.JSON(http.StatusOK, turns)
	}
}

func resumeTurn(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.Sessions.ResumeTurn(c, c.Param("id"), c.Param("turnID")); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, port.ErrSessionTurnRunning) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "resumed"})
	}
}

func streamEvents(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		since, _ := strconv.ParseInt(c.DefaultQuery("since_seq", "0"), 10, 64)
		// Subscribe before backfill so live events published during the poll are buffered.
		ch := h.Sessions.Subscribe(sessionID)
		defer h.Sessions.Unsubscribe(sessionID, ch)
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		maxSent := since
		for _, ev := range h.Sessions.StreamEvents(sessionID, since) {
			data, _ := json.Marshal(ev)
			c.SSEvent("message", string(data))
			if ev.Seq > maxSent {
				maxSent = ev.Seq
			}
		}
		c.Writer.Flush()

		c.Stream(func(w io.Writer) bool {
			ev, ok := <-ch
			if !ok {
				return false
			}
			if ev.Seq <= maxSent {
				return true
			}
			maxSent = ev.Seq
			data, _ := json.Marshal(ev)
			c.SSEvent("message", string(data))
			return true
		})
	}
}

func decideApproval(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Approved bool   `json:"approved"`
			Scope    string `json:"scope"` // once | session
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Scope == "" {
			req.Scope = "once"
		}
		if err := h.Sessions.DecideApproval(c, c.Param("id"), req.Approved, req.Scope); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func resolveAskUser(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Answer string `json:"answer"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Answer == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "answer required"})
			return
		}
		if err := h.Sessions.ResolveAskUser(c.Param("id"), req.Answer); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func pollEvents(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		since, _ := strconv.ParseInt(c.DefaultQuery("since", "0"), 10, 64)
		events := h.Sessions.StreamEvents(c.Param("id"), since)
		c.JSON(http.StatusOK, events)
	}
}

func listAgents(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		agents, err := h.Agents.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, agents)
	}
}

func createAgent(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var a domain.Agent
		if err := c.ShouldBindJSON(&a); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if a.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent id required"})
			return
		}
		if a.Mode == "" {
			a.Mode = domain.AgentModePrimary
		}
		domain.NormalizeAgentBindings(&a)
		if err := h.Agents.Upsert(c, a); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, a)
	}
}

func getAgent(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		agent, err := h.Agents.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, agent)
	}
}

// listAgentAvailableSkills returns skills usable for this agent in a project
// workDir: agent.skillIds (DB) ∪ filesystem skills (user + project dirs).
func listAgentAvailableSkills(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Skills == nil || h.Agents == nil || h.Projects == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "skills unavailable"})
			return
		}
		agent, err := h.Agents.Get(c, c.Param("id"))
		if err != nil || agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		projectID := strings.TrimSpace(c.Query("projectId"))
		workDir := h.Projects.ResolveWorkDir(c, projectID)
		skills, err := service.ListAvailableSkillsForAgent(c, h.Skills, *agent, workDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if skills == nil {
			skills = []service.AvailableSkill{}
		}
		c.JSON(http.StatusOK, skills)
	}
}

func updateAgent(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var a domain.Agent
		if err := c.ShouldBindJSON(&a); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		a.ID = id
		if a.Mode == "" {
			a.Mode = domain.AgentModePrimary
		}
		domain.NormalizeAgentBindings(&a)
		if err := h.Agents.Upsert(c, a); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

func deleteAgent(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.Agents.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func resetAgent(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		agent, err := h.Agents.ResetFromTemplate(c, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, agent)
	}
}

func listMemories(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusOK, []domain.Memory{})
			return
		}
		projectID := strings.TrimSpace(c.Query("projectId"))
		agentID := strings.TrimSpace(c.Query("agentId"))
		scopes := []domain.MemoryScopeRef{{
			Scope:   domain.MemoryScopeUser,
			ScopeID: domain.MemoryUserScopeID,
		}}
		if projectID != "" {
			scopes = append(scopes, domain.MemoryScopeRef{
				Scope:   domain.MemoryScopeProject,
				ScopeID: projectID,
			})
		}
		if agentID != "" {
			scopes = append(scopes, domain.MemoryScopeRef{
				Scope:   domain.MemoryScopeAgent,
				ScopeID: agentID,
			})
		}
		items, err := h.Store.Memories().Search(c, domain.MemoryQuery{
			Scopes: scopes,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if items == nil {
			items = []domain.Memory{}
		}
		c.JSON(http.StatusOK, items)
	}
}

func deleteMemory(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store unavailable"})
			return
		}
		scope := domain.MemoryScope(strings.TrimSpace(c.Query("scope")))
		scopeID := strings.TrimSpace(c.Query("scopeId"))
		key := strings.TrimSpace(c.Query("key"))
		if scope == "" || key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope and key are required"})
			return
		}
		switch scope {
		case domain.MemoryScopeUser:
			if scopeID == "" {
				scopeID = domain.MemoryUserScopeID
			}
		case domain.MemoryScopeProject, domain.MemoryScopeAgent:
			if scopeID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "scopeId is required"})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be user, project, or agent"})
			return
		}
		if err := h.Store.Memories().Delete(c, scope, scopeID, key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func resolveTableScopeQuery(c *gin.Context) (domain.TableScope, string, error) {
	scope := domain.TableScope(strings.TrimSpace(c.Query("scope")))
	scopeID := strings.TrimSpace(c.Query("scopeId"))
	switch scope {
	case domain.TableScopeUser:
		if scopeID == "" {
			scopeID = domain.TableUserScopeID
		}
		return scope, scopeID, nil
	case domain.TableScopeProject, domain.TableScopeAgent:
		if scopeID == "" {
			return "", "", fmt.Errorf("scopeId is required")
		}
		return scope, scopeID, nil
	default:
		return "", "", fmt.Errorf("scope must be user, project, or agent")
	}
}

func listTables(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.TableStore == nil {
			c.JSON(http.StatusOK, []domain.TableInfo{})
			return
		}
		scope := strings.TrimSpace(c.Query("scope"))
		scopeID := strings.TrimSpace(c.Query("scopeId"))
		projectID := strings.TrimSpace(c.Query("projectId"))
		agentID := strings.TrimSpace(c.Query("agentId"))

		var refs []domain.TableScopeRef
		if scope != "" {
			s, id, err := resolveTableScopeQuery(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			refs = []domain.TableScopeRef{{Scope: s, ScopeID: id}}
		} else {
			refs = []domain.TableScopeRef{{Scope: domain.TableScopeUser, ScopeID: domain.TableUserScopeID}}
			if projectID != "" {
				refs = append(refs, domain.TableScopeRef{Scope: domain.TableScopeProject, ScopeID: projectID})
			}
			if agentID != "" {
				refs = append(refs, domain.TableScopeRef{Scope: domain.TableScopeAgent, ScopeID: agentID})
			}
			_ = scopeID
		}
		items, err := h.TableStore.ListTables(c, refs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if items == nil {
			items = []domain.TableInfo{}
		}
		c.JSON(http.StatusOK, items)
	}
}

func listTableRows(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.TableStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "table store unavailable"})
			return
		}
		scope, scopeID, err := resolveTableScopeQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		table := strings.TrimSpace(c.Param("table"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		rows, err := h.TableStore.Query(c, domain.TableQuery{
			Scope:   scope,
			ScopeID: scopeID,
			Table:   table,
			Order:   strings.TrimSpace(c.Query("order")),
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []domain.TableRow{}
		}
		c.JSON(http.StatusOK, rows)
	}
}

func getTableRow(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.TableStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "table store unavailable"})
			return
		}
		scope, scopeID, err := resolveTableScopeQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row, err := h.TableStore.Get(c, scope, scopeID, c.Param("table"), c.Param("key"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func deleteTableRow(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.TableStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "table store unavailable"})
			return
		}
		scope, scopeID, err := resolveTableScopeQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.TableStore.Delete(c, scope, scopeID, c.Param("table"), c.Param("key")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func sendMessage(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.SendMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		modelID := req.ModelID
		if modelID == "" {
			session, err := h.Sessions.Get(c, c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			modelID = session.ModelID
		}
		if cfgs, _ := h.LLMConfig.GetAll(c); len(cfgs) > 0 {
			if _, _, err := h.LLMConfig.ResolveProvider(c, modelID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		turnID, err := h.Sessions.StartTurn(c, c.Param("id"), req)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, port.ErrSessionTurnRunning) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"turnId": turnID})
	}
}

func cancelTurn(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		h.Sessions.CancelTurn(c, c.Param("turnID"))
		c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
	}
}

func downloadTurnLog(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		turnID := c.Param("turnID")
		events := h.Sessions.StreamEvents(sessionID, 0)
		data, err := h.TurnLogs.LoadTurnLogZip(turnID, events)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "turn log not found"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", turnID))
		c.Header("Content-Type", "application/zip")
		c.Data(http.StatusOK, "application/zip", data)
	}
}

func createProject(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.CreateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := h.Projects.Create(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, p)
	}
}

func listProjects(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		projects, err := h.Projects.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, projects)
	}
}

func getProject(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := h.Projects.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if p.ID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func updateProject(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpdateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := h.Projects.Update(c, c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func deleteProject(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.Projects.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func listProjectSessions(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := h.Projects.SessionsForProject(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sessions)
	}
}

func listLLMModels(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		models := h.LLMConfig.ListModels(c)
		c.JSON(http.StatusOK, models)
	}
}

func getLLMPresets(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := h.Config.Get(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg.LLM.Providers)
	}
}

func getLLMConfigs(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfgs, err := h.LLMConfig.GetAll(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfgs)
	}
}

func createLLMConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertLLMProviderConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.LLMConfig.Upsert(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, cfg)
	}
}

func updateLLMConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertLLMProviderConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.LLMConfig.Upsert(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func deleteLLMConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.LLMConfig.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func fetchLLMModelsFromRequest(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertLLMProviderConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		models, err := h.LLMConfig.FetchModelsFromRequest(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	}
}

func refreshLLMModels(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		models, err := h.LLMConfig.FetchModels(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	}
}

func toggleLLMModel(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.LLMConfig.ToggleModel(c, c.Param("id"), c.Param("modelName"), req.Enabled)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func getSearchConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := h.SearchConfig.Get(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func updateSearchConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertSearchConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.SearchConfig.Upsert(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func getConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := h.Config.Get(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func updateConfig(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpdateConfigFileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := h.Config.Update(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.Sandbox != nil && req.Runtime != nil {
			h.Sandbox.Configure(cfg.Runtime.Sandbox)
		}
		if h.Browser != nil && req.Runtime != nil {
			h.Browser.Configure(cfg.Runtime.Browser)
		}
		c.JSON(http.StatusOK, cfg)
	}
}

func getSandboxStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Sandbox == nil {
			c.JSON(http.StatusOK, domain.SandboxStatus{
				Enabled:        false,
				Backend:        domain.SandboxBackendDisabled,
				Degraded:       true,
				DegradedReason: "sandbox not initialized",
			})
			return
		}
		// Refresh policy from persisted config so Status matches disk.
		if h.Config != nil {
			if cfg, err := h.Config.Get(c); err == nil {
				h.Sandbox.Configure(cfg.Runtime.Sandbox)
			}
		}
		c.JSON(http.StatusOK, h.Sandbox.Status())
	}
}

func getBrowserStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Browser == nil {
			c.JSON(http.StatusOK, domain.BrowserStatus{
				Available:      false,
				Enabled:        false,
				Engine:         "none",
				Mode:           "none",
				DegradedReason: "browser not initialized",
			})
			return
		}
		if h.Config != nil {
			if cfg, err := h.Config.Get(c); err == nil {
				h.Browser.Configure(cfg.Runtime.Browser)
			}
		}
		c.JSON(http.StatusOK, h.Browser.Status())
	}
}

func listProjectFiles(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		nodes, err := h.Projects.ListFiles(c, c.Param("id"), path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, nodes)
	}
}

func readProjectFile(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}

		// Raw mode: serve file directly with Content-Type header
		if c.Query("raw") == "true" {
			data, ct, err := h.Projects.ReadFileRaw(c, c.Param("id"), path)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Strip charset param to avoid mime type parsing issues
			if i := strings.Index(ct, ";"); i != -1 {
				ct = ct[:i]
			}
			c.Header("Content-Type", ct)
			c.Data(http.StatusOK, ct, data)
			return
		}

		fc, err := h.Projects.ReadFileContent(c, c.Param("id"), path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, fc)
	}
}

func writeProjectFile(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		if strings.TrimSpace(req.Path) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}
		if err := h.Projects.WriteFileContent(c, c.Param("id"), req.Path, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": req.Path})
	}
}

// serveProjectFile serves a project file directly at /projects/:id/files/:filepath
func serveProjectFile(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		filepath := strings.TrimPrefix(c.Param("filepath"), "/")
		if filepath == "" || filepath == "content" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
			return
		}
		data, ct, err := h.Projects.ReadFileRaw(c, c.Param("id"), filepath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if i := strings.Index(ct, ";"); i != -1 {
			ct = ct[:i]
		}
		// Inject inspect listener into HTML responses
		if ct == "text/html" {
			data = append(data, []byte(dqInspectScript)...)
		}
		c.Header("Content-Type", ct)
		c.Data(http.StatusOK, ct, data)
	}
}

// proxyDevServer proxies external dev servers (e.g. localhost:3000) and injects the inspect script.
func proxyDevServer(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.Param("target"), "/")
		if raw == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target is required"})
			return
		}
		// Convert localhost-3000/path to http://localhost:3000/path
		// Replace first dash after the host portion with colon
		parts := strings.SplitN(raw, "/", 2)
		host := strings.Replace(parts[0], "-", ":", 1) // localhost-3000 → localhost:3000
		path := ""
		if len(parts) > 1 {
			path = "/" + parts[1]
		}
		targetURL := "http://" + host + path
		if q := c.Request.URL.RawQuery; q != "" {
			targetURL += "?" + q
		}

		resp, err := http.Get(targetURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		ct := resp.Header.Get("Content-Type")
		if i := strings.Index(ct, ";"); i != -1 {
			ct = ct[:i]
		}

		// Inject <base> tag and inspect script into HTML
		if ct == "text/html" {
			baseTag := fmt.Sprintf(`<base href="http://%s/">`, host)
			body = bytes.Replace(body, []byte("<head>"), []byte("<head>"+baseTag), 1)
			body = append(body, []byte(dqInspectScript)...)
		}

		for k, vs := range resp.Header {
			for _, v := range vs {
				c.Header(k, v)
			}
		}
		c.Header("Content-Type", ct)
		c.Data(resp.StatusCode, ct, body)
	}
}

func getProjectGitChanges(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		changes, err := h.Projects.GetGitChanges(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, changes)
	}
}

func getProjectGitBranches(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		branches, err := h.Projects.ListGitBranches(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, branches)
	}
}

func checkoutProjectGitBranch(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Branch string `json:"branch"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		branches, err := h.Projects.CheckoutGitBranch(c, c.Param("id"), req.Branch)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, branches)
	}
}

// ---- MCP Servers ----

func listMCPServers(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		servers, err := h.MCPServers.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, servers)
	}
}

func createMCPServer(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertMCPServerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.MCPServers.Create(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func getMCPServer(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := h.MCPServers.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

func updateMCPServer(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertMCPServerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.MCPServers.Update(c, c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

func deleteMCPServer(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.MCPServers.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func refreshMCPTools(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		tools, err := h.MCPServers.RefreshTools(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"tools": tools})
	}
}

func toggleMCPTool(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		srv, err := h.MCPServers.ToggleTool(c, c.Param("id"), c.Param("toolName"), req.Enabled)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, srv)
	}
}

func beginMCPOAuth(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RedirectURI string `json:"redirectUri"`
		}
		_ = c.ShouldBindJSON(&req)
		url, state, err := h.MCPServers.BeginOAuth(c, c.Param("id"), req.RedirectURI)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"authorizeUrl": url, "state": state})
	}
}

func completeMCPOAuth(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Code        string `json:"code"`
			State       string `json:"state"`
			AccessToken string `json:"accessToken"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		srv, err := h.MCPServers.CompleteOAuth(c, c.Param("id"), req.Code, req.State, req.AccessToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, srv)
	}
}

func listConnectorCatalog(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, service.DefaultConnectorCatalog())
	}
}

func installConnectorCatalog(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		catalogID := c.Param("id")
		var entry *domain.ConnectorCatalogEntry
		for _, e := range service.DefaultConnectorCatalog() {
			if e.ID == catalogID {
				cp := e
				entry = &cp
				break
			}
		}
		if entry == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "catalog entry not found"})
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = c.ShouldBindJSON(&body)
		req := service.InstallCatalogEntry(*entry, body.Name)
		s, err := h.MCPServers.Create(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

// ---- Automations ----

func listAutomations(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Automations == nil {
			c.JSON(http.StatusOK, []domain.Automation{})
			return
		}
		items, err := h.Automations.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func createAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Automations == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "automations not configured"})
			return
		}
		var req domain.UpsertAutomationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		a, err := h.Automations.Create(c, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, a)
	}
}

func getAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		a, err := h.Automations.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

func updateAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.UpsertAutomationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		a, err := h.Automations.Update(c, c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

func deleteAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.Automations.Delete(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func toggleAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		a, err := h.Automations.Toggle(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

func runAutomation(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		a, err := h.Automations.RunNow(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

func automationWebhook(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Automations == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "automations not configured"})
			return
		}
		path := "/hooks/" + strings.TrimPrefix(c.Param("path"), "/")
		a, err := h.Automations.FindByWebhookPath(c, path)
		if err != nil {
			// also try raw path param
			a, err = h.Automations.FindByWebhookPath(c, c.Param("path"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
		}
		a, err = h.Automations.RunNow(c, a.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, a)
	}
}

// ---- Model Configs ----

func getModelConfigs(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := h.Config.Get(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		models := cfg.LLM.Models
		if models == nil {
			models = []domain.ModelConfig{}
		}
		c.JSON(http.StatusOK, models)
	}
}

func updateModelConfigs(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var models []domain.ModelConfig
		if err := c.ShouldBindJSON(&models); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Validate entries
		for i, m := range models {
			if m.Model == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model name required at index %d", i)})
				return
			}
		}
		cfg, err := h.Config.Get(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		cfg.LLM.Models = models
		if _, err := h.Config.Update(c, domain.UpdateConfigFileRequest{LLM: &cfg.LLM}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.LLMConfig != nil {
			h.LLMConfig.SetModelConfigs(models)
		}
		c.JSON(http.StatusOK, models)
	}
}

// ---- Version ----

func getVersion() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": Version})
	}
}
