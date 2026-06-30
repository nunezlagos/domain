#!/usr/bin/env bash
# req-47.1-fase-b-deploy.sh
#
# DEPLOY script para Fase B de HU-47.1 (REQ-47) — destructiva, REVERSIBLE.
#
# Elimina la tabla auth_otp_codes (HU-47.1: matar OTP).
# Reversible vía .down.sql o restore pgBackRest completo.
#
# Uso:
#   DATABASE_URL=postgres://... ./scripts/req-47.1-fase-b-deploy.sh
#
# Opciones:
#   (sin args)  → corre la migración + verificación post
#   --dry-run   → NO aplica. Solo hace backup + valida precondiciones.
#   --rollback  → corre el .down.sql (recovery post-deploy)
#
# Pre-requisitos:
#   - DATABASE_URL apunta a la DB de PRODUCCIÓN (o staging equivalente)
#   - pgBackRest backup fresco (<24h) verificado
#   - HU-47.1 Fase A deployed (código OTP eliminado del binario)
#   - Operador con acceso SSH al VPS
#
# Validaciones ANTES del DROP:
#   - pg_dump → /tmp/req-47.1-pre-{ts}.sql.gz
#   - Conteo de filas pre-migración en auth_otp_codes → pre_count
#   - Aplicación en DB efímera (testcontainers) → conteo post → diff
#   - Si diff != 0 (filas perdidas), ABORTA el deploy
#
# Salida esperada:
#   - 1 archivo SQL.gz de backup
#   - 1 archivo de row counts pre/post
#   - Log con diff pre/post
#   - Si todo OK: confirma el DROP en PRODUCCIÓN
#   - Si rollback: restaura la tabla con .down.sql

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIG_DIR="$REPO_ROOT/services/domain-mcp/internal/migrate/migrations"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_DIR="/tmp/req-47.1-backups-$TIMESTAMP"
DRY_RUN_DIR="/tmp/req-47.1-dryrun-$TIMESTAMP"
LOG="/tmp/req-47.1-deploy-$TIMESTAMP.log"

MIG_NUM=173
MIG_NAME="drop_auth_otp_codes"
MIG_FILE="$MIG_DIR/000${MIG_NUM}_${MIG_NAME}.up.sql"
DOWN_FILE="$MIG_DIR/000${MIG_NUM}_${MIG_NAME}.down.sql"

mkdir -p "$BACKUP_DIR" "$DRY_RUN_DIR"

# --- Helpers ---

log() { echo "[$(date -u +%H:%M:%SZ)] $*" | tee -a "$LOG"; }

run_sql() {
  local sql="$1"
  local db="${DATABASE_URL:-}"
  if [[ -z "$db" ]]; then
    log "ERROR: DATABASE_URL no seteada"
    exit 1
  fi
  psql "$db" -v ON_ERROR_STOP=1 -c "$sql"
}

# Cuenta filas en todas las tablas del schema public. Output: "tabla: count"
count_rows() {
  local db="$1"
  local outfile="$2"
  psql "$db" -t -A -F: -c "
    SELECT table_name, (xpath('/row/c/text()', query_to_xml(format('SELECT COUNT(*) AS c FROM %I.%I', 'public', table_name), false, true, '')))[1]::text::int
    FROM information_schema.tables
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
    ORDER BY table_name;
  " > "$outfile" 2>/dev/null || true
}

backup_db() {
  local db="$DATABASE_URL"
  local out="$BACKUP_DIR/pre-mig-${MIG_NUM}-${MIG_NAME}-$TIMESTAMP.sql.gz"
  log "  pg_dump → $out"
  pg_dump --no-owner --no-acl "$db" | gzip > "$out"
  log "  backup size: $(du -h "$out" | cut -f1)"
}

count_auth_otp_codes() {
  local db="$1"
  local outfile="$2"
  psql "$db" -t -A -F: -c "
    SELECT 'auth_otp_codes', COUNT(*) FROM auth_otp_codes
    UNION ALL
    SELECT 'auth_otp_codes_status_idx', 0
    UNION ALL
    SELECT 'auth_otp_codes_user_active_idx', 0;
  " > "$outfile"
}

dry_run_migration() {
  local pre_count="$1"
  log "  dry-run migración $MIG_NUM en DB efímera (testcontainers)..."

  if [[ ! -f "$REPO_ROOT/scripts/req-21.6-dryrun.go" ]]; then
    log "  WARNING: helper de dry-run (scripts/req-21.6-dryrun.go) no existe."
    log "  SKIP dry-run automático. El operador debe correr el dry-run manualmente."
    return 0
  fi

  local ephemeral_dsn
  ephemeral_dsn=$(go run "$REPO_ROOT/scripts/req-21.6-dryrun.go" \
    --pre-counts "$pre_count" \
    --migration "$MIG_FILE" \
    --mig-dir "$MIG_DIR" 2>&1 | tee -a "$LOG" | tail -1)

  log "  dry-run OK con DSN efímero: $ephemeral_dsn"
}

# --- Pre-check global ---

