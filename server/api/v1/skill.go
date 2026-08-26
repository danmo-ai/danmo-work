package v1

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/runtime/tool/builtin"
	"danmo-work/core/service"

	"github.com/gin-gonic/gin"
)

type SkillHandler struct {
	Skills   *service.SkillManager
	Projects *service.ProjectManager
	Importer *service.SkillImporter
	DataDir  string
}

func listSkills(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var projectDir string
		projectID := c.Query("project_id")
		if projectID != "" && h.Projects != nil {
			if proj, err := h.Projects.Get(c, projectID); err == nil {
				projectDir = proj.Directory
			}
		}

		skills, err := h.Skills.List(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if projectDir != "" {
			skills = h.Skills.Scan(projectDir)
			projRoots := paths.ProjectSkillDirs(projectDir)
			for i := range skills {
				for _, root := range projRoots {
					if paths.PathUnderRoot(skills[i].Dir, root) {
						skills[i].ProjectID = projectID
						break
					}
				}
			}
		}

		for i := range skills {
			h.normalizeSkill(&skills[i], projectDir)
		}
		c.JSON(http.StatusOK, skills)
	}
}

func (h *SkillHandler) skillDisplayPath(skillDir, projectDir string) string {
	agentsHome, _ := os.UserHomeDir()
	agentsHome = filepath.Join(agentsHome, ".agents")
	pluginDirs := paths.PluginSkillDirs(h.DataDir)
	if h.Skills != nil {
		pluginDirs = append(pluginDirs, h.Skills.PluginSkillDirs()...)
	}
	return builtin.SkillPathForPromptWithPlugins(skillDir, h.DataDir, agentsHome, projectDir, pluginDirs)
}

func (h *SkillHandler) normalizeSkill(sk *domain.Skill, projectDir string) {
	if sk == nil {
		return
	}
	sk.Dir = h.skillDisplayPath(sk.Dir, projectDir)
}

func getSkill(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		skill, err := h.Skills.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if skill == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		h.normalizeSkill(skill, "")
		c.JSON(http.StatusOK, skill)
	}
}

func createSkill(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sk domain.Skill
		if err := c.ShouldBindJSON(&sk); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if sk.ID == "" {
			if sk.Name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "id or name required"})
				return
			}
			sk.ID = sk.Name
		}
		if err := h.Skills.Upsert(c, sk); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if out, err := h.Skills.Get(c, sk.ID); err == nil && out != nil {
			h.normalizeSkill(out, "")
			c.JSON(http.StatusCreated, out)
			return
		}
		h.normalizeSkill(&sk, "")
		c.JSON(http.StatusCreated, sk)
	}
}

func updateSkill(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sk domain.Skill
		if err := c.ShouldBindJSON(&sk); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sk.ID = id
		if err := h.Skills.Upsert(c, sk); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out, err := h.Skills.Get(c, id)
		if err != nil || out == nil {
			h.normalizeSkill(&sk, "")
			c.JSON(http.StatusOK, sk)
			return
		}
		h.normalizeSkill(out, "")
		c.JSON(http.StatusOK, out)
	}
}

func deleteSkill(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := h.Skills.Delete(c, id); err != nil {
			if errors.Is(err, service.ErrBuiltinSkill) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := h.Skills.DeleteFiles(c, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func importSkillDir(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}
		skill, files, err := h.Importer.Import(req.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.Skills.Upsert(c, *skill); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = h.Skills.DeleteFiles(c, skill.ID)
		for _, f := range files {
			if err := h.Skills.UpsertFile(c, f); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		sourcePath := skill.SourcePath
		if out, err := h.Skills.Get(c, skill.ID); err == nil && out != nil {
			out.SourcePath = sourcePath
			h.normalizeSkill(out, "")
			skill = out
		} else {
			h.normalizeSkill(skill, "")
		}
		c.JSON(http.StatusCreated, gin.H{"skill": skill, "fileCount": len(files)})
	}
}

func exportSkillMD(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		skill, err := h.Skills.Get(c, c.Param("id"))
		if err != nil || skill == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		md := h.Importer.ToSkillMD(*skill)
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.String(http.StatusOK, md)
	}
}

func resetSkill(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		skill, err := h.Skills.ResetFromTemplate(c, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.normalizeSkill(skill, "")
		c.JSON(http.StatusOK, skill)
	}
}

func listSkillFiles(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		files, err := h.Skills.Files(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Omit content blobs from listing responses.
		out := make([]domain.SkillFile, len(files))
		for i, f := range files {
			f.Content = nil
			out[i] = f
		}
		c.JSON(http.StatusOK, out)
	}
}

func getSkillFile(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		fpath := strings.TrimPrefix(c.Param("path"), "/")
		f, err := h.Skills.File(c, c.Param("id"), fpath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if isTextFile(f.Path) {
			c.String(http.StatusOK, string(f.Content))
		} else {
			c.Data(http.StatusOK, "application/octet-stream", f.Content)
		}
	}
}

func upsertSkillFile(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		skillID := c.Param("id")
		fpath := strings.TrimPrefix(c.Param("path"), "/")
		if _, err := h.Skills.Get(c, skillID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		path, err := service.NormalizeSkillResourcePath(fpath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		f := domain.SkillFile{
			ID:      skillID + ":" + path,
			SkillID: skillID,
			Path:    path,
			Content: []byte(req.Content),
			Size:    int64(len(req.Content)),
		}
		if err := h.Skills.UpsertFile(c, f); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, domain.SkillFile{
			ID:      f.ID,
			SkillID: skillID,
			Path:    path,
			Size:    f.Size,
		})
	}
}

func deleteSkillFile(h *SkillHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		skillID := c.Param("id")
		fpath := strings.TrimPrefix(c.Param("path"), "/")
		if _, err := h.Skills.Get(c, skillID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		if err := h.Skills.DeleteFile(c, skillID, fpath); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func isTextFile(path string) bool {
	textExt := []string{".md", ".txt", ".yaml", ".yml", ".json", ".toml", ".py", ".sh", ".js", ".ts", ".go", ".rs", ".html", ".css", ".xml", ".csv", ".ini", ".cfg"}
	lower := strings.ToLower(path)
	for _, ext := range textExt {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
