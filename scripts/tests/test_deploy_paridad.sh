#!/usr/bin/env bash
# scripts/tests/test_deploy_paridad.sh
#
# Los dos caminos de deploy no hacían lo mismo, y la diferencia costó ~2 h de producción
# caída el 2026-08-09:
#
#   services/install.sh  — ... + pg_dump + pre-flight del borde HTTP + build + up + curl /healthz
#   scripts/deploy.sh    — validate + fetch + detect + build + restart + wait-healthy
#
# Las tres omisiones son de la misma clase: install.sh protege producción de algo y
# deploy.sh no. Ninguna dolía mientras el auto-deploy estuvo roto, porque main() nunca
# pasaba de fetch. Al arreglarlo, las tres se volvieron alcanzables el mismo día.
#
# 1. BACKUP — el init-container domain-migrate aborta si la BD tiene datos y no hubo
#    pg_dump (DOMAIN_BACKUP_DONE!=1). Su mensaje dice "Redeploy vía services/install.sh":
#    fue escrito para ESE camino. El auto-deploy usa deploy.sh, así que nunca podía
#    satisfacerlo. Medido: migrate Exited(1) y mcp/admin/caddy abajo por depends_on.
# 2. PRE-FLIGHT — install.sh valida el Caddyfile contra la imagen con plugins ANTES de
#    bajar nada, y su comentario explica por qué: "validar después del down significaría
#    diagnosticar con el sitio ya caído". Caddy es el único borde HTTP del host.
# 3. SELF-CHECK — `make wait-healthy` mira healthchecks de docker. Un container puede
#    estar healthy y el sitio no responder; hoy pasó exactamente eso.
#
# MÉTODO: repo fake + stubs de make/docker/curl/backup.sh en el PATH, y se afirma sobre
# el ORDEN registrado en un log de invocaciones. El orden es el punto: un backup después
# del restart no sirve de nada, y un pre-flight después del down tampoco.
#
# Uso: bash scripts/tests/test_deploy_paridad.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEPLOY="$REPO_ROOT/scripts/deploy.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failed=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; failed=$((failed + 1)); }

# Caja con repo git real (deploy.sh hace git diff), stubs que registran su invocación en
# orden, y el layout que el script espera. $1 = nombre, $2 = rc del stub de backup.sh.
preparar() {
  local nombre="$1" backup_rc="${2:-0}"
  local caja="$WORK/$nombre"
  mkdir -p "$caja/bin" "$caja/repo/services/caddy" "$caja/repo/services/scripts" "$caja/repo/scripts/lib"

  cp "$REPO_ROOT/scripts/lib/"*.sh "$caja/repo/scripts/lib/"
  cp "$REPO_ROOT/services/scripts/pg-backup-guard.sh" "$caja/repo/services/scripts/" 2>/dev/null || true

  (
    cd "$caja/repo"
    git init -q -b main; git config user.email t@t; git config user.name t
    printf 'DOMAIN_FIELD_ENC_KEY=test\n' > services/.env
    chmod 600 services/.env
    printf ':80 {\n}\n' > services/caddy/Caddyfile
    echo base > README.md
    git add -A; git commit -q -m base
    # un cambio bajo un servicio REAL para que detect no de noop
    mkdir -p services/domain-mcp && echo v2 > services/domain-mcp/main.go
    git add -A; git commit -q -m cambio
    # fetch_phase hace pull_ff contra origin: sin remoto ni upstream aborta antes de
    # llegar a nada, y el test mediria eso en vez de lo que quiere medir
    git clone -q --bare . "$caja/origin.git"
    git remote add origin "$caja/origin.git"
    git fetch -q origin
    git branch -q --set-upstream-to=origin/main main
  ) >/dev/null 2>&1

  local log="$caja/invocaciones.log"
  : > "$log"

  for cmd in make docker curl; do
    cat > "$caja/bin/$cmd" <<STUB
#!/usr/bin/env bash
printf '%s %s\n' "$cmd" "\$*" >> "$log"
[[ "$cmd" == curl ]] && echo 200
exit 0
STUB
    chmod +x "$caja/bin/$cmd"
  done

  cat > "$caja/repo/services/scripts/backup.sh" <<STUB
#!/usr/bin/env bash
printf 'backup.sh %s\n' "\$*" >> "$log"
exit $backup_rc
STUB
  chmod +x "$caja/repo/services/scripts/backup.sh"

  printf '%s' "$caja"
}

correr_deploy() {
  local caja="$1"; shift
  (
    export PATH="$caja/bin:$PATH"
    export DEPLOY_REPO_ROOT="$caja/repo"
    export DEPLOY_ENV_FILE="$caja/repo/services/.env"
    export LOG_FILE="$caja/deploy.log"
    export PREV_SHA=HEAD~1
    bash "$DEPLOY" "$@"
  ) >/dev/null 2>&1
  echo $?
}

# el orden importa mas que la presencia: devuelve el numero de linea de la primera
# invocacion que matchea, o vacio si nunca ocurrio
linea_de() { grep -n -m1 -- "$2" "$1/invocaciones.log" 2>/dev/null | cut -d: -f1; }

