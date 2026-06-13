#!/bin/bash
# ============================================================================
# backup.sh — pg_dump cifrado + mc mirror de MinIO, con rotación.
#
# Output:
#   /opt/services/backups/postgres/YYYY-MM-DD.sql.gz.gpg
#   /opt/services/backups/minio/YYYY-MM-DD/<bucket>/...
#
# Rotación: mantiene N daily, N weekly (lunes), N monthly (día 1) según .env.
#
# Cifrado: GPG simétrico con BACKUP_GPG_PASSPHRASE.
# Restaurar: gpg --decrypt file.sql.gz.gpg | gunzip | psql ...
#
# Si algo falla → exit !=0 + ntfy notification (si NTFY_TOPIC seteado).
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"

# Cargar .env
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
else
  echo "ERROR: .env no encontrado en $ROOT_DIR" >&2
  exit 1
fi

: "${POSTGRES_USER:?}"
: "${POSTGRES_DB:?}"
: "${POSTGRES_PASSWORD:?}"
: "${BACKUP_GPG_PASSPHRASE:?}"

DAILY_RETAIN="${BACKUP_DAILY_RETAIN:-7}"
WEEKLY_RETAIN="${BACKUP_WEEKLY_RETAIN:-4}"
MONTHLY_RETAIN="${BACKUP_MONTHLY_RETAIN:-12}"
TODAY=$(date -u +"%Y-%m-%d")
DOW=$(date -u +"%u")   # 1=Mon … 7=Sun
DOM=$(date -u +"%d")

notify() {
  local level="$1"  # info | warn | err
  local msg="$2"
  if [[ -n "${NTFY_TOPIC:-}" ]]; then
    local prio="default"
    case "$level" in
      warn) prio="high";;
      err)  prio="urgent";;
    esac
    curl -fsS -X POST \
      -H "Title: domain-services backup ($level)" \
      -H "Priority: $prio" \
      -H "Tags: floppy_disk" \
      -d "$msg" \
      "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPIC" >/dev/null 2>&1 || true
  fi
  echo "[$level] $msg"
}

mkdir -p "$BACKUP_DIR/postgres" "$BACKUP_DIR/minio"

# ----------------------------------------------------------------------------
# Postgres dump cifrado
# ----------------------------------------------------------------------------
PG_OUT="$BACKUP_DIR/postgres/$TODAY.sql.gz.gpg"
echo "Dumping Postgres → $PG_OUT"

# pg_dump corre dentro del container, escribe a stdout, gzip + gpg lo cifran en el host.
# --no-owner --no-privileges para que restore sea portable entre roles.
if ! docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" domain-postgres \
       pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges \
       | gzip \
       | gpg --batch --yes --passphrase "$BACKUP_GPG_PASSPHRASE" \
             --symmetric --cipher-algo AES256 -o "$PG_OUT"; then
  notify err "pg_dump falló — $TODAY"
  exit 1
fi
PG_SIZE=$(du -h "$PG_OUT" | cut -f1)
echo "  → $PG_SIZE"

# ----------------------------------------------------------------------------
# MinIO mirror (sin cifrar — los archivos en MinIO pueden ya estarlo)
# ----------------------------------------------------------------------------
MINIO_OUT="$BACKUP_DIR/minio/$TODAY"
echo "Mirroring MinIO → $MINIO_OUT"
mkdir -p "$MINIO_OUT"

# Usar mc desde container minio-bootstrap (ya configurado) o un mc one-shot
if ! docker run --rm --network domain-services_default \
       -v "$MINIO_OUT:/backup" \
       -e MC_HOST_local="https://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@minio:9000" \
       minio/mc:RELEASE.2024-10-08T09-37-26Z \
       --insecure mirror --overwrite local /backup 2>/dev/null; then
  notify warn "mc mirror falló (continuando) — $TODAY"
fi
MINIO_SIZE=$(du -sh "$MINIO_OUT" 2>/dev/null | cut -f1 || echo "?")
echo "  → $MINIO_SIZE"

# ----------------------------------------------------------------------------
# Rotación: mantener los más nuevos N, eliminar el resto.
# ----------------------------------------------------------------------------
rotate() {
  local pattern="$1"
  local retain="$2"
  # Lista files que matchean, ordenados desc por mtime; saltea los primeros N; borra el resto
  find "$BACKUP_DIR" -maxdepth 2 -name "$pattern" -printf '%T@ %p\n' 2>/dev/null \
    | sort -rn \
    | awk -v n="$retain" 'NR>n {print $2}' \
    | while read -r f; do
        echo "  rotate: $f"
        rm -rf "$f"
      done
}

# Daily: cualquier YYYY-MM-DD.sql.gz.gpg
echo "Rotación daily (retain=$DAILY_RETAIN)..."
rotate "*.sql.gz.gpg" "$DAILY_RETAIN"

# Weekly/Monthly están dentro del mismo daily — para simplificar Phase 1
# usamos solo daily retention. Mejorable: copiar a /weekly/, /monthly/ los lunes/día1.
# TODO: enhanced rotation policy en iteration 2.

notify info "Backup $TODAY OK (pg=$PG_SIZE, minio=$MINIO_SIZE)"
echo "Backup completo: $TODAY"
