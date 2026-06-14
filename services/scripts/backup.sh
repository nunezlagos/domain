#!/bin/bash
# pg_dump cifrado + mc mirror MinIO + rotación. Restaurar: gpg -d file.gpg | gunzip | psql ...
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"

[[ -f "$ROOT_DIR/.env" ]] || { echo "ERROR: .env no encontrado" >&2; exit 1; }
set -a; source "$ROOT_DIR/.env"; set +a

: "${POSTGRES_USER:?}"
: "${POSTGRES_DB:?}"
: "${POSTGRES_PASSWORD:?}"
: "${BACKUP_GPG_PASSPHRASE:?}"

DAILY_RETAIN="${BACKUP_DAILY_RETAIN:-2}"
TODAY=$(date -u +"%Y-%m-%d")

notify() {
  local level="$1" msg="$2"
  if [[ -n "${NTFY_TOPIC:-}" ]]; then
    local prio="default"
    case "$level" in warn) prio="high";; err) prio="urgent";; esac
    curl -fsS -X POST \
      -H "Title: domain-services backup ($level)" \
      -H "Priority: $prio" -H "Tags: floppy_disk" \
      -d "$msg" \
      "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPIC" >/dev/null 2>&1 || true
  fi
  echo "[$level] $msg"
}

mkdir -p "$BACKUP_DIR/postgres" "$BACKUP_DIR/minio"

PG_OUT="$BACKUP_DIR/postgres/$TODAY.sql.gz.gpg"
echo "Postgres → $PG_OUT"

if ! docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" domain-postgres \
       pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges \
       | gzip \
       | gpg --batch --yes --passphrase "$BACKUP_GPG_PASSPHRASE" \
             --symmetric --cipher-algo AES256 -o "$PG_OUT"; then
  notify err "pg_dump falló — $TODAY"
  exit 1
fi
PG_SIZE=$(du -h "$PG_OUT" | cut -f1)

MINIO_OUT="$BACKUP_DIR/minio/$TODAY"
mkdir -p "$MINIO_OUT"
echo "MinIO → $MINIO_OUT"

if ! docker run --rm --network minio_default \
       -v "$MINIO_OUT:/backup" \
       -e MC_HOST_local="https://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@minio:9000" \
       minio/mc:RELEASE.2024-10-08T09-37-26Z \
       --insecure mirror --overwrite local /backup 2>/dev/null; then
  notify warn "mc mirror falló — $TODAY"
fi
MINIO_SIZE=$(du -sh "$MINIO_OUT" 2>/dev/null | cut -f1 || echo "?")

# Retención: conservar solo los N más recientes (postgres dumps + carpetas MinIO).
find "$BACKUP_DIR/postgres" -maxdepth 1 -name "*.sql.gz.gpg" -printf '%T@ %p\n' 2>/dev/null \
  | sort -rn | awk -v n="$DAILY_RETAIN" 'NR>n {print $2}' \
  | while read -r f; do echo "rotate: $f"; rm -f "$f"; done

find "$BACKUP_DIR/minio" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' 2>/dev/null \
  | sort -rn | awk -v n="$DAILY_RETAIN" 'NR>n {print $2}' \
  | while read -r f; do echo "rotate: $f"; rm -rf "$f"; done

notify info "Backup $TODAY OK (pg=$PG_SIZE, minio=$MINIO_SIZE)"
