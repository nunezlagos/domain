#!/usr/bin/env bash
# scripts/auto-deploy-check.sh
#
# REQ-5 de issue-57.1 — decide si hay que desplegar, y solo eso. El deploy en sí lo hace
# scripts/deploy.sh, que ya tiene sus 5 fases, su trap de rollback y su --dry-run.
#
# La regla que implementa: SOLO despliega el último tag v*, y solo si ese tag es la punta de
# origin/main. Un push a main sin tag no despliega — si no se chequeara, deploy.sh haría
# pull_ff de origin/main y pondría en producción código que nadie publicó.
#
# Uso:
#   ./scripts/auto-deploy-check.sh              # ciclo normal (lo llama el timer)
#   ./scripts/auto-deploy-check.sh --dry-run    # decide igual, y el deploy no toca nada
#
# Pre-req: los mismos que deploy.sh, más `flock` (util-linux, ya en el VPS).
#
# En el VPS lo instala y lo enciende install.sh (STEP 7): un redeploy deja esto andando.
# Para mirarlo o apagarlo:
#   systemctl list-timers domain-auto-deploy.timer
#   journalctl -u domain-auto-deploy.service -f
#   sudo systemctl disable --now domain-auto-deploy.timer

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${DEPLOY_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
DEPLOY_SH="$REPO_ROOT/scripts/deploy.sh"
LOCK_FILE="$REPO_ROOT/.git/auto-deploy.lock"

# DOMAINSERV-268: para decidir si el rango del tag pide SVC=all ANTES de invocar deploy.sh.
# log_phase (orchestrator.sh) escribe en ./.deploy.log, que es justo el archivo del que
# auto-deploy-alert.sh saca el motivo que publica en ntfy; el log() local solo va a stdout,
# así que la alerta publicaría las líneas del deploy anterior.
# shellcheck source=lib/detect_changed.sh
source "$SCRIPT_DIR/lib/detect_changed.sh"
# shellcheck source=lib/orchestrator.sh
source "$SCRIPT_DIR/lib/orchestrator.sh"

# DOMAINSERV-262: la normalización de propiedad vive solo en install.sh, o sea que es un
# EVENTO y no un invariante. Este unit no declara User=, así que corre como root y cada ref
# que trae el fetch nace de root: el estado que install.sh dejó prolijo se degrada solo,
# entre corridas del instalador, sin que nadie lo note hasta que sysadmin no puede operar su
# propio repo. Renormalizar acá lo convierte en invariante.
NORMALIZAR_LIB="$REPO_ROOT/services/scripts/normalizar-duenos.sh"
DEPLOY_EJECUTADO="no"

cd "$REPO_ROOT"

log() { echo "auto-deploy: $*"; }

# El alcance es escalonado a propósito, y la razón es el costo: la lib hace un fork de chown
# POR RUTA. Medido en este checkout: 7462 rutas el árbol entero contra 409 solo .git, y con
# el timer cada 10 min eso son ~144 recorridos por día. Un ciclo NOOP solo hace fetch, que
# escribe únicamente bajo .git; el árbol completo lo reescribe pull_ff, que solo corre si
# hubo deploy. Normalizar de más acá no es gratis: alarga la ventana del lock.
normalizar_al_terminar() {
  local rc=$?
  [[ -f "$NORMALIZAR_LIB" ]] || exit "$rc"

  local dueno destino
  dueno="${INSTALL_OWNER:-$(stat -c '%U' "$REPO_ROOT" 2>/dev/null || echo sysadmin)}"
  # si el root del repo YA es de root, renormalizar cementaría la mezcla en vez de deshacerla
  [[ "$dueno" == "root" ]] && exit "$rc"

  if [[ "$DEPLOY_EJECUTADO" == "si" ]]; then
    destino="$REPO_ROOT"
  else
    destino="$REPO_ROOT/.git"
  fi
  [[ -d "$destino" ]] || exit "$rc"

  # shellcheck source=/dev/null
  source "$NORMALIZAR_LIB"
  # if explícito y no `(( ... )) && log`: bajo set -e un AND-list que evalúa a 0 devuelve 1
  # y el trap se llevaría puesto el rc real del deploy, que la suite verifica (caso g)
  if normalizar_duenos "$destino" "$dueno"; then
    if (( DUENOS_CAMBIADOS > 0 )); then
      log "propiedad renormalizada a $dueno en $destino ($DUENOS_CAMBIADOS rutas volvían a ser de root)"
    fi
  else
    log "no se pudo renormalizar la propiedad de $destino"
  fi
  # bash ya preserva el rc original al salir de un trap EXIT sin exit explícito (verificado),
  # así que este exit es redundante — se deja porque los returns tempranos de arriba SÍ
  # necesitan exit para cortar el trap, y alternarlos confundiría. Lo que no puede aparecer
  # nunca acá es un `exit 0`: enmascararía el fallo del deploy y la alerta no se dispararía.
  exit "$rc"
}

# El lock vive en .git/ y no en /tmp a propósito: lo que no puede solaparse son dos deploys
# del MISMO checkout, y dos checkouts distintos no tienen por qué esperarse entre sí.
tomar_lock_o_retirarse() {
  exec 9>"$LOCK_FILE"
  if ! flock -n 9; then
    log "hay un ciclo en curso sobre este checkout, me retiro"
    exit 0
  fi
}

ultimo_tag_publicado() {
  # sort -V y no sort: alfabéticamente v0.10.0 queda ANTES que v0.9.0, o sea que el
  # auto-deploy se apagaría solo en cuanto el minor llegue a dos dígitos
  git tag -l 'v*' | sort -V | tail -n1
}

decidir_y_desplegar() {
  git fetch --tags --quiet origin

  local tag
  tag="$(ultimo_tag_publicado)"
  if [[ -z "$tag" ]]; then
    log "no hay ningún tag v* publicado, nada que desplegar"
    return 0
  fi

  local sha_tag sha_actual sha_main
  sha_tag="$(git rev-list -n1 "$tag")"
  sha_actual="$(git rev-parse HEAD)"
  if [[ "$sha_tag" == "$sha_actual" ]]; then
    log "$tag ya está desplegado, nada que hacer"
    return 0
  fi

  sha_main="$(git rev-parse origin/main)"
  if [[ "$sha_tag" != "$sha_main" ]]; then
    log "el último tag no es la punta de origin/main: desplegarlo publicaría código sin tag"
    log "  tag=${sha_tag:0:8}  origin/main=${sha_main:0:8}  (esperando el tag de ese commit)"
    return 0
  fi

  # DOMAINSERV-268: `make restart SVC=all` es down+build+up de TODO el stack, no un
  # restart. Un tag que toque services/Makefile, cualquier docker-compose o un directorio
  # nuevo bajo services/ cae en esa regla, y hasta acá eso se ejecutaba solo, cada 10
  # minutos, sin nadie mirando. El gate vive ACÁ y no en deploy.sh porque allá fetch_phase
  # ya hizo pull_ff: HEAD quedaría en el tag, el ciclo siguiente lo vería "ya desplegado" y
  # el código quedaría sin desplegar EN SILENCIO. Además, con CHANGED_SVC=all ya asignado,
  # el trap de rollback dispararía el mismo `make restart SVC=all` que se quiere evitar.
  local svc
  svc="$(detect_changed_services "$sha_actual" "$sha_tag")"
  if [[ "$svc" == "all" ]]; then
    log_phase "auto-deploy: $tag toca services/Makefile, un docker-compose o un directorio nuevo bajo services/, o sea SVC=all, que baja el stack entero. NO se despliega sin supervisión: corrélo a mano con 'cd /opt/services && sudo ./scripts/deploy.sh'"
    return 1
  fi

  log "$tag es nuevo: desplegando ${sha_actual:0:8} -> ${sha_tag:0:8}"
  # se marca ANTES de invocar: si el deploy falla a mitad, pull_ff ya reescribió el worktree
  # como root y es justamente ahí donde hay que renormalizar el árbol entero
  DEPLOY_EJECUTADO="si"
  PREV_SHA="$sha_actual" "$DEPLOY_SH" "$@"
}

tomar_lock_o_retirarse
# el trap se arma DESPUÉS del lock a propósito: la rama que se retira por un ciclo en curso
# no puede ponerse a hacer chown sobre un deploy ajeno que está a mitad de camino
trap normalizar_al_terminar EXIT
decidir_y_desplegar "$@"
