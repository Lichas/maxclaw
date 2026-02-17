package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMCPClient struct {
	initErr  error
	listErr  error
	callErr  error
	closeErr error

	tools       []mcpRemoteTool
	callResults map[string]mcpCallResult

	initializeCalled bool
	closeCalled      bool
	lastCallName     string
	lastCallArgs     map[string]interface{}
}

func (c *fakeMCPClient) Initialize(ctx context.Context) error {
	c.initializeCalled = true
	return c.initErr
}

func (c *fakeMCPClient) ListTools(ctx context.Context) ([]mcpRemoteTool, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.tools, nil
}

func (c *fakeMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (mcpCallResult, error) {
	c.lastCallName = name
	c.lastCallArgs = args
	if c.callErr != nil {
		return mcpCallResult{}, c.callErr
	}
	if res, ok := c.callResults[name]; ok {
		return res, nil
	}
	return mcpCallResult{}, nil
}

func (c *fakeMCPClient) Close() error {
	c.closeCalled = true
	return c.closeErr
}

type fakeMCPFactory struct {
	clients map[string]*fakeMCPClient
	errs    map[string]error
}

func (f fakeMCPFactory) New(opts MCPServerOptions) (mcpClient, error) {
	if err, ok := f.errs[opts.Name]; ok {
		return nil, err
	}
	client, ok := f.clients[opts.Name]
	if !ok {
		return nil, assert.AnError
	}
	return client, nil
}

func TestMCPConnectorRegistersRemoteTools(t *testing.T) {
	reg := NewRegistry()
	client := &fakeMCPClient{
		tools: []mcpRemoteTool{
			{
				Name:        "search-web",
				Description: "Search from MCP",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"query"},
				},
			},
		},
		callResults: map[string]mcpCallResult{
			"search-web": {
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "result line 1"},
					map[string]interface{}{"type": "text", "text": "result line 2"},
				},
			},
		},
	}

	connector := newMCPConnectorWithFactory(
		map[string]MCPServerOptions{
			"docs": {Command: "npx"},
		},
		fakeMCPFactory{
			clients: map[string]*fakeMCPClient{"docs": client},
			errs:    map[string]error{},
		},
	)

	err := connector.Connect(context.Background(), reg)
	require.NoError(t, err)
	assert.True(t, client.initializeCalled)
	assert.Equal(t, []string{"mcp_docs_search_web"}, connector.RegisteredTools())

	result, err := reg.Execute(context.Background(), "mcp_docs_search_web", map[string]interface{}{
		"query": "nanobot",
	})
	require.NoError(t, err)
	assert.Equal(t, "result line 1\nresult line 2", result)
	assert.Equal(t, "search-web", client.lastCallName)
	assert.Equal(t, "nanobot", client.lastCallArgs["query"])
}

func TestMCPConnectorContinuesWhenServerFails(t *testing.T) {
	reg := NewRegistry()
	working := &fakeMCPClient{
		tools: []mcpRemoteTool{
			{Name: "ping", Description: "pong", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		},
	}

	connector := newMCPConnectorWithFactory(
		map[string]MCPServerOptions{
			"broken":  {Command: "bad"},
			"working": {Command: "ok"},
		},
		fakeMCPFactory{
			clients: map[string]*fakeMCPClient{"working": working},
			errs:    map[string]error{"broken": assert.AnError},
		},
	)

	err := connector.Connect(context.Background(), reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
	assert.True(t, working.initializeCalled)

	_, ok := reg.Get("mcp_working_ping")
	assert.True(t, ok, "working server tools should still be registered")
}

func TestMCPConnectorCloseClosesClients(t *testing.T) {
	reg := NewRegistry()
	client := &fakeMCPClient{
		tools: []mcpRemoteTool{
			{Name: "ping", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		},
	}
	connector := newMCPConnectorWithFactory(
		map[string]MCPServerOptions{"x": {Command: "ok"}},
		fakeMCPFactory{
			clients: map[string]*fakeMCPClient{"x": client},
			errs:    map[string]error{},
		},
	)

	require.NoError(t, connector.Connect(context.Background(), reg))
	require.NoError(t, connector.Close())
	assert.True(t, client.closeCalled)
}

func TestRenderMCPToolResultStructuredContent(t *testing.T) {
	out := renderMCPToolResult(mcpCallResult{
		Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "ok"},
		},
		StructuredContent: map[string]interface{}{"k": "v"},
	})
	assert.Contains(t, out, "ok")
	assert.Contains(t, out, `"k": "v"`)
}
