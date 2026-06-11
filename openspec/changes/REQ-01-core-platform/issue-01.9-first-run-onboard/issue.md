# issue-01.9-first-run-onboard

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** tooling

## Historia de usuario

**Como** desarrollador que hace `docker compose up && ./bin/domain server` por primera vez
**Quiero** tener 3 formas de arrancar el sistema:
  1. `./bin/domain onboard` (comando standalone)
  2. `/domain-login` (slash command dentro de opencode/claude-code)
  3. Forzado al instalar la herramienta: si el MCP server está registrado, el agente no puede usar ninguna tool sin estar logueado
**Para** no tener que conectarme a la DB a mano, no saltarme el login, y poder usar la herramienta sin fricción una vez configurada

## Contexto

Problema triple:

1. **Fresh install inutilizable:** los seeders (HU-01.7) crean 33 rows de catalog data (plans, model_registry, platform_policies, project_templates) pero **NO crean org, user, ni api_key**. Sin user, no se puede hacer login OTP. Sin API key, no se puede usar el CLI/MCP. El primer usuario DEBE hacer:
   - `psql ... INSERT INTO organizations (...)` (a mano)
   - `psql ... INSERT INTO users (...)` (a mano)
   - `curl POST /api/v1/auth/request-otp` + Mailpit + `verify-otp` (manualmente)

2. **Sin enforcement de auth en el agente:** si el user registra el MCP server en opencode pero nunca se loguea, el server arranca con `os.Exit(1)` (no tiene API key), pero el agente igual lo intenta usar. Necesitamos: **si la herramienta está instalada → forzar login al primer uso**.

3. **Sin entry point en el agente:** si el agente está hablando con el user y el token expira / el user quiere loguearse, no hay forma de disparar el wizard desde dentro del chat. Necesitamos un **slash command `/domain-login`** que el agente mismo puede invocar.

4. **Expiración de API key:** la `api_keys.expires_at` es nullable (sin DEFAULT), por lo que la key **NO expira automáticamente**. Esto es por diseño — el user controla cuándo la revoca via `revoked_at`. Decisión confirmada: **dura hasta que el user la cambie/revoque manualmente**.

## Criterios de aceptación

### Escenario 1: Fresh install + `domain onboard` (CLI standalone)

```gherkin
Dado que la DB no tiene users (COUNT(users) = 0)
Y el server domain está corriendo en http://localhost:8000
Cuando ejecuto `./bin/domain onboard`
Entonces el comando me pide el email via prompt
Y luego llama POST /api/v1/auth/bootstrap con ese email
Y el server crea la primera org + user + api key automáticamente
Y el comando imprime la API key en pantalla
Y guarda la API key en ~/.config/domain/credentials.json (chmod 600)
Y me pregunta si quiero configurar opencode MCP server
Y si respondo Y, llama domain setup opencode con la API key
Y termina con exit code 0
```

### Escenario 2: Ya hay users → usa OTP normal

```gherkin
Dado que la DB tiene al menos 1 user
Cuando ejecuto `./bin/domain onboard`
Entonces el comando detecta "ya hay users" y NO intenta bootstrap
Y me pide el email via prompt
Y llama POST /api/v1/auth/request-otp
Y me pide el código de 6 dígitos via prompt
Y llama POST /api/v1/auth/verify-otp
Y el server emite una nueva API key
Y el resto del flujo es igual al Escenario 1
```

### Escenario 3: Slash command `/domain-login` desde opencode

```gherkin
Dado que opencode está corriendo y tiene el MCP server "domain" registrado
Y el user NO tiene credenciales en ~/.config/domain/credentials.json
O las credenciales son inválidas
Cuando el user escribe "/domain-login" en el chat de opencode
Entonces el agente reconoce el slash command (definido en .opencode/commands/domain-login.md)
Y el agente EJECUTA el binario domain onboard via shell
Y el binario detecta que no hay credenciales, dispara el wizard
Y el agente muestra los prompts al user, captura las respuestas
Y al final el agente confirma "logged in, you can now use domain_mem_save, etc."
Y el MCP server está listo para usar en el siguiente tool call
```

### Escenario 4: Forzar login al instalar herramienta (CRÍTICO)

```gherkin
Dado que el user ejecutó `domain setup opencode --api-key INVALID`
Cuando el MCP server arranca (opencode lo invoca)
Entonces el binario domain-mcp valida la API key al boot
Y si es inválida: imprime "API key inválida, corré /domain-login" y exit 1
Y opencode detecta el exit 1 y muestra el error al user
Y el user NO puede llamar ninguna tool hasta loguearse correctamente
```

```gherkin
Dado que el user borró accidentalmente las credenciales después de instalar
Y opencode tiene el MCP server registrado
Cuando el agente intenta llamar domain_mem_save
Entonces el binario domain-mcp falla al boot con "API key inválida"
Y el tool call retorna error
Y el agente (via slash command /domain-login) puede re-loguear al user
```

### Escenario 5: Anti-enumeration en bootstrap

```gherkin
Dado que la DB tiene 5 users pero ninguno con email "intruso@spam.com"
Cuando llamo POST /api/v1/auth/bootstrap con ese email
Entonces el server responde 400 con error "email_not_in_any_org" (no auto-crea user)
Y el comportamiento es consistente con la regla "bootstrap es first-time-only"
```

### Escenario 6: Server no responde

