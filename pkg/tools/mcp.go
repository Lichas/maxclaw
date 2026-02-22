package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

const (
	mcpJSONRPCVersion  = "2.0"
	mcpProtocolVersion = "2024-11-05"
	mcpClientName      = "maxclaw"
	mcpClientVersion   = "0.1.0"
	maxToolNameLength  = 64
)

// MCPServerOptions 是 MCP 服务器配置（兼容 Claude Desktop / Cursor 的 mcpServers 条目）。
type MCPServerOptions struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
	URL     string
}

type mcpRemoteTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

type mcpCallResult struct {
	Content           []interface{}
	StructuredContent interface{}
	IsError           bool
}

type mcpClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]mcpRemoteTool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (mcpCallResult, error)
	Close() error
}

type mcpClientFactory interface {
	New(opts MCPServerOptions) (mcpClient, error)
}

type defaultMCPClientFactory struct{}

func (defaultMCPClientFactory) New(opts MCPServerOptions) (mcpClient, error) {
	if strings.TrimSpace(opts.Command) != "" {
		transport, err := newStdioMCPTransport(opts)
		if err != nil {
			return nil, err
		}
		return &jsonRPCMCPClient{transport: transport}, nil
	}
	if strings.TrimSpace(opts.URL) != "" {
		return &jsonRPCMCPClient{
			transport: newHTTPMCPTransport(opts),
		}, nil
	}
	return nil, fmt.Errorf("mcp server %q has neither command nor url", opts.Name)
}

// MCPConnector 负责连接 MCP 服务器并将其工具注册为本地 Tool。
type MCPConnector struct {
	mu sync.Mutex

	servers        map[string]MCPServerOptions
	factory        mcpClientFactory
	clients        map[string]mcpClient
	registered     []string
	connected      bool
	lastConnectErr error
}

// NewMCPConnector 创建 MCP 连接器。
func NewMCPConnector(servers map[string]MCPServerOptions) *MCPConnector {
	return newMCPConnectorWithFactory(servers, defaultMCPClientFactory{})
}

func newMCPConnectorWithFactory(servers map[string]MCPServerOptions, factory mcpClientFactory) *MCPConnector {
	copied := make(map[string]MCPServerOptions, len(servers))
	for name, server := range servers {
		cfg := server
		cfg.Name = name
		if cfg.Args == nil {
			cfg.Args = []string{}
		}
		if cfg.Env == nil {
			cfg.Env = map[string]string{}
		}
		copied[name] = cfg
	}
	return &MCPConnector{
		servers: copied,
		factory: factory,
		clients: map[string]mcpClient{},
	}
}

// Connect 连接所有 MCP 服务器，并注册它们暴露的工具。
// 即使部分服务器失败，其他服务器仍会继续连接。
func (c *MCPConnector) Connect(ctx context.Context, registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is required")
	}

	c.mu.Lock()
	if c.connected {
		err := c.lastConnectErr
		c.mu.Unlock()
		return err
	}
	c.connected = true
	c.mu.Unlock()

	names := make([]string, 0, len(c.servers))
	for name := range c.servers {
		names = append(names, name)
	}
	sort.Strings(names)

	nextClients := make(map[string]mcpClient)
	registered := make([]string, 0)
	errs := make([]string, 0)

	for _, name := range names {
		server := c.servers[name]
		client, err := c.factory.New(server)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		if err := client.Initialize(ctx); err != nil {
			_ = client.Close()
			errs = append(errs, fmt.Sprintf("%s: initialize failed: %v", name, err))
			continue
		}

		remoteTools, err := client.ListTools(ctx)
		if err != nil {
			_ = client.Close()
			errs = append(errs, fmt.Sprintf("%s: list tools failed: %v", name, err))
			continue
		}

		for _, remoteTool := range remoteTools {
			wrapper := newMCPToolWrapper(name, remoteTool, client)
			if err := registry.Register(wrapper); err != nil {
				errs = append(errs, fmt.Sprintf("%s: register %s failed: %v", name, wrapper.Name(), err))
				continue
			}
			registered = append(registered, wrapper.Name())
		}

		nextClients[name] = client
	}

	c.mu.Lock()
	c.clients = nextClients
	c.registered = registered
	if len(errs) > 0 {
		c.lastConnectErr = fmt.Errorf(strings.Join(errs, "; "))
	} else {
		c.lastConnectErr = nil
	}
	c.mu.Unlock()

	return c.lastConnectErr
}

