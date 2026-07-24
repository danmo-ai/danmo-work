package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
)

// mcpClient is a minimal MCP (Model Context Protocol) client that connects
// to an MCP server and discovers its tools. It supports stdio and HTTP
// (SSE / streamable-http) transports.
type mcpClient struct {
	server domain.MCPServer
}

// discoverTools connects to the MCP server and returns the list of tools.
func discoverTools(ctx context.Context, srv domain.MCPServer) ([]domain.MCPToolDef, error) {
	c := &mcpClient{server: srv}
	switch srv.Transport {
	case "stdio":
		return c.discoverStdio(ctx)
	case "sse", "streamable-http":
		return c.discoverHTTP(ctx)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", srv.Transport)
	}
}

// ---- stdio transport ----

func (c *mcpClient) discoverStdio(ctx context.Context) ([]domain.MCPToolDef, error) {
	if c.server.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}

	args := splitArgs(c.server.Args)
	cmd := exec.CommandContext(ctx, c.server.Command, args...)
	cmd.Env = appendEnv(c.server.Env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	var mu sync.Mutex
	respCh := make(chan json.RawMessage, 1)

	// Background reader for JSON-RPC responses
	go func() {
		for {
			line, err := readJSONLine(reader)
			if err != nil {
				return
			}
			// Check if it's a response (has "id" field) or a notification
			var msg struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(line, &msg); err != nil || msg.ID == nil {
				continue // skip notifications
			}
			mu.Lock()
			respCh <- msg.Result
			mu.Unlock()
		}
	}()

	// Send initialize
	if err := sendRequest(stdin, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "danmo-work", "version": "1.0.0"},
	}); err != nil {
		return nil, err
	}

	// Wait for initialize response
	select {
	case <-respCh:
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("initialize timeout")
	}

	// Send initialized notification
	if err := sendNotification(stdin, "notifications/initialized", map[string]any{}); err != nil {
		return nil, err
	}

	// Send tools/list
	if err := sendRequest(stdin, 2, "tools/list", map[string]any{}); err != nil {
		return nil, err
	}

	// Wait for tools/list response
	select {
	case result := <-respCh:
		return parseToolsResult(result)
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("tools/list timeout")
	}
}

// ---- HTTP transport (streamable-http) ----

func (c *mcpClient) discoverHTTP(ctx context.Context) ([]domain.MCPToolDef, error) {
	if c.server.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP transport")
	}

	url := strings.TrimRight(c.server.URL, "/")
	// For SSE transport, the MCP endpoint might be at the base URL
	if c.server.Transport == "sse" {
		// SSE uses the same URL for JSON-RPC POST
		url = strings.TrimSuffix(url, "/sse")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Initialize
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "danmo-work", "version": "1.0.0"},
		},
	}
	if err := c.httpPost(ctx, client, url, initReq); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	// Initialized notification
	notifReq := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	}
	_ = c.httpPost(ctx, client, url, notifReq)

	// Tools/list
	toolsReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	result, err := c.httpPostResult(ctx, client, url, toolsReq)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	return parseToolsResult(result)
}

func (c *mcpClient) httpPost(ctx context.Context, client *http.Client, url string, payload any) error {
	_, err := c.httpPostResult(ctx, client, url, payload)
	return err
}

func (c *mcpClient) httpPostResult(ctx context.Context, client *http.Client, url string, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.server.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Try to parse as JSON-RPC response
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		// Might be SSE format, try to extract JSON from data: lines
		return extractSSEData(respBody)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func extractSSEData(data []byte) (json.RawMessage, error) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var rpcResp struct {
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &rpcResp); err == nil && rpcResp.Result != nil {
				return rpcResp.Result, nil
			}
		}
	}
	return nil, fmt.Errorf("no JSON-RPC result found in response")
}

// ---- helpers ----

func parseToolsResult(result json.RawMessage) ([]domain.MCPToolDef, error) {
	var resp struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}
	tools := make([]domain.MCPToolDef, len(resp.Tools))
	for i, t := range resp.Tools {
		tools[i] = domain.MCPToolDef{
			Name:        t.Name,
			Description: t.Description,
			Enabled:     true,
		}
	}
	return tools, nil
}

func readJSONLine(reader *bufio.Reader) (json.RawMessage, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		return json.RawMessage(line), nil
	}
}

func sendRequest(w io.Writer, id int, method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	return writeJSON(w, msg)
}

func sendNotification(w io.Writer, method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return writeJSON(w, msg)
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func splitArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

func appendEnv(envStr string) []string {
	var env []string
	for _, line := range strings.Split(envStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			env = append(env, line)
		}
	}
	return env
}
