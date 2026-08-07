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

cd "$REPO_ROOT"

log() { echo "auto-deploy: $*"; }

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

  log "$tag es nuevo: desplegando ${sha_actual:0:8} -> ${sha_tag:0:8}"
  PREV_SHA="$sha_actual" "$DEPLOY_SH" "$@"
}

tomar_lock_o_retirarse
decidir_y_desplegar "$@"
