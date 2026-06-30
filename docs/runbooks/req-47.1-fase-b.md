# HU-47.1 Fase B — DROP auth_otp_codes (runbook)

## Contexto

HU-47.1 "Simple Login" elimina el flujo OTP. Tiene 2 fases:

- **Fase A (✅ implementada)**: eliminar el código OTP (service, endpoints, mailer, CLI).
  Build/vet/tests verde. Deployable.
- **Fase B (este runbook)**: eliminar la tabla `auth_otp_codes` de la DB. **Destructiva, REVERSIBLE**.

## Por qué Fase B es separada

- **Fase A es deployable sola**: el sistema funciona sin OTP (REQs 36+37 son el reemplazo).
  La tabla `auth_otp_codes` queda huérfana pero inofensiva.
- **Fase B es destructiva**: una vez ejecutado, no se puede deshacer sin `.down.sql` o restore pgBackRest.

## Pre-requisitos

1. **Fase A deployada y validada en producción** (binario sin código OTP corriendo).
2. **pgBackRest backup fresco** (<24h) verificado:
   ```bash
   pgbackrest info --stanza=domain
   ```
3. **Ventana de mantenimiento** acordada con stakeholders (operación toma ~1-5 min).
4. **Acceso SSH al VPS** y al repo local con `DATABASE_URL` apuntando a prod.

## Procedimiento

### Paso 1: Validar pre-condiciones (dry-run)

```bash
cd /path/to/repo
DATABASE_URL=postgres://... ./scripts/req-47.1-fase-b-deploy.sh --dry-run
```

**Qué hace**:
- Verifica que pgBackRest tiene backup reciente
- Cuenta filas en `auth_otp_codes` (pre-count)
- Levanta DB efímera (testcontainers) y aplica el `.up.sql`
- Cuenta filas post-aplicación en la DB efímera
- Diff pre/post → si hay filas perdidas en `auth_otp_codes`, ABORTA

**Output esperado**:
```
=== Pre-check HU-47.1 Fase B ===
  pgBackRest último backup: 2026-06-26T10:00:00Z (5h ago) — OK
  filas actuales en auth_otp_codes: 0
=== Pre-check OK ===
=== Migración 173: drop_auth_otp_codes ===
  --dry-run: no aplica nada en prod
  ...
✓ auth_otp_codes removida (verificado vía information_schema)
```

### Paso 2: Aplicar en producción

**Misma ventana de mantenimiento.** Confirma con stakeholders antes.

```bash
DATABASE_URL=postgres://prod-dsn ./scripts/req-47.1-fase-b-deploy.sh
```

El script:
1. Hace backup pre-migración → `/tmp/req-47.1-backups-{ts}/pre-mig-173-drop_auth_otp_codes-{ts}.sql.gz`
2. Cuenta filas pre-migración
3. Corre dry-run en testcontainers
4. **Si todo OK**, aplica `.up.sql` en PRODUCCIÓN
5. Verifica post-aplicación (`auth_otp_codes` no debe existir en `information_schema`)
6. Log completo en `/tmp/req-47.1-deploy-{ts}.log`

### Paso 3: Validar post-deploy

```bash
psql "$DATABASE_URL" -c "\d auth_otp_codes"          # debe fallar con "relation does not exist"
psql "$DATABASE_URL" -c "SELECT count(*) FROM auth_otp_codes;"  # SQLSTATE 42P01
```

Si ambos fallan con "relation does not exist" → OK.

Adicional (sanity):
```bash
curl http://localhost:8080/healthz                       # debe seguir 200
curl -X POST http://localhost:8080/api/v1/auth/request-otp  # debe dar 404 (endpoint eliminado)
curl http://localhost:8080/api/v1/auth/first-run        # debe seguir 200
```

## Rollback

Si algo sale mal DESPUÉS de aplicar (ej: el sistema no funciona):

### Opción A: Rollback quirúrgico (recomendado, <1 min)

```bash
DATABASE_URL=postgres://prod-dsn ./scripts/req-47.1-fase-b-deploy.sh --rollback
```

Ejecuta `.down.sql` que recrea la tabla con la estructura original + RLS + GRANTs.

### Opción B: Restore completo pgBackRest (último recurso, ~10-30 min)

```bash
pgbackrest restore --stanza=domain --type=time --target="2026-06-26T14:00:00+00:00"
```

Restaurar a un punto específico pre-deploy. Requiere downtime más largo.

## Riesgos conocidos

| Riesgo | Mitigación |
|---|---|
| Backup pgBackRest corrupto | Verificar ANTES con `pgbackrest verify` |
| DB efímera (testcontainers) no levanta | Script avisa y permite continuar (pero skip dry-run) |
| Aplicación tiene queries hardcoded a `auth_otp_codes` | Fase A los eliminó — grep pre-deploy |
| Restore pgBackRest incompleto | Tener backup fresco validado |

## Archivos relacionados

- Migración up: `services/domain-mcp/internal/migrate/migrations/000173_drop_auth_otp_codes.up.sql`
- Migración down: `services/domain-mcp/internal/migrate/migrations/000173_drop_auth_otp_codes.down.sql`
- Script deploy: `scripts/req-47.1-fase-b-deploy.sh`
- HU state: `openspec/changes/REQ-47-simple-login/HU-47.1-simple-login/state.yaml`

## Contacto

Operador del VPS (sysadmin en vps-domain). Verificar acceso ANTES del deploy.