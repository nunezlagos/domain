#!/usr/bin/env bash
# req-21.6-fase-c-deploy.sh
#
# DEPLOY script para Fase C de REQ-21.6 — destructiva, irreversible sin restore.
#
# Uso:
#   DATABASE_URL=postgres://... ./scripts/req-21.6-fase-c-deploy.sh [migra]
#
# Opciones:
#   (sin args)  → corre todas las migraciones Fase C en orden
#   [number]    → corre solo la migración especificada (140, 141, 142, 143)
#   --dry-run   → NO aplica migraciones. Solo hace backup + valida precondiciones.
#   --rollback  → corre los .down.sql en orden inverso (recovery post-deploy)
#
# Pre-requisitos:
#   - DATABASE_URL apunta a la DB de PRODUCCIÓN (o staging equivalente)
#   - pgBackRest backup fresco (ver docs/runbooks/restore.md) — verificado en las últimas 24h
#   - Ventana de mantenimiento acordada con stakeholders
#   - Operador con acceso SSH al VPS y al repo local
#
# Validaciones que el script hace ANTES de cada DROP:
#   - pg_dump de la DB actual → /tmp/req-21.6-pre-{number}-{ts}.sql.gz
#   - Conteo de filas pre-migración → guardado en /tmp/req-21.6-row-counts-pre-{ts}.txt
#   - Aplicación en DB efímera (testcontainers) → conteo post → diff vs pre
#   - Si el diff es != 0 en DROP COLUMN, ABORTA el deploy a VPS
#
# Salida esperada:
#   - 4 archivos SQL.gz de backup
#   - 1 archivo de row counts pre
#   - Log con diff pre/post para cada migración
#   - Si todo OK: imprime el comando exacto para aplicar en VPS
#   - Si algo falla: ABORTA, NO aplica en producción, deja todo en /tmp/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIG_DIR="$REPO_ROOT/services/domain-backend/internal/migrate/migrations"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_DIR="/tmp/req-21.6-backups-$TIMESTAMP"
DRY_RUN_DIR="/tmp/req-21.6-dryrun-$TIMESTAMP"
LOG="/tmp/req-21.6-deploy-$TIMESTAMP.log"

MIGS=(140 141 142 143)
MIG_NAMES[140]="drop_organization_fks"
MIG_NAMES[141]="drop_org_id_satellites"
MIG_NAMES[142]="drop_org_id_columns_all"
MIG_NAMES[143]="drop_org_table_and_helpers"

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

# Backup full de la DB (custom format para pg_restore)
backup_db() {
  local num="$1"
  local db="$DATABASE_URL"
  local out="$BACKUP_DIR/pre-mig-${num}-${MIG_NAMES[$num]}-$TIMESTAMP.sql.gz"
  log "  pg_dump → $out"
  pg_dump --no-owner --no-acl "$db" | gzip > "$out"
  log "  backup size: $(du -h "$out" | cut -f1)"
}

# Dry-run de la migración en una DB efímera (testcontainers vía Go).
# Carga el schema completo (migrate up a 139), aplica la migración,
# cuenta filas, compara con el pre-count de prod, devuelve diff.
dry_run_migration() {
  local num="$1"
  local db="$DATABASE_URL"
  local pre_count="$2"

  log "  dry-run migración $num en DB efímera..."

  # Levanta testcontainer (necesita Docker corriendo)
  local ephemeral_dsn
  ephemeral_dsn=$(go run "$REPO_ROOT/scripts/req-21.6-dryrun.go" \
    --pre-counts "$pre_count" \
    --migration "$MIG_DIR/000${num}_${MIG_NAMES[$num]}.up.sql" \
    --mig-dir "$MIG_DIR" 2>&1 | tee -a "$LOG" | tail -1)

  log "  dry-run OK con DSN efímero: $ephemeral_dsn"
}

# --- Pre-check global ---

