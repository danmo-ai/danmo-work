package v1

import (
	"net/http"

	"danmo-work/core/domain"

	"github.com/gin-gonic/gin"
)

func listKnowledgeBases(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		bases, err := h.Knowledge.ListBases(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if bases == nil {
			bases = []domain.KnowledgeBase{}
		}
		c.JSON(http.StatusOK, bases)
	}
}

func createKnowledgeBase(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		var req domain.CreateKnowledgeBaseRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		base, err := h.Knowledge.CreateBase(c, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, base)
	}
}

func getKnowledgeBase(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		base, err := h.Knowledge.GetBase(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, base)
	}
}

func updateKnowledgeBase(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		var req domain.UpdateKnowledgeBaseRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		base, err := h.Knowledge.UpdateBase(c, c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, base)
	}
}

func deleteKnowledgeBase(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		if err := h.Knowledge.DeleteBase(c, c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func listKnowledgeDocs(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		docs, err := h.Knowledge.ListDocs(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if docs == nil {
			docs = []domain.KnowledgeDoc{}
		}
		c.JSON(http.StatusOK, docs)
	}
}

func createKnowledgeDoc(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		var req domain.UpsertKnowledgeDocRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		doc, err := h.Knowledge.CreateDoc(c, c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, doc)
	}
}

func getKnowledgeDoc(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		doc, err := h.Knowledge.GetDoc(c, c.Param("docId"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, doc)
	}
}

func updateKnowledgeDoc(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		var req domain.UpsertKnowledgeDocRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		doc, err := h.Knowledge.UpdateDoc(c, c.Param("docId"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, doc)
	}
}

func deleteKnowledgeDoc(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Knowledge == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "knowledge unavailable"})
			return
		}
		if err := h.Knowledge.DeleteDoc(c, c.Param("docId")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
