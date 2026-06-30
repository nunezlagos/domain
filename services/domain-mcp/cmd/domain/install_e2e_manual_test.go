//go:build e2einstall

// E2E manual del install local (HU-01.10/01.14). NO corre en CI:
// requiere docker + mutará configs reales del user (credentials.json,
// ~/.config/opencode/opencode.json, .env del repo). Correr explícito:
//
//	go test -tags=e2einstall -count=1 -run TestE2EInstall ./cmd/domain/ -args -e2e
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var e2eFlag = flag.Bool("e2e", false, "habilita el E2E real del install")

// chdirRepoRoot sube hasta encontrar go.mod (idempotente entre tests
// del mismo proceso, que comparten cwd).
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	for i := 0; i < 5; i++ {
		if _, err := os.Stat("go.mod"); err == nil {
			return
		}
		require.NoError(t, os.Chdir(".."))
	}
	t.Fatal("go.mod no encontrado subiendo 5 niveles")
}

func TestE2EInstall_LocalReal(t *testing.T) {
	if !*e2eFlag {
		t.Skip("pasar -args -e2e para correr el E2E real")
	}


	chdirRepoRoot(t)





	home, err := os.UserHomeDir()
	require.NoError(t, err)
	binDir := filepath.Join(home, "go", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	for _, target := range []struct{ out, pkg string }{
		{filepath.Join(binDir, "domain-mcp"), "./cmd/domain-mcp"},
		{filepath.Join(binDir, "domain"), "./cmd/domain"},
	} {
		build := exec.Command("go", "build", "-o", target.out, target.pkg)
		out, buildErr := build.CombinedOutput()
		require.NoError(t, buildErr, "build %s: %s", target.pkg, out)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInstall([]string{
		"--mode", "local",
		"--non-interactive",
		"--agents", "opencode",
		"--base-url", "http://localhost:8000",
		"--no-init",
	})
	require.Equal(t, 0, code, "install debe terminar exit 0")


	credPath := filepath.Join(home, ".config", "domain", "credentials.json")
	data, err := os.ReadFile(credPath)
	require.NoError(t, err, "credentials.json debe existir post-install")
	var creds struct {
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(data, &creds))
	require.NotEmpty(t, creds.APIKey, "API key generada automáticamente")


	envPath := filepath.Join(home, ".config", "domain", "env")
	envData, err := os.ReadFile(envPath)
	require.NoError(t, err, "~/.config/domain/env debe existir")
	require.Contains(t, string(envData), "DOMAIN_DATABASE_URL=")
	require.Contains(t, string(envData), "DOMAIN_BASE_URL=")


	ocPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	ocData, err := os.ReadFile(ocPath)
	require.NoError(t, err, "opencode.json global debe existir")
	var oc map[string]any
	require.NoError(t, json.Unmarshal(ocData, &oc))
	mcp, _ := oc["mcp"].(map[string]any)
	require.NotNil(t, mcp["domain"], "entry mcp.domain presente")
	entry := mcp["domain"].(map[string]any)
	cmd, _ := entry["command"].([]any)
	require.NotEmpty(t, cmd, "command no vacío")
	first, _ := cmd[0].(string)
	require.NotEmpty(t, first, "command[0] apunta al binario domain-mcp")
	t.Logf("opencode command: %v", cmd)



	if systemdUserAvailable() {
		out, _ := exec.Command("systemctl", "--user", "is-active", serviceName).CombinedOutput()
		require.Equal(t, "active", strings.TrimSpace(string(out)),
			"domain.service debe quedar activo post-install")
		code, err := httpGet("http://localhost:8000/health")
		require.NoError(t, err)
		require.Equal(t, 200, code, "/health responde con el service corriendo")
		t.Log("systemd: domain.service activo y /health 200")
	} else {
		t.Log("systemd user manager no disponible: paso de service skippeado")
	}
}

// TestE2EInstall_ServerBootsWithoutEnv verifica plug-and-play del CLI:
// `domain server` arranca SIN env vars y desde fuera del repo (toma la
// config de ~/.config/domain/env vía loadEnvCascade) y responde /health.
func TestE2EInstall_ServerBootsWithoutEnv(t *testing.T) {
	if !*e2eFlag {
		t.Skip("pasar -args -e2e para correr el E2E real")
	}
	chdirRepoRoot(t)

	bin := filepath.Join(t.TempDir(), "domain")
	build := exec.Command("go", "build", "-o", bin, "./cmd/domain")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build domain: %s", out)

	cmd := exec.Command(bin, "server")
	cmd.Dir = t.TempDir() // cwd fuera del repo: sin .env local
	home, _ := os.UserHomeDir()
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill() }()


	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := httpGet("http://localhost:8000/health")
		if err == nil && resp == 200 {
			t.Log("server /health 200 sin env vars — plug-and-play OK")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("server no respondió /health en 15s. Output:\n%s", output.String())
}

func httpGet(url string) (int, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

var httpClient = &http.Client{Timeout: 2 * time.Second}

// TestE2EInstall_MCPBootsAsOpenCode reproduce 1:1 cómo OpenCode lanza
// domain-mcp: lee del opencode.json global el command y el environment
// del entry "domain", lanza ESE binario con ESE entorno y manda el
// initialize MCP. Si esto falla, es exactamente el "-32000 Connection
// closed" del agente — y acá capturamos el stderr con la causa real.
func TestE2EInstall_MCPBootsAsOpenCode(t *testing.T) {
	if !*e2eFlag {
		t.Skip("pasar -args -e2e para correr el E2E real")
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	ocPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(ocPath)
	require.NoError(t, err, "opencode.json global debe existir")
	var oc struct {
		MCP map[string]struct {
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
		} `json:"mcp"`
	}
	require.NoError(t, json.Unmarshal(data, &oc))
	entry, ok := oc.MCP["domain"]
	require.True(t, ok, "entry mcp.domain presente")
	require.NotEmpty(t, entry.Command)

	bin := entry.Command[0]
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Fatalf("el command de opencode.json apunta a un binario inexistente: %s", bin)
	}

	cmd := exec.Command(bin, entry.Command[1:]...)
	env := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	for k, v := range entry.Environment {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Dir = t.TempDir() // opencode lanza desde el proyecto del user (cualquier cwd)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill() }()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e-opencode","version":"0"}}}`
	_, err = fmt.Fprintln(stdin, initReq)
	require.NoError(t, err)

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		if scanner.Scan() {
			ch <- result{line: scanner.Text()}
			return
		}
		ch <- result{err: fmt.Errorf("EOF sin respuesta — esto ES el -32000. stderr del boot: %s", stderrBuf.String())}
	}()

	select {
	case res := <-ch:
		require.NoError(t, res.err)
		require.Contains(t, res.line, `"result"`, "initialize debe responder result")
		t.Log("domain-mcp lanzado como opencode: initialize OK")
	case <-time.After(25 * time.Second):
		t.Fatalf("timeout esperando initialize (stderr: %s)", stderrBuf.String())
	}
}

// TestE2EInstall_MCPBootsWithoutEnv verifica el fix -32000: domain-mcp
// arranca SIN env vars (toma config de ~/.config/domain/env +
// credentials.json) y responde al handshake MCP initialize.
func TestE2EInstall_MCPBootsWithoutEnv(t *testing.T) {
	if !*e2eFlag {
		t.Skip("pasar -args -e2e para correr el E2E real")
	}
	chdirRepoRoot(t)

	bin := filepath.Join(t.TempDir(), "domain-mcp")
	build := exec.Command("go", "build", "-o", bin, "./cmd/domain-mcp")
	build.Dir, _ = os.Getwd()
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build domain-mcp: %s", out)

	cmd := exec.Command(bin)

	home, _ := os.UserHomeDir()
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill() }()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e","version":"0"}}}`
	_, err = fmt.Fprintln(stdin, initReq)
	require.NoError(t, err)

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		if scanner.Scan() {
			ch <- result{line: scanner.Text()}
			return
		}
		ch <- result{err: fmt.Errorf("EOF sin respuesta (stderr: %s)", stderrBuf.String())}
	}()

	select {
	case res := <-ch:
		require.NoError(t, res.err, "domain-mcp murió al boot — el -32000 sigue vivo")
		require.Contains(t, res.line, `"jsonrpc"`, "respuesta JSON-RPC esperada")
		require.Contains(t, res.line, `"result"`, "initialize debe responder result, no error")
		t.Logf("initialize OK: %.120s...", res.line)
	case <-time.After(20 * time.Second):
		t.Fatalf("timeout esperando respuesta de initialize (stderr: %s)", stderrBuf.String())
	}
}
