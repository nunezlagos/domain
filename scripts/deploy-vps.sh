#!/usr/bin/env bash
# scripts/deploy-vps.sh — automatiza el deploy del stack domain-services al VPS.
#
# Lee credenciales de .env.vps (gitignored, chmod 600). Sube el código
# y reinicia los servicios afectados por los commits recientes:
#   - domain-mcp (rename desde domain-backend)
#   - domain-admin (Django placeholder, nuevo)
#   - caddy (sin cambio funcional, solo comentario)
#
# USO:
#   ./scripts/deploy-vps.sh                  # rsync + restart (default)
#   ./scripts/deploy-vps.sh --dry-run        # muestra qué haría sin tocar
#   ./scripts/deploy-vps.sh --mode git       # usa git pull en lugar de rsync
#   ./scripts/deploy-vps.sh --skip-restart   # sube código pero no reinicia
#   ./scripts/deploy-vps.sh --test           # solo prueba conexión SSH
#
# REQUISITOS: sshpass, rsync, ssh, bash 4+.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env.vps"

# Defaults
DRY_RUN=false
UPLOAD_MODE="rsync"
SKIP_RESTART=false
TEST_ONLY=false

usage() {
  sed -n '2,18p' "$0"
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)      DRY_RUN=true ;;
    --mode)         UPLOAD_MODE="$2"; shift ;;
    --mode=*)       UPLOAD_MODE="${1#--mode=}" ;;
    --skip-restart) SKIP_RESTART=true ;;
    --test)         TEST_ONLY=true ;;
    -h|--help)      usage 0 ;;
    *)              echo "flag desconocida: $1" >&2; usage 1 ;;
  esac
  shift
done

# --- Cargar .env.vps ---
if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: no existe $ENV_FILE" >&2
  echo "  Crear con: cp .env.vps.example .env.vps && chmod 600 .env.vps" >&2
  exit 1
fi
# shellcheck disable=SC1091
source "$ENV_FILE"

: "${VPS_HOST:?falta VPS_HOST en .env.vps}"
: "${VPS_USER:?falta VPS_USER en .env.vps}"
: "${VPS_PASSWORD:?falta VPS_PASSWORD en .env.vps}"
: "${VPS_DEPLOY_PATH:?falta VPS_DEPLOY_PATH en .env.vps}"

# --- Helpers ---
log()  { printf '\033[36m[deploy]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[deploy]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[31m[deploy]\033[0m %s\n' "$*" >&2; }

ssh_run() {
  local cmd="$1"
  sshpass -p "$VPS_PASSWORD" ssh \
    -o StrictHostKeyChecking=accept-new \
    -o UserKnownHostsFile="$HOME/.ssh/known_hosts" \
    -o LogLevel=ERROR \
    "${VPS_USER}@${VPS_HOST}" \
    "$cmd"
}

rsync_run() {
  local args=(
    -az --delete
    --exclude='.git'
    --exclude='.env'
    --exclude='.env.local'
    --exclude='.env.vps'
    --exclude='backups/'
    --exclude='*.log'
    --exclude='.DS_Store'
  )
  if [[ "$DRY_RUN" == true ]]; then
    args+=(--dry-run)
  fi
  sshpass -p "$VPS_PASSWORD" rsync "${args[@]}" \
    -e "ssh -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=$HOME/.ssh/known_hosts -o LogLevel=ERROR" \
    "$REPO_ROOT/" \
    "${VPS_USER}@${VPS_HOST}:${VPS_DEPLOY_PATH}/"
}

# --- Pre-flight ---
log "target: ${VPS_USER}@${VPS_HOST}:${VPS_DEPLOY_PATH}"
log "mode:   $UPLOAD_MODE"
[[ "$DRY_RUN" == true ]] && log "dry-run: activado (no se modifica nada)"
[[ "$SKIP_RESTART" == true ]] && log "skip-restart: activado (no se reinician servicios)"
[[ "$TEST_ONLY" == true ]] && log "test-only: solo prueba SSH"

# --- Test de conexión ---
log "test SSH…"
if ! ssh_run "echo 'SSH OK' && uname -a && docker --version && docker compose version"; then
  err "SSH falló. Verificar credenciales y que el VPS expone 22."
  exit 2
fi

if [[ "$TEST_ONLY" == true ]]; then
  log "test OK, saliendo"
  exit 0
fi

# --- Subida de código ---
case "$UPLOAD_MODE" in
  rsync)
    log "rsync de $REPO_ROOT → ${VPS_DEPLOY_PATH}/"
    rsync_run
    log "rsync OK"
    ;;
  git)
    log "git pull en VPS"
    ssh_run "cd ${VPS_DEPLOY_PATH} && git fetch origin && git reset --hard origin/services"
    log "git pull OK"
    ;;
  *)
    err "UPLOAD_MODE desconocido: $UPLOAD_MODE (rsync|git)"
    exit 1
    ;;
esac

# --- Restart servicios ---
if [[ "$SKIP_RESTART" == true ]]; then
  log "skip-restart: saltando restart"
  log "DONE (sin restart)"
  exit 0
fi

log "restart de servicios en VPS…"
log "  - make -C ${VPS_DEPLOY_PATH} down (orden inverso)"
log "  - make -C ${VPS_DEPLOY_PATH} build (imágenes nuevas: domain-mcp, domain-admin)"
log "  - make -C ${VPS_DEPLOY_PATH} up (5 servicios)"

if [[ "$DRY_RUN" == true ]]; then
  log "dry-run: saltando restart"
else
  ssh_run "cd ${VPS_DEPLOY_PATH} && sudo make down && sudo make build && sudo make up && sudo make wait-healthy"
fi

log "DONE"
log "Verificación manual:"
log "  curl http://${VPS_HOST}/healthz     # → 200 (domain-mcp)"
log "  curl http://${VPS_HOST}/            # → HTML Django placeholder"
log "  ssh ${VPS_USER}@${VPS_HOST} 'docker ps'"