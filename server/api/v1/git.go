package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"danmo-work/core/service"

	"github.com/gin-gonic/gin"
)

func getProjectGitRemotes(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		remotes, err := h.Git.ListGitRemotes(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, remotes)
	}
}

func addProjectGitRemote(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		remotes, err := h.Git.AddGitRemote(c, c.Param("id"), req.Name, req.URL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, remotes)
	}
}

func getGitCredentials(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		creds, err := h.Git.GitCredentialStatus(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"credentials": creds})
	}
}

func putGitCredentials(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Host     string `json:"host"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.Git.PutGitCredential(c, c.Param("id"), req.Host, req.Username, req.Token); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		creds, err := h.Git.GitCredentialStatus(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"credentials": []service.GitCredentialInfo{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"credentials": creds})
	}
}

func deleteGitCredential(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := strings.TrimSpace(c.Query("host"))
		if host == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host required"})
			return
		}
		if err := h.Git.DeleteGitCredential(c, host); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func stageProjectFiles(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Files  []string `json:"files"`
			Staged bool     `json:"staged"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		changes, err := h.Git.StageFiles(c, c.Param("id"), req.Files, req.Staged)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, changes)
	}
}

func commitProjectGit(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Message string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log, err := h.Git.Commit(c, c.Param("id"), req.Message)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, log)
	}
}

func getProjectGitLog(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 20
		if v := c.Query("limit"); v != "" {
			if n, err := fmt.Sscanf(v, "%d", &limit); err != nil || n != 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
		}
		log, err := h.Git.GetGitLog(c, c.Param("id"), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, log)
	}
}

func streamGitOp(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		op := strings.TrimSpace(c.Query("op"))
		ch, err := h.Git.StreamGitOp(c, c.Param("id"), op)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, service.ErrGitBusy) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Stream(func(w io.Writer) bool {
			ev, ok := <-ch
			if !ok {
				return false
			}
			data, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			return true
		})
	}
}
