# Design: issue-01.10-deploy-modes-update

## Arquitectura

```
┌────────────────────────────────────────────────────────────┐
│  domain install [flags]                                    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 1. Detección de estado actual                        │   │
│  │    - ~/.config/domain/credentials.json existe?        │   │
│  │    - DB tiene users? (GET /auth/first-run)           │   │
│  │    - .env tiene DOMAIN_DATABASE_URL?                  │   │
│  │    - docker compose ps (servicios up?)               │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 2. Selección de deployment mode                      │   │
│  │    - local: docker compose up -d (Postgres+S3+SMTP) │   │
│  │    - cloud: pide DSN, valida, escribe .env          │   │
│  │    - hybrid: por servicio, [local/cloud/none]        │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 3. Backups automáticos (SIEMPRE, antes de mutación) │   │
│  │    - credentials.json → *.bak.<RFC3339>              │   │
│  │    - .env → *.bak.<RFC3339>                          │   │
│  │    - opencode.json → *.bak.<RFC3339>                 │   │
│  │    - AGENTS.md → *.bak.<RFC3339> (si domain-managed) │   │
│  │    - .md stubs → *.bak.<RFC3339> (si domain-managed) │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 4. Migrate up + Seeders (idempotentes)                │   │
│  │    - pg_advisory_lock durante seed run                │   │
│  │    - skip-by-hash en cada seeder                     │   │
│  │    - si primer run: bootstrap endpoint                │   │
│  │    - si ya hay users: skip bootstrap (use OTP)        │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 5. Init: archivar .md a BD (idempotente)             │   │
│  │    - skip-by-hash (ya existe en BD)                  │   │
│  │    - crear stubs (si writeStub=true)                  │   │
│  │    - NO eliminar archivos del user                    │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 6. Configure agent:                                  │   │
│  │    - opencode.json: patch (idempotente)              │   │
│  │    - .opencode/commands/domain-login.md: install     │   │
│  │    - AGENTS.md: inject priority marker (idempotente)  │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 7. Resumen + exit 0                                  │   │
│  │    "✓ Install complete. Mode: local. Backups: 3.     │   │
│  │     Seeders: 33 (4 ran, 29 skipped). Init: 14 .md.   │   │
│  │     OpenCode: configured. MCP: 58 tools."             │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────┘

       ↓

┌────────────────────────────────────────────────────────────┐
│  domain update [flags]                                      │
│  (separado de install: solo para upgrades)                  │
│                                                             │
│  - Backups (mismos 5 archivos)                              │
│  - Migrate up                                               │
│  - Seeders (skip-by-hash, idempotente)                      │
│  - Init --no-stub (solo verifica que BD está sincronizado)   │
│  - No toca opencode (ya configurado)                       │
│  - Resumen: "Updated. Backups: N. Migrations: M.          │
│              Seeders: ran/skipped. Init: synced."           │
└────────────────────────────────────────────────────────────┘

       ↓

┌────────────────────────────────────────────────────────────┐
│  domain restore <backup-path>                              │
│  (one-shot: restaura un archivo de un backup)                │
│                                                             │
│  - Lee el backup                                            │
│  - Valida que el formato es esperado (JSON / .md / etc)     │
│  - Escribe sobre el archivo actual                           │
│  - Para credentials.json: valida que la key funciona         │
│    (ping a /auth/first-run con la key)                      │
│  - Resumen: "Restored <path>. Key validated." o error      │
└────────────────────────────────────────────────────────────┘
```

## Decisiones arquitectónicas

### 1. `install` vs `onboard`: relación y diferencia

**Decisión:** `install` es el comando NUEVO que reemplaza a `onboard` con superpoderes. `onboard` queda como alias deprecated.

**Razón:** `install` cubre deployment mode + init + backups + agent config. `onboard` solo cubre first-run + credenciales. `install` es el comando correcto para un dev que empieza.

`onboard` queda por compatibilidad con scripts que ya lo usan (HU-01.9).

### 2. Backups: ¿siempre o solo con flag?

**Decisión:** backups SIEMPRE, automáticos, antes de cualquier mutación.

**Razón:** el costo de un backup es ~1ms (es un copy). El costo de un update que rompe configs del user es horas de debugging. Defense in depth: mejor un backup innecesario que un recovery doloroso.

**Override:** `--no-backup` para CI/scripts donde el user sabe lo que hace. Pero el default es siempre backup.

### 3. Init + MCP priority: marker vs reemplazo

**Decisión:** el .md stub sigue existiendo, pero AGENTS.md tiene prioridad via marker.

**Razón:** el user dijo "no eliminar .md". El stub sirve como backup visual del original (el user puede leerlo y entender qué se archivó). Pero el agente, por convención de opencode, lee AGENTS.md PRIMERO. Si AGENTS.md dice "MCP tiene prioridad", el agente obedece.

**Marker:** `<!-- domain-managed -->` en AGENTS.md. `domain restore` busca este marker para identificar qué restaurar.

### 4. Seeders: ¿idempotentes hoy o refactor?

**Decisión:** los seeders YA son idempotentes (skip-by-hash, ya verificado en auditoría previa). Solo falta exponer un comando "seed all" que los corra todos en orden.

**Razón:** el framework de seeders tiene `Registry.Sorted()` que los ordena topológicamente. Solo falta un wrapper `cmd/domain/main.go: runSeedAll()` que ejecute todos los `Env != EnvProd` y loggee counts.

### 5. docker compose: dependencia opcional

**Decisión:** el binario NO requiere docker instalado. Si `local` mode y docker no disponible → error claro con sugerencia ("install Docker Desktop o usa --mode=cloud").

