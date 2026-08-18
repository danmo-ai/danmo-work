package builtin

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"danmo-work/core/domain"
)

type FileOp struct{}

func (h *FileOp) Name() string                { return "file_op" }
func (h *FileOp) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *FileOp) Describe(args map[string]any) string {
	action, _ := args["action"].(string)
	path, _ := args["path"].(string)
	dest, _ := args["destination"].(string)
	switch action {
	case "move", "copy":
		return action + " " + path + " -> " + dest
	case "delete":
		return "delete " + path
	default:
		return "file_op " + path
	}
}

func (h *FileOp) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "file_op",
		Description: "Moves, copies, or deletes a file or directory on the local filesystem.\n\n" +
			"**Important**: All paths are relative to the project root directory. Use relative paths like 'src/main.go' instead of absolute paths.\n\n" +
			"- action 'move' renames/moves path to destination; 'copy' duplicates it; 'delete' removes it.\n" +
			"- The destination must NOT already exist — this tool never overwrites silently. Delete or move the destination first.\n" +
			"- Directories can only be deleted with recursive=true.\n" +
			"- Deleting a symlink is refused; copying a directory fails on symlinks inside it.\n" +
			"- Do NOT use exec_shell mv/cp/rm for file management — use this tool instead.\n" +
			"- The result summarizes what changed for audit.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":      map[string]any{"type": "string", "enum": []string{"move", "copy", "delete"}, "description": "Operation to perform"},
				"path":        map[string]any{"type": "string", "description": "Relative file or directory path from project root (e.g., 'src/old.go')"},
				"destination": map[string]any{"type": "string", "description": "Relative destination path from project root (required for move/copy)"},
				"recursive":   map[string]any{"type": "boolean", "description": "Required (true) to delete a directory with its contents (default: false)"},
			},
			"required": []string{"action", "path"},
		},
	}
}

func (h *FileOp) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	action, _ := input["action"].(string)
	path, _ := input["path"].(string)
	if action == "" {
		return domain.ToolResult{}, fmt.Errorf("action is required (move | copy | delete)")
	}
	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	workDir := workDirFromInput(input)

	resolvedPath, err := resolveWritePath(workDir, path)
	if err != nil {
		return domain.ToolResult{}, err
	}
	if resolvedPath == filepath.Clean(workDir) {
		return domain.ToolResult{}, fmt.Errorf("refusing to operate on the project root")
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.ToolResult{}, fmt.Errorf("cannot %s %q: no such file or directory", action, path)
		}
		return domain.ToolResult{}, err
	}

	switch action {
	case "move":
		dest, destPath, err := resolveOpDest(workDir, input)
		if err != nil {
			return domain.ToolResult{}, err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot move %q to %q: %w", path, destPath, err)
		}
		if err := os.Rename(resolvedPath, dest); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot move %q to %q: %w", path, destPath, err)
		}
		noteReadFile(input, dest)
		return domain.ToolResult{
			Content: fmt.Sprintf("Moved %q -> %q", path, destPath),
			Meta:    map[string]any{"path": path, "destination": destPath, "op": "move"},
		}, nil

	case "copy":
		dest, destPath, err := resolveOpDest(workDir, input)
		if err != nil {
			return domain.ToolResult{}, err
		}
		if info.IsDir() {
			n, err := copyDirTree(resolvedPath, dest)
			if err != nil {
				return domain.ToolResult{}, err
			}
			return domain.ToolResult{
				Content: fmt.Sprintf("Copied directory %q -> %q (%d files)", path, destPath, n),
				Meta:    map[string]any{"path": path, "destination": destPath, "op": "copy", "files_copied": n},
			}, nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return domain.ToolResult{}, fmt.Errorf("cannot copy symlink %q — copy its target instead", path)
		}
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot copy file %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot copy file %q: %w", path, err)
		}
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot copy file %q: %w", path, err)
		}
		return domain.ToolResult{
			Content: fmt.Sprintf("Copied %q -> %q (%d bytes)", path, destPath, len(data)),
			Meta:    map[string]any{"path": path, "destination": destPath, "op": "copy", "bytes_copied": len(data)},
		}, nil

	case "delete":
		if info.Mode()&os.ModeSymlink != 0 {
			return domain.ToolResult{}, fmt.Errorf("refusing to delete symlink %q — delete its target instead", path)
		}
		if info.IsDir() {
			if !boolFromInput(input, "recursive") {
				return domain.ToolResult{}, fmt.Errorf("cannot delete directory %q: set recursive=true to delete it with its contents", path)
			}
			if err := os.RemoveAll(resolvedPath); err != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot delete directory %q: %w", path, err)
			}
			return domain.ToolResult{
				Content: fmt.Sprintf("Deleted directory %q (recursive)", path),
				Meta:    map[string]any{"path": path, "op": "delete", "recursive": true},
			}, nil
		}
		if err := os.Remove(resolvedPath); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot delete file %q: %w", path, err)
		}
		return domain.ToolResult{
			Content: fmt.Sprintf("Deleted file %q", path),
			Meta:    map[string]any{"path": path, "op": "delete"},
		}, nil

	default:
		return domain.ToolResult{}, fmt.Errorf("unknown action %q (move | copy | delete)", action)
	}
}

// resolveOpDest validates the destination argument for move/copy: it must be
// inside the project root and must not already exist.
func resolveOpDest(workDir string, input map[string]any) (resolved, display string, err error) {
	destPath, _ := input["destination"].(string)
	if destPath == "" {
		return "", "", fmt.Errorf("destination is required for move/copy")
	}
	dest, err := resolveWritePath(workDir, destPath)
	if err != nil {
		return "", "", err
	}
	if dest == filepath.Clean(workDir) {
		return "", "", fmt.Errorf("refusing to operate on the project root")
	}
	if _, err := os.Lstat(dest); err == nil {
		return "", "", fmt.Errorf("destination %q already exists — delete or move it first", destPath)
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	return dest, destPath, nil
}

// copyDirTree recursively copies src into dst (dst must not exist). Symlinks
// inside the tree are refused instead of silently skipped.
func copyDirTree(src, dst string) (int, error) {
	count := 0
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("cannot copy directory: %w", err)
			}
		case d.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("cannot copy symlink %q inside directory %q", rel, src)
		default:
			if err := copyFileWithMode(p, target, d); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("cannot copy directory %q: %w", src, err)
	}
	return count, nil
}

func copyFileWithMode(src, dst string, entry os.DirEntry) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := entry.Info(); statErr == nil {
		mode = info.Mode().Perm()
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
