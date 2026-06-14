# issue-01.10-deploy-modes-update

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** tooling + feature

## Historia de usuario

**Como** usuario que quiere instalar Domain por primera vez (o actualizar)
**Quiero** un wizard que:
1. Pregunte el modo de deployment (local / cloud / hybrid) y actúe en consecuencia
2. Detecte si ya está instalado y NO destruya nada (ni .md, ni flows, ni configs)
3. Haga backups automáticos antes de cualquier mutación
4. Maneje updates con seeders idempotentes (skip lo que ya existe)
5. Configure el agente IA para priorizar el MCP sobre los .md locales (silently)

**Para** que la instalación sea reproducible, segura, y que el agente siempre use Domain como fuente de verdad sin que el user tenga que mantenerlo manualmente

## Contexto

Problemas a resolver (todos detectados durante tests funcionales previos):

1. **No hay selector de deployment mode:** un fresh install asume
   "local con docker compose". Pero el user puede querer cloud
   (Postgres managed: RDS, Neon, Supabase) o hybrid (algunos servicios
   selfhosted, otros en cloud). Hoy hay que editar `.env` a mano.

2. **`domain init` ya existe pero no se integra con `onboard`:** un user
   que corre `domain onboard` NO obtiene la conversión de sus .md
   automáticamente. Hay que correr 2 comandos separados.

3. **El agente prioriza el .md stub sobre el MCP:** el stub generado
   por `init` dice "usá domain MCP" pero el agente puede leerlo y
   hacer lo contrario (los LLMs son text completion, no ejecutan
   instrucciones literalmente). Necesitamos un mecanismo más fuerte.

4. **No hay `domain update`:** updates requieren correr migrations +
   seeders + init manualmente. Riesgo de olvidar un paso.

5. **No hay backups automáticos:** un update puede romper configs
   del user (mcp.json, AGENTS.md). El user tiene que acordarse de
   hacer backup antes.

6. **Seeders no son idempotentes o no se sabe:** algunos seeders
   skipean, otros crean. No hay un solo comando "aplicar todo lo
   nuevo sin romper lo viejo".

## Criterios de aceptación

### Escenario 1: Fresh install + `local` mode

```gherkin
Dado que NO hay credenciales de Domain en ~/.config/domain/
Y Docker está disponible
Y el user corre `domain install` (o `domain onboard` con el nuevo flag --mode=local)
Cuando el wizard se ejecuta
Entonces pregunta: "Deployment mode? [local/cloud/hybrid]" → user elige local
Y arranca docker compose up -d (Postgres + MinIO + Mailpit)
Y espera a que los servicios reporten healthy (max 90s)
Y detecta first-run, dispara bootstrap endpoint
Y crea API key, la guarda en ~/.config/domain/credentials.json (chmod 600)
Y corre seeders (skip los que ya existen)
Y corre `domain init` para archivar .md a BD y crear stubs
Y configura opencode (si está instalado): patchea opencode.json + instala /domain-login
Y Inyecta prioridad MCP en AGENTS.md (raíz del proyecto)
Y termina con exit 0 y un resumen
```

### Escenario 2: Fresh install + `cloud` mode

```gherkin
Dado que NO hay credenciales de Domain
Y el user corre `domain install --mode=cloud`
Cuando el wizard se ejecuta
Entonces pregunta: "Database URL? (postgres://user:pass@host:5432/db?sslmode=disable)"
Y el user pega la URL (de Neon, RDS, Supabase, etc)
Y el binario valida la conexión (ping)
Y si falla: "Connection failed, check your DSN" + retry
Y si OK: escribe .env con DOMAIN_DATABASE_URL
Y NO arranca Docker
Y continua con bootstrap + init + configure agent
```

### Escenario 3: Fresh install + `hybrid` mode (selfhosted + cloud)

```gherkin
Dado que NO hay credenciales
Y el user corre `domain install --mode=hybrid`
Cuando el wizard se ejecuta
Entonces pregunta por cada servicio: [local/cloud/none]
  - PostgreSQL: [default local]
  - S3 storage: [default cloud (MinIO local) o R2/S3]
  - SMTP: [default local (Mailpit) o Sendgrid/Resend]
Y segun las respuestas, decide que arranca (docker) y que no
Y al final: "Postgres: local docker, S3: cloud, SMTP: local docker. OK?"
Y si confirm: arranca lo necesario, escribe .env con todo
```

### Escenario 4: Ya está instalado (idempotente)

```gherkin
Dado que el user ya corrió `domain install` antes (existe ~/.config/domain/credentials.json)
Y la DB tiene al menos 1 user
Y el user corre `domain install` de nuevo
Cuando el wizard se ejecuta
Entonces detecta "ya está instalado" via GET /auth/first-run → user_count > 0
Y NO corre bootstrap (sería ErrNotFirstRun)
Y NO elimina credenciales existentes
Y NO reemplaza .md (init detecta content_hash, skip)
Y NO corre seeders que ya corrieron
Y ofrece: [re-bootstrap / update / configure-agent / exit]
Y por default: solo verifica que todo siga funcionando y exit
```

### Escenario 5: Update (`domain update`)