// Close 关闭所有已连接的 MCP 客户端。
func (c *MCPConnector) Close() error {
	c.mu.Lock()
	clients := c.clients
	c.clients = map[string]mcpClient{}
	c.registered = nil
	c.connected = false
	c.lastConnectErr = nil
	c.mu.Unlock()

	if len(clients) == 0 {
		return nil
	}

	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	sort.Strings(names)

	errs := make([]error, 0)
	for _, name := range names {
		if err := clients[name].Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// RegisteredTools 返回已经注册到本地工具注册表中的 MCP 工具名列表。
func (c *MCPConnector) RegisteredTools() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.registered))
	copy(out, c.registered)
	sort.Strings(out)
	return out
}

type mcpToolWrapper struct {
	BaseTool
	serverName   string
	originalName string
	client       mcpClient
}

func newMCPToolWrapper(serverName string, remoteTool mcpRemoteTool, client mcpClient) *mcpToolWrapper {
	description := strings.TrimSpace(remoteTool.Description)
	if description == "" {
		description = remoteTool.Name
	}

	return &mcpToolWrapper{
		BaseTool: BaseTool{
			name:        buildMCPToolName(serverName, remoteTool.Name),
			description: description,
			parameters:  normalizeToolSchema(remoteTool.InputSchema),
		},
		serverName:   serverName,
		originalName: remoteTool.Name,
		client:       client,
	}
}

func (t *mcpToolWrapper) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	result, err := t.client.CallTool(ctx, t.originalName, params)
	if err != nil {
		return "", fmt.Errorf("mcp tool %s/%s call failed: %w", t.serverName, t.originalName, err)
	}
	return renderMCPToolResult(result), nil
}

func buildMCPToolName(serverName, toolName string) string {
	base := "mcp_" + sanitizeMCPToolSegment(serverName) + "_" + sanitizeMCPToolSegment(toolName)
	if len(base) <= maxToolNameLength {
		return base
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(base))
	suffix := fmt.Sprintf("_%x", h.Sum32())
	maxPrefix := maxToolNameLength - len(suffix)
	if maxPrefix < 1 {
		return base[:maxToolNameLength]
	}
	prefix := strings.TrimRight(base[:maxPrefix], "_")
	if prefix == "" {
		prefix = "mcp"
	}
	return prefix + suffix
}

func sanitizeMCPToolSegment(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "tool"
	}

	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteRune('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "tool"
	}
	return out
}

func normalizeToolSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	normalized := make(map[string]interface{}, len(schema))
	for k, v := range schema {
		normalized[k] = v
	}

	if _, ok := normalized["type"]; !ok {
		normalized["type"] = "object"
	}
	if normalized["type"] == "object" {
		if _, ok := normalized["properties"]; !ok {
			normalized["properties"] = map[string]interface{}{}
		}
	}
	return normalized
}

func renderMCPToolResult(result mcpCallResult) string {
	parts := make([]string, 0, len(result.Content)+1)
	for _, item := range result.Content {
		text := renderMCPContent(item)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if result.StructuredContent != nil {
		if raw, err := json.MarshalIndent(result.StructuredContent, "", "  "); err == nil {
			if strings.TrimSpace(string(raw)) != "" && string(raw) != "null" {
				parts = append(parts, string(raw))
			}
		}
	}

	out := strings.TrimSpace(strings.Join(parts, "\n"))
	if out == "" {
		return "(no output)"
	}
	return out
}

func renderMCPContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case map[string]interface{}:
		if t, ok := v["type"].(string); ok && t == "text" {
			if text, ok := v["text"].(string); ok {
				return text
			}
		}
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
		return fmt.Sprintf("%v", v)
	default:
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
		return fmt.Sprintf("%v", v)
	}
}

