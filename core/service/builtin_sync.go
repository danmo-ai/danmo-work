package service

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/resource/home"
)

// SyncBuiltinToFS materializes the embedded builtin resources (agents, skills,
// knowledge bases) into the data home so they go through the same filesystem
// scan/ingest paths as user-created resources.
func SyncBuiltinToFS(dataDir string) error {
	hash, err := home.BuiltinManifestHash()
	if err != nil {
		return fmt.Errorf("builtin manifest: %w", err)
	}

	versionFile := filepath.Join(dataDir, ".builtin_version")
	storedHash, _ := os.ReadFile(versionFile)
	if strings.TrimSpace(string(storedHash)) == hash {
		return nil
	}

	log.Printf("[builtin] version changed, syncing...")

	agentsDir := filepath.Join(dataDir, "agents")
	skillsDir := filepath.Join(dataDir, "skills")

	ts := time.Now().Format("2006-01-02T150405")
	backupDir := filepath.Join(dataDir, ".backups", ts)
	if _, err := os.Stat(agentsDir); err == nil {
		copyDirectory(agentsDir, filepath.Join(backupDir, "agents"))
	}
	if _, err := os.Stat(skillsDir); err == nil {
		copyDirectory(skillsDir, filepath.Join(backupDir, "skills"))
	}
	log.Printf("[builtin] backup: %s", backupDir)

	cleanBuiltinAgents(agentsDir)
	cleanBuiltinSkills(skillsDir)

	if err := copyBuiltinFiles(dataDir); err != nil {
		return fmt.Errorf("copy builtin files: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(versionFile, []byte(hash), 0o644); err != nil {
		return err
	}

	log.Printf("[builtin] sync complete (hash=%s)", hash[:16])
	return nil
}

func cleanBuiltinAgents(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	builtinNames := builtinAgentFiles()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if _, ok := builtinNames[entry.Name()]; ok {
			log.Printf("[builtin] removing old builtin agent: %s", entry.Name())
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func cleanBuiltinSkills(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	builtinIDs := builtinSkillDirs()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := builtinIDs[entry.Name()]; ok {
			log.Printf("[builtin] removing old builtin skill: %s", entry.Name())
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}
}

func builtinAgentFiles() map[string]struct{} {
	out := map[string]struct{}{}
	templates, err := home.LoadAgentTemplates()
	if err != nil {
		return out
	}
	for _, t := range templates {
		out[t.Agent.ID+".md"] = struct{}{}
	}
	return out
}

func builtinSkillDirs() map[string]struct{} {
	out := map[string]struct{}{}
	templates, err := home.LoadSkillTemplates()
	if err != nil {
		return out
	}
	for _, t := range templates {
		out[t.Skill.ID] = struct{}{}
	}
	return out
}

func copyBuiltinFiles(dataDir string) error {
	files, err := home.LoadBuiltinFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		target := filepath.Join(dataDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, f.Content, 0o644); err != nil {
			log.Printf("[builtin] write %s: %v", target, err)
		}
	}
	log.Printf("[builtin] copied %d files", len(files))
	return nil
}

func copyDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer srcFile.Close()
		dstFile, err := os.Create(target)
		if err != nil {
			return nil
		}
		defer dstFile.Close()
		_, _ = io.Copy(dstFile, srcFile)
		return nil
	})
}

func BuiltinVersionHash() string {
	h, _ := home.BuiltinManifestHash()
	return h
}
