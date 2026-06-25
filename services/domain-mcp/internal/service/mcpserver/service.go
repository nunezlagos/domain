// Package mcpserver — issue-12.4 service CRUD para MCP servers externos +
// auto-discovery de tools + materialización como skills.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/mcp/client"
	"nunezlagos/domain/internal/service/mcpserver/mcpserverdb"
)

var (
	ErrUnknown          = errors.New("not_found")
	ErrInvalidTransport = errors.New("invalid_transport")
	ErrCommandRequired  = errors.New("command_required_for_stdio")
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
	TransportSSE   = "sse"
)

// Server representa un MCP externo registrado.
type Server struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Transport       string     `json:"transport"`
	Command         string     `json:"command,omitempty"`
	Args            []string   `json:"args"`
	URL             string     `json:"url,omitempty"`
	Enabled         bool       `json:"enabled"`
	Status          string     `json:"status"`
	LastConnectedAt *time.Time `json:"last_connected_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	RetryCount      int        `json:"retry_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Tool descubierta de un servidor MCP.
type Tool struct {
	ID           uuid.UUID       `json:"id"`
	MCPServerID  uuid.UUID       `json:"mcp_server_id"`
	ToolName     string          `json:"tool_name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	Enabled      bool            `json:"enabled"`
	DiscoveredAt time.Time       `json:"discovered_at"`
}

// CreateInput para POST.
type CreateInput struct {
	Name      string
	Transport string
	Command   string
	Args      []string
	Env       map[string]string // se cifra antes de persistir
	URL       string
}

// Service operaciones sobre mcp_servers + sync de tools.
type Service struct {
	Pool   *pgxpool.Pool
	Cipher *crypto.Cipher
	Logger *slog.Logger
}

func (s *Service) q() *mcpserverdb.Queries { return mcpserverdb.New(s.Pool) }

// Create registra un nuevo servidor MCP externo. NO conecta — usar SyncTools.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (*Server, error) {
	if in.Transport == "" {
		in.Transport = TransportStdio
	}
	switch in.Transport {
	case TransportStdio:
		if in.Command == "" {
			return nil, ErrCommandRequired
		}
	case TransportHTTP, TransportSSE:
		if in.URL == "" {
			return nil, fmt.Errorf("%w: url required for %s", ErrInvalidTransport, in.Transport)
		}
	default:
		return nil, ErrInvalidTransport
	}

	var envCipher []byte
	if len(in.Env) > 0 && s.Cipher != nil {
		raw, _ := json.Marshal(in.Env)
		ct, err := s.Cipher.Encrypt(raw)
		if err != nil {
			return nil, fmt.Errorf("cipher env: %w", err)
		}
		envCipher = ct
	}

	row, err := s.q().InsertServer(ctx, mcpserverdb.InsertServerParams{
		Name:      in.Name,
		Transport: in.Transport,
		Command:   nullStr(in.Command),
		Args:      in.Args,
		EnvCipher: envCipher,
		Url:       nullStr(in.URL),
	})
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	srv := &Server{
		ID: row.ID, Name: in.Name, Transport: in.Transport,
		Command: in.Command, Args: in.Args, URL: in.URL,
		Enabled: true, Status: "pending",
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	return srv, nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Get devuelve un server por id+org.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Server, error) {
	row, err := s.q().GetServer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	srv := toServerFromGet(row)
	return &srv, nil
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Server, error) {
	rows, err := s.q().ListServers(ctx)
	if err != nil {
		return nil, err
	}
	var out []Server
	for _, row := range rows {
		out = append(out, toServerFromList(row))
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := s.q().DeleteServer(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

// SyncTools conecta al servidor, descubre sus tools y persiste el cache.
// Para transport=stdio spawnea el proceso, hace handshake, lista tools, cierra.
// Idempotente: tools con mismo name se actualizan.
func (s *Service) SyncTools(ctx context.Context, orgID, id uuid.UUID) ([]Tool, error) {
	srv, err := s.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if srv.Transport != TransportStdio {
		return nil, fmt.Errorf("transport %s not yet supported", srv.Transport)
	}

	env, err := s.decryptEnv(ctx, srv.ID)
	if err != nil {
		s.markFailed(ctx, srv.ID, fmt.Sprintf("decrypt env: %v", err))
		return nil, err
	}

	cli, err := client.NewStdioClient(ctx, client.Config{
		Command: srv.Command, Args: srv.Args, Env: env,
	})
	if err != nil {
		s.markFailed(ctx, srv.ID, fmt.Sprintf("spawn: %v", err))
		return nil, fmt.Errorf("spawn: %w", err)
	}
	defer cli.Close()

	if err := cli.Initialize(ctx); err != nil {
		s.markFailed(ctx, srv.ID, fmt.Sprintf("initialize: %v", err))
		return nil, fmt.Errorf("initialize: %w", err)
	}
	tools, err := cli.ListTools(ctx)
	if err != nil {
		s.markFailed(ctx, srv.ID, fmt.Sprintf("list_tools: %v", err))
		return nil, fmt.Errorf("list_tools: %w", err)
	}

	s.markConnected(ctx, srv.ID)
	persisted, err := s.upsertTools(ctx, srv.ID, orgID, tools)
	if err != nil {
		return nil, fmt.Errorf("upsert tools: %w", err)
	}
	if s.Logger != nil {
		s.Logger.InfoContext(ctx, "mcp server synced",
			slog.String("name", srv.Name),
			slog.Int("tools", len(persisted)))
	}
	return persisted, nil
}

// decryptEnv devuelve env en formato KEY=VALUE (compatible exec.Cmd.Env).
func (s *Service) decryptEnv(ctx context.Context, id uuid.UUID) ([]string, error) {
	ct, err := s.q().GetServerEnvCipher(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(ct) == 0 {
		return nil, nil
	}
	if s.Cipher == nil {
		return nil, errors.New("env cifrado pero Cipher no configurado")
	}
	plain, err := s.Cipher.Decrypt(ct)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out, nil
}

func (s *Service) upsertTools(ctx context.Context, serverID, orgID uuid.UUID, tools []client.Tool) ([]Tool, error) {
	var out []Tool
	for _, t := range tools {
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{}`)
		}
		row, err := s.q().UpsertTool(ctx, mcpserverdb.UpsertToolParams{
			McpServerID: serverID,
			ToolName:    t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, toToolFromUpsert(row))
	}
	return out, nil
}

