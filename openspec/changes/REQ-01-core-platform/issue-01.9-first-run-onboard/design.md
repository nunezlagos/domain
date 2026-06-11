# Design: issue-01.9-first-run-onboard

## Arquitectura

```
┌────────────────────────────────────────────────────────────┐
│  CLI (domain onboard)                                       │
│  ┌────────────────────┐                                     │
│  │ TUI wizard         │ stdin/stdout (readline-like)       │
│  │ - prompt email     │                                     │
│  │ - prompt code (si  │                                     │
│  │   ya hay users)    │                                     │
│  │ - prompt y/N       │                                     │
│  │   opencode?        │                                     │
│  └─────────┬──────────┘                                     │
│            │ HTTP (curl-like via net/http)                  │
└────────────┼───────────────────────────────────────────────┘
             │
             ▼
┌────────────────────────────────────────────────────────────┐
│  Server (POST /api/v1/auth/bootstrap)                        │
│  ┌────────────────────┐                                     │
│  │ Handler            │ Valida JSON, loguea                 │
│  │     │              │                                     │
│  │     ▼              │                                     │
│  │ bootstrap.Service  │ Lock advisory                       │
│  │     │              │ ↓                                   │
│  │     ├─ users==0?   │                                     │
│  │     │  ├─ sí → crear org+user+key                       │
│  │     │  └─ no → 400 email_not_in_any_org                  │
│  │     │              │                                     │
│  │     └─ si email    │                                     │
│  │        existe en   │                                     │
│  │        otra org →   │                                     │
│  │        400          │                                     │
│  │     else:          │                                     │
│  │        OTP flow    │                                     │
│  └────────────────────┘                                     │
└────────────────────────────────────────────────────────────┘
```

## Decisiones arquitectónicas

### 1. Endpoint separado `/auth/bootstrap` vs reusar `/auth/request-otp`

**Decisión:** endpoint nuevo.

**Por qué:** `/auth/request-otp` requiere user pre-existente. El bootstrap es un caso semánticamente distinto: "soy el primer usuario del sistema, créame". Mezclar ambos en el mismo endpoint haría que cualquiera pudiera auto-crearse un user con solo conocer el flujo, lo que es un agujero de seguridad.

### 2. Race condition prevention: `pg_advisory_xact_lock`

**Decisión:** usar advisory lock de Postgres al inicio del endpoint bootstrap.

**Por qué:** dos `domain onboard` ejecutados simultáneamente podrían crear dos users/orgs/api_keys. El advisory lock (`pg_advisory_xact_lock(BOOTSTRAP_LOCK_KEY)`) garantiza que solo uno ejecute la sección crítica. El lock se libera automáticamente al COMMIT o ROLLBACK.

```go
const BootstrapLockKey int64 = 0x424F4F54  // "BOOT"

func (s *Service) Bootstrap(ctx context.Context, in BootstrapInput) (*BootstrapResult, error) {
    tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return nil, err }
    defer tx.Rollback(ctx)

    // Lock advisory: solo un bootstrap a la vez.
    if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", BootstrapLockKey); err != nil {
        return nil, err
    }

    var count int
    err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
    if err != nil { return nil, err }
    if count > 0 {
        return nil, ErrNotFirstRun
    }

    // ... crear org + user + api key
    return result, tx.Commit(ctx)
}
```

### 3. TUI minimalista vs TUI pesada (charmbracelet/bubbletea)

**Decisión:** TUI minimalista con `bufio.Scanner` y `fmt.Scanln` (stdlib).

**Por qué:**
- El flow es de 4 inputs (email, código, y/N, server URL). No justifica 20MB de binario extra.
- ptools (la inspiración mencionada) tampoco usa TUI pesada — usa prompts con `bufio.Scanner` + formato ASCII simple.
- Mantenibilidad: si queremos TUI real después (charm), la hacemos como HU separada.

```go
// internal/cli/onboard/tui.go
func ask(prompt, defaultVal string) (string, error) {
    reader := bufio.NewReader(os.Stdin)
    if defaultVal != "" {
        fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, defaultVal)
    } else {
        fmt.Fprintf(os.Stderr, "%s: ", prompt)
    }
    line, err := reader.ReadString('\n')
    if err != nil { return "", err }
    return strings.TrimSpace(line), nil
}
```

### 4. Detección de opencode: solo opencode, no claude-code

**Decisión:** el comando `onboard` pregunta solo por opencode (no claude-code, no claude-desktop).

**Por qué:** opencode es el agente más usado en este proyecto (lo demuestran el config y el `bin/domain` ya habla de él). Soporte para los otros agentes ya existe via `domain setup [claude-code|claude-desktop]` — el user puede llamarlo manualmente después.

### 5. Bugfix del path detection: `os.Stat` en vez de `strings.Replace`

**Decisión:** reemplazar la heurística actual (`strings.Replace(ex, "/domain", "/domain-mcp", 1)`) con un algoritmo de búsqueda:

