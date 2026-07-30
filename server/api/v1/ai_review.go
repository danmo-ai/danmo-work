package v1

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"danmo-work/core/store/turnlog"

	"github.com/gin-gonic/gin"
)

func createTurnSnapshots(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		sess, err := h.Sessions.Get(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		var body struct {
			TurnID string   `json:"turnId"`
			Paths  []string `json:"paths"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		if strings.TrimSpace(body.TurnID) == "" || len(body.Paths) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "turnId and paths required"})
			return
		}
		metas, err := h.AIReview.SnapshotPaths(c.Request.Context(), sess.ProjectID, sessionID, body.TurnID, body.Paths)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"snapshots": metas})
	}
}

func getAIReviewStatus(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		sess, err := h.Sessions.Get(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		turnID := c.Query("turnId")
		path := c.Query("path")
		if turnID == "" || path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "turnId and path required"})
			return
		}
		st, err := h.AIReview.Status(c.Request.Context(), sess.ProjectID, sessionID, turnID, path)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, st)
	}
}

func getAIReviewDiff(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		sess, err := h.Sessions.Get(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		turnID := c.Query("turnId")
		path := c.Query("path")
		if turnID == "" || path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "turnId and path required"})
			return
		}
		diff, err := h.AIReview.Diff(c.Request.Context(), sess.ProjectID, sessionID, turnID, path)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, diff)
	}
}

func revertAIReviewFile(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		turnID := c.Param("turnID")
		sess, err := h.Sessions.Get(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Path) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}
		if err := h.AIReview.Revert(c.Request.Context(), sess.ProjectID, sessionID, turnID, body.Path); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, turnlog.ErrSnapshotNotFound) || errors.Is(err, turnlog.ErrSnapshotNoContent) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": body.Path})
	}
}

func applyAIReviewHunks(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		turnID := c.Param("turnID")
		sess, err := h.Sessions.Get(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		var body struct {
			Path        string `json:"path"`
			AcceptAll   bool   `json:"acceptAll"`
			HunkIndexes []int  `json:"hunkIndexes"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Path) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}
		if err := h.AIReview.ApplyHunks(c.Request.Context(), sess.ProjectID, sessionID, turnID, body.Path, body.AcceptAll, body.HunkIndexes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "path": body.Path})
	}
}

func listSessionFileChanges(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AIReview == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai review unavailable"})
			return
		}
		sessionID := c.Param("id")
		turnID := c.Query("turnId")
		afterSeq, _ := strconv.ParseInt(c.DefaultQuery("afterSeq", "0"), 10, 64)
		recs, err := h.AIReview.ListFileChanges(sessionID, turnID, afterSeq)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"changes": recs})
	}
}
