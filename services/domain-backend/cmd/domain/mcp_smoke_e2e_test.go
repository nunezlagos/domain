//go:build e2einstall

// Smoke E2E de los tools MCP "como opencode": lanza domain-mcp con el
// command + environment del opencode.json (proyecto si existe, sino
// global), hace el handshake completo y ejercita tools reales:
// tools/list, domain_policy_list, domain_mem_save + domain_mem_search.
//
//	go test -tags=e2einstall -run TestE2E_MCPSmoke ./cmd/domain/ -args -e2e
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mcpSession maneja un proceso domain-mcp por stdio con JSON-RPC.
type mcpSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  *json.Encoder
	lines  *bufio.Scanner
	stderr *strings.Builder
}

func startMCPAsOpenCode(t *testing.T) *mcpSession {
	t.Helper()
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Preferir el opencode.json del proyecto (el caso real del user);
	// fallback al global.
	candidates := []string{
		filepath.Join(home, "Proyectos", "domain", "opencode.json"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
	}
	var entry struct {
		Command     []string          `json:"command"`
		Environment map[string]string `json:"environment"`
	}
	var src string
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var oc struct {
			MCP map[string]json.RawMessage `json:"mcp"`
		}
		if json.Unmarshal(data, &oc) != nil {
			continue
		}
		raw, ok := oc.MCP["domain"]
		if !ok {
			continue
		}
		require.NoError(t, json.Unmarshal(raw, &entry))
		src = p
		break
	}
	require.NotEmpty(t, entry.Command, "ningún opencode.json tiene mcp.domain")
	t.Logf("lanzando como opencode: %s (config %s)", entry.Command[0], src)

	cmd := exec.Command(entry.Command[0], entry.Command[1:]...)
	env := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	for k, v := range entry.Environment {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Dir = t.TempDir()

	stdinPipe, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	require.NoError(t, cmd.Start())

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)

	s := &mcpSession{
		t:      t,
		cmd:    cmd,
		stdin:  json.NewEncoder(stdinPipe),
		lines:  scanner,
		stderr: &stderrBuf,
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return s
}

// call manda un request y espera la response con ese id.
func (s *mcpSession) call(id int, method string, params any) map[string]any {
	s.t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	require.NoError(s.t, s.stdin.Encode(req))

	type lineRes struct {
		line string
		ok   bool
	}
	ch := make(chan lineRes, 1)
	go func() {
		for s.lines.Scan() {
			line := s.lines.Text()
			var probe struct {
				ID *int `json:"id"`
			}
			if json.Unmarshal([]byte(line), &probe) == nil && probe.ID != nil && *probe.ID == id {
				ch <- lineRes{line: line, ok: true}
				return
			}
			// notificaciones u otras responses: ignorar
		}
		ch <- lineRes{}
	}()
	select {
	case res := <-ch:
		require.True(s.t, res.ok, "EOF esperando response de %s (stderr: %s)", method, s.stderr.String())
		var parsed map[string]any
		require.NoError(s.t, json.Unmarshal([]byte(res.line), &parsed))
		require.Nil(s.t, parsed["error"], "%s devolvió error: %v", method, parsed["error"])
		result, _ := parsed["result"].(map[string]any)
		require.NotNil(s.t, result, "%s sin result", method)
		return result
	case <-time.After(30 * time.Second):
		s.t.Fatalf("timeout esperando %s (stderr: %s)", method, s.stderr.String())
		return nil
	}
}

// notify manda una notification (sin id, sin response).
func (s *mcpSession) notify(method string) {
	s.t.Helper()
	require.NoError(s.t, s.stdin.Encode(map[string]any{"jsonrpc": "2.0", "method": method}))
}

// callTool invoca tools/call y retorna el texto del primer content.
func (s *mcpSession) callTool(id int, name string, args map[string]any) (string, bool) {
	s.t.Helper()
	result := s.call(id, "tools/call", map[string]any{"name": name, "arguments": args})
	isError, _ := result["isError"].(bool)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return "", isError
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text, isError
}

func TestE2E_MCPSmoke_ToolsWorkAsOpenCode(t *testing.T) {
	if !*e2eFlag {
		t.Skip("pasar -args -e2e para correr el smoke real")
	}
	s := startMCPAsOpenCode(t)

	// 1. Handshake completo
	initRes := s.call(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smoke-opencode", "version": "0"},
	})
	require.NotNil(t, initRes["serverInfo"], "initialize debe traer serverInfo")
	s.notify("notifications/initialized")

	// 2. tools/list: los tools clave del producto presentes
	toolsRes := s.call(2, "tools/list", nil)
	tools, _ := toolsRes["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range tools {
		m, _ := tl.(map[string]any)
		if n, _ := m["name"].(string); n != "" {
			names[n] = true
		}
	}
	for _, want := range []string{
		"domain_mem_save", "domain_mem_search", "domain_mem_context",
		"domain_policy_get", "domain_policy_list",
		"domain_skill_search", "domain_search_global",
	} {
		require.True(t, names[want], "tool faltante en tools/list: %s", want)
	}
	t.Logf("tools/list OK: %d tools expuestos", len(tools))

	// 3. domain_policy_list: el seeder dejó policies baseline
	policyOut, isErr := s.callTool(3, "domain_policy_list", map[string]any{})
	require.False(t, isErr, "domain_policy_list error: %s", policyOut)
	require.Contains(t, policyOut, "sdd-tdd-strict", "policies seeded visibles vía MCP")

	// 4. domain_policy_get: body completo de una rule
	getOut, isErr := s.callTool(4, "domain_policy_get", map[string]any{"slug": "sdd-tdd-strict"})
	require.False(t, isErr, "domain_policy_get error: %s", getOut)
	require.Contains(t, getOut, "TDD", "body de la policy presente")

	// 5. Ciclo memoria: save (con auto-create del project) → search
	marker := fmt.Sprintf("smoke-opencode-%d", os.Getpid())
	saveOut, isErr := s.callTool(5, "domain_mem_save", map[string]any{
		"project_slug": "smoke-opencode",
		"content":      "Prueba E2E de domain en opencode: marcador " + marker,
		"type":         "manual",
	})
	require.False(t, isErr, "domain_mem_save error: %s", saveOut)
	require.Contains(t, saveOut, "id", "save devuelve id")

	searchOut, isErr := s.callTool(6, "domain_mem_search", map[string]any{
		"project_slug": "smoke-opencode",
		"query":        marker,
	})
	require.False(t, isErr, "domain_mem_search error: %s", searchOut)
	require.Contains(t, searchOut, marker, "la observación guardada se encuentra")

	// 6. domain_project_list: el project auto-creado aparece
	projOut, isErr := s.callTool(7, "domain_project_list", map[string]any{})
	require.False(t, isErr, "domain_project_list error: %s", projOut)
	require.Contains(t, projOut, "smoke-opencode", "project auto-creado visible")

	t.Log("SMOKE OK: handshake + tools/list + policies + mem_save/mem_search funcionando como opencode")
}