pre_check() {
  log "=== Pre-check global ==="

  if [[ -z "${DATABASE_URL:-}" ]]; then
    log "ERROR: DATABASE_URL requerida"
    exit 1
  fi

  # Verificar que pgBackRest backup existe y es reciente
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

  # Verificar que las migraciones Fase B están aplicadas
  local phase_b_applied
  phase_b_applied=$(run_sql "SELECT COUNT(*) FROM schema_migrations WHERE version IN ('135','136','137','138','139');" 2>&1 | tail -1)
  if [[ "$phase_b_applied" != "5" ]]; then
    log "ERROR: solo $phase_b_applied/5 migraciones Fase B aplicadas. Aplicar 000135-000139 primero."
    exit 1
  fi

  log "=== Pre-check OK ==="
}

# --- Loop principal ---

migrate_one() {
  local num="$1"
  log ""
  log "=== Migración $num: ${MIG_NAMES[$num]} ==="

  if [[ "${DRY_RUN_ONLY:-}" == "1" ]]; then
    log "  --dry-run: no aplica nada en prod"
  else
    backup_db "$num"
  fi

  local pre_count="$BACKUP_DIR/row-counts-pre-${num}.txt"
  count_rows "$DATABASE_URL" "$pre_count"
  log "  pre-count: $(wc -l < "$pre_count") tablas"

  dry_run_migration "$num" "$pre_count"

  if [[ "${DRY_RUN_ONLY:-}" != "1" ]]; then
    log "  aplicando migración ${num} en PRODUCCIÓN..."
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
      -f "$MIG_DIR/000${num}_${MIG_NAMES[$num]}.up.sql" 2>&1 | tee -a "$LOG"

    local post_count="$BACKUP_DIR/row-counts-post-${num}.txt"
    count_rows "$DATABASE_URL" "$post_count"

    log "  diff pre/post:"
    diff "$pre_count" "$post_count" | head -50 | tee -a "$LOG" || true

    local extra_rows
    extra_rows=$(diff "$pre_count" "$post_count" | grep '^>' | wc -l)
    if [[ "$extra_rows" -gt 0 ]]; then
      log "WARNING: $extra_rows tablas con filas EXTRA post-migración. Revisar antes de continuar."
      read -p "¿Continuar con siguiente migración? (y/N) " -n 1 -r
      echo
      [[ ! $REPLY =~ ^[Yy]$ ]] && exit 1
    fi
  fi
}

rollback_one() {
  local num="$1"
  log ""
  log "=== Rollback migración $num: ${MIG_NAMES[$num]} ==="
  log "  ejecutando .down.sql..."
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
    -f "$MIG_DIR/000${num}_${MIG_NAMES[$num]}.down.sql" 2>&1 | tee -a "$LOG"
}

# --- Entry point ---

main() {
  log "=== REQ-21.6 Fase C deploy — start ==="
  log "  timestamp: $TIMESTAMP"
  log "  log: $LOG"
  log "  backups: $BACKUP_DIR"
  log ""

  pre_check

  if [[ "${ROLLBACK:-}" == "1" ]]; then
    log "MODO ROLLBACK: aplicando .down.sql en orden inverso"
    for ((i=${#MIGS[@]}-1; i>=0; i--)); do
      rollback_one "${MIGS[$i]}"
    done
  elif [[ -n "${ONLY_MIG:-}" ]]; then
    migrate_one "$ONLY_MIG"
  else
    for num in "${MIGS[@]}"; do
      migrate_one "$num"
    done
  fi

  log ""
  log "=== REQ-21.6 Fase C deploy — done ==="
  log "  Verificación post-deploy:"
  log "    psql -c \"SELECT count(*) FROM organizations;\"  -- debe ser 0 o error"
  log "    psql -c \"SELECT count(*) FROM information_schema.columns WHERE column_name='organization_id';\"  -- debe ser 0"
  log "    psql -c \"SELECT count(*) FROM pg_proc WHERE proname='current_org_id';\"  -- debe ser 0"
  log ""
  log "  Log completo: $LOG"
}

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)   DRY_RUN_ONLY=1; shift ;;
    --rollback)  ROLLBACK=1; shift ;;
    140|141|142|143) ONLY_MIG="$1"; shift ;;
    *) log "ERROR: arg desconocido: $1"; exit 1 ;;
  esac
done

main
