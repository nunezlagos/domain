#!/usr/bin/env bash
# REQ-66: backup diario de Postgres con retención.
#
# Diseño:
#   - Corre como cron job en la VM (no en container) para no depender de
#     que un container particular esté arriba.
#   - pg_dump custom format (-Fc) → 1 archivo por dump, comprimido,
#     restaurable con pg_restore + opcionalmente parcial.
#   - Nombre: domain-YYYYMMDD-HHMMSS.dump
#   - Retención: borra archivos > RETENTION_DAYS (default 7).
#   - Falla loud: si pg_dump returns != 0, exit != 0 (cron mail al
#     root si está configurado). NO crear archivo vacío.
#
# Uso manual:
#   ./backup_postgres.sh                     # corre 1 backup
#   RETENTION_DAYS=14 ./backup_postgres.sh   # cambiar retention
#
# Instalación cron (en VM):
#   sudo cp backup_postgres.sh /opt/services/scripts/
#   sudo install -m 0755 -d /opt/services/backups/postgres
#   # /etc/cron.d/domain-backup:
#   0 3 * * * ubuntu /opt/services/scripts/backup_postgres.sh \
#     >> /var/log/domain-backup.log 2>&1

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

STAMP=$(date -u +'%Y%m%d-%H%M%S')
OUT="$BACKUP_DIR/domain-${STAMP}.dump"
LOG_PREFIX="[backup ${STAMP}]"

echo "$LOG_PREFIX start db=$PG_DB user=$PG_USER -> $OUT"

# pg_dump custom format. -Z 6 compresión media (balance CPU/size).
# --no-owner / --no-acl para que el restore sea portable a otro cluster
# sin chocar con roles inexistentes.
if ! docker exec "$PG_CONTAINER" \
    pg_dump -U "$PG_USER" -d "$PG_DB" \
            -Fc -Z 6 --no-owner --no-acl \
    > "$OUT.partial"; then
  echo "$LOG_PREFIX pg_dump FAILED" >&2
  rm -f "$OUT.partial"
  exit 1
fi

mv "$OUT.partial" "$OUT"
SIZE=$(stat -c%s "$OUT")
echo "$LOG_PREFIX done size=${SIZE}B"

# Retención: borrar dumps más viejos que RETENTION_DAYS.
DELETED=$(find "$BACKUP_DIR" -maxdepth 1 -name 'domain-*.dump' \
                  -mtime "+$RETENTION_DAYS" -print -delete | wc -l)
echo "$LOG_PREFIX retention: borrados $DELETED archivos > ${RETENTION_DAYS}d"

# Listar lo que queda (ayuda a auditar).
echo "$LOG_PREFIX vigentes:"
ls -lh "$BACKUP_DIR"/domain-*.dump 2>/dev/null | tail -5 | sed "s/^/$LOG_PREFIX   /"
