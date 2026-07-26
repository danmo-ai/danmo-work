package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// Dialer opens MCP sessions for stdio and HTTP transports.
type Dialer struct {
	// ExtraHeaders merges into every HTTP request (e.g. resolved secrets).
	HeaderResolver func(srv domain.MCPServer) map[string]string
}

func NewDialer() *Dialer {
	return &Dialer{}
}

func (d *Dialer) Dial(ctx context.Context, srv domain.MCPServer) (port.MCPSession, error) {
	switch srv.Transport {
	case "stdio", "":
		if srv.Command == "" && srv.URL != "" {
			return d.dialHTTP(ctx, srv)
		}
		return d.dialStdio(ctx, srv)
	case "sse", "streamable-http":
		return d.dialHTTP(ctx, srv)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", srv.Transport)
	}
}

// ---- stdio session ----

type stdioSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64
	pending map[int64]chan rpcResponse
	closed  atomic.Bool
}

type rpcResponse struct {
	Result json.RawMessage
	Err    error
}

func (d *Dialer) dialStdio(ctx context.Context, srv domain.MCPServer) (port.MCPSession, error) {
	if srv.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}
	args := splitArgs(srv.Args)
	cmd := exec.CommandContext(ctx, srv.Command, args...)
	cmd.Env = append(os.Environ(), parseEnv(srv.Env)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}
	s := &stdioSession{
		cmd:     cmd,
		stdin:   stdin,
		reader:  bufio.NewReader(stdout),
		pending: make(map[int64]chan rpcResponse),
	}
	s.nextID.Store(1)
	go s.readLoop()
	if err := s.initialize(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *stdioSession) readLoop() {
	for {
		line, err := readJSONLine(s.reader)
		if err != nil {
			s.failAll(err)
			return
		}
		var envelope struct {
			ID     *json.RawMessage `json:"id"`
			Result json.RawMessage  `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		if envelope.ID == nil || envelope.Method != "" {
			continue // notification or request from server
		}
		var id int64
		if err := json.Unmarshal(*envelope.ID, &id); err != nil {
			// string ids — ignore for our client (we use int)
			continue
		}
		s.mu.Lock()
		ch := s.pending[id]
		delete(s.pending, id)
		s.mu.Unlock()
		if ch == nil {
			continue
		}
		if envelope.Error != nil {
			ch <- rpcResponse{Err: fmt.Errorf("RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)}
		} else {
			ch <- rpcResponse{Result: envelope.Result}
		}
	}
}

func (s *stdioSession) failAll(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ch := range s.pending {
		ch <- rpcResponse{Err: err}
		delete(s.pending, id)
	}
}

func (s *stdioSession) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if s.closed.Load() {
		return nil, fmt.Errorf("session closed")
	}
	id := s.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	s.mu.Lock()
	_, err = s.stdin.Write(data)
	s.mu.Unlock()
	if err != nil {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		return resp.Result, resp.Err
	}
}

func (s *stdioSession) notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.stdin.Write(data)
	return err
}

func (s *stdioSession) initialize(ctx context.Context) error {
	_, err := s.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "danmo-work", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	_ = s.notify("notifications/initialized", map[string]any{})
	return nil
}

func (s *stdioSession) ListTools(ctx context.Context) ([]port.MCPToolInfo, error) {
	result, err := s.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return parseToolsResult(result)
}

func (s *stdioSession) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	result, err := s.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	return formatCallResult(result)
}

func (s *stdioSession) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	_ = s.stdin.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	return nil
}

// ---- HTTP session ----

type httpSession struct {
	url     string
	client  *http.Client
	headers map[string]string
	nextID  atomic.Int64
	mu      sync.Mutex
	session string // MCP-Session-Id when provided
}

func (d *Dialer) dialHTTP(ctx context.Context, srv domain.MCPServer) (port.MCPSession, error) {
	if srv.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP transport")
	}
	url := strings.TrimRight(srv.URL, "/")
	if srv.Transport == "sse" {
		url = strings.TrimSuffix(url, "/sse")
	}
	headers := map[string]string{}
	for k, v := range srv.Headers {
		headers[k] = v
	}
	if d.HeaderResolver != nil {
		for k, v := range d.HeaderResolver(srv) {
			headers[k] = v
		}
	}
	s := &httpSession{
		url:    url,
		client: &http.Client{Timeout: 60 * time.Second},
		headers: headers,
	}
	s.nextID.Store(1)
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *httpSession) initialize(ctx context.Context) error {
	_, err := s.post(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "danmo-work", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	_ = s.postNotify(ctx, "notifications/initialized", map[string]any{})
	return nil
}

func (s *httpSession) ListTools(ctx context.Context) ([]port.MCPToolInfo, error) {
	result, err := s.post(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return parseToolsResult(result)
}

func (s *httpSession) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	result, err := s.post(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	return formatCallResult(result)
}

func (s *httpSession) Close() error { return nil }

func (s *httpSession) post(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := s.nextID.Add(1)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	return s.doPOST(ctx, payload)
}

func (s *httpSession) postNotify(ctx context.Context, method string, params any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	_, err := s.doPOST(ctx, payload)
	return err
}

func (s *httpSession) doPOST(ctx context.Context, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	s.mu.Lock()
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	if s.session != "" {
		req.Header.Set("Mcp-Session-Id", s.session)
	}
	s.mu.Unlock()

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		s.mu.Lock()
		s.session = sid
		s.mu.Unlock()
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	// Notification responses may be empty
	if len(bytes.TrimSpace(respBody)) == 0 {
		return nil, nil
	}
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return extractSSEData(respBody)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// ---- shared helpers ----

func parseToolsResult(result json.RawMessage) ([]port.MCPToolInfo, error) {
	var resp struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}
	out := make([]port.MCPToolInfo, len(resp.Tools))
	for i, t := range resp.Tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out[i] = port.MCPToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}
	}
	return out, nil
}

func formatCallResult(result json.RawMessage) (string, error) {
	if len(result) == 0 {
		return "", nil
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool            `json:"isError"`
		Meta    json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		// Return raw JSON if shape unexpected
		return string(result), nil
	}
	var b strings.Builder
	for i, c := range resp.Content {
		if i > 0 {
			b.WriteByte('\n')
		}
		if c.Text != "" {
			b.WriteString(c.Text)
		} else {
			raw, _ := json.Marshal(c)
			b.Write(raw)
		}
	}
	text := b.String()
	if resp.IsError {
		return "", fmt.Errorf("mcp tool error: %s", text)
	}
	if text == "" {
		return string(result), nil
	}
	return text, nil
}

func extractSSEData(data []byte) (json.RawMessage, error) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var rpcResp struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &rpcResp); err == nil {
				if rpcResp.Error != nil {
					return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
				}
				if rpcResp.Result != nil {
					return rpcResp.Result, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no JSON-RPC result found in response")
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

func splitArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

func parseEnv(envStr string) []string {
	var env []string
	for _, line := range strings.Split(envStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			env = append(env, line)
		}
	}
	return env
}