func (s *Service) markConnected(ctx context.Context, id uuid.UUID) {
	_ = s.q().MarkServerConnected(ctx, id)
}

func (s *Service) markFailed(ctx context.Context, id uuid.UUID, errMsg string) {
	_ = s.q().MarkServerFailed(ctx, mcpserverdb.MarkServerFailedParams{
		ID:        id,
		LastError: &errMsg,
	})
}

// ListTools retorna las tools descubiertas para un server.
func (s *Service) ListTools(ctx context.Context, orgID, serverID uuid.UUID) ([]Tool, error) {
	rows, err := s.q().ListToolsByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	var out []Tool
	for _, row := range rows {
		out = append(out, toToolFromList(row))
	}
	return out, nil
}

// InvokeTool ejecuta una tool externa: spawnea proceso, conecta, llama, cierra.
// Versión simple: 1 conexión por invocación. Versión avanzada mantendría pool
// de conexiones long-lived (issue-12.4 mejora).
func (s *Service) InvokeTool(ctx context.Context, orgID, serverID uuid.UUID,
	toolName string, args map[string]any) (*client.CallResult, error) {

	srv, err := s.Get(ctx, orgID, serverID)
	if err != nil {
		return nil, err
	}
	env, err := s.decryptEnv(ctx, srv.ID)
	if err != nil {
		return nil, err
	}
	cli, err := client.NewStdioClient(ctx, client.Config{
		Command: srv.Command, Args: srv.Args, Env: env,
	})
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}
	defer cli.Close()
	if err := cli.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	return cli.CallTool(ctx, toolName, args)
}

func toServerFromGet(r mcpserverdb.GetServerRow) Server {
	srv := Server{
		ID:         r.ID,
		Name:       r.Name,
		Transport:  r.Transport,
		Command:    r.Command,
		Args:       r.Args,
		URL:        r.Url,
		Enabled:    r.Enabled,
		Status:     r.Status,
		LastError:  r.LastError,
		RetryCount: int(r.RetryCount),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.LastConnectedAt.Valid {
		t := r.LastConnectedAt.Time
		srv.LastConnectedAt = &t
	}
	return srv
}

func toServerFromList(r mcpserverdb.ListServersRow) Server {
	srv := Server{
		ID:         r.ID,
		Name:       r.Name,
		Transport:  r.Transport,
		Command:    r.Command,
		Args:       r.Args,
		URL:        r.Url,
		Enabled:    r.Enabled,
		Status:     r.Status,
		LastError:  r.LastError,
		RetryCount: int(r.RetryCount),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.LastConnectedAt.Valid {
		t := r.LastConnectedAt.Time
		srv.LastConnectedAt = &t
	}
	return srv
}

func toToolFromUpsert(r mcpserverdb.UpsertToolRow) Tool {
	return Tool{
		ID:           r.ID,
		MCPServerID:  r.McpServerID,
		ToolName:     r.ToolName,
		Description:  r.Description,
		InputSchema:  json.RawMessage(r.InputSchema),
		Enabled:      r.Enabled,
		DiscoveredAt: r.DiscoveredAt,
	}
}

func toToolFromList(r mcpserverdb.ListToolsByServerRow) Tool {
	return Tool{
		ID:           r.ID,
		MCPServerID:  r.McpServerID,
		ToolName:     r.ToolName,
		Description:  r.Description,
		InputSchema:  json.RawMessage(r.InputSchema),
		Enabled:      r.Enabled,
		DiscoveredAt: r.DiscoveredAt,
	}
}