```go
func findDomainMCPSibling(domainPath string) (string, error) {
    dir := filepath.Dir(domainPath)
    candidate := filepath.Join(dir, "domain-mcp")
    if _, err := os.Stat(candidate); err == nil {
        return candidate, nil
    }
    return "", fmt.Errorf("domain-mcp not found next to %s; pass --mcp-binary", domainPath)
}
```

**Por qué:** el `strings.Replace` actual es frágil — falla si el binario está en otro path (e.g., `~/go/bin/domain` no matchea `~/go/bin/domain-mcp` por el patrón "/domain" en "/domain-mcp" — ejemplo: `strings.Replace("/go/bin/domain", "/domain", "/domain-mcp", 1)` da `/go/bin/domain-mcp` correcto, pero `strings.Replace("/usr/local/bin/domain", "/domain", "/domain-mcp", 1)` da `/usr/local/bin/domain-mcp` también correcto. **Funciona en paths absolutos**, pero falla en:
- Binarios con `/domain` en path (e.g., `/projects/domain/bin/domain`)
- Symlinks

`os.Stat` es robusto porque solo verifica existencia.

## API Contract

### `POST /api/v1/auth/bootstrap`

**Request:**
```json
{
  "email": "admin@saargo.com",
  "key_name": "default",
  "org_name": "Saargo"
}
```

**Response 200 (first run, success):**
```json
{
  "data": {
    "user_id": "00000000-0000-0000-0000-000000000010",
    "organization_id": "00000000-0000-0000-0000-000000000001",
    "api_key": "domk_live_...",
    "api_key_id": "...",
    "email": "admin@saargo.com",
    "method": "bootstrap"
  }
}
```

**Response 400 (not first run):**
```json
{
  "error": {
    "code": "email_not_in_any_org",
    "message": "bootstrap is first-run only; use /auth/request-otp instead"
  }
}
```

**Response 422 (invalid email):**
```json
{
  "error": {
    "code": "invalid_email",
    "message": "email format invalid"
  }
}
```

## CLI Contract

### `domain onboard [flags]`

**Flags:**
- `--base-url URL` (default: `http://localhost:8000`) — domain server URL
- `--non-interactive` — skip prompts, fail if any input missing
- `--no-opencode` — skip opencode config step
- `--email EMAIL` (non-interactive mode) — pre-fill email
- `--key-name NAME` (default: "default") — name for the API key

**Interactive flow (default):**
1. Detect first-run by hitting `GET /api/v1/auth/first-run` (new endpoint or check via existing)
2. Prompt: "Server URL [http://localhost:8000]:"
3. If first run: prompt "Your email:"
4. Else: prompt "Your email:" (uses OTP flow)
5. If first run: POST `/auth/bootstrap`, get API key
6. Else: POST `/auth/request-otp`, prompt "Enter 6-digit code:", POST `/auth/verify-otp`
7. Save API key to `~/.config/domain/credentials.json` (chmod 600)
8. Prompt: "Configure opencode MCP server? [Y/n]:"
9. If Y: invoke `domain setup opencode --api-key KEY --base-url URL`
10. Print summary, exit 0

**Output example:**
```
Domain Onboard Wizard

Detecting first-run... yes (DB is empty)

? Server URL [http://localhost:8000]:
? Your email: admin@saargo.com

→ Bootstrapping organization "Saargo" + user admin@saargo.com...
✓ API key: domk_live_SYvmJXHRIUrLey44w9kWzrcg06E-u8fS8mAtIpFbBIo
  (saved to /home/user/.config/domain/credentials.json, mode 0600)

? Configure opencode MCP server? [Y/n]: Y
→ Adding domain MCP server to opencode.json...
✓ Done! opencode will discover 58 domain tools on next start.

Try it:
  export $(cat /home/user/.config/domain/credentials.json | jq -r '.api_key')
  opencode
  Then: "list the platform policies"
```

## Slash Command Contract

### `/domain-login` (opencode)

**Archivo:** `~/.config/opencode/commands/domain-login.md`

**Frontmatter:**
```yaml
---
description: Trigger the Domain onboarding wizard (login or register)
agent: build
---
```

**Body (instrucciones para el agente):**
```markdown
# /domain-login

The user wants to authenticate with the Domain MCP server.

## Steps

1. **Run the onboard wizard** in non-blocking mode:
   ```bash
   echo "" | /path/to/domain onboard --no-opencode --base-url "${DOMAIN_BASE_URL:-http://localhost:8000}"
   ```
   The wizard will:
   - Prompt for the user's email (if needed)
   - Send a 6-digit code to that email
   - Prompt the user to enter the code
   - Save the API key to `~/.config/domain/credentials.json`

2. **Re-configure opencode** to use the new key:
   ```bash
   /path/to/domain setup opencode --api-key "$(jq -r '.api_key' ~/.config/domain/credentials.json)" --base-url "${DOMAIN_BASE_URL:-http://localhost:8000}"
   ```

3. **Confirm to the user**:
   - Print only "✓ Logged in to Domain. You can now use domain_mem_save, domain_policy_list, etc."
   - **DO NOT** print the API key in the chat (it would be logged in the agent's history).

## Security note

The API key is sensitive. Never echo it in chat, never write it to files
that the agent could read, never include it in tool call results that
the user could accidentally copy. The user sees it once during the wizard
on their terminal; that's the only place it should appear.
```

**Comportamiento esperado:**
- User escribe `/domain-login` en opencode
- El agente reconoce el slash command (lee el .md)
- El agente EJECUTA `domain onboard` via Bash tool
- El binario muestra prompts al user (vía stderr)
- El user responde (input a stdin)
- El binario completa el flow
- El agente solo imprime "✓ Logged in" al user (sin la key)

### `/domain-login` (claude-code)

**Archivo:** `.claude/commands/domain-login.md` (project-scope) o `~/.claude/commands/domain-login.md` (user-scope)

Mismo contenido que opencode pero adaptado a la convención de claude-code (mismo formato .md).

## Enforcement de API key

### Gate al boot del MCP server

`cmd/domain-mcp/main.go` YA TIENE la validación (exit 1 si no hay key). Mejora pendiente:

```go
apiKey := os.Getenv("DOMAIN_API_KEY")
if apiKey == "" {
    fmt.Fprintln(os.Stderr, "domain-mcp: API key missing. Run /domain-login to authenticate.")
    os.Exit(1)
}

// ... resolve principal ...

if err != nil {
    fmt.Fprintln(os.Stderr, "domain-mcp: API key inválida o revocada. Run /domain-login to re-authenticate.")
    os.Exit(1)
}
```

**Comportamiento esperado:**
- opencode invoca el MCP server al primer tool call
- El binario valida la key al boot (NO en cada request, es costoso)
- Si la key es inválida → exit 1 → opencode muestra error al user
- El user ejecuta `/domain-login` → re-autentica → exit 0 → tools funcionan

### NO auto-revoked en el server (importante)

**Decisión consciente:** el server domain NO revoca la API key si la conexión del MCP server es "rara" o "cambia de IP". Razón: el agente podría reiniciarse, el user podría cambiar de red, etc. La API key es válida hasta que el user la revoque explícitamente via `domain keys revoke` (o via API).

## Comportamiento de expiración de API key

**Confirmado:** la columna `api_keys.expires_at` es nullable. Sin valor = no expira.

**Por qué:** el producto es multi-tenant y self-hosted. El user es responsable de la rotación. No queremos forzar re-login cada 30 días porque sería fricción sin seguridad real (un atacante con la key tendría 30 días de todos modos).

**Mecanismo de rotación:**
- Manual via API: `POST /api/v1/api-keys` (nueva) + `DELETE /api/v1/api-keys/{id}` (revoke)
- O via CLI: `domain keys ls`, `domain keys revoke`, `domain keys regenerate` (futuro, HU separada)
- O editando la DB directamente: `UPDATE api_keys SET revoked_at = NOW() WHERE id = '...'`

**Tradeoff:** "no expira" es lo que el user pidió. Si en el futuro quieren rotación automática, abrir HU separada (HU-02.1.1?).

## TDD detallado

1. `internal/auth/bootstrap/service_test.go`:
   - `TestBootstrap_FirstRun_CreatesOrgUserKey` — DB vacía, todo OK
   - `TestBootstrap_NotFirstRun_Rejected` — DB con 1 user, retorna `ErrNotFirstRun`
   - `TestBootstrap_InvalidEmail_Rejected` — "not-an-email" rechazado
   - `TestBootstrap_RaceCondition_AdvisoryLock` — dos goroutines llaman simultáneo, solo uno crea

2. `internal/cli/onboard/tui_test.go`:
   - `TestAsk_DefaultValue` — input vacío con default
   - `TestAsk_RequiredValue` — input vacío sin default
   - `TestAskY_N_DefaultY` — input "Y" / "y" / "" → true; "n" → false

3. `internal/api/handler/auth_test.go` (nuevo):
   - `TestAuthBootstrap_FirstRun` — POST retorna 200 con key
   - `TestAuthBootstrap_NotFirstRun` — POST retorna 400

4. `cmd/domain/main_test.go` (o integration test del CLI):
   - `TestOnboard_NonInteractive_Success` — flujo completo con stdin mockeado

## Sabotajes / canarys

- **Race condition canary:** el test de 2 goroutines simultáneas verifica que advisory lock funciona. Si alguien refactorea y mueve el lock afuera, el test rompe.
- **Path detection canary:** test de `findDomainMCPSibling` con paths absolutos y relativos, symlinks, archivos inexistentes.
- **Email validation canary:** test con emails malformados ("@", "no-at-sign", "two@@at").
- **Credentials file permissions canary:** test verifica que el archivo se crea con mode 0600 (no 0644 u otros).
