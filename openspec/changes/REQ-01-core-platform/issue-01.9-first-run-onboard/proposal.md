# Proposal: issue-01.9-first-run-onboard

## Intención

Hacer el producto utilizable end-to-end con un solo comando (`./bin/domain onboard`). Cierra el loop entre seeders, auth, y setup del cliente MCP.

## Scope

**Incluye:**
- Endpoint `POST /api/v1/auth/bootstrap` (server-side) que auto-crea org+user+api_key si la DB está vacía
- Endpoint `GET /api/v1/auth/first-run` (server-side) que indica si es first-run
- Comando `domain onboard` (CLI) con TUI minimalista (readline)
- Detección de opencode y patch del config
- Bugfix del path detection de `domain-mcp` en `domain setup opencode`
- Tests de comportamiento + E2E
- Docs en `docs/quickstart.md`

**No incluye:**
- TUI pesada con charmbracelet/bubbletea (futuro, HU separada si querés)
- Soporte para claude-code/claude-desktop en `onboard` (ya existe `domain setup [agent]`)
- SSO (Google, GitHub) — futuro
- Email verification con link en lugar de código — futuro

## Enfoque técnico

1. **Endpoint separado** `/auth/bootstrap` (no reusar `/auth/request-otp`). Defense: bootstrap es first-run-only, OTP es login normal.
2. **Race protection** con `pg_advisory_xact_lock` constante (`BOOTSTRAP_LOCK_KEY`).
3. **Org name derivation:** usar el email domain (parte después de `@`) sanitized. Fallback: "Default Org".
4. **TUI minimalista** con `bufio.Scanner` y `fmt.Scanln`. Sin deps externas.
5. **Credentials storage** en `~/.config/domain/credentials.json` con mode 0600.
6. **Opencode detection** via `os.Stat("~/.config/opencode/opencode.json")`.
7. **Path detection fix** via `os.Stat` en vez de `strings.Replace`.

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Race: dos onboard simultáneos | `pg_advisory_xact_lock(BOOTSTRAP_LOCK_KEY)` |
| Security: cualquiera auto-crea user | Endpoint SOLO funciona si `COUNT(users) == 0`. Después, retorna `email_not_in_any_org`. |
| TUI input inválido (email malformado, código no-numérico) | Validación en el endpoint + loop con re-prompt en el CLI |
| Email domain weird (e.g., `+tag@sub.example.co.uk`) | Sanitización: lowercase, alphanum + dash, max 50 chars |
| Path detection rota (e.g., symlink, /domain en dir name) | `os.Stat` en vez de `strings.Replace` |
| API key leak via shell history | CLI limpia el buffer después de leer (no es perfecto, pero defense in depth) |
| Creds file mode 0644 (leíble por otros) | `os.OpenFile` con mode 0600 explícito |
| **Slash command malicioso** (agente externo fuerza login) | El slash command SOLO invoca el binario, NO captura la API key. El user la ve directo, el agente solo confirma "logged in". Documentado en el .md. |
| **API key leak via agente** (output del binario visible en chat) | Decisión: el binario imprime al stderr (no stdout), el agente lee solo el "logged in" message. Documentado en el slash command. |
| **MCP server arranca sin auth y acepta tool calls** | `domain-mcp main` ya hace `os.Exit(1)` si la key es inválida. El agente ve el exit 1 y NO puede llamar tools. |
| **Expiración de API key inesperada** | Confirmado: `expires_at` nullable, NO expira automáticamente. El user controla la rotación. Documentado. |

## Testing

- **Unit (bootstrap service):** first-run, not-first-run, invalid email, race
- **Unit (TUI):** ask with default, ask required, y/N
- **Unit (auth handler):** POST /auth/bootstrap happy + sad paths
- **Integration (E2E CLI):** flujo completo con stdin mockeado
- **Manual:** correr el flujo real, verificar que opencode descubre 58 tools

## Rollback plan

Si el flujo tiene bugs que rompen onboarding:
1. Revert del PR (un solo commit)
2. Los seeders (HU-01.7) siguen funcionando, así que el fresh install sigue corriendo
3. El user puede volver al flujo manual (psql + curl + manual setup)

El endpoint `/auth/bootstrap` es NUEVO, no rompe nada existente. La CLI es NUEVA. El bugfix del path detection es backward-compatible.

## Out of scope (futuro)

- TUI con charmbracelet — si el flujo crece a 10+ inputs
- Onboarding de múltiples agentes (claude-code, claude-desktop) en el mismo comando
- Pre-flight checks (¿está el server corriendo? ¿hay espacio en disco?)
- Importación de tokens existentes desde otros productos
- Modo "silent" que lee todo del env sin prompts
