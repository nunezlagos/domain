# Proposal: issue-01.10-deploy-modes-update

## Intención

Hacer la instalación y updates de Domain seguros, reproducibles, y no destructivos. Cubre los 3 puntos del feedback del usuario:

1. **Selector de deployment mode** (local / cloud / hybrid) con detección automática de entorno
2. **Instalación no destructiva** que solo AGREGA (init archiva .md, no elimina; AGENTS.md se inyecta con marker; flows/credenciales/custom .md se preservan)
3. **Updates con backups automáticos** y seeders idempotentes (skip-by-hash) que corren en install Y update sin borrar nada

## Scope

**Incluye:**
- Comando `domain install` (nuevo, reemplaza a `onboard` con superpoderes)
- Selector de deployment mode (local / cloud / hybrid)
- Backups automáticos antes de cualquier mutación
- Init idempotente integrado en el flow de install
- AGENTS.md injection con marker `<!-- domain-managed -->` para prioridad MCP
- Comando `domain update` (separado de install)
- Comando `domain restore <backup-path>` para recovery puntual
- Seeders idempotentes (skip-by-hash) ya existen, falta exponer `domain seed all`
- 3 modos de deployment: local (docker compose), cloud (DSN), hybrid (per-service)

**No incluye:**
- TUI con charmbracelet (futuro, si el flow crece)
- Self-update de binarios (descargar nueva version via curl — fuera de scope)
- Rollback completo a versión anterior (requiere DB snapshot)
- Cloud-managed domain (SaaS) — distinto de cloud-self-hosted

## Enfoque técnico

1. **Comando `install`** orquesta todos los pasos (deployment, backups, migrate, seed, bootstrap, init, agent config). Es idempotente: corre N veces sin romper nada.
2. **Backups siempre automáticos** (con `--no-backup` para override explícito). Costo: ~1ms por archivo. Beneficio: recovery instantáneo.
3. **AGENTS.md injection con marker** para prioridad MCP. El .md stub sigue existiendo (no se elimina) pero el agente lee AGENTS.md primero.
4. **Seeders idempotentes** ya están; falta exponer el comando que los corre todos.
5. **DSN validation** rechaza `sslmode=disable` en URLs de cloud providers conocidos.
6. **`onboard` queda como alias deprecated** de `install` (backward compat).

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Race: dos installs simultáneos | Skip-by-hash + advisory lock durante seed |
| Backup falla por permisos | Comando aborta con mensaje claro ANTES de mutar |
| AGENTS.md injection invasivo | Marker `<!-- domain-managed -->`; restore lo identifica |
| DSN sin sslmode=disable en cloud | Validación rechaza; user debe usar sslmode=require |
| docker no disponible (local mode) | Error claro: "install Docker or use --mode=cloud" |
| Update rompe flujo del user | Backups automáticos + restore command |
| Custom .md del user se tocan | Init SOLO detecta patterns conocidos (CLAUDE.md, etc) |
| Seeders duplican data | Skip-by-hash (ya implementado en HU-01.7) |

## Testing

- **Unit (deployment):** local/cloud/hybrid mode selectors, DSN validation, docker detection
- **Unit (backup):** create, restore, skip-if-missing, timestamp format
- **Unit (idempotency):** correr install 2 veces, segunda skip
- **Unit (AGENTS.md):** marker injection, restore identifica marker
- **Integration (E2E con docker):** levantar stack + install + verificar

## Rollback plan

- `domain restore <path>`: restaura un archivo de un backup específico
- Backups son timestamped (RFC3339), no se sobrescriben entre updates
- `domain rollback`: comando futuro que restaura TODOS los backups (out of scope)

## Out of scope (futuro)

- TUI con charmbracelet (si el flow crece > 10 inputs)
- Self-update de binarios (descargar via curl)
- Rollback completo (restore todo de un backup)
- Cloud-managed domain (SaaS) — distinto a cloud-self-hosted
- SSO/SAML para enterprise
- Backup encryption (los .bak son plaintext, sensibles si tienen API keys)
