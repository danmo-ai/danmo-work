package builtin

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"os"

	// Register PNG/JPEG/GIF decoders with image.DecodeConfig (format sniffing).
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"danmo-work/core/domain"
)

const maxImageBytes = 20 * 1024 * 1024 // 20 MB cap to protect the context window

// ReadImage reads an image file and returns it as a multimodal part.
// Supports PNG, JPEG, GIF, and WebP.
type ReadImage struct {
	// SupportsImage reports whether the routed model accepts image input.
	// Nil (or true) is permissive; providers reject unsupported images.
	SupportsImage func(modelID string) bool
}

func (h *ReadImage) Name() string                { return "read_image" }
func (h *ReadImage) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *ReadImage) Describe(args map[string]any) string {
	path, _ := args["path"].(string)
	if path == "" {
		return "read_image"
	}
	return "read_image " + path
}
func (h *ReadImage) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "read_image",
		Description: "Reads an image file (PNG, JPEG, GIF, WebP) from the local filesystem and returns it " +
			"to the model for visual analysis.\n\n" +
			"**Important**: All paths are relative to the project root directory. Use relative paths like " +
			"'docs/screenshot.png' instead of absolute paths.\n\n" +
			"- Only use this when you actually need to SEE the image (visual inspection, screenshots, UI mockups, charts).\n" +
			"- Not for reading text files — use read_file for text.\n" +
			"- Files larger than 20 MB are refused; resize or crop externally if needed.\n" +
			"- The current model must accept image input.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative image file path from project root (e.g., 'docs/screenshot.png')"},
			},
			"required": []string{"path"},
		},
	}
}

func (h *ReadImage) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	if h.SupportsImage != nil {
		modelID, _ := input["__model_id"].(string)
		if modelID != "" && !h.SupportsImage(modelID) {
			return domain.ToolResult{}, fmt.Errorf("the current model (%s) does not accept image input — read_image is unavailable", modelID)
		}
	}

	workDir := workDirFromInput(input)
	resolvedPath, info, err := readFilePath(workDir, path)
	if err != nil {
		return domain.ToolResult{}, err
	}
	if info.IsDir() {
		return domain.ToolResult{}, fmt.Errorf("%q is a directory, not an image file", path)
	}
	if info.Size() > maxImageBytes {
		return domain.ToolResult{}, fmt.Errorf("image %q is %d bytes — larger than the %d MB read_image cap. Resize it externally and retry", path, info.Size(), maxImageBytes/(1024*1024))
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot read image %q: %w", path, err)
	}

	mime := detectImageMime(data)
	if mime == "" {
		return domain.ToolResult{}, fmt.Errorf("%q is not a supported image (PNG, JPEG, GIF, WebP)", path)
	}

	width, height := 0, 0
	if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data)); cfgErr == nil {
		width, height = cfg.Width, cfg.Height
	}

	desc := fmt.Sprintf("Read image %q (%d bytes, %s)", path, len(data), mime)
	if width > 0 {
		desc = fmt.Sprintf("Read image %q (%dx%d, %d bytes, %s)", path, width, height, len(data), mime)
	}
	return domain.ToolResult{
		Content: desc,
		Meta: map[string]any{
			"path":      path,
			"mime_type": mime,
			"width":     width,
			"height":    height,
			"bytes":     len(data),
			"op":        "read_image",
		},
		Parts: []domain.ToolResultPart{{
			Type:     "image",
			MimeType: mime,
			Data:     base64.StdEncoding.EncodeToString(data),
		}},
	}, nil
}

// detectImageMime sniffs the image format from magic bytes.
func detectImageMime(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}
