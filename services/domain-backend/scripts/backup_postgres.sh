#!/usr/bin/env bash
# REQ-66 + REQ-77: backup diario de Postgres con retención y encriptación.
#
# Diseño:
#   - Corre como cron job en la VM (no en container) para no depender de
#     que un container particular esté arriba.
#   - pg_dump custom format (-Fc) → 1 archivo por dump, comprimido,
#     restaurable con pg_restore + opcionalmente parcial.
#   - REQ-77: si BACKUP_GPG_PASSPHRASE está seteada, el dump se encripta
#     con gpg --symmetric (AES256). Output .dump.gpg. Sin la passphrase
#     el archivo no se puede restaurar. Si la var está vacía, el dump
#     queda en claro (.dump) — útil para dev local sin secretos.
#   - Nombre: domain-YYYYMMDD-HHMMSS.dump[.gpg]
#   - Retención: borra archivos > RETENTION_DAYS (default 7).
#   - Falla loud: si pg_dump o gpg returns != 0, exit != 0. NO crear
#     archivos vacíos.
#
# Uso manual:
#   ./backup_postgres.sh                     # corre 1 backup
#   RETENTION_DAYS=14 ./backup_postgres.sh   # cambiar retention
#
# Variables relevantes en /opt/services/.env:
#   POSTGRES_USER, POSTGRES_DB
#   BACKUP_GPG_PASSPHRASE   (si vacío, dump sin encriptar)
#   BACKUP_DAILY_RETAIN     (default 7)

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/opt/services/backups/postgres}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
ENV_FILE="${ENV_FILE:-/opt/services/.env}"
PG_CONTAINER="${PG_CONTAINER:-domain-postgres}"

if [[ ! -d "$BACKUP_DIR" ]]; then
  echo "ERROR: $BACKUP_DIR no existe — creálo: sudo install -d -o ubuntu $BACKUP_DIR" >&2
  exit 1
fi
if [[ ! -r "$ENV_FILE" ]]; then
  echo "ERROR: $ENV_FILE no legible" >&2
  exit 1
fi

# Cargar POSTGRES_USER/POSTGRES_DB del .env (no necesitamos password
# porque pg_dump corre DENTRO del container postgres con peer auth).
# shellcheck disable=SC1090
source "$ENV_FILE"
PG_USER="${POSTGRES_USER:-domain}"
PG_DB="${POSTGRES_DB:-domain}"
GPG_PASSPHRASE="${BACKUP_GPG_PASSPHRASE:-}"

STAMP=$(date -u +'%Y%m%d-%H%M%S')
EXT="dump"
[[ -n "$GPG_PASSPHRASE" ]] && EXT="dump.gpg"
OUT="$BACKUP_DIR/domain-${STAMP}.${EXT}"
LOG_PREFIX="[backup ${STAMP}]"

echo "$LOG_PREFIX start db=$PG_DB user=$PG_USER encrypted=$([[ -n "$GPG_PASSPHRASE" ]] && echo yes || echo no) -> $OUT"

# pg_dump → [gpg] → archivo. Si hay passphrase, encadenar con pipe a gpg
# --symmetric AES256. Sin passphrase, escribir directo.
# Usamos pipefail (set -e arriba) para que un error en cualquier eslabón
# aborte y no deje un archivo parcial.
if [[ -n "$GPG_PASSPHRASE" ]]; then
  command -v gpg >/dev/null || { echo "$LOG_PREFIX gpg no instalado — apt install gpg" >&2; exit 1; }
  if ! docker exec "$PG_CONTAINER" \
        pg_dump -U "$PG_USER" -d "$PG_DB" -Fc -Z 6 --no-owner --no-acl \
      | gpg --batch --yes --pinentry-mode loopback \
            --passphrase "$GPG_PASSPHRASE" \
            --symmetric --cipher-algo AES256 \
            -o "$OUT.partial"; then
    echo "$LOG_PREFIX pg_dump | gpg FAILED" >&2
    rm -f "$OUT.partial"
    exit 1
  fi
else
  if ! docker exec "$PG_CONTAINER" \
      pg_dump -U "$PG_USER" -d "$PG_DB" -Fc -Z 6 --no-owner --no-acl \
      > "$OUT.partial"; then
    echo "$LOG_PREFIX pg_dump FAILED" >&2
    rm -f "$OUT.partial"
    exit 1
  fi
fi

mv "$OUT.partial" "$OUT"
SIZE=$(stat -c%s "$OUT")
echo "$LOG_PREFIX done size=${SIZE}B"

# Retención: borrar dumps más viejos que RETENTION_DAYS (cualquier extensión).
DELETED=$(find "$BACKUP_DIR" -maxdepth 1 \
                  \( -name 'domain-*.dump' -o -name 'domain-*.dump.gpg' \) \
                  -mtime "+$RETENTION_DAYS" -print -delete | wc -l)
echo "$LOG_PREFIX retention: borrados $DELETED archivos > ${RETENTION_DAYS}d"

# Listar lo que queda (ayuda a auditar).
echo "$LOG_PREFIX vigentes:"
ls -lh "$BACKUP_DIR"/domain-*.dump* 2>/dev/null | tail -5 | sed "s/^/$LOG_PREFIX   /"
