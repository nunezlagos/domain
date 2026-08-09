#!/usr/bin/env bash
# normalizar-duenos_certs_test.sh
#
# Reproduce el incidente del 2026-08-09: el chown -R de normalizar_duenos cambió el dueño de
# certs/postgres/server.key, postgres se negó a arrancar ("must be owned by the database user
# or root") y entró en crash loop. Sin postgres no hubo DNS para su nombre, sin DNS falló el
# migrate, y sin migrate no arrancaron mcp, caddy ni admin. Producción caída.
#
# El caso no lo cubría la suite anterior porque su argumento era el bit de OTROS, y server.key
# está en 600: ahí ese razonamiento no aplica.

set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$AQUI/normalizar-duenos.sh"

PASS=0; FAIL=0
pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

[ -r "$LIB" ] || { fail "no existe $LIB"; exit 1; }
# shellcheck source=/dev/null
source "$LIB"
declare -F normalizar_duenos >/dev/null || { fail "la lib no define normalizar_duenos"; exit 1; }

# chown stubeado: cambiar dueños de verdad exige root. El stub registra a qué rutas se las
# habría cambiado, que es exactamente lo que este test necesita saber.
CHOWN_LOG="$WORK/chown.log"
chown() {
  local a
  for a in "$@"; do
    case "$a" in -*) continue ;; esac
    printf '%s\n' "$a" >> "$CHOWN_LOG"
  done
  return 0
}

arbol() {
  local d="$WORK/opt"
  rm -rf "$d"; : > "$CHOWN_LOG"
  mkdir -p "$d/certs/postgres" "$d/services/domain-mcp" "$d/backups"
  printf 'key\n'  > "$d/certs/postgres/server.key"; chmod 600 "$d/certs/postgres/server.key"
  printf 'crt\n'  > "$d/certs/postgres/server.crt"
  printf 'conf\n' > "$d/services/domain-mcp/config.yml"
  printf 'env\n'  > "$d/.env"; chmod 600 "$d/.env"
  printf '%s' "$d"
}

echo "--- el incidente: server.key NO se toca"
d=$(arbol)
normalizar_duenos "$d" sysadmin >/dev/null 2>&1
if grep -qF "certs/postgres/server.key" "$CHOWN_LOG"; then
  fail "el chown alcanzó server.key: postgres entraría en crash loop y tumbaría el stack"
else
  pass "server.key quedó intacto"
fi

echo "--- nada bajo certs/ se toca"
if grep -qF "/certs" "$CHOWN_LOG"; then
  fail "el chown alcanzó rutas bajo certs/: $(grep -F '/certs' "$CHOWN_LOG" | head -3 | tr '\n' ' ')"
else
  pass "certs/ entero quedó afuera del recorrido"
fi

echo "--- pero el resto SÍ se normaliza"
# sin esto la exclusión podría estar apagando el paso entero y el test seguiría verde
if grep -qF "services/domain-mcp/config.yml" "$CHOWN_LOG"; then
  pass "los archivos fuera de certs/ sí se normalizan"
else
  fail "no se normalizó nada fuera de certs/: la exclusión apagó el paso completo"
fi

if grep -qF "/.env" "$CHOWN_LOG"; then
  pass "el .env se normaliza (está fuera de certs/)"
else
  fail "el .env no se normalizó"
fi

echo "--- la guarda de ruta sigue viva"
if normalizar_duenos "" sysadmin >/dev/null 2>&1; then
  fail "INSTALL_DIR vacío no abortó"
else
  pass "INSTALL_DIR vacío sigue abortando"
fi

echo ""
if [ "$FAIL" -gt 0 ]; then
  echo "RED — $FAIL fallaron, $PASS pasaron"
  exit 1
fi
echo "GREEN — los $PASS asserts pasaron"
