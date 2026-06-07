# Proposal: HU-03.3-admin-commands

## Intención

Proveer comandos administrativos para mantener la salud del sistema: diagnosticar problemas (doctor), gestionar observaciones duplicadas o similares (conflicts), configurar y operar sincronización cloud (cloud), y coordinar transferencia de datos entre instancias (sync). Algunos comandos son proxies a funcionalidad definida en otras REQs; implementamos el CLI handler y delegamos a store/cloud/sync layers que pueden ser stubs hasta que esas HUs estén completas.

## Scope

**Incluye:**

- Comando `doctor` con flags `--project`, `--check`, `--json` — ejecuta checks de diagnóstico sobre DB, migraciones, FTS5, disco
- Comando `conflicts list|show|stats|scan|deferred` — CRUD sobre tabla de conflictos
- Comando `cloud config|status|enroll|serve|upgrade` — gestión de configuración y servidor cloud
- Comando `sync` con flags `--import`, `--status`, `--cloud`, `--project`, `--all` — sincronización
- CLI handlers completos con validación, flags, y output consistente
- Stub calls a store/cloud/sync layers que se implementan en otras REQs

**No incluye:**

- Algoritmo de detección de conflictos (REQ-10)
- Servidor HTTP cloud completo (REQ-09, REQ-05)
- Engine de sync bidireccional (REQ-07, REQ-09)
- Lógica de diagnóstico profundo (REQ-12)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Stub pattern | Los CLI handlers llaman a funciones de paquete `internal/store/diagnostics.go`, `internal/cloud/`, `internal/sync/` que pueden ser implementaciones parciales o retornar "not implemented yet" |
| Doctor checks | Array de `Check{Name, Run func() (Status, Detail)}` ejecutados secuencialmente |
| Conflicts model | Reutilizar `store.Candidate` de HU-01.2; `conflicts list` consulta `observations` con normalized_hash repetidos |
| Cloud config | Archivo JSON en `~/.memoria/cloud.json`; token en `~/.memoria/cloud.token` con permiso 0600 |
| Sync flow | Llamadas orquestadas a cloud pull + local merge + cloud push |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Comandos admin son "no implementados" por mucho tiempo | Alta | Stub claro con mensaje "not available until REQ-X is implemented" |
| Cloud token se loggea en output | Media | Sanitizar output de cloud config (omitir token) |
| Sync conflict resolution no trivial | Alta | Sync inicial es pull/push simple; resolución de conflictos en REQ-10 |

## Testing

- **Doctor:** test con DB saludable, DB corrupta, check específico, --json output
- **Conflicts:** test con y sin conflictos, show detalle, stats, scan, deferred
- **Cloud:** test config save/load, enroll, serve (puerto ocupado), status sin config
- **Sync:** test status, --cloud (stub), --import, --all, error sin cloud config