type jsonRPCMCPClient struct {
	transport mcpTransport
}

func (c *jsonRPCMCPClient) Initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": mcpProtocolVersion,
		"clientInfo": map[string]interface{}{
			"name":    mcpClientName,
			"version": mcpClientVersion,
		},
		"capabilities": map[string]interface{}{},
	}

	if _, err := c.transport.Request(ctx, "initialize", params); err != nil {
		return err
	}
	return c.transport.Notify(ctx, "notifications/initialized", map[string]interface{}{})
}

func (c *jsonRPCMCPClient) ListTools(ctx context.Context) ([]mcpRemoteTool, error) {
	cursor := ""
	collected := make([]mcpRemoteTool, 0)

	for {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		result, err := c.transport.Request(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}

		var parsed struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &parsed); err != nil {
			return nil, fmt.Errorf("parse tools/list response failed: %w", err)
		}

		for _, tool := range parsed.Tools {
			schema := map[string]interface{}{}
			if len(tool.InputSchema) > 0 {
				if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
					// schema 解析失败时退化到空对象参数
					schema = map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					}
				}
			}
			collected = append(collected, mcpRemoteTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			})
		}

		if parsed.NextCursor == "" {
			break
		}
		cursor = parsed.NextCursor
	}

	return collected, nil
}

func (c *jsonRPCMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (mcpCallResult, error) {
	if args == nil {
		args = map[string]interface{}{}
	}

	result, err := c.transport.Request(ctx, "tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return mcpCallResult{}, err
	}

	var parsed mcpCallResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return mcpCallResult{}, fmt.Errorf("parse tools/call response failed: %w", err)
	}
	return parsed, nil
}

func (c *jsonRPCMCPClient) Close() error {
	return c.transport.Close()
}

type mcpTransport interface {
	Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params interface{}) error
	Close() error
}

type mcpJSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *mcpJSONRPCError `json:"error,omitempty"`
}

type mcpJSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *mcpJSONRPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return fmt.Sprintf("json-rpc error code=%d", e.Code)
	}
	return fmt.Sprintf("json-rpc error code=%d message=%s", e.Code, e.Message)
}

type mcpResponseEnvelope struct {
	result json.RawMessage
	err    error
}

type stdioMCPTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	nextID int64

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan mcpResponseEnvelope

	closed    chan struct{}
	closeOnce sync.Once
}

func newStdioMCPTransport(opts MCPServerOptions) (*stdioMCPTransport, error) {
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		return nil, fmt.Errorf("mcp stdio server %q command is empty", opts.Name)
	}

	cmd := exec.Command(command, opts.Args...)
	cmd.Env = append(os.Environ(), envMapToPairs(opts.Env)...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio %q stdin pipe failed: %w", opts.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio %q stdout pipe failed: %w", opts.Name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio %q stderr pipe failed: %w", opts.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp stdio %q start failed: %w", opts.Name, err)
	}

	t := &stdioMCPTransport{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		stderr:  stderr,
		pending: map[string]chan mcpResponseEnvelope{},
		closed:  make(chan struct{}),
	}

	go io.Copy(io.Discard, stderr)
	go t.readLoop()

	return t, nil
}

func (t *stdioMCPTransport) Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	idKey := strconv.FormatInt(id, 10)

	req := mcpJSONRPCRequest{
		JSONRPC: mcpJSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal json-rpc request failed: %w", err)
	}

	ch := make(chan mcpResponseEnvelope, 1)
	t.pendingMu.Lock()
	t.pending[idKey] = ch
	t.pendingMu.Unlock()

	if err := t.writeFrame(raw); err != nil {
		t.removePending(idKey)
		return nil, err
	}

	select {
	case <-ctx.Done():
		t.removePending(idKey)
		return nil, ctx.Err()
	case <-t.closed:
		t.removePending(idKey)
		return nil, fmt.Errorf("mcp transport closed")
	case resp := <-ch:
		if resp.err != nil {
			return nil, resp.err
		}
		return resp.result, nil
	}
}