```gherbin
Dado que Domain está instalado y corriendo
Y hay una nueva versión de los binarios
Y el user corre `domain update`
Cuando el comando se ejecuta
Entonces crea un backup de:
  - ~/.config/domain/credentials.json → credentials.json.bak.<timestamp>
  - .env → .env.bak.<timestamp>
  - opencode.json → opencode.json.bak.<timestamp>
  - AGENTS.md → AGENTS.md.bak.<timestamp> (si fue tocado por domain)
  - Cualquier .md que init haya tocado → *.md.bak.<timestamp>
Y corre migrations up
Y corre seeders (skip lo existente)
Y corre init en modo --no-stub (solo verifica que backups están en BD)
Y verifica que el server arranca OK
Y termina con: "Updated. Backups: 4 files. Server: OK."
```

### Escenario 6: Agent prioriza MCP sobre .md (silent priority)

```gherkin
Dado que init archivó CLAUDE.md y creo el stub
Y el stub dice "use domain MCP"
Cuando el agente (opencode) arranca
Entonces lee AGENTS.md PRIMERO (convención de opencode)
Y AGENTS.md contiene una linea inyectada por domain:
  "When .md files contain 'use domain MCP' or 'domain init' markers,
   the agent must call the domain MCP server for those queries."
Y el agente prioriza el MCP server sobre el .md stub
Y el .md stub sigue existiendo (no se elimina) pero es ignorado en practice
```

### Escenario 7: No destruction on update

```gherkin
Dado que el user tiene 5 .md custom en su proyecto (que NO son de IA)
Y el user corre `domain update`
Cuando el comando se ejecuta
Entonces init corre pero SOLO detecta los patterns conocidos (CLAUDE.md, etc)
Y NO toca los .md custom (no son del scope de init)
Y el .env del user se preserva con backup
Y los flows custom del user NO se modifican (viven en la DB, no en disco)
Y el comando termina con: "5 custom .md preserved, 0 changes"
```

### Escenario 8: Backup restoration

```gherkin
Dado que el user tiene un backup de credentials.json (creado por un update anterior)
Y la key actual fue revocada o se perdió
Cuando el user corre `domain restore ~/.config/domain/credentials.json.bak.20260611T120000Z`
Entonces el binario lee el backup y lo copia a la ubicación original
Y verifica que la key es válida (ping a /auth/first-run con la key)
Y si OK: "Restored from backup. Key validated."
Y si falla: "Backup key is invalid/revoked. Need new onboarding."
```

## Análisis breve

- **Qué pide realmente:**
  - Wizard con selector de deployment mode (local / cloud / hybrid)
  - Idempotencia total: `domain install` puede correr N veces sin romper nada
  - Updates seguros con backups automáticos
  - Prioridad MCP silenciosa via AGENTS.md injection
  - `domain update` y `domain restore` como comandos nuevos
  - Seeders idempotentes (skip-by-hash)
  - Integración de `onboard` + `init` en un solo flow

- **Módulos sospechados:**
  - `cmd/domain/main.go`: nuevos subcommands `install`, `update`, `restore`
  - `internal/cli/onboard/wizard.go`: extender con deployment mode + init + AGENTS.md injection
  - `internal/cli/onboard/deployment.go` (nuevo): helpers para docker compose / DSN / hybrid
  - `internal/cli/onboard/backup.go` (nuevo): helpers para backup y restore
  - `internal/service/workflowimport/service.go`: agregar `UpdateOnly` mode (no toca disco, solo verifica BD)
  - `internal/seeds/seeds.go`: ya idempotente (skip-by-hash); falta exponer comando "seed all"
  - `internal/cli/setup/targets.go`: ya tiene InstallSlashCommand; falta InstallMCPriority

- **Riesgos:**
  - **docker compose no disponible** (cloud mode): el binario detecta y skip el paso
  - **DSN inválido** (cloud mode): el binario valida antes de continuar; el user puede retry
  - **Backup falla por permisos** (e.g., dir read-only): el comando aborta con mensaje claro
  - **Init se ejecuta 2 veces accidentalmente**: debe ser idempotente (skip-by-hash). Ya lo es.
  - **AGENTS.md injection es invasivo**: el user podría revertirlo si no le gusta. Plan: agregar un marker `<!-- domain-managed -->` para que `domain restore` lo identifique.
  - **Cloud Postgres requiere TLS**: el binario NO debe aceptar `sslmode=disable` en cloud mode. Validation: `sslmode != disable` si la URL contiene `rds.amazonaws.com`, `neon.tech`, `supabase.co`.
  - **Race en init concurrente**: si el user corre `domain install` en 2 terminales simultáneos, los 2 hacen init. Solution: advisory lock o solo skip-by-hash (ya idempotente).

- **Esfuerzo tentativo:** L (5-7 horas)
- **Dependencias:** HU-01.9 (onboard wizard), HU-12.7 (init stub), seeds HU-01.7

## TDD plan

1. **Red:** test del selector de deployment mode con tabla de decisión
2. **Red:** test de idempotencia: correr install 2 veces, segunda vez skip
3. **Red:** test de backups: `domain update` crea los 4 archivos .bak
4. **Red:** test de AGENTS.md injection: marker presente, restore lo detecta
5. **Red:** test de DSN validation: sslmode=disable rechazado en cloud mode
6. **Green:** implementación
7. **Sabotaje:** test de race condition (2 installs simultáneos via test goroutines)
8. **Test E2E:** integration test con docker (levanta stack, corre install, valida)

## Out of scope (futuro)

- TUI con charmbracelet/bubbletea (si el flow crece > 10 inputs)
- Self-update de binarios (descargar nueva version via curl)
- Rollback completo a una versión anterior (requiere snapshot del DB)
- Cloud-managed domain (SaaS) — distinto de cloud-self-hosted
- SSO/SAML para enterprise
