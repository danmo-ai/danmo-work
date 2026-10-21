package domain

import (
	"context"
	"strings"
)

type mcpWorkDirContextKey struct{}

// WithMCPWorkDir attaches the session project directory so stdio MCP args can
// expand ${WORKDIR} (ArtCraft automation roots and similar).
func WithMCPWorkDir(ctx context.Context, dir string) context.Context {
	if ctx == nil {
		return nil
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ctx
	}
	return context.WithValue(ctx, mcpWorkDirContextKey{}, dir)
}

// MCPWorkDir returns the project directory stored by WithMCPWorkDir.
func MCPWorkDir(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(mcpWorkDirContextKey{}).(string)
	return s
}