func (t *stdioMCPTransport) Notify(ctx context.Context, method string, params interface{}) error {
	req := mcpJSONRPCRequest{
		JSONRPC: mcpJSONRPCVersion,
		Method:  method,
		Params:  params,
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal json-rpc notification failed: %w", err)
	}
	return t.writeFrame(raw)
}

func (t *stdioMCPTransport) Close() error {
	var closeErr error
	t.closeOnce.Do(func() {
		close(t.closed)
		t.failAllPending(fmt.Errorf("mcp transport closed"))

		if t.stdin != nil {
			if err := t.stdin.Close(); err != nil {
				closeErr = errors.Join(closeErr, err)
			}
		}
		if t.stderr != nil {
			if err := t.stderr.Close(); err != nil {
				closeErr = errors.Join(closeErr, err)
			}
		}
		if t.cmd != nil && t.cmd.Process != nil {
			done := make(chan error, 1)
			go func() {
				done <- t.cmd.Wait()
			}()
			select {
			case err := <-done:
				if err != nil {
					closeErr = errors.Join(closeErr, err)
				}
			case <-time.After(3 * time.Second):
				_ = t.cmd.Process.Kill()
				if err := <-done; err != nil {
					closeErr = errors.Join(closeErr, err)
				}
			}
		}
	})
	return closeErr
}

func (t *stdioMCPTransport) readLoop() {
	for {
		select {
		case <-t.closed:
			return
		default:
		}

		msg, err := t.readFrame()
		if err != nil {
			t.failAllPending(fmt.Errorf("mcp read failed: %w", err))
			_ = t.Close()
			return
		}
		if err := t.dispatchMessage(msg); err != nil {
			continue
		}
	}
}

func (t *stdioMCPTransport) readFrame() ([]byte, error) {
	for {
		firstLine, err := t.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(firstLine)
		if trimmed == "" {
			continue
		}

		// 兼容 newline-delimited JSON 消息（部分 SDK 使用）
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return []byte(trimmed), nil
		}

		contentLength := -1
		if v, ok := parseContentLengthHeader(firstLine); ok {
			contentLength = v
		}

		// 读取剩余 header，直到空行
		for {
			line, err := t.stdout.ReadString('\n')
			if err != nil {
				return nil, err
			}
			lineTrim := strings.TrimRight(line, "\r\n")
			if lineTrim == "" {
				break
			}
			if v, ok := parseContentLengthHeader(lineTrim); ok {
				contentLength = v
			}
		}

		if contentLength <= 0 {
			return nil, fmt.Errorf("missing Content-Length header")
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(t.stdout, body); err != nil {
			return nil, err
		}
		return body, nil
	}
}

func parseContentLengthHeader(line string) (int, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	if strings.ToLower(strings.TrimSpace(parts[0])) != "content-length" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func (t *stdioMCPTransport) dispatchMessage(raw []byte) error {
	var batch []json.RawMessage
	if err := json.Unmarshal(raw, &batch); err == nil {
		for _, item := range batch {
			_ = t.dispatchOne(item)
		}
		return nil
	}
	return t.dispatchOne(raw)
}

func (t *stdioMCPTransport) dispatchOne(raw []byte) error {
	var resp mcpJSONRPCResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return err
	}
	if len(resp.ID) == 0 {
		return nil
	}

	key := normalizeRPCID(resp.ID)
	t.pendingMu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.pendingMu.Unlock()
	if !ok {
		return nil
	}

	if resp.Error != nil {
		ch <- mcpResponseEnvelope{err: resp.Error}
		return nil
	}
	ch <- mcpResponseEnvelope{result: resp.Result}
	return nil
}

func (t *stdioMCPTransport) writeFrame(payload []byte) error {
	select {
	case <-t.closed:
		return fmt.Errorf("mcp transport closed")
	default:
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if _, err := io.WriteString(t.stdin, header); err != nil {
		return fmt.Errorf("mcp write header failed: %w", err)
	}
	if _, err := t.stdin.Write(payload); err != nil {
		return fmt.Errorf("mcp write payload failed: %w", err)
	}
	return nil
}

func (t *stdioMCPTransport) removePending(id string) {
	t.pendingMu.Lock()
	delete(t.pending, id)
	t.pendingMu.Unlock()
}

func (t *stdioMCPTransport) failAllPending(err error) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	for id, ch := range t.pending {
		ch <- mcpResponseEnvelope{err: err}
		delete(t.pending, id)
	}
}

