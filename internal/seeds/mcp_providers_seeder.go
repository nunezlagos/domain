package seeds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// MCPProvidersSeeder siembra los built-ins públicos de MCPs instalables.
// Diferente de mcp_servers (HU-12.4) que es para MCPs EXTERNOS consumidos
// por domain. Esta tabla es para MCPs que domain OFRECE al cliente IA.
type MCPProvidersSeeder struct{}

func (s *MCPProvidersSeeder) Name() string    { return "mcp_providers" }
func (s *MCPProvidersSeeder) Version() int    { return 1 }
func (s *MCPProvidersSeeder) Order() int      { return 40 }
func (s *MCPProvidersSeeder) IsDevOnly() bool { return false }

type mcpProviderEntry struct {
	Name        string
	Description string
	Command     string
	DefaultArgs []string
	EnvTemplate map[string]string
	RequiredEnv []string
	Tags        []string
}

func (s *MCPProvidersSeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	var rep Report

	providers := []mcpProviderEntry{
		{
			Name: "filesystem", Description: "Read/write local files within allowed directories",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			EnvTemplate: map[string]string{}, RequiredEnv: []string{},
			Tags: []string{"fs", "read", "write", "files"},
		},
		{
			Name: "fetch", Description: "HTTP fetch with size and URL constraints",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-fetch"},
			EnvTemplate: map[string]string{}, RequiredEnv: []string{},
			Tags: []string{"http", "fetch", "web"},
		},
		{
			Name: "github", Description: "GitHub API: repos, issues, PRs, search",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-github"},
			EnvTemplate: map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}"},
			RequiredEnv: []string{"GITHUB_TOKEN"},
			Tags: []string{"github", "git", "vcs", "api"},
		},
		{
			Name: "git", Description: "Local git operations: log, diff, show, commit",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-git"},
			EnvTemplate: map[string]string{}, RequiredEnv: []string{},
			Tags: []string{"git", "vcs", "local"},
		},
		{
			Name: "memory", Description: "In-memory key-value store with namespaces",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-memory"},
			EnvTemplate: map[string]string{}, RequiredEnv: []string{},
			Tags: []string{"memory", "kv", "store"},
		},
		{
			Name: "time", Description: "Time and timezone conversions",
			Command: "npx", DefaultArgs: []string{"-y", "@modelcontextprotocol/server-time"},
			EnvTemplate: map[string]string{}, RequiredEnv: []string{},
			Tags: []string{"time", "date", "timezone"},
		},
	}

	for _, p := range providers {
		envJSON, err := json.Marshal(p.EnvTemplate)
		if err != nil {
			return rep, fmt.Errorf("marshal env %s: %w", p.Name, err)
		}
		if p.DefaultArgs == nil {
			p.DefaultArgs = []string{}
		}
		if p.RequiredEnv == nil {
			p.RequiredEnv = []string{}
		}
		if p.Tags == nil {
			p.Tags = []string{}
		}

		var existingID string
		err = tx.QueryRow(ctx, `
			SELECT id::text FROM mcp_providers
			WHERE name = $1 AND organization_id IS NULL
		`, p.Name).Scan(&existingID)

		if err == nil {
			_, uerr := tx.Exec(ctx, `
				UPDATE mcp_providers
				SET description = $2, command = $3, default_args = $4,
				    env_template = $5::jsonb, required_env = $6, tags = $7
				WHERE id = $1::uuid
			`, existingID, p.Description, p.Command, p.DefaultArgs, string(envJSON), p.RequiredEnv, p.Tags)
			if uerr != nil {
				return rep, fmt.Errorf("update provider %s: %w", p.Name, uerr)
			}
			rep.Updated++
			continue
		}

		_, ierr := tx.Exec(ctx, `
			INSERT INTO mcp_providers
				(name, description, command, default_args, env_template, required_env, tags, is_built_in, is_public, organization_id)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, TRUE, TRUE, NULL)
		`, p.Name, p.Description, p.Command, p.DefaultArgs, string(envJSON), p.RequiredEnv, p.Tags)
		if ierr != nil {
			return rep, fmt.Errorf("insert provider %s: %w", p.Name, ierr)
		}
		rep.Created++
	}
	return rep, nil
}
