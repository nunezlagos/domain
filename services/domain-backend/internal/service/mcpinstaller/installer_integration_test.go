//go:build integration

// F1: mcpinstaller service — list / install / uninstall MCP providers
// en configs de clientes IA (opencode, claude-code, claude-desktop).

package mcpinstaller_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/service/mcpinstaller"
)

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	reg := seeds.NewRegistry()
	reg.Register(&seeds.MCPProvidersSeeder{})
	_, err = reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
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

func TestInstaller_List_ReturnsBuiltins(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	svc := mcpinstaller.New(pool)
	providers, err := svc.List(context.Background())
	require.NoError(t, err)

	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	require.Equal(t, []string{"fetch", "filesystem", "git", "github", "memory", "time"}, names)
}

func TestInstaller_Install_OpenCodeWritesEntry(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	configPath := filepath.Join(tmp, "opencode.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600))

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:    "filesystem",
		Agent:       mcpinstaller.AgentOpenCode,
		ConfigPath:  configPath,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var got opencodeConfig
	require.NoError(t, json.Unmarshal(data, &got))

	entry, ok := got.MCP["filesystem"]
	require.True(t, ok, "entry 'filesystem' debe existir en opencode.json")
	require.Equal(t, "npx", entry.Command[0])
	require.True(t, entry.Enabled)
}

func TestInstaller_Install_Idempotent(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "opencode.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600))

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "filesystem",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err)

	_, err = svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "filesystem",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err, "segunda install no debe fallar")

	data, _ := os.ReadFile(configPath)
	var got opencodeConfig
	require.NoError(t, json.Unmarshal(data, &got))
	require.Len(t, got.MCP, 1, "no debe duplicar la entry")
}

func TestInstaller_Install_BackupCreated(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "opencode.json")
	original := `{"mcp":{"other":{"command":["a"],"enabled":true}}}`
	require.NoError(t, os.WriteFile(configPath, []byte(original), 0o600))

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "filesystem",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err)

	matches, err := filepath.Glob(configPath + ".backup-*")
	require.NoError(t, err)
	require.NotEmpty(t, matches, "debe existir un backup con timestamp")

	backupData, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	require.Equal(t, original, string(backupData), "backup debe tener el contenido original")
}

func TestInstaller_Install_FailsIfRequiredEnvMissing(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "opencode.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600))

	t.Setenv("GITHUB_TOKEN", "")

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "github",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.Error(t, err, "debe fallar porque GITHUB_TOKEN no está seteado")
	require.Contains(t, err.Error(), "GITHUB_TOKEN")
}

func TestInstaller_Uninstall_RemovesEntry(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "opencode.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600))

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "fetch",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err)

	err = svc.Uninstall(context.Background(), mcpinstaller.UninstallInput{
		Provider:   "fetch",
		Agent:      mcpinstaller.AgentOpenCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err)

	data, _ := os.ReadFile(configPath)
	var got opencodeConfig
	require.NoError(t, json.Unmarshal(data, &got))
	_, present := got.MCP["fetch"]
	require.False(t, present, "fetch debe haber sido removido")
}

func TestInstaller_Install_ClaudeCodeWritesMCPJson(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".mcp.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers":{}}`), 0o600))

	svc := mcpinstaller.New(pool)
	_, err := svc.Install(context.Background(), mcpinstaller.InstallInput{
		Provider:   "filesystem",
		Agent:      mcpinstaller.AgentClaudeCode,
		ConfigPath: configPath,
	})
	require.NoError(t, err)

	data, _ := os.ReadFile(configPath)
	var got struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal(data, &got))

	entry, ok := got.MCPServers["filesystem"]
	require.True(t, ok, "filesystem debe estar en mcpServers de .mcp.json")
	require.Equal(t, "npx", entry.Command)
}