type httpMCPTransport struct {
	endpoint string
	client   *http.Client

	nextID int64

	mu        sync.Mutex
	sessionID string
}

func newHTTPMCPTransport(opts MCPServerOptions) *httpMCPTransport {
	return &httpMCPTransport{
		endpoint: strings.TrimSpace(opts.URL),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (t *httpMCPTransport) Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	resp, err := t.send(ctx, mcpJSONRPCRequest{
		JSONRPC: mcpJSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (t *httpMCPTransport) Notify(ctx context.Context, method string, params interface{}) error {
	_, err := t.send(ctx, mcpJSONRPCRequest{
		JSONRPC: mcpJSONRPCVersion,
		Method:  method,
		Params:  params,
	})
	return err
}

func (t *httpMCPTransport) Close() error { return nil }

func (t *httpMCPTransport) send(ctx context.Context, request mcpJSONRPCRequest) (mcpJSONRPCResponse, error) {
	if strings.TrimSpace(t.endpoint) == "" {
		return mcpJSONRPCResponse{}, fmt.Errorf("mcp http endpoint is empty")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return mcpJSONRPCResponse{}, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(payload))
	if err != nil {
		return mcpJSONRPCResponse{}, fmt.Errorf("build request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)

	if sid := t.getSessionID(); sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return mcpJSONRPCResponse{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if sid := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sid != "" {
		t.setSessionID(sid)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcpJSONRPCResponse{}, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return mcpJSONRPCResponse{}, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(string(body)) == "" {
		return mcpJSONRPCResponse{}, nil
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	requestID := normalizeRequestID(request.ID)
	if strings.Contains(contentType, "text/event-stream") {
		return parseSSEResponseByID(body, requestID)
	}
	return parseJSONRPCResponseByID(body, requestID)
}

func (t *httpMCPTransport) getSessionID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionID
}

func (t *httpMCPTransport) setSessionID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = id
}

func parseSSEResponseByID(body []byte, requestID string) (mcpJSONRPCResponse, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	dataLines := make([]string, 0)
	processEvent := func() (mcpJSONRPCResponse, bool, error) {
		if len(dataLines) == 0 {
			return mcpJSONRPCResponse{}, false, nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		resp, err := parseJSONRPCResponseByID([]byte(payload), requestID)
		if err != nil {
			return mcpJSONRPCResponse{}, false, nil
		}
		return resp, true, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if resp, ok, err := processEvent(); ok || err != nil {
				return resp, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return mcpJSONRPCResponse{}, fmt.Errorf("parse sse response failed: %w", err)
	}
	if resp, ok, err := processEvent(); ok || err != nil {
		return resp, err
	}
	return mcpJSONRPCResponse{}, fmt.Errorf("no matching response found in sse stream")
}

func parseJSONRPCResponseByID(body []byte, requestID string) (mcpJSONRPCResponse, error) {
	var single mcpJSONRPCResponse
	if err := json.Unmarshal(body, &single); err == nil && single.JSONRPC != "" {
		if requestID == "" || normalizeRPCID(single.ID) == requestID || len(single.ID) == 0 {
			return single, nil
		}
	}

	var batch []mcpJSONRPCResponse
	if err := json.Unmarshal(body, &batch); err == nil {
		for _, item := range batch {
			if requestID == "" || normalizeRPCID(item.ID) == requestID {
				return item, nil
			}
		}
	}

	return mcpJSONRPCResponse{}, fmt.Errorf("no matching json-rpc response id=%s", requestID)
}

func normalizeRequestID(id interface{}) string {
	if id == nil {
		return ""
	}
	switch v := id.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeRPCID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return normalizeRequestID(v)
}

func envMapToPairs(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return out
}
