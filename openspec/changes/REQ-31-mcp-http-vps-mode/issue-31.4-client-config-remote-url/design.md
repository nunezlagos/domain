# Design: issue-31.4-client-config-remote-url

## Contexto

REQ-31 introduce el modo remoto (HTTPS + Bearer). Para que un
cliente (developer local) apunte al VPS, hoy tiene que editar
manualmente `~/.config/domain/env` y setear `DOMAIN_REMOTE_URL` +
remover `DOMAIN_DATABASE_URL`. Eso es propenso a errores y no
descubrible (¿cómo sabe el user que existe esa env var?).

El flag `--remote-url` en `domain install` automatiza esto. Es
similar a `gh auth login` (configura el cliente con el server) o
`aws configure` (escribe `~/.aws/credentials`).

## Decisión arquitectónica

**Estrategia:** flag nuevo en `parseInstallFlags` + branch
condicional en `runInstall` que skipea pasos de infra local +
helper que UPSERT del env.

1. **Flag parsing** (`cmd/domain/install_cli.go:339`):
   ```go
   case "--remote-url":
       if i+1 >= len(args) { return f, errors.New("missing value for --remote-url") }
       f.remoteURL = args[i+1]
       i++
   case "--api-key":
       if i+1 >= len(args) { return f, errors.New("missing value for --api-key") }
       f.apiKey = args[i+1]
       i++
   ```

2. **Validación al inicio de `runInstall`:**
   - Scheme: `https://` o `http://localhost*` (este último solo
     para dev). Acepta `http://` solo si hostname es `localhost` o
     `127.0.0.1`.
   - Quitar path: si la URL tiene path, normalizar a scheme +
     host + port.

3. **Modo de ejecución:** si `flags.remoteURL != ""`, el install
   corre en `modeRemote`. Esto cambia:
   - Step 4 (start services): skip.
   - Step 5 (migrate): skip.
   - Step 6 (seed): skip.
   - Step 7 (global MCP env): escribe SOLO `DOMAIN_REMOTE_URL` +
     `DOMAIN_BASE_URL` (sin DSN).
   - Step 8 (systemd): skip.
   - Step 9 (API key): si `flags.apiKey != ""`, skip todo el flujo
     de first-run/OTP, solo persiste la key.
   - Step 10 (configure agents): ejecuta normal.
   - Step 11 (init): skip con warning "init requires server access;
     run `domain init` manually after first remote call".

4. **UPSERT del env (`~/.config/domain/env`):**
   - Helper `writeRemoteEnvFile(path, remoteURL, baseURL string)
     error`:
     1. Lee el env file actual (si existe).
     2. Parsea línea por línea.
     3. Remueve `DOMAIN_DATABASE_URL`, `DOMAIN_DATABASE_AUTH_URL`,
        `DOMAIN_HTTP_PORT` (todas locales).
     4. Appende `DOMAIN_REMOTE_URL=<url>` y
        `DOMAIN_BASE_URL=<url>`.
     5. Escribe con chmod 600.
   - Si el archivo tenía valores que el user quiere preservar
     (e.g. `DOMAIN_OTEL_ENDPOINT`), se mantienen.

5. **`--api-key` flow:** si está seteado, el install NO llama a
   `ensureAPIKey` (que es el flujo first-run/OTP). En su lugar,
   persiste la key directamente:
   ```go
   creds := &onboard.Credentials{
       APIKey:  flags.apiKey,
       BaseURL: flags.baseURL,
       IssuedAt: time.Now().UTC(),
   }
   persistCredentials(creds)
   ```
   Y opcionalmente valida online con un best-effort
   `GET /auth/first-run` (3s timeout) para confirmar. Si falla por
   red, warning pero exit 0 (la key puede ser válida, solo que la
   red está flaky).

6. **Idempotencia:** al inicio, leer el env file. Si
   `DOMAIN_REMOTE_URL` ya coincide con el flag, log "already
   configured, skip" + skip todo el bloque de UPSERT.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Comando separado `domain remote-setup` (no en `install`) | Menos descubrible. Install es el lugar natural. |
| B | Auto-detect de "debería ser remoto" (e.g. si no hay docker) | Mágico y propenso a errores. Explícito es mejor. |
| C | `domain install` siempre pregunta "local o remote" | Hoy pregunta mode (local/cloud/hybrid). Remote es un 4to modo. Decisión: lo hacemos flag, no otro mode, para no romper scripts. |
| D | Tomar la URL del `opencode.json` del repo | El user puede no tener opencode.json en el repo. Flag es universal. |

## Por qué flag --remote-url gana

- **Consistente con otros flags:** `--mode`, `--base-url`,
  `--dsn`, `--email`. Mismo patrón.
- **Explícito:** cero ambigüedad. El user sabe qué URL va a usar.
- **Skip de pasos automático:** el install detecta el modo y
  skipea lo que no aplica.
- **Combinable:** `--remote-url + --api-key --non-interactive` es
  el flujo de un solo comando para configurar un cliente nuevo.

## Detalle de implementación

- `installFlags` agrega `remoteURL string` y `apiKey string`.
- `runInstall` envuelve cada step en `if flags.remoteURL == ""
  { ... }`. Sin flag, comportamiento actual.
- `writeRemoteEnvFile` en nuevo archivo
  `internal/cli/install/remote_env.go`.
- Comando standalone `domain setup remote [--url URL]` que solo
  configura el env (sin todo el flujo de install). Útil para el
  user que ya tiene `domain-mcp` instalado y solo quiere cambiar
  el server apuntado.

## Riesgos

- **R1:** El user quiere volver a modo local después de haber
  corrido `--remote-url`. **Mitigación:** `domain install
  --mode local` (sin `--remote-url`) configura el modo local de
  vuelta, removiendo `DOMAIN_REMOTE_URL` y restaurando
  `DOMAIN_DATABASE_URL` si el .env.example lo tiene.
- **R2:** Validación online de la API key puede fallar por red
  (VPS down, DNS issue). **Mitigación:** best-effort, warning
  pero no fatal. El user puede validar después con
  `domain projects ls`.
- **R3:** Un script que llama a `domain install` con
  `--remote-url` puede capturar la API key en logs (env vars en
  process list). **Aceptable:** es la convención Unix para
  secrets. Documentar en `--help` que la key puede aparecer en
  `ps auxef`.

## Sabotaje test (referencia)

Comentar la rama que REMUEVE `DOMAIN_DATABASE_URL` del env file
(sabotaje: deja DSN coexistente) → test que assserta
"DOMAIN_DATABASE_URL ausente en modo remoto" DEBE FALLAR →
restaurar lógica de UPSERT → test verde.
