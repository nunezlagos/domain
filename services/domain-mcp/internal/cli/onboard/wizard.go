// Package onboard — issue-01.9 TUI wizard minimalista para first-run setup.
//
// Filosofia: el flow es de 4 inputs (server URL, email, codigo OTP opcional,
// y/N opencode). No justifica una TUI pesada (charmbracelet = 20MB binario).
// Usamos bufio.Scanner + fmt.Scanln para prompts en stderr, resultados en stdout.
//
// El flow:
//   1. Detectar first-run via GET /auth/first-run
//   2. Si first-run: POST /auth/bootstrap, save creds, exit
//   3. Si no: pedir email, POST /auth/request-otp, pedir codigo,
//      POST /auth/verify-otp, save creds
//   4. Preguntar si configurar opencode, si Y: invocar domain setup opencode
//   5. Exit 0
package onboard

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Credentials se guarda en ~/.config/domain/credentials.json (chmod 600).
type Credentials struct {
	APIKey      string    `json:"api_key"`
	APIKeyID    uuid.UUID `json:"api_key_id"`
	UserID      uuid.UUID `json:"user_id"`
	OrgID       uuid.UUID `json:"organization_id"`
	Email       string    `json:"email"`
	BaseURL     string    `json:"base_url"`
	IssuedAt    time.Time `json:"issued_at"`
}

// Wizard orquesta el flujo.
type Wizard struct {
	BaseURL     string
	In          io.Reader // stdin para prompts (default: os.Stdin)
	Out         io.Writer // stdout para resultados (default: os.Stdout)
	Err         io.Writer // stderr para prompts (default: os.Stderr)
	HTTPClient  *http.Client
	NonInteractive bool
	NoOpencode  bool

	DomainBinPath string

	DomainMCPPath string


	SaveCredentials func(*Credentials) error
}

// New retorna un wizard con defaults sensatos.
func New(baseURL string) *Wizard {
	return &Wizard{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		SaveCredentials: SaveCredentialsDefault,
	}
}

// Run ejecuta el wizard completo. Retorna nil en exito, error en fallo.
func (w *Wizard) Run(ctx context.Context) error {
	fmt.Fprintln(w.Err, "Domain Onboard Wizard")
	fmt.Fprintln(w.Err, "")


	fmt.Fprintln(w.Err, "Detecting first-run...")
	isFirstRun, userCount, err := w.detectFirstRun(ctx)
	if err != nil {
		return fmt.Errorf("detect first-run: %w", err)
	}
	fmt.Fprintf(w.Err, "  %s (DB has %d users)\n", boolLabel(isFirstRun, "yes", "no"), userCount)
	fmt.Fprintln(w.Err, "")


	baseURL, err := w.ask("Server URL", w.BaseURL, w.NonInteractive)
	if err != nil {
		return err
	}
	w.BaseURL = strings.TrimRight(baseURL, "/")


	email, err := w.ask("Your email", "", w.NonInteractive)
	if err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))


	creds, err := w.auth(ctx, isFirstRun, email)
	if err != nil {
		return err
	}
	creds.BaseURL = w.BaseURL
	creds.IssuedAt = time.Now()


	if err := w.SaveCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Fprintf(w.Err, "✓ API key saved to %s (mode 0600)\n", CredentialsPath())
	fmt.Fprintln(w.Err, "")


	if w.NoOpencode {
		fmt.Fprintln(w.Out, "✓ Onboard complete (skipping opencode config).")
		return nil
	}

	if !w.NonInteractive {
		ans, err := w.askYesNo("Configure opencode MCP server?", true)
		if err != nil {
			return err
		}
		if !ans {
			fmt.Fprintln(w.Out, "✓ Onboard complete. Run `domain setup opencode` later to configure.")
			return nil
		}
	}

	if err := w.setupOpencode(ctx, creds.APIKey); err != nil {

		fmt.Fprintf(w.Err, "⚠ opencode setup failed: %v\n", err)
		fmt.Fprintln(w.Err, "  You can run it later with: domain setup opencode")
	} else {
		fmt.Fprintln(w.Err, "✓ opencode MCP server configured. Restart opencode to discover 58 tools.")
	}

	fmt.Fprintln(w.Out, "✓ Onboard complete.")
	return nil
}

// auth ejecuta el flujo segun first-run.
func (w *Wizard) auth(ctx context.Context, isFirstRun bool, email string) (*Credentials, error) {
	if isFirstRun {
		fmt.Fprintf(w.Err, "→ Bootstrapping organization + user %s...\n", email)
		return w.bootstrap(ctx, email)
	}
	fmt.Fprintf(w.Err, "→ Sending OTP to %s...\n", email)
	if err := w.requestOTP(ctx, email); err != nil {
		return nil, err
	}
	code, err := w.ask("Enter 6-digit code from your email", "", w.NonInteractive)
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	return w.verifyOTP(ctx, email, code)
}