```gherkin
Dado que el server domain está caído o no accesible
Cuando ejecuto `./bin/domain onboard`
Entonces el comando intenta una conexión al health endpoint
Y si falla, reporta el error con sugerencia ("¿arrancaste el server?")
Y termina con exit code 1
```

### Escenario 7: opencode no instalado

```gherkin
Dado que el user no tiene ~/.config/opencode/opencode.json
Cuando ejecuto `./bin/domain onboard` y respondo Y a "configure opencode"
Entonces el comando skip el paso opencode con un warning
Y NO falla el flujo (el user puede configurar opencode después manualmente)
Y termina con exit code 0
```

### Escenario 8: API key NO expira automáticamente

```gherkin
Dado que creé una API key el 2026-06-11
Y la columna expires_at es NULL
Cuando pasan 30 días (2026-07-11)
Entonces la API key SIGUE funcionando
Y la única forma de invalidarla es UPDATE api_keys SET revoked_at = NOW() WHERE id = ...
Y esto es por diseño: el user controla la rotación
```

## Análisis breve

- **Qué pide realmente:**
  - Endpoint `POST /api/v1/auth/bootstrap` (server-side) que auto-crea org+user+api_key si la DB está vacía
  - Endpoint `GET /api/v1/auth/first-run` (server-side) que indica si es first-run
  - Comando `domain onboard` (CLI) con TUI minimalista (readline, no charm) que orquesta el flujo
  - Slash command `/domain-login` para opencode (`.opencode/commands/domain-login.md`) y claude-code (`.claude/commands/domain-login.md`)
  - Enforcement de API key al boot del MCP server (gate en `domain-mcp` que valida antes de aceptar requests)
  - Helper de detección de opencode (chequea `~/.config/opencode/opencode.json`)
  - Bugfix del path detection de `domain-mcp` en `domain setup opencode` (heurística `strings.Replace` es frágil)
  - Documentación: `docs/quickstart.md` con el flujo paso a paso

- **Módulos sospechados:**
  - `internal/api/handler/auth.go` — agregar handler `bootstrap` y `first-run`
  - `internal/auth/bootstrap/` (nuevo) — service con la lógica de first-run detection
  - `cmd/domain/main.go` — nuevo subcommand `onboard`
  - `internal/cli/onboard/` (nuevo) — TUI wizard
  - `cmd/domain-mcp/main.go` — ya tiene exit(1) si no hay key, mejorar mensaje
  - `internal/cli/setup/targets.go` — bugfix path detection
  - `.opencode/commands/domain-login.md` (nuevo) — slash command para opencode
  - `.claude/commands/domain-login.md` (nuevo) — slash command para claude-code
  - `internal/cli/setup/setup.go` (o nuevo archivo) — instalar los slash commands

- **Riesgos:**
  - **Security:** endpoint bootstrap NO debe permitir crear users en cualquier momento. Solo si la DB está vacía. Después de eso, OTP normal. Defense in depth: validar `COUNT(users) == 0` con `pg_advisory_xact_lock`.
  - **Race conditions:** dos onboard simultáneos podrían crear dos users. Fix: advisory lock.
  - **Slash command malicioso:** si un agente externo puede invocar `/domain-login`, podría intentar forzar login con email controlado. Defense: el slash command SOLO muestra el binario al user, el user lo ejecuta manualmente via `Bash` tool con su consentimiento. El agente NO captura la API key ni la exfiltra.
  - **API key leak via slash command:** el agente podría ver la API key en el output del binario. Decisión: el binario imprime la key al user, el agente la ignora (solo dice "logged in, ready"). **Documentar este riesgo en el slash command.**
  - **TUI input inválido:** email malformado, código no-numérico. Validación defensiva.
  - **Multi-org:** el primer user pertenece a "Saargo" (derivado del email domain). Decisión: usar el email domain sanitized como org name.
  - **Expiración:** confirmar que la decisión de "no expira" es la correcta (es lo que el user pidió).

- **Esfuerzo tentativo:** M-L (4-6 horas)
- **Dependencias:** HU-01.7 (seeders), HU-01.4 (project_templates), auth/apikey (HU-02.1)

## TDD plan

1. **Red:** test del endpoint `bootstrap` con `app_user` mockeado. Verifica:
   - DB vacía → user+org+key creados
   - DB con 1 user → retorna `email_not_in_any_org`
   - Race: dos bootstraps simultáneos → solo uno crea user
2. **Green:** implementación del endpoint.
3. **Refactor:** extraer lógica a `internal/auth/bootstrap/Service`.
4. **Sabotaje:** test de path traversal (email con caracteres especiales).
5. **Test E2E CLI:** `domain onboard` con stdin mockeado.
6. **Test E2E Slash:** el binario se invoca via shell, lee prompts desde stdin, escribe credenciales.
7. **Test enforcement:** `domain-mcp` con API key inválida retorna exit 1 con mensaje claro.
8. **Test persistencia:** API key sin `expires_at` sigue funcionando después de N días (simulado).

## Out of scope (futuro)

- Soporte para registro de SSO (Google, GitHub) — futuro, requiere OAuth.
- Selección interactiva del agent (opencode vs claude-code vs claude-desktop). Por ahora solo pregunta por opencode.
- Verificación del email via link en lugar de código. Requiere setup de email real.
- Auto-rotación de API keys. Por ahora es manual (issue-02.1 cubre revoke + regenerate).
- TUI pesada con charmbracelet — si el flujo crece a 10+ inputs (futuro).
