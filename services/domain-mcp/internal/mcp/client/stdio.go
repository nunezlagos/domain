// Package client — issue-12.4 MCP client externo via stdio.
//
// Spawnea un proceso (típicamente `npx @modelcontextprotocol/server-X`) y se
// comunica via JSON-RPC 2.0 sobre stdin/stdout siguiendo la spec MCP.
//
// Lifecycle:
//   1. NewStdioClient(...) spawnea el proceso
//   2. Initialize() hace el handshake
//   3. ListTools() descubre tools disponibles
//   4. CallTool(name, args) invoca una tool
//   5. Close() termina el proceso gracefully
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Tool describe una tool expuesta por el servidor MCP externo.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// CallResult del tools/call.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// StdioClient mantiene un proceso + JSON-RPC bidireccional.
type StdioClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader

	nextID  atomic.Int64

	mu       sync.Mutex
	pending  map[int64]chan jsonrpcResponse

	closed atomic.Bool
}

type jsonrpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  any            `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Config para spawnar el proceso.
type Config struct {
	Command string
	Args    []string
	Env     []string // formato KEY=VALUE
}

// NewStdioClient spawnea el proceso y arranca el reader de stdout.
// El caller DEBE llamar Initialize antes de cualquier otra operación.
func NewStdioClient(ctx context.Context, cfg Config) (*StdioClient, error) {
	if cfg.Command == "" {
		return nil, errors.New("command required")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	c := &StdioClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int64]chan jsonrpcResponse),
	}
	go c.readLoop()
	return c, nil
}

// readLoop lee líneas JSON-RPC desde stdout y rutea responses a los waiters
// registrados en pending.
func (c *StdioClient) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			c.closeAllPending(fmt.Errorf("stdout closed: %w", err))
			return
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {

			continue
		}
		if resp.ID == 0 {
			continue // notification, no respondemos
		}
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (c *StdioClient) closeAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		ch <- jsonrpcResponse{ID: id, Error: &jsonrpcError{Code: -1, Message: err.Error()}}
		delete(c.pending, id)
	}
}

// call envía un request JSON-RPC y espera la response (timeout 30s default).
func (c *StdioClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, errors.New("client closed")
	}
	id := c.nextID.Add(1)
	req := jsonrpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	ch := make(chan jsonrpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	body = append(body, '\n')
	if _, err := c.stdin.Write(body); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write stdin: %w", err)
	}

	timeout := 30 * time.Second
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("rpc timeout: %s", method)
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Initialize hace el handshake MCP: initialize → initialized notification.
func (c *StdioClient) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "domain-mcp-client",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{},
	}
	if _, err := c.call(ctx, "initialize", params); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	notif := map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}
	body, _ := json.Marshal(notif)
	body = append(body, '\n')
	_, _ = c.stdin.Write(body)
	return nil
}

// ListTools envía tools/list y retorna las tools descubiertas.
func (c *StdioClient) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode tools: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invoca una tool por nombre con argumentos.
func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	raw, err := c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}
	var result CallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode result: %w", err)
	}
	return &result, nil
}

// Close termina el proceso gracefully. Idempotente.
func (c *StdioClient) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	_ = c.stdin.Close()
	if c.cmd.Process != nil {

		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = c.cmd.Process.Kill()
		}
	}
	return nil
}