func (w *Wizard) detectFirstRun(ctx context.Context) (bool, int, error) {
	resp, err := w.doGET(ctx, "/api/v1/auth/first-run")
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		IsFirstRun bool `json:"is_first_run"`
		UserCount  int  `json:"user_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, 0, err
	}
	return out.IsFirstRun, out.UserCount, nil
}

func (w *Wizard) bootstrap(ctx context.Context, email string) (*Credentials, error) {
	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := w.doPOST(ctx, "/api/v1/auth/bootstrap", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 400 {

		return nil, fmt.Errorf("bootstrap rejected: another user was created concurrently; re-run onboard")
	}
	if resp.StatusCode == 422 {
		return nil, fmt.Errorf("email format invalid")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		UserID uuid.UUID `json:"user_id"`
		OrgID  uuid.UUID `json:"organization_id"`
		APIKey string    `json:"api_key"`
		KeyID  uuid.UUID `json:"api_key_id"`
		Email  string    `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	fmt.Fprintf(w.Err, "✓ Organization + user created\n")
	return &Credentials{
		APIKey:   out.APIKey,
		APIKeyID: out.KeyID,
		UserID:   out.UserID,
		OrgID:    out.OrgID,
		Email:    out.Email,
	}, nil
}

func (w *Wizard) requestOTP(ctx context.Context, email string) error {
	body, _ := json.Marshal(map[string]string{"identifier": email})
	resp, err := w.doPOST(ctx, "/api/v1/auth/request-otp", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("request-otp status %d", resp.StatusCode)
	}
	return nil
}

func (w *Wizard) verifyOTP(ctx context.Context, email, code string) (*Credentials, error) {
	body, _ := json.Marshal(map[string]string{"identifier": email, "code": code})
	resp, err := w.doPOST(ctx, "/api/v1/auth/verify-otp", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid code (or expired)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("verify-otp status %d", resp.StatusCode)
	}
	var out struct {
		UserID uuid.UUID `json:"user_id"`
		OrgID  uuid.UUID `json:"organization_id"`
		APIKey string    `json:"api_key"`
		KeyID  uuid.UUID `json:"api_key_id"`
		Email  string    `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &Credentials{
		APIKey:   out.APIKey,
		APIKeyID: out.KeyID,
		UserID:   out.UserID,
		OrgID:    out.OrgID,
		Email:    out.Email,
	}, nil
}

// ask muestra un prompt en stderr y lee la respuesta desde In.
// Si defaultVal no es vacio, lo muestra entre corchetes y lo usa si la
// respuesta es vacia. En modo NonInteractive, retorna el default o falla.
func (w *Wizard) ask(prompt, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		if defaultVal == "" {
			return "", fmt.Errorf("--non-interactive requires a default for %q", prompt)
		}
		return defaultVal, nil
	}
	var display string
	if defaultVal != "" {
		display = prompt + " [" + defaultVal + "]: "
	} else {
		display = prompt + ": "
	}
	fmt.Fprint(w.Err, display)
	scanner := bufio.NewScanner(w.In)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("stdin cerrado")
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// askYesNo muestra un y/N prompt. defaultYes se usa si la respuesta es vacia
// o en modo NonInteractive.
func (w *Wizard) askYesNo(prompt string, defaultYes bool) (bool, error) {
	if w.NonInteractive {
		return defaultYes, nil
	}
	defLabel := "Y/n"
	if !defaultYes {
		defLabel = "y/N"
	}
	fmt.Fprintf(w.Err, "? %s [%s]: ", prompt, defLabel)
	scanner := bufio.NewScanner(w.In)
	if !scanner.Scan() {
		return false, errors.New("stdin cerrado")
	}
	line := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if line == "" {
		return defaultYes, nil
	}
	return line == "y" || line == "yes", nil
}

// setupOpencode invoca `domain setup opencode --api-key KEY --base-url URL`.
// El path del binario es DomainBinPath; el path del MCP es DomainMCPPath.
func (w *Wizard) setupOpencode(ctx context.Context, apiKey string) error {
	bin := w.DomainBinPath
	if bin == "" {
		bin = "domain" // fallback: asume en PATH
	}
	args := []string{"setup", "opencode", "--api-key", apiKey, "--base-url", w.BaseURL}
	if w.DomainMCPPath != "" {
		args = append(args, "--mcp-binary", w.DomainMCPPath)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = w.Out
	cmd.Stderr = w.Err
	return cmd.Run()
}

func (w *Wizard) doGET(ctx context.Context, path string) (*http.Response, error) {
	return w.do(ctx, http.MethodGet, path, nil)
}

func (w *Wizard) doPOST(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return w.do(ctx, http.MethodPost, path, body)
}

func (w *Wizard) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	url := w.BaseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return w.HTTPClient.Do(req)
}

func boolLabel(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// CredentialsPath retorna la ruta canonica de las credenciales.
func CredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/domain/credentials.json"
	}
	return filepath.Join(home, ".config", "domain", "credentials.json")
}

// SaveCredentialsDefault persiste con mode 0600. Si el archivo existe,
// hace backup .bak antes de sobrescribir.
func SaveCredentialsDefault(c *Credentials) error {
	path := CredentialsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {

		_ = os.Rename(path, path+".bak")
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