**Razón:** hay usuarios que usan Domain via Docker Desktop, otros via cloud Postgres. El binario debe detectar el entorno y adaptarse.

### 6. Hybrid mode: granularidad

**Decisión:** hybrid permite elegir POR SERVICIO (Postgres / S3 / SMTP), no todo o nada.

**Razón:** un user puede querer Postgres en RDS (managed) pero S3 self-hosted (R2 es caro, MinIO es gratis). Hybrid granular es más flexible.

**Granularidad actual:** 3 servicios (Postgres, S3, SMTP). Si en el futuro hay más (Redis, etc), se agrega al TUI wizard.

## API Contract

### `domain install [flags]`

**Flags:**
- `--mode {local|cloud|hybrid}` (default: interactive prompt)
- `--base-url URL` (default: `http://localhost:8000`)
- `--non-interactive` — sin prompts, fallar si input falta
- `--no-backup` — skip backups (CI/scripts)
- `--no-init` — skip init (el user ya lo corrió antes manualmente)
- `--no-opencode` — skip opencode config
- `--dsn URL` (non-interactive cloud mode)
- `--postgres {local|cloud|none}` (hybrid mode)
- `--s3 {local|cloud|none}` (hybrid mode)
- `--smtp {local|cloud|none}` (hybrid mode)

**Interactive flow (default):**
1. Detección de estado actual (installed / fresh / partial)
2. Si fresh: prompt "Deployment mode? [local/cloud/hybrid]"
3. Si local: docker compose up -d, wait healthy
4. Si cloud: prompt DSN, validate connection
5. Si hybrid: prompts por servicio
6. Backups automáticos (loggea qué se respaldó)
7. Migrate up
8. Seeders (logs created/updated/skipped)
9. Bootstrap u OTP (igual a onboard)
10. Init (skip si ya hecho)
11. Configure agent
12. Resumen, exit 0

### `domain update [flags]`

**Flags:**
- `--no-backup` — skip backups
- `--no-seed` — skip seeders
- `--no-migrate` — skip migrations

**Flow:**
1. Backups
2. Migrate up
3. Seeders
4. Init --no-stub (solo verifica BD, no toca disco)
5. Verificar que server arranca
6. Resumen, exit 0

### `domain restore <backup-path>`

**Flow:**
1. Lee el backup
2. Valida formato
3. Escribe sobre el archivo actual
4. Si es credentials.json: valida key (ping)
5. Resumen, exit 0 o 1

## TDD detallado

1. `internal/cli/onboard/deployment_test.go`:
   - `TestDeploymentMode_Local_DockerAvailable_OK` — mockea exec.LookPath("docker"), verifica docker compose up
   - `TestDeploymentMode_Local_DockerMissing_Error` — exec.LookPath falla, error claro
   - `TestDeploymentMode_Cloud_ValidDSN_OK` — DSN parsea, ping OK
   - `TestDeploymentMode_Cloud_InvalidDSN_Error` — DSN no parsea, error
   - `TestDeploymentMode_Cloud_PlaintextDSN_Rejects` — sslmode=disable en URL de cloud, rejected
   - `TestDeploymentMode_Hybrid_PerServiceChoice` — usuario elige Postgres=cloud, S3=local, SMTP=local

2. `internal/cli/onboard/backup_test.go`:
   - `TestBackup_CreatesBackupWithTimestamp` — archivo modificado → .bak.<RFC3339> creado
   - `TestBackup_SkipsIfFileDoesNotExist` — no falla si archivo no existe
   - `TestBackup_OverwritesPreviousBackup` — segunda corrida sobrescribe .bak (con nuevo timestamp)
   - `TestRestore_ValidatesCredentialsKey` — restore de credentials.json hace ping a /auth/first-run

3. `internal/cli/onboard/wizard_test.go` (extender):
   - `TestRun_DetectsAlreadyInstalled` — credentials.json existe + DB tiene users → skip bootstrap
   - `TestRun_Idempotency_NextRunSkipsEverything` — segunda corrida solo verifica
   - `TestRun_LocalMode_StartsDockerCompose` — llama a `docker compose up -d`
   - `TestRun_CloudMode_WritesEnvFile` — escribe .env con DOMAIN_DATABASE_URL
   - `TestRun_HybridMode_PartialDocker` — solo arranca servicios local

4. `cmd/domain/main_test.go` (extender):
   - `TestRunUpdate_BackupThenMigrateThenSeed` — secuencia completa con mocks

5. `internal/service/workflowimport/service_test.go` (extender):
   - `TestImport_UpdateOnly_DoesNotWriteStubs` — modo --no-stub, verifica que disco intacto

6. Sabotajes:
   - `TestSabotage_RaceCondition_TwoInstallsConcurrent` — 2 goroutines corren install simultáneo, ambas terminan OK (skip-by-hash evita duplicación)
   - `TestSabotage_DSNWithoutSSL_Rejects` — postgres://user:pass@host/db sin sslmode=disable en URL cloud → rejected

## Sabotajes / canarys

- **Race condition canary:** el test de 2 installs simultáneos verifica que el skip-by-hash + skip-by-user-count evita duplicación.
- **DSN security canary:** el test de DSN sin sslmode=disable en cloud mode verifica que el binario rechaza.
- **AGENTS.md marker canary:** el test verifica que `domain restore` puede identificar y restaurar archivos con `<!-- domain-managed -->`.
- **Backup siempre canary:** el test verifica que aunque el usuario pase `--no-backup`, hay un warning al stderr.
- **Init idempotency canary:** correr init 2 veces con archivos modificados entre corrida, verifica que la 2da detecta el cambio y actualiza.
