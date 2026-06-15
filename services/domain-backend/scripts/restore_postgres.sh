#!/usr/bin/env bash
# REQ-66: restore desde un dump generado por backup_postgres.sh.
#
# Diseño:
#   - DESTRUCTIVO: drop/recreate de la DB. Por eso pide confirmación
#     explícita (variable I_KNOW_THIS_IS_DESTRUCTIVE=yes) salvo --target
#     que apunta a otra DB de staging.
#   - Si --target=domain_restore_check (default), restaura a una DB
#     SEPARADA y deja la real intacta. Sirve para probar que el dump
#     es bueno sin riesgo.
#
# Uso:
#   ./restore_postgres.sh /path/to/domain-XXXX.dump
#     → restaura en domain_restore_check (smoke test, no toca prod)
#
#   I_KNOW_THIS_IS_DESTRUCTIVE=yes \
#     ./restore_postgres.sh --target=domain /path/to/dump
#     → restaura en la DB real (drop + recreate)

set -euo pipefail

TARGET="domain_restore_check"
DUMP=""
ENV_FILE="${ENV_FILE:-/opt/services/.env}"
PG_CONTAINER="${PG_CONTAINER:-domain-postgres}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target=*) TARGET="${1#--target=}"; shift ;;
    --target)   TARGET="$2"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | head -25
      exit 0
      ;;
    *) DUMP="$1"; shift ;;
  esac
done

if [[ -z "$DUMP" ]]; then
  echo "Uso: $0 [--target=<dbname>] /path/to/file.dump" >&2
  exit 2
fi
if [[ ! -r "$DUMP" ]]; then
  echo "ERROR: dump no legible: $DUMP" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"
PG_USER="${POSTGRES_USER:-domain}"
SOURCE_DB="${POSTGRES_DB:-domain}"

echo "[restore] dump=$DUMP target=$TARGET"

# Confirmación si el target ES la DB real.
if [[ "$TARGET" == "$SOURCE_DB" ]]; then
  if [[ "${I_KNOW_THIS_IS_DESTRUCTIVE:-no}" != "yes" ]]; then
    echo "ERROR: target=$TARGET es la DB REAL." >&2
    echo "Para confirmar drop+recreate: I_KNOW_THIS_IS_DESTRUCTIVE=yes $0 ..." >&2
    exit 3
  fi
  echo "[restore] CONFIRMADO: voy a tirar+recrear $TARGET"
fi

# Drop + create (terminar conexiones primero — clave si la app está up).
docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d postgres -v ON_ERROR_STOP=1 -c "
  SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
  WHERE datname = '$TARGET' AND pid <> pg_backend_pid();
" >/dev/null

docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS \"$TARGET\";" \
  -c "CREATE DATABASE \"$TARGET\" OWNER \"$PG_USER\";"

# pg_restore en custom format. Sin --no-acl/--no-owner acá porque el
# dump ya viene "limpio" desde backup_postgres.sh.
echo "[restore] aplicando dump (esto puede tardar)..."
docker exec -i "$PG_CONTAINER" \
  pg_restore -U "$PG_USER" -d "$TARGET" --no-owner --no-acl < "$DUMP"

# Smoke: contar algunas tablas core para confirmar que cargó algo.
echo "[restore] smoke check tablas core:"
docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$TARGET" -c "
  SELECT 'organizations' AS t, COUNT(*) FROM organizations
  UNION ALL SELECT 'users', COUNT(*) FROM users
  UNION ALL SELECT 'project_tickets', COUNT(*) FROM project_tickets
  UNION ALL SELECT 'captured_prompts', COUNT(*) FROM captured_prompts;
"

echo "[restore] OK"