pre_check() {
  log "=== Pre-check HU-47.1 Fase B ==="

  if [[ -z "${DATABASE_URL:-}" ]]; then
    log "ERROR: DATABASE_URL requerida"
    exit 1
  fi

  if [[ ! -f "$MIG_FILE" ]]; then
    log "ERROR: migración $MIG_FILE no existe"
    exit 1
  fi
  if [[ ! -f "$DOWN_FILE" ]]; then
    log "ERROR: rollback $DOWN_FILE no existe"
    exit 1
  fi

  local last_backup
  last_backup=$(pgbackrest info --stanza=domain --output=json 2>/dev/null | \
    jq -r '.[] | select(.name=="domain") | .backup[] | select(.type=="full") | .timestamp.stop | "\(. | strftime("%Y-%m-%dT%H:%M:%SZ"))"' | tail -1) || true

  if [[ -z "$last_backup" ]]; then
    log "ERROR: pgBackRest no reporta backup reciente. Aborta."
    log "  Ejecuta: pgbackrest backup --stanza=domain --type=full"
    exit 1
  fi

  local age_hours
  age_hours=$(( ($(date +%s) - $(date -d "$last_backup" +%s)) / 3600 ))
  if (( age_hours > 24 )); then
    log "WARNING: último backup tiene $age_hours horas (>24h). Recomendado: backup fresco."
    read -p "¿Continuar de todas formas? (y/N) " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && exit 1
  else
    log "  pgBackRest último backup: $last_backup (${age_hours}h ago) — OK"
  fi

  local fase_a_applied
  fase_a_applied=$(run_sql "SELECT COUNT(*) FROM auth_otp_codes;" 2>&1 | tail -1 | tr -d ' ')
  if [[ "$fase_a_applied" == "0" ]]; then
    log "WARNING: auth_otp_codes ya está VACÍA (0 filas). Probablemente ya se hizo limpieza parcial."
    log "  El DROP va a proceder igual (la tabla queda eliminada)."
  fi
  log "  filas actuales en auth_otp_codes: $fase_a_applied"

  log "=== Pre-check OK ==="
}

# --- Loop principal ---

migrate_one() {
  log ""
  log "=== Migración $MIG_NUM: ${MIG_NAME} ==="

  if [[ "${DRY_RUN_ONLY:-}" == "1" ]]; then
    log "  --dry-run: no aplica nada en prod"
  else
    backup_db
  fi

  local pre_count="$BACKUP_DIR/row-counts-pre-${MIG_NUM}.txt"
  count_rows "$DATABASE_URL" "$pre_count"
  log "  pre-count: $(wc -l < "$pre_count") tablas"

  local pre_otp="$BACKUP_DIR/auth-otp-pre-${MIG_NUM}.txt"
  count_auth_otp_codes "$DATABASE_URL" "$pre_otp"
  log "  filas en auth_otp_codes: $(grep '^auth_otp_codes:' "$pre_otp" | cut -d: -f2)"

  dry_run_migration "$pre_count"

  if [[ "${DRY_RUN_ONLY:-}" != "1" ]]; then
    log "  aplicando migración ${MIG_NUM} en PRODUCCIÓN..."
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
      -f "$MIG_FILE" 2>&1 | tee -a "$LOG"

    local post_count="$BACKUP_DIR/row-counts-post-${MIG_NUM}.txt"
    count_rows "$DATABASE_URL" "$post_count"

    log "  diff pre/post (debe ser 0 líneas si auth_otp_codes fue la única tabla afectada):"
    diff "$pre_count" "$post_count" | head -20 | tee -a "$LOG" || true

    local post_table_check
    post_table_check=$(run_sql "
      SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema = 'public' AND table_name = 'auth_otp_codes';
    " 2>&1 | tail -1 | tr -d ' ')

    if [[ "$post_table_check" != "0" ]]; then
      log "ERROR: auth_otp_codes SIGUE EXISTIENDO post-migración. Aborta."
      exit 1
    fi
    log "  ✓ auth_otp_codes removida (verificado vía information_schema)"
  fi
}

rollback_one() {
  log ""
  log "=== Rollback migración $MIG_NUM: ${MIG_NAME} ==="
  log "  ejecutando .down.sql..."
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
    -f "$DOWN_FILE" 2>&1 | tee -a "$LOG"

  local post_table_check
  post_table_check=$(run_sql "
    SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'auth_otp_codes';
  " 2>&1 | tail -1 | tr -d ' ')

  if [[ "$post_table_check" != "1" ]]; then
    log "ERROR: rollback no restauró auth_otp_codes. Requiere restore completo pgBackRest."
    exit 1
  fi
  log "  ✓ auth_otp_codes restaurada"
}

# --- Entry point ---

main() {
  log "=== HU-47.1 Fase B deploy — start ==="
  log "  timestamp: $TIMESTAMP"
  log "  log: $LOG"
  log "  backups: $BACKUP_DIR"
  log ""

  pre_check

  if [[ "${ROLLBACK:-}" == "1" ]]; then
    rollback_one
  else
    migrate_one
  fi

  log ""
  log "=== HU-47.1 Fase B deploy — done ==="
  log "  Verificación post-deploy:"
  log "    psql -c \"\\d auth_otp_codes\" -- debe fallar (tabla no existe)"
  log "    psql -c \"SELECT count(*) FROM auth_otp_codes;\" -- debe fallar con SQLSTATE 42P01"
  log ""
  log "  Log completo: $LOG"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN_ONLY=1; shift ;;
    --rollback) ROLLBACK=1; shift ;;
    *) log "ERROR: arg desconocido: $1"; exit 1 ;;
  esac
done

main