antes_que() {
  local label="$1" caja="$2" primero="$3" segundo="$4"
  local a b; a=$(linea_de "$caja" "$primero"); b=$(linea_de "$caja" "$segundo")
  if [[ -z "$a" ]]; then fail "$label — '$primero' nunca se invocó"; return; fi
  if [[ -z "$b" ]]; then fail "$label — '$segundo' nunca se invocó"; return; fi
  if (( a < b )); then pass "$label"; else
    fail "$label — '$primero' (línea $a) ocurrió DESPUÉS de '$segundo' (línea $b)"
  fi
}

ocurrio() {
  local label="$1" caja="$2" patron="$3"
  if [[ -n "$(linea_de "$caja" "$patron")" ]]; then pass "$label"; else
    fail "$label — '$patron' nunca se invocó. Log: $(tr '\n' '|' < "$caja/invocaciones.log")"
  fi
}

no_ocurrio() {
  local label="$1" caja="$2" patron="$3"
  if [[ -z "$(linea_de "$caja" "$patron")" ]]; then pass "$label"; else
    fail "$label — '$patron' se invocó y no debía"
  fi
}

# --- 1. BACKUP ---------------------------------------------------------------------

test_deploy_hace_backup_antes_de_levantar() {
  local T="TestDeploy_ConCambios_HaceBackupAntesDelRestart"
  local caja; caja="$(preparar backup-ok)"
  correr_deploy "$caja" >/dev/null
  ocurrio "$T — el deploy corre backup.sh" "$caja" "backup.sh"
  # un backup DESPUES del restart no protege de nada: migrate ya corrio
  antes_que "$T — el backup ocurre ANTES del restart" "$caja" "backup.sh" "make restart"
}

test_deploy_backup_falla_aborta_sin_restart() {
  local T="TestDeploy_BackupFalla_AbortaSinRestart"
  local caja; caja="$(preparar backup-falla 1)"
  local rc; rc=$(correr_deploy "$caja")
  [[ "$rc" != "0" ]] && pass "$T — el deploy sale != 0" \
    || fail "$T — el deploy salió 0 con el backup fallando"
  # la decisión que install.sh ya tomó: no se migra sin red de seguridad
  no_ocurrio "$T — NO llega al restart" "$caja" "make restart"
}

test_noop_no_paga_backup() {
  local T="TestDeploy_Noop_NoPagaBackup"
  local caja; caja="$(preparar noop)"
  # sin cambios bajo services/ el ciclo es noop y no levanta nada
  ( cd "$caja/repo" && git commit -q --allow-empty -m vacio ) >/dev/null 2>&1
  ( export PATH="$caja/bin:$PATH" DEPLOY_REPO_ROOT="$caja/repo" \
      DEPLOY_ENV_FILE="$caja/repo/services/.env" LOG_FILE="$caja/deploy.log" PREV_SHA=HEAD~1
    bash "$DEPLOY" ) >/dev/null 2>&1
  # el timer corre cada 10 min: un pg_dump de 59M por ciclo son 144 dumps por día
  no_ocurrio "$T — un ciclo noop no hace pg_dump" "$caja" "backup.sh"
}

test_dry_run_no_toca_la_base() {
  local T="TestDeploy_DryRun_NoTocaLaBase"
  local caja; caja="$(preparar dry-run)"
  correr_deploy "$caja" --dry-run >/dev/null
  no_ocurrio "$T — --dry-run no corre backup.sh" "$caja" "backup.sh"
}

# --- 2. PRE-FLIGHT DEL BORDE HTTP --------------------------------------------------

test_preflight_valida_caddy_antes_del_restart() {
  local T="TestDeploy_ValidaElCaddyfileAntesDeBajarNada"
  local caja; caja="$(preparar preflight)"
  correr_deploy "$caja" >/dev/null
  ocurrio "$T — valida el Caddyfile" "$caja" "caddy validate"
  # el punto entero: abortar con el stack anterior INTACTO y sirviendo
  antes_que "$T — la validación ocurre ANTES del restart" "$caja" "caddy validate" "make restart"
}

# --- 3. SELF-CHECK HTTP ------------------------------------------------------------

test_verify_hace_self_check_http() {
  local T="TestDeploy_Verify_HaceSelfCheckHTTP"
  local caja; caja="$(preparar selfcheck)"
  correr_deploy "$caja" >/dev/null
  # wait-healthy mira healthchecks de docker; hoy quedaron containers healthy con el
  # sitio caído porque caddy no existía. Solo un GET real lo distingue.
  ocurrio "$T — hace un GET a /healthz" "$caja" "healthz"
  antes_que "$T — el self-check va DESPUÉS del restart" "$caja" "make restart" "healthz"
}

test_deploy_hace_backup_antes_de_levantar
test_deploy_backup_falla_aborta_sin_restart
test_noop_no_paga_backup
test_dry_run_no_toca_la_base
test_preflight_valida_caddy_antes_del_restart
test_verify_hace_self_check_http

if (( failed > 0 )); then
  echo "RED — $failed asserts fallaron"
  exit 1
fi

echo "GREEN — deploy.sh protege lo mismo que install.sh"
exit 0
