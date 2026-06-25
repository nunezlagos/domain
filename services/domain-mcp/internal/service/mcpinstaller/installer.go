package mcpinstaller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/mcpinstaller/mcpinstallerdb"
	"nunezlagos/domain/internal/store/txctx"
)

type Agent string

const (
	AgentOpenCode      Agent = "opencode"
	AgentClaudeCode    Agent = "claude-code"
	AgentClaudeDesktop Agent = "claude-desktop"
)

type Provider struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	DefaultArgs []string          `json:"default_args"`
	EnvTemplate map[string]string `json:"env_template"`
	RequiredEnv []string          `json:"required_env"`
	Tags        []string          `json:"tags"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}

func (s *Service) q(ctx context.Context) *mcpinstallerdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return mcpinstallerdb.New(tx)
	}
	return mcpinstallerdb.New(s.Pool)
}

func (s *Service) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.q(ctx).ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}

	var out []Provider
	for _, r := range rows {
		out = append(out, toProvider(r))
	}
	return out, nil
}

func toProvider(r mcpinstallerdb.ListProvidersRow) Provider {
	p := Provider{
		Name:        r.Name,
		Description: r.Description,
		Command:     r.Command,
		DefaultArgs: r.DefaultArgs,
		RequiredEnv: r.RequiredEnv,
		Tags:        r.Tags,
	}
	if len(r.EnvTemplate) > 0 {
		_ = json.Unmarshal(r.EnvTemplate, &p.EnvTemplate)
	}
	if p.EnvTemplate == nil {
		p.EnvTemplate = map[string]string{}
	}
	return p
}

func (s *Service) getProvider(ctx context.Context, name string) (*Provider, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range providers {
		if providers[i].Name == name {
			return &providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %s not found", name)
}

type InstallInput struct {
	Provider   string
	Agent      Agent
	ConfigPath string
}

type InstallResult struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path,omitempty"`
}

type UninstallInput struct {
	Provider   string
	Agent      Agent
	ConfigPath string
}

func (s *Service) Install(ctx context.Context, in InstallInput) (*InstallResult, error) {
	if in.Provider == "" || in.ConfigPath == "" {
		return nil, fmt.Errorf("provider and config_path required")
	}

	p, err := s.getProvider(ctx, in.Provider)
	if err != nil {
		return nil, err
	}

	for _, e := range p.RequiredEnv {
		if os.Getenv(e) == "" {
			return nil, fmt.Errorf("required env var %s is not set", e)
		}
	}

	originalData, _ := os.ReadFile(in.ConfigPath)

	var backupPath string
	if len(originalData) > 0 {
		backupPath = in.ConfigPath + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.WriteFile(backupPath, originalData, 0o600); err != nil {
			return nil, fmt.Errorf("backup: %w", err)
		}
	}

	switch in.Agent {
	case AgentOpenCode:
		err = writeOpenCodeConfig(in.ConfigPath, p)
	case AgentClaudeCode, AgentClaudeDesktop:
		err = writeClaudeConfig(in.ConfigPath, p)
	default:
		return nil, fmt.Errorf("unsupported agent: %s", in.Agent)
	}
	if err != nil {
		return nil, err
	}
	return &InstallResult{Path: in.ConfigPath, BackupPath: backupPath}, nil
}

func (s *Service) Uninstall(_ context.Context, in UninstallInput) error {
	if in.ConfigPath == "" {
		return fmt.Errorf("config_path required")
	}
	data, err := os.ReadFile(in.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	switch in.Agent {
	case AgentOpenCode:
		var cfg opencodeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		delete(cfg.MCP, in.Provider)
		return writeJSON(in.ConfigPath, cfg)
	case AgentClaudeCode, AgentClaudeDesktop:
		var cfg claudeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		delete(cfg.MCPServers, in.Provider)
		return writeJSON(in.ConfigPath, cfg)
	default:
		return fmt.Errorf("unsupported agent: %s", in.Agent)
	}
}

type opencodeConfig struct {
	MCP map[string]opencodeMCP `json:"mcp"`
}

type opencodeMCP struct {
	Command []string          `json:"command"`
	Enabled bool              `json:"enabled"`
	Env     map[string]string `json:"environment,omitempty"`
	Type    string            `json:"type"`
}

type claudeConfig struct {
	MCPServers map[string]claudeMCP `json:"mcpServers"`
}

type claudeMCP struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func writeOpenCodeConfig(path string, p *Provider) error {
	data, _ := os.ReadFile(path)
	var cfg opencodeConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if cfg.MCP == nil {
		cfg.MCP = map[string]opencodeMCP{}
	}
	env := expandEnv(p.EnvTemplate)
	cfg.MCP[p.Name] = opencodeMCP{
		Command: append([]string{p.Command}, p.DefaultArgs...),
		Enabled: true,
		Env:     env,
		Type:    "local",
	}
	return writeJSON(path, cfg)
}

func writeClaudeConfig(path string, p *Provider) error {
	data, _ := os.ReadFile(path)
	var cfg claudeConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]claudeMCP{}
	}
	cfg.MCPServers[p.Name] = claudeMCP{
		Command: p.Command,
		Args:    p.DefaultArgs,
		Env:     expandEnv(p.EnvTemplate),
	}
	return writeJSON(path, cfg)
}

func expandEnv(t map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range t {
		val := v
		if len(val) > 2 && val[:2] == "${" && val[len(val)-1] == '}' {
			key := val[2 : len(val)-1]
			if envVal := os.Getenv(key); envVal != "" {
				val = envVal
			}
		}
		out[k] = val
	}
	return out
}

func writeJSON(path string, v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}